#!/bin/sh
# The SSH server in the background, then Apache as the image normally runs it.
set -e
/usr/sbin/sshd
exec docker-entrypoint.sh apache2-foreground
