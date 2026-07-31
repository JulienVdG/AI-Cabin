#!/bin/bash
# Forward localhost:43080 → host.docker.internal:43080 (greyproxy API)
# Required for --inject to work when greyproxy runs on host
echo "[entrypoint] Starting socat greyproxy API forward..."
socat TCP-LISTEN:43080,fork,reuseaddr TCP:host.docker.internal:43080 &
