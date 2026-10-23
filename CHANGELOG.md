# Changelog

## [Unreleased]

### Added
- Add full OpenAI Responses streaming item lifecycle: output_item.added, content_part.added/done, output_item.done for message and reasoning items, and reasoning_text deltas, with all items reflected in response.completed.
- Add leading role:assistant chunk and unique tool_calls indexes to chat completions streams.
- Add ping after message_start and signature_delta before thinking blocks close in Anthropic streams.

### Changed
- Lift system/developer Responses input items into params.system instead of forwarding roles the upstream rejects; drop replayed reasoning items from request input.
- Synthesize terminal events (Anthropic message_delta/message_stop, OpenAI finish_reason chunk) when the upstream stream ends without a finish event, so clients no longer hang waiting for stream completion.
- Merge consecutive reasoning deltas into one Anthropic thinking block.
- Derive stop_reason tool_use / finish_reason tool_calls when tool blocks were emitted.
- Make in-stream error events format-aware: Anthropic-shaped event: error envelopes and OpenAI payloads with a top-level type:error for Responses clients.
- Stop emitting upstream toolResult events as invalid role:tool chat completions chunks.
- Remove the OpenAI-style [DONE] marker from Anthropic streams.
- Default claude-proxy.sh model to deepseek/deepseek-v4.1-flash.

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
