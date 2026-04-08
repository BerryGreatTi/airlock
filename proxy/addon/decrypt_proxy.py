"""mitmproxy addon that transparently decrypts ENC[age:...] patterns in HTTP requests."""

import datetime
import json
import os
import re
import logging
import sys

from mitmproxy import http

ENC_PATTERN = re.compile(r"ENC\[age:[A-Za-z0-9+/=]+\]")

MAPPING_PATH = os.environ.get("AIRLOCK_MAPPING_PATH", "/run/airlock/mapping.json")
PASSTHROUGH_HOSTS_RAW = os.environ.get(
    "AIRLOCK_PASSTHROUGH_HOSTS", "api.anthropic.com,auth.anthropic.com"
)
ALLOWED_HOSTS_RAW = os.environ.get("AIRLOCK_ALLOWED_HOSTS", "")


def _split_csv_env(raw: str) -> list[str]:
    """Parse a comma-separated env var into a trimmed, non-empty list."""
    return [h.strip() for h in raw.split(",") if h.strip()]


class DecryptAddon:
    def __init__(
        self,
        mapping_path: str = MAPPING_PATH,
        passthrough_hosts: list[str] | None = None,
        allowed_hosts: list[str] | None = None,
    ):
        self.mapping: dict[str, str] = {}
        self.passthrough: set[str] = set()
        # Allow-list is split into two sets: exact host matches and suffix
        # patterns (stored as ".example.com" with the leading dot so
        # `endswith` naturally rejects bare `example.com` — its length is
        # one shorter than the suffix). Empty = allow all HTTP traffic
        # (back-compat default). All entries are lowercased on insert and
        # the host is lowercased on lookup: RFC 1035 §2.3.3 says hostnames
        # are case-insensitive, and the Swift GUI's `NetworkAllowlistPolicy`
        # normalizes the same way, so GUI guardrails and runtime
        # enforcement agree on case semantics.
        self._allow_exact: set[str] = set()
        self._allow_suffix: set[str] = set()
        self._mapping_path: str = mapping_path
        self._last_mtime: float = 0.0

        if passthrough_hosts is None:
            passthrough_hosts = _split_csv_env(PASSTHROUGH_HOSTS_RAW)
        self.passthrough = {h.lower() for h in passthrough_hosts}

        if allowed_hosts is None:
            allowed_hosts = _split_csv_env(ALLOWED_HOSTS_RAW)
        for host in allowed_hosts:
            host = host.lower()
            if host.startswith("*."):
                self._allow_suffix.add(host[1:])
            else:
                self._allow_exact.add(host)

        self._load_mapping(mapping_path)

    def _load_mapping(self, path: str) -> None:
        try:
            self._last_mtime = os.path.getmtime(path)
            with open(path) as f:
                self.mapping = json.load(f)
            logging.info("Loaded %d secret mappings", len(self.mapping))
        except FileNotFoundError:
            logging.warning("Mapping file not found: %s", path)
        except json.JSONDecodeError as e:
            logging.error("Invalid mapping JSON: %s", e)

    def _maybe_reload_mapping(self) -> None:
        try:
            mtime = os.path.getmtime(self._mapping_path)
            if mtime != self._last_mtime:
                self._load_mapping(self._mapping_path)
        except OSError:
            pass

    def is_passthrough(self, host: str) -> bool:
        return host.lower() in self.passthrough

    def is_allowed(self, host: str) -> bool:
        """Return True if host passes the allow-list filter.

        An empty allow-list means "no restriction" (back-compat). Otherwise:
        - exact host match wins, or
        - any suffix pattern `*.example.com` matches hosts ending in
          `.example.com` (the stored form has a leading dot so bare
          `example.com` is naturally excluded — cookie-scope rule).
        """
        if not self._allow_exact and not self._allow_suffix:
            return True
        host = host.lower()
        if host in self._allow_exact:
            return True
        for suffix in self._allow_suffix:
            if host.endswith(suffix):
                return True
        return False

    def replace_secrets(self, text: str) -> str:
        if not ENC_PATTERN.search(text):
            return text
        result = text
        for enc_value, plain_value in self.mapping.items():
            result = result.replace(enc_value, plain_value)
        return result

    def _emit_log(self, host: str, action: str, location: str | None = None, key: str | None = None) -> None:
        """Emit a structured JSON log line. NEVER includes secret values."""
        entry = {
            "time": datetime.datetime.now().strftime("%H:%M:%S"),
            "host": host,
            "action": action,
        }
        if location:
            entry["location"] = location
        if key:
            entry["key"] = key
        print(json.dumps(entry), flush=True)

    def request(self, flow: http.HTTPFlow) -> None:
        host = flow.request.pretty_host

        # Allow-list enforcement runs BEFORE passthrough classification.
        # Otherwise users could accidentally exempt blocked hosts by adding
        # them to passthrough, which is the opposite of the intent.
        if not self.is_allowed(host):
            flow.response = http.Response.make(
                403,
                (
                    b'{"error":"blocked_by_airlock",'
                    b'"detail":"host is not in the workspace network allow-list"}'
                ),
                {"content-type": "application/json"},
            )
            self._emit_log(host, "blocked")
            return

        if self.is_passthrough(host):
            self._emit_log(host, "passthrough")
            return

        self._maybe_reload_mapping()

        decrypted = False

        for name, value in flow.request.headers.items():
            replaced = self.replace_secrets(value)
            if replaced != value:
                flow.request.headers[name] = replaced
                self._emit_log(host, "decrypt", location="header", key=name)
                decrypted = True

        if flow.request.query:
            for key, value in flow.request.query.items():
                replaced = self.replace_secrets(value)
                if replaced != value:
                    flow.request.query[key] = replaced
                    self._emit_log(host, "decrypt", location="query", key=key)
                    decrypted = True

        if flow.request.content:
            try:
                body = flow.request.content.decode("utf-8")
                replaced = self.replace_secrets(body)
                if replaced != body:
                    flow.request.content = replaced.encode("utf-8")
                    self._emit_log(host, "decrypt", location="body")
                    decrypted = True
            except UnicodeDecodeError:
                pass

        if not decrypted:
            self._emit_log(host, "none")

    def response(self, flow: http.HTTPFlow) -> None:
        """Log response metadata for audit trail. Never logs response body."""
        host = flow.request.pretty_host
        status = flow.response.status_code
        content_type = flow.response.headers.get("content-type", "")
        size = len(flow.response.content) if flow.response.content else 0
        self._emit_log(
            host,
            "response",
            location=f"status:{status}",
            key=f"type:{content_type},size:{size}",
        )


# Only instantiate when loaded by mitmproxy, not during tests
if "pytest" not in sys.modules and "unittest" not in sys.modules:
    addons = [DecryptAddon()]
