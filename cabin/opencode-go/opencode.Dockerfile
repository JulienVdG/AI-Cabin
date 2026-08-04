# Using Debian Trixie for consistency with host.
FROM golang:1.26-trixie

# Define the version as an argument.
ARG OPENCODE_VERSION=1.4.9

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

COPY .deps/greywall /usr/local/bin/greywall
RUN chmod +x /usr/local/bin/greywall

# Greyproxy CA certificate for HTTPS inspection
COPY .deps/greyproxy-ca.crt /usr/local/share/ca-certificates/greyproxy.crt
RUN update-ca-certificates

# Download and install OpenCode.
RUN curl -L "https://github.com/anomalyco/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-x64.tar.gz" -o opencode.tar.gz && \
    tar -xzf opencode.tar.gz && \
    mv opencode /opt/opencode && \
    chmod +x /opt/opencode && \
    rm opencode.tar.gz

# Create entrypoint.d directory for hooks
RUN mkdir -p /docker-entrypoint.d

# Copy generic entrypoint from .deps (base bundle)
COPY .deps/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Copy entrypoint.d hooks from .deps (socat-greyproxy, git-agent-email, port-forward)
COPY .deps/docker-entrypoint.d/ /docker-entrypoint.d/
RUN chmod +x /docker-entrypoint.d/*.sh 2>/dev/null || true

# Copy profile.d env scripts from .deps (go bundle)
COPY .deps/profile.d/ /etc/profile.d/
RUN chmod +x /etc/profile.d/ai-cabin-*.sh 2>/dev/null || true

ENTRYPOINT ["/docker-entrypoint.sh"]

# Greybash/greyopencode wrappers from .deps (base + agent-opencode bundles)
COPY .deps/greybash /usr/local/bin/greybash
COPY .deps/greyopencode /usr/local/bin/greyopencode
RUN chmod +x /usr/local/bin/greybash /usr/local/bin/greyopencode

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
RUN mkdir -p .local/share .local/state .local/bin .cache .config/greywall .config/opencode desk go

EXPOSE 9090

# Default command: Start the web interface with greywall sandbox
CMD ["greyopencode", "web", "--port", "9090", "--hostname", "0.0.0.0"]
