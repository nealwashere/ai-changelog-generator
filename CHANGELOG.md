# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-02-22

### Added

- Added GitHub Actions workflow to automatically create GitHub release notes from `CHANGELOG.md` when a version tag is pushed

### Changed

- Updated README API key placeholder from `sk-ant-...` to `{ANTHROPIC_TOKEN}` in all usage examples

## [1.0.0] - 2026-02-22

### Added

- AI-powered changelog generation using the Anthropic API with streaming output
- Smart diff strategy: sends full diff when changes are under 2000 lines, falls back to stat-only for larger changesets
- Release workflow via `--version` flag: generates changelog, commits `CHANGELOG.md`, and creates an annotated git tag
- Semver validation ensuring new versions are strictly greater than the last release tag
- First-release support: diffs entire history when no prior tags exist
- Preview mode: streams changelog to stdout without committing or tagging
- Configurable Anthropic model via `--model` flag (defaults to `claude-sonnet-4-6`)
- `Makefile` with `build`, `install`, and `uninstall` targets
