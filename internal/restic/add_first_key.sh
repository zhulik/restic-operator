#!/bin/sh

set -eu

# export RESTIC_REPOSITORY="/test"
# export RESTIC_HOST="restic-operator"
# export RESTIC_USER="restic"
# export NEW_KEY_FILE="/tmp/key.txt"
# echo "new-password" > "$NEW_KEY_FILE"

# restic init --insecure-no-password > /dev/null

# Get the empty key
EMPTY_KEY=$(restic key list --insecure-no-password --json | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

# Add the new key
restic key add --host "$RESTIC_HOST" --user "$RESTIC_USER" --new-password-file "$NEW_KEY_FILE" --insecure-no-password

# Remove the empty key
restic key remove "$EMPTY_KEY" --password-file "$NEW_KEY_FILE" > /dev/null

# restic key list --password-file /tmp/key.txt
