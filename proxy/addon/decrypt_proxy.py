"""mitmproxy addon that transparently decrypts ENC[age:...] patterns in HTTP requests."""

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
            with open(path) as f:
                self.mapping = json.load(f)
            logging.info("Loaded %d secret mappings", len(self.mapping))
        except FileNotFoundError:
            logging.warning("Mapping file not found: %s", path)
        except json.JSONDecodeError as e:
            logging.error("Invalid mapping JSON: %s", e)

    def is_passthrough(self, host: str) -> bool:
        return host in self.passthrough

    def replace_secrets(self, text: str) -> str:
        if not ENC_PATTERN.search(text):
            return text
        result = text
        for enc_value, plain_value in self.mapping.items():
            result = result.replace(enc_value, plain_value)
        return result

    def request(self, flow: http.HTTPFlow) -> None:
        if self.is_passthrough(flow.request.pretty_host):
            return
        for name, value in flow.request.headers.items():
            replaced = self.replace_secrets(value)
            if replaced != value:
                flow.request.headers[name] = replaced
        if flow.request.query:
            for key, value in flow.request.query.items():
                replaced = self.replace_secrets(value)
                if replaced != value:
                    flow.request.query[key] = replaced
        if flow.request.content:
            try:
                body = flow.request.content.decode("utf-8")
                replaced = self.replace_secrets(body)
                if replaced != body:
                    flow.request.content = replaced.encode("utf-8")
            except UnicodeDecodeError:
                pass


# Only instantiate when loaded by mitmproxy, not during tests
if "pytest" not in sys.modules and "unittest" not in sys.modules:
    addons = [DecryptAddon()]
