#!/bin/sh
set -e

# Install the greywall sandbox binary, the greyproxy CA certificate (HTTPS
# inspection) and the greybash wrapper. install is used instead of cp so the
# executable bit is set explicitly: Materialize writes fragments as 0644.
DEPS=/opt/ai-cabin-deps

install -m 0755 "$DEPS/greywall" /usr/local/bin/greywall
install -m 0755 "$DEPS/greybash" /usr/local/bin/greybash
install -m 0644 "$DEPS/greyproxy-ca.crt" /usr/local/share/ca-certificates/greyproxy.crt
update-ca-certificates
