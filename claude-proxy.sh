#!/usr/bin/env bash
# Claude Code CLI launcher for CommandCode Bridge proxy.
# Reads proxy_token from data/config.json so the token never appears in
# the command line, environment files, or shell history.
#
# Uses claude --settings to override the global ~/.claude/settings.json env
# block, which would otherwise point at a different provider.
#
# Usage:
#   ccp                      # interactive, default model
#   ccp -p "hello"           # one-shot prompt
#   ccp --model zai-org/GLM-5.2        # override main/sonnet/opus model
#   ccp --model X --haiku Y            # override both tiers
# All other args are passed through to claude.
set -euo pipefail

CONFIG="${CONFIG:-data/config.json}"

# Resolve config path relative to this script's location.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${SCRIPT_DIR}/${CONFIG}"

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "Config not found: $CONFIG_PATH" >&2
  exit 1
fi

# Extract proxy_token without jq dependency.
PROXY_TOKEN="$(grep -o '"proxy_token"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_PATH" | head -1 | sed 's/.*:.*"\(.*\)"/\1/')"
if [[ -z "$PROXY_TOKEN" ]]; then
  echo "proxy_token is empty in $CONFIG_PATH" >&2
  exit 1
fi

# Defaults — overridable via --model / --haiku flags below.
MODEL="deepseek/deepseek-v4-pro"
HAIKU="deepseek/deepseek-v4-flash"

# Parse our own flags out of argv; everything else passes through to claude.
PASSTHROUGH=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --model)
      MODEL="$2"; shift 2 ;;
    --model=*)
      MODEL="${1#--model=}"; shift ;;
    --haiku)
      HAIKU="$2"; shift 2 ;;
    --haiku=*)
      HAIKU="${1#--haiku=}"; shift ;;
    *)
      PASSTHROUGH+=("$1"); shift ;;
  esac
done

# Build settings JSON safely using python3 to avoid JSON injection if the
# token contains quotes or backslashes. Falls back to heredoc if python3
# is unavailable.
if command -v python3 &>/dev/null; then
  SETTINGS=$(python3 -c '
import json, sys
token = sys.argv[1]
model = sys.argv[2]
haiku = sys.argv[3]
print(json.dumps({
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:3050",
    "ANTHROPIC_API_KEY": token,
    "ANTHROPIC_AUTH_TOKEN": token,
    "ANTHROPIC_CUSTOM_HEADERS": "",
    "ANTHROPIC_MODEL": model,
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": haiku,
    "ANTHROPIC_DEFAULT_SONNET_MODEL": model,
    "ANTHROPIC_DEFAULT_OPUS_MODEL": model
  },
  "model": model
}))
' "$PROXY_TOKEN" "$MODEL" "$HAIKU")
else
  # Fallback: escape token for JSON (handles " and \)
  ESCAPED_TOKEN="${PROXY_TOKEN//\\/\\\\}"
  ESCAPED_TOKEN="${ESCAPED_TOKEN//\"/\\\"}"
  SETTINGS=$(cat <<EOF
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:3050",
    "ANTHROPIC_API_KEY": "${ESCAPED_TOKEN}",
    "ANTHROPIC_AUTH_TOKEN": "${ESCAPED_TOKEN}",
    "ANTHROPIC_CUSTOM_HEADERS": "",
    "ANTHROPIC_MODEL": "${MODEL}",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "${HAIKU}",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "${MODEL}",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "${MODEL}"
  },
  "model": "${MODEL}"
}
EOF
  )
fi

exec claude --settings "$SETTINGS" "${PASSTHROUGH[@]+"${PASSTHROUGH[@]}"}"
