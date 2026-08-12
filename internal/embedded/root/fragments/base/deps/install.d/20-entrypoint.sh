#!/bin/sh
set -e

# Install the cabin entrypoint: the generic docker-entrypoint.sh plus the
# merged /docker-entrypoint.d hooks (base socat-greyproxy, git-agent email,
# cabin-local port-forward scripts). The empty-glob guard skips a hooks dir
# with no hooks (a directory without hooks is not required for boot).
DEPS=/opt/ai-cabin-deps

mkdir -p /docker-entrypoint.d
install -m 0755 "$DEPS/docker-entrypoint.sh" /docker-entrypoint.sh
install -m 0755 "$DEPS/docker-entrypoint.d/"* /docker-entrypoint.d/ 2>/dev/null || true
