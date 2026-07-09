#!/usr/bin/env bash
# Claude Code CLI launcher for CommandCode Bridge proxy.
# Reads proxy_token from data/config.json so the token never appears in
# the command line, environment files, or shell history.
#
# Uses claude --settings to override the global ~/.claude/settings.json env
# block, which would otherwise point at a different provider.
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

MODEL="deepseek/deepseek-v4-pro"
HAIKU="deepseek/deepseek-v4-flash"

# Build settings JSON that overrides the global settings.json env block.
# The proxy accepts the token via x-api-key (Anthropic SDK convention).
SETTINGS=$(cat <<EOF
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:3050",
    "ANTHROPIC_API_KEY": "${PROXY_TOKEN}",
    "ANTHROPIC_MODEL": "${MODEL}",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "${HAIKU}",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "${MODEL}",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "${MODEL}"
  },
  "model": "${MODEL}"
}
EOF
)

exec claude --settings "$SETTINGS" "$@"
