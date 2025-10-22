#!/bin/sh

set -eu

export RESTIC_REPOSITORY="/test"
export RESTIC_HOST="restic-operator"
export RESTIC_USER="restic"
echo "new-password" > /tmp/key.txt



restic init --insecure-no-password > /dev/null

EMPTY_KEY=$(restic key list --insecure-no-password --json |  sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

restic key add --host $RESTIC_HOST --user $RESTIC_USER --new-password-file /tmp/key.txt --insecure-no-password
restic key remove $EMPTY_KEY --password-file /tmp/key.txt > /dev/null

# restic key list --password-file /tmp/key.txt