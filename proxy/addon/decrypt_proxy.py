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


class DecryptAddon:
    def __init__(
        self,
        mapping_path: str = MAPPING_PATH,
        passthrough_hosts: list[str] | None = None,
    ):
        self.mapping: dict[str, str] = {}
        self.passthrough: set[str] = set()
        self._mapping_path: str = mapping_path
        self._last_mtime: float = 0.0

        if passthrough_hosts is not None:
            self.passthrough = set(passthrough_hosts)
        else:
            self.passthrough = {
                h.strip()
                for h in PASSTHROUGH_HOSTS_RAW.split(",")
                if h.strip()
            }

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
        return host in self.passthrough

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
