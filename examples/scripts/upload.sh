#!/bin/sh
set -eu

: "${ARTIFACT_NAME:?ARTIFACT_NAME is required}"
: "${ARTIFACT_DIR:=artifacts}"

mkdir -p "$ARTIFACT_DIR"
printf 'upload %s to %s\n' "$ARTIFACT_NAME" "$ARTIFACT_DIR"
printf '%s\n' "$ARTIFACT_NAME" > "$ARTIFACT_DIR/$ARTIFACT_NAME.txt"
printf 'wrote %s\n' "$ARTIFACT_DIR/$ARTIFACT_NAME.txt"
