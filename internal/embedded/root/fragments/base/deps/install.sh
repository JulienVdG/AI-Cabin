#!/bin/sh
set -e

# Run every install step in lexicographic order (Debian run-parts
# convention: 10-greywall < 50-pi). Each step installs its bundle-contributed
# content from the build deps dir (/opt/ai-cabin-deps) into the image. A
# bundle registers a step by shipping install.d/<NN>-<name>.sh; the Dockerfile
# only calls this dispatcher, so adding a feature needs no Dockerfile change.
for step in /opt/ai-cabin-deps/install.d/*.sh; do
  # Guard: an absent install.d/ leaves the glob unexpanded (single literal).
  [ -e "$step" ] || continue
  echo "==> $(basename "$step")"
  /bin/sh "$step"
done
