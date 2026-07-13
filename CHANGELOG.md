# Changelog

## [1.2.0] - 2026-07-13

### Added
- Add streaming flusher, token usage logging, and Anthropic cache mapping.
- Add --model/--haiku flag support to claude-proxy.sh.

### Changed
- Add nil-metadata regression test for Anthropic converter.
- Gitignore .wrongstack/ tooling directory.

### Fixed
- Address Gemini Code Assist review feedback (nil check, goroutine leak, and log safety).
- Prevent nil metadata/thinking from serializing as JSON null.
- Fail-fast on unreadable config instead of silently using defaults.
- Correct Docker Compose port mapping and SELinux volume labels.
- Set ANTHROPIC_AUTH_TOKEN for interactive mode auth.

## [1.1.4] - 2026-07-08

### Added
- Add native Anthropic support.
- Make models endpoint public.
- Add token count lifecycle logs.
- Log Anthropic thinking propagation.
- Add OpenAI Responses endpoints.

### Changed
- Update default log level to debug.
- Add OpenAI request debug fields.
- Simplify API key extraction.

### Fixed
- Preserve Anthropic tool result names.
- Preserve tool role for Anthropic tool results.
- Fall back to config key on invalid bearer.
- Preserve OpenAI content block tool results.
- Stream OpenAI tool calls as deltas.
- Support OpenAI input image blocks.
- Support OpenAI Responses top-level tools.

## [1.1.3] - 2026-07-08

### Added
- No added changes.

### Changed
- No changed changes.

### Fixed
- Improve Claude CLI compatibility.

## [1.1.2] - 2026-07-08

### Added
- Comprehensive request lifecycle logging across client, handlers, models, session, and main packages.
- Debug-level logs for request received, session created, upstream forwarded, upstream status, config loaded, and session store initialized events.
- Error-level logs for marshal, decode, and transport failures.
- Warn-level logs for non-200 upstream responses and zero output token cases.
- Explicit LOG_FILE environment variable in docker-compose.yml.
- Logger argument to session.NewStore for session creation and cleanup logging.

## [1.1.1] - 2026-07-08

### Added
- Claude Code CLI support via x-api-key header authentication.
- Upstream non-200 status code logging for debugging.

### Changed
- Update default model environment variables in READMEs.

### Fixed
- Fix Anthropic content array conversion using []any instead of []map[string]any.
- Fix Anthropic tool_result conversion to set role to "tool" and support array content.

## [1.1.0] - 2026-07-07

### Added
- Improve message conversion and streaming.

### Changed
- Add license information.
- Add setup instructions to READMEs.
- Initial project commit.

### Fixed
- No fixed changes.
