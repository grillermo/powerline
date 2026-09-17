#!/bin/sh
# Builds powerline-zsh. Set HOSTNAME_COLOR (0-255) in .env or the environment
# to pin the hostname segment color; otherwise it is derived from the hostname.
set -e
cd "$(dirname "$0")"
[ -z "$HOSTNAME_COLOR" ] && [ -f .env ] && . ./.env
go build -ldflags "-X main.hostColor=${HOSTNAME_COLOR}" -o powerline-zsh powerline-zsh.go
