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

# Build settings JSON that overrides the global settings.json env block.
# --settings MERGES with the global config, so we must explicitly override
# every env var that interferes:
#   - ANTHROPIC_AUTH_TOKEN: global sets "dummy"; interactive (cli) mode uses
#     this for auth, so it must be the real proxy token.
#   - ANTHROPIC_CUSTOM_HEADERS: global injects Cloudflare gateway headers;
#     cleared so they don't leak to the local proxy.
#   - ANTHROPIC_BASE_URL: global points at Cloudflare gateway.
SETTINGS=$(cat <<EOF
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:3050",
    "ANTHROPIC_API_KEY": "${PROXY_TOKEN}",
    "ANTHROPIC_AUTH_TOKEN": "${PROXY_TOKEN}",
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

exec claude --settings "$SETTINGS" "${PASSTHROUGH[@]+"${PASSTHROUGH[@]}"}"
