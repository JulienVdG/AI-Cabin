# Using Debian Trixie for consistency with host.
FROM golang:1.26-trixie

# Define the version as an argument.
ARG PI_VERSION=v0.72.1

# Install tools, including bash-completion.
RUN apt-get update && apt-get install -y \
    git \
    sqlite3 \
    curl \
    jq \
    vim \
    ca-certificates \
    bash-completion \
    less \
    bubblewrap socat strace \
    netcat-openbsd iproute2 iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Copy the deps fragments to a build dir and run the numbered install steps.
COPY .deps/ /opt/ai-cabin-deps/
RUN /bin/sh /opt/ai-cabin-deps/install.sh \
 && rm -rf /opt/ai-cabin-deps

ENTRYPOINT ["/docker-entrypoint.sh"]

# User setup.
RUN useradd -m ai_agent
WORKDIR /home/ai_agent

# Set ownership.
RUN chown -R ai_agent:ai_agent /home/ai_agent
USER ai_agent
WORKDIR /home/ai_agent

# Add greywall sandbox indicator to prompt
RUN echo 'if [ "$GREYWALL_SANDBOX" = "1" ]; then debian_chroot="🔒"; fi' >> /home/ai_agent/.bashrc

# Create future mount-points so that owner is ai_agent
RUN mkdir -p .local/share .local/state .local/bin .cache .config/greywall .pi/agent desk go

# Default command: sleep infinity for manual testing
CMD ["sleep", "infinity"]
