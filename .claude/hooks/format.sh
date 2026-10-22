#!/bin/bash
# PostToolUse: format the edited file if the formatter is installed; never fails the edit.
f=$(jq -r '.tool_input.file_path // empty' 2>/dev/null)
[ -n "$f" ] && [ -f "$f" ] || exit 0
root="${CLAUDE_PROJECT_DIR:-$(pwd)}"

case "$f" in
  *.go)
    command -v gofmt >/dev/null && gofmt -w "$f" ;;
  *.ts|*.tsx|*.css|*.json)
    p="$root/web/node_modules/.bin/prettier"
    case "$f" in "$root"/web/*) [ -x "$p" ] && "$p" --write --log-level warn "$f" ;; esac ;;
esac
exit 0
