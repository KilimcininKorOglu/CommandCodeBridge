# Changelog

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
