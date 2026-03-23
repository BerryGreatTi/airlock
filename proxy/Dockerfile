FROM mitmproxy/mitmproxy:latest

COPY addon/decrypt_proxy.py /opt/addon/decrypt_proxy.py

ENTRYPOINT ["mitmdump", "-s", "/opt/addon/decrypt_proxy.py", "--set", "connection_strategy=lazy"]
