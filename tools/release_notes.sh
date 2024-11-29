#!/usr/bin/env bash
set -eu

awk '
  /^<!--/,/^-->/ { next }
  /^## \[([0-9]+\.[0-9]+\.[0-9]+)\]\s*.*/ {
    if (!found) {
      found = 1
    } else {
      exit
    }
  }
  found { print }
' "${1:-CHANGELOG.md}"
