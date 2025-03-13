#!/usr/bin/env bash

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[@]}")" && pwd)

exec pkl eval -f json "${SCRIPT_DIR}/gitsync.config.pkl" \
    > "${SCRIPT_DIR}/gitsync.config.json"
