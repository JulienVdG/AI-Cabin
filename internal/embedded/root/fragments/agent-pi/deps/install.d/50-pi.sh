#!/bin/sh
set -e

# Download and install the pi.dev toolchain: ripgrep (Pi search), fd (Pi file
# search) and the pi binary, then the greywall-sandboxed pi wrapper. The pi
# version is passed via env (the Dockerfile ARG PI_VERSION is in scope for the
# RUN instruction); rg and fd are pinned like the reference cabins.

# Resolve the target architecture. rg and fd use the Rust release triple
# (x86_64/aarch64-unknown-linux-gnu); pi uses its own short naming
# (linux-x64/arm64).
case "$(uname -m)" in
  x86_64)
    RUST_ARCH="x86_64-unknown-linux-gnu"
    RG_ARCH="x86_64-unknown-linux-musl"
    SHORT_ARCH="linux-x64"
    ;;
  aarch64)
    RUST_ARCH="aarch64-unknown-linux-gnu"
    RG_ARCH="aarch64-unknown-linux-gnu"
    SHORT_ARCH="linux-arm64"
    ;;
  *)
    echo "cabin: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

DEPS=/opt/ai-cabin-deps

# ripgrep (Pi search).
curl -fsSL "https://github.com/BurntSushi/ripgrep/releases/download/15.2.0/ripgrep-15.2.0-${RG_ARCH}.tar.gz" -o rg.tar.gz
tar -xzf rg.tar.gz
install -m 0755 ripgrep-*/rg /usr/local/bin/rg
rm -rf rg.tar.gz ripgrep-*

# fd (Pi file search).
curl -fsSL "https://github.com/sharkdp/fd/releases/download/v10.2.0/fd-v10.2.0-${RUST_ARCH}.tar.gz" -o fd.tar.gz
tar -xzf fd.tar.gz
install -m 0755 fd-v*/fd /usr/local/bin/fd
rm -rf fd.tar.gz fd-v*

# pi.
mkdir -p /opt
curl -fsSL "https://github.com/earendil-works/pi/releases/download/${PI_VERSION}/pi-${SHORT_ARCH}.tar.gz" -o pi.tar.gz
tar -xzf pi.tar.gz -C /opt
rm -f pi.tar.gz

# greywall-sandboxed pi wrapper.
install -m 0755 "$DEPS/pi" /usr/local/bin/pi
