# Using Debian Trixie for consistency with host.
FROM golang:1.26-trixie

# Define the version as an argument.
ARG PI_VERSION=v0.72.1

# Install tools, including bash-completion.
RUN apt-get update && apt-get install -y \
    git \
    sqlite3 \
    curl \
    vim \
    ca-certificates \
    bash-completion \
    less \
    bubblewrap socat strace \
    netcat-openbsd iproute2 iputils-ping \
    && rm -rf /var/lib/apt/lists/*

COPY greywall /usr/local/bin/greywall
RUN chmod +x /usr/local/bin/greywall

# Greyproxy CA certificate for HTTPS inspection
COPY greyproxy-ca.crt /usr/local/share/ca-certificates/greyproxy.crt
RUN update-ca-certificates

# Download and install ripgrep (for Pi search)
RUN curl -L "https://github.com/BurntSushi/ripgrep/releases/download/14.1.1/ripgrep-14.1.1-x86_64-unknown-linux-musl.tar.gz" -o rg.tar.gz && \
    tar -xzf rg.tar.gz && \
    mv ripgrep-*/rg /usr/local/bin/rg && \
    chmod +x /usr/local/bin/rg && \
    rm -rf rg.tar.gz ripgrep-*

# Download and install fd (for Pi file search)
RUN curl -L "https://github.com/sharkdp/fd/releases/download/v10.2.0/fd-v10.2.0-x86_64-unknown-linux-gnu.tar.gz" -o fd.tar.gz && \
    tar -xzf fd.tar.gz && \
    mv fd-v*/fd /usr/local/bin/fd && \
    chmod +x /usr/local/bin/fd && \
    rm -rf fd.tar.gz fd-v*

# Download and install Pi.
RUN curl -L "https://github.com/badlogic/pi-mono/releases/download/${PI_VERSION}/pi-linux-x64.tar.gz" -o pi.tar.gz && \
    mkdir -p /opt && \
    tar -xzf pi.tar.gz -C /opt && \
    rm pi.tar.gz

# Create entrypoint.d directory for hooks
RUN mkdir -p /docker-entrypoint.d

# Copy generic entrypoint from .deps/_common
COPY .deps/_common/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Copy entrypoint.d hooks from .deps/_common (git email)
COPY .deps/_common/docker-entrypoint.d/ /docker-entrypoint.d/
# Copy entrypoint.d hooks from .deps/_greywall (socat greyproxy)
COPY .deps/_greywall/docker-entrypoint.d/ /docker-entrypoint.d/
RUN chmod +x /docker-entrypoint.d/*.sh 2>/dev/null || true

ENTRYPOINT ["/docker-entrypoint.sh"]

# Greybash and Greypi wrappers (greywall sandboxed shells)
COPY greybash /usr/local/bin/greybash
COPY greypi /usr/local/bin/greypi
RUN chmod +x /usr/local/bin/greybash /usr/local/bin/greypi

# User setup.
RUN useradd -m ai_agent
WORKDIR /home/ai_agent

# Set ownership.
RUN chown -R ai_agent:ai_agent /home/ai_agent
USER ai_agent
WORKDIR /home/ai_agent

# Ensure bash-completion is sourced in the agent's shell.
RUN echo 'if [ -f /etc/bash_completion ]; then . /etc/bash_completion; fi' >> /home/ai_agent/.bashrc

# Add greywall sandbox indicator to prompt
RUN echo 'if [ "$GREYWALL_SANDBOX" = "1" ]; then debian_chroot="🔒"; fi' >> /home/ai_agent/.bashrc

# Create future mount-points so that owner is ai_agent
RUN mkdir -p .local/share .local/state .local/bin .pi/agent

# Default command: sleep infinity for manual testing
CMD ["sleep", "infinity"]
