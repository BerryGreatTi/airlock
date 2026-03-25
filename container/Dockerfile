FROM node:22-bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    python3 \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

RUN useradd -m -s /bin/bash airlock

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

COPY entrypoint-keepalive.sh /usr/local/bin/entrypoint-keepalive.sh
RUN chmod +x /usr/local/bin/entrypoint-keepalive.sh

USER airlock
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
