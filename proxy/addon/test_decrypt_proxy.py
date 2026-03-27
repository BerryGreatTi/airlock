import json
import os
import tempfile
from unittest.mock import MagicMock
import pytest


def _make_addon(mapping: dict, passthrough: list[str] | None = None):
    """Helper to create a DecryptAddon with a temporary mapping file."""
    from decrypt_proxy import DecryptAddon
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
    json.dump(mapping, f)
    f.flush()
    f.close()
    addon = DecryptAddon(f.name, passthrough_hosts=passthrough)
    os.unlink(f.name)
    return addon


def _make_flow(
    host: str = "api.stripe.com",
    headers: dict | None = None,
    query: dict | None = None,
    body: bytes | None = None,
):
    """Create a mock mitmproxy HTTPFlow for testing request interception."""
    flow = MagicMock()
    flow.request.pretty_host = host

    # Headers: use a real dict-like object so items()/setitem work
    h = dict(headers or {})
    flow.request.headers = h

    # Query: use a real dict-like object
    q = dict(query or {})
    flow.request.query = q

    # Body
    flow.request.content = body

    return flow


def _add_response(flow, status_code=200, headers=None, content=b""):
    """Add a mock response to an existing flow."""
    flow.response = MagicMock()
    flow.response.status_code = status_code
    flow.response.headers = dict(headers or {})
    flow.response.content = content
    return flow


def test_load_mapping():
    from decrypt_proxy import DecryptAddon
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump({"ENC[age:abc123]": "real_secret"}, f)
        f.flush()
        mapping_path = f.name
    try:
        addon = DecryptAddon(mapping_path)
        assert addon.mapping["ENC[age:abc123]"] == "real_secret"
    finally:
        os.unlink(mapping_path)


def test_replace_in_string():
    from decrypt_proxy import DecryptAddon
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump({"ENC[age:token1]": "sk_live_real", "ENC[age:token2]": "pk_live_real"}, f)
        f.flush()
        mapping_path = f.name
    try:
        addon = DecryptAddon(mapping_path)
        assert addon.replace_secrets("Bearer ENC[age:token1]") == "Bearer sk_live_real"
        assert addon.replace_secrets("key=ENC[age:token2]&other=value") == "key=pk_live_real&other=value"
    finally:
        os.unlink(mapping_path)


def test_no_replacement_when_no_match():
    from decrypt_proxy import DecryptAddon
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump({"ENC[age:abc]": "secret"}, f)
        f.flush()
        mapping_path = f.name
    try:
        addon = DecryptAddon(mapping_path)
        assert addon.replace_secrets("no encrypted content here") == "no encrypted content here"
    finally:
        os.unlink(mapping_path)


def test_passthrough_hosts():
    from decrypt_proxy import DecryptAddon
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump({}, f)
        f.flush()
        mapping_path = f.name
    try:
        addon = DecryptAddon(mapping_path, passthrough_hosts=["api.anthropic.com", "auth.anthropic.com"])
        assert addon.is_passthrough("api.anthropic.com")
        assert addon.is_passthrough("auth.anthropic.com")
        assert not addon.is_passthrough("api.stripe.com")
    finally:
        os.unlink(mapping_path)


# --- Request interception tests using mock HTTPFlow ---


def test_request_replaces_headers():
    mapping = {"ENC[age:token1]": "sk_live_real"}
    addon = _make_addon(mapping)
    flow = _make_flow(
        host="api.stripe.com",
        headers={"Authorization": "Bearer ENC[age:token1]", "Content-Type": "application/json"},
    )
    addon.request(flow)
    assert flow.request.headers["Authorization"] == "Bearer sk_live_real"
    assert flow.request.headers["Content-Type"] == "application/json"


def test_request_replaces_query_params():
    mapping = {"ENC[age:qtoken]": "real_api_key"}
    addon = _make_addon(mapping)
    flow = _make_flow(
        host="api.example.com",
        query={"api_key": "ENC[age:qtoken]", "format": "json"},
    )
    addon.request(flow)
    assert flow.request.query["api_key"] == "real_api_key"
    assert flow.request.query["format"] == "json"


def test_request_replaces_body():
    mapping = {"ENC[age:bodytoken]": "secret_value"}
    addon = _make_addon(mapping)
    body = b'{"secret": "ENC[age:bodytoken]", "other": "plain"}'
    flow = _make_flow(host="api.example.com", body=body)
    addon.request(flow)
    assert flow.request.content == b'{"secret": "secret_value", "other": "plain"}'


def test_request_skips_passthrough_host():
    mapping = {"ENC[age:token1]": "should_not_appear"}
    addon = _make_addon(mapping, passthrough=["api.anthropic.com"])
    flow = _make_flow(
        host="api.anthropic.com",
        headers={"Authorization": "Bearer ENC[age:token1]"},
    )
    addon.request(flow)
    # Header should remain encrypted for passthrough hosts
    assert flow.request.headers["Authorization"] == "Bearer ENC[age:token1]"


def test_request_skips_binary_body():
    mapping = {"ENC[age:bintoken]": "plaintext"}
    addon = _make_addon(mapping)
    # Binary content that cannot be decoded as UTF-8
    binary_body = bytes([0x80, 0x81, 0x82, 0xFF, 0xFE])
    flow = _make_flow(host="api.example.com", body=binary_body)
    addon.request(flow)
    # Body should remain unchanged (not crash)
    assert flow.request.content == binary_body


def test_request_handles_none_body():
    mapping = {"ENC[age:tok]": "val"}
    addon = _make_addon(mapping)
    flow = _make_flow(host="api.example.com", body=None)
    addon.request(flow)
    # Should not crash on None body
    assert flow.request.content is None


def test_request_handles_empty_query():
    mapping = {"ENC[age:tok]": "val"}
    addon = _make_addon(mapping)
    flow = _make_flow(host="api.example.com", query={})
    # Ensure empty query dict is falsy (matching the `if flow.request.query:` guard)
    flow.request.query = {}
    addon.request(flow)
    # Should not crash


def test_request_multiple_tokens_in_body():
    mapping = {
        "ENC[age:tok1]": "secret1",
        "ENC[age:tok2]": "secret2",
    }
    addon = _make_addon(mapping)
    body = b'key1=ENC[age:tok1]&key2=ENC[age:tok2]'
    flow = _make_flow(host="api.example.com", body=body)
    addon.request(flow)
    assert flow.request.content == b'key1=secret1&key2=secret2'


def test_request_no_mapping_loaded():
    addon = _make_addon({})
    flow = _make_flow(
        host="api.example.com",
        headers={"Authorization": "Bearer ENC[age:nomatch]"},
    )
    addon.request(flow)
    # With empty mapping, ENC tokens remain as-is
    assert flow.request.headers["Authorization"] == "Bearer ENC[age:nomatch]"


def test_load_mapping_missing_file():
    from decrypt_proxy import DecryptAddon
    addon = DecryptAddon("/nonexistent/path/mapping.json", passthrough_hosts=[])
    assert addon.mapping == {}


def test_load_mapping_invalid_json():
    from decrypt_proxy import DecryptAddon
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
    f.write("not valid json {{{")
    f.flush()
    f.close()
    try:
        addon = DecryptAddon(f.name, passthrough_hosts=[])
        assert addon.mapping == {}
    finally:
        os.unlink(f.name)


def test_replace_secrets_multiple_occurrences_same_token():
    mapping = {"ENC[age:repeat]": "value"}
    addon = _make_addon(mapping)
    text = "first=ENC[age:repeat] second=ENC[age:repeat]"
    result = addon.replace_secrets(text)
    assert result == "first=value second=value"


def test_replace_secrets_non_overlapping_keys():
    """Verify replacement works correctly with distinct ENC tokens."""
    mapping = {
        "ENC[age:alpha]": "secret_a",
        "ENC[age:beta]": "secret_b",
        "ENC[age:gamma]": "secret_c",
    }
    addon = _make_addon(mapping)
    text = "a=ENC[age:alpha] b=ENC[age:beta] c=ENC[age:gamma]"
    result = addon.replace_secrets(text)
    assert "a=secret_a" in result
    assert "b=secret_b" in result
    assert "c=secret_c" in result
    # Ensure no ENC tokens remain
    import re
    assert not re.search(r"ENC\[age:", result)


def test_replace_secrets_preserves_surrounding_text():
    """Verify that text around ENC tokens is not corrupted."""
    mapping = {"ENC[age:tok]": "val"}
    addon = _make_addon(mapping)
    text = "prefix-ENC[age:tok]-suffix and more text"
    result = addon.replace_secrets(text)
    assert result == "prefix-val-suffix and more text"


# --- Hot-reload tests ---


def test_mapping_hot_reload():
    """Verify mapping is reloaded when the file changes on disk."""
    from decrypt_proxy import DecryptAddon
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
    json.dump({"ENC[age:v1]": "secret_v1"}, f)
    f.flush()
    f.close()

    addon = DecryptAddon(f.name, passthrough_hosts=[])
    assert addon.replace_secrets("ENC[age:v1]") == "secret_v1"

    # Update the mapping file with new content
    import time
    time.sleep(0.05)  # ensure mtime changes
    with open(f.name, "w") as fh:
        json.dump({"ENC[age:v2]": "secret_v2"}, fh)

    # Trigger reload via request
    flow = _make_flow(host="api.example.com", headers={"Auth": "ENC[age:v2]"})
    addon.request(flow)
    assert flow.request.headers["Auth"] == "secret_v2"

    os.unlink(f.name)


def test_mapping_hot_reload_no_change():
    """Verify no unnecessary reload when file hasn't changed."""
    from decrypt_proxy import DecryptAddon
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
    json.dump({"ENC[age:tok]": "val"}, f)
    f.flush()
    f.close()

    addon = DecryptAddon(f.name, passthrough_hosts=[])
    original_mtime = addon._last_mtime

    # Request without file change should not reload
    flow = _make_flow(host="api.example.com", headers={"X": "plain"})
    addon.request(flow)
    assert addon._last_mtime == original_mtime

    os.unlink(f.name)


# --- Structured JSON logging tests ---


def test_request_emits_log_on_decrypt(capsys):
    mapping = {"ENC[age:tok1]": "secret_val"}
    addon = _make_addon(mapping)
    flow = _make_flow(
        host="api.stripe.com",
        headers={"Authorization": "Bearer ENC[age:tok1]"},
    )
    addon.request(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["host"] == "api.stripe.com"
    assert log["action"] == "decrypt"
    assert log["location"] == "header"
    assert log["key"] == "Authorization"
    assert "secret" not in captured.out


def test_request_emits_log_on_passthrough(capsys):
    addon = _make_addon({}, passthrough=["api.anthropic.com"])
    flow = _make_flow(host="api.anthropic.com", headers={"Auth": "token"})
    addon.request(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "passthrough"
    assert log["host"] == "api.anthropic.com"


def test_request_emits_log_on_no_match(capsys):
    addon = _make_addon({"ENC[age:other]": "val"})
    flow = _make_flow(host="cdn.example.com", headers={"Accept": "text/html"})
    addon.request(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "none"


# --- Response audit logging tests ---


def test_response_emits_log(capsys):
    addon = _make_addon({})
    flow = _make_flow(host="api.stripe.com")
    _add_response(flow, status_code=200, headers={"content-type": "application/json"}, content=b'{"ok":true}')
    addon.response(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["host"] == "api.stripe.com"
    assert log["action"] == "response"
    assert "status:200" in log["location"]
    assert "type:application/json" in log["key"]
    assert "size:11" in log["key"]


def test_response_logs_passthrough_host(capsys):
    addon = _make_addon({}, passthrough=["api.anthropic.com"])
    flow = _make_flow(host="api.anthropic.com")
    _add_response(flow, status_code=200, headers={"content-type": "text/event-stream"}, content=b"data: chunk")
    addon.response(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "response"
    assert log["host"] == "api.anthropic.com"


def test_response_handles_none_content(capsys):
    addon = _make_addon({})
    flow = _make_flow(host="api.example.com")
    _add_response(flow, status_code=204, headers={}, content=None)
    addon.response(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "response"
    assert "status:204" in log["location"]
    assert "size:0" in log["key"]


def test_response_handles_missing_content_type(capsys):
    addon = _make_addon({})
    flow = _make_flow(host="api.example.com")
    _add_response(flow, status_code=200, headers={}, content=b"data")
    addon.response(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "response"
    assert "type:" in log["key"]
    assert "size:4" in log["key"]


def test_response_logs_error_status(capsys):
    addon = _make_addon({})
    flow = _make_flow(host="api.example.com")
    _add_response(flow, status_code=500, headers={"content-type": "text/plain"}, content=b"Internal Server Error")
    addon.response(flow)
    captured = capsys.readouterr()
    log = json.loads(captured.out.strip())
    assert log["action"] == "response"
    assert "status:500" in log["location"]


def test_response_does_not_log_body_content(capsys):
    addon = _make_addon({})
    flow = _make_flow(host="api.example.com")
    _add_response(flow, status_code=200, headers={"content-type": "application/json"}, content=b'{"secret":"supersecret123"}')
    addon.response(flow)
    captured = capsys.readouterr()
    assert "supersecret123" not in captured.out
