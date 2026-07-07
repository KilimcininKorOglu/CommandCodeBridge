# Changelog

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
