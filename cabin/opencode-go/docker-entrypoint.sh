#!/bin/bash
set -e

# Run all entrypoint.d scripts
if [ -d /docker-entrypoint.d ]; then
    for f in /docker-entrypoint.d/*.sh; do
        if [ -x "$f" ]; then
            "$f"
        fi
    done
fi

# Execute the main command
exec "$@"
