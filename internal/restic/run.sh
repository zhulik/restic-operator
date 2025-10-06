#!/bin/sh

set -eu

apk add ruby &&

ruby -e "$1"
