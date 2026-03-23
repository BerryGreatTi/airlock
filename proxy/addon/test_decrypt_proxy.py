import json
import os
import tempfile
import pytest


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
