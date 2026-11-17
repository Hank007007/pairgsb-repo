#!/bin/sh
set -eu
printf 'hello from an action with %s\n' "${TARGET:-nobody}"
