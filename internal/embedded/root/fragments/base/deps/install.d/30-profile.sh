#!/bin/sh
set -e

# Install the profile.d env plumbing: copy any ai-cabin-*.sh script that the
# active bundles contribute (the go bundle ships .deps/profile.d/ai-cabin-go.sh
# which sets the GOPATH bin PATH). The content stays bundle-owned; only the
# copy mechanics live here.
DEPS=/opt/ai-cabin-deps

mkdir -p /etc/profile.d
install -m 0755 "$DEPS/profile.d/ai-cabin-"*.sh /etc/profile.d/ 2>/dev/null || true
