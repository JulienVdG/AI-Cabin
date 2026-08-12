#!/bin/sh
set -e

# Download and install the opencode binary, then the greywall-sandboxed
# opencode wrapper. The version is passed via env (the Dockerfile ARG
# OPENCODE_VERSION is in scope for the RUN instruction).
DEPS=/opt/ai-cabin-deps

curl -fsSL "https://github.com/anomalyco/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-x64.tar.gz" -o opencode.tar.gz
tar -xzf opencode.tar.gz
install -m 0755 opencode /opt/opencode
rm -f opencode.tar.gz

# greywall-sandboxed opencode wrapper.
install -m 0755 "$DEPS/opencode" /usr/local/bin/opencode
