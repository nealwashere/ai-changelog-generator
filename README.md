# AI Changelog Generator

Generates and updates the changelog for a release using a release version and an AI provider token.

Each run diffs from the **last release tag to HEAD**. If no tags exist yet, it diffs the entire history.

## Requirements

- Go 1.23+
- An [Anthropic API key](https://console.anthropic.com/)

## Install

```bash
git clone git@github.com:nealwashere/ai-changelog-generator.git
cd ai-changelog-generator
make install
```

This builds the binary and places it in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH` — if it isn't, add this to your shell config (`.bashrc`, `.zshrc`, `config.fish`, etc.):

```bash
# bash / zsh
export PATH="$(go env GOPATH)/bin:$PATH"

# fish
fish_add_path (go env GOPATH)/bin
```

To remove the binary:

```bash
make uninstall
```

## Usage

```bash
changelog-generator --api-key {ANTHROPIC_TOKEN} --version {SEMVER}
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--api-key` | — | `$ANTHROPIC_API_KEY` | Anthropic API key |
| `--version` | `-v` | — | Release version (e.g. `1.2.0`) — updates `CHANGELOG.md` and creates a git tag |
| `--repo` | `-r` | `.` | Path to git repo |
| `--model` | `-m` | `claude-sonnet-4-6` | Anthropic model ID |
| `--output` | `-o` | stdout / `CHANGELOG.md` | Output file (overrides default in release mode) |
| `--max-diff` | — | `2000` | Max changed lines before switching to stat-only mode |

The API key can also be set via the `ANTHROPIC_API_KEY` environment variable. The `--api-key` flag takes precedence if both are set.

## Release workflow

Pass `--version` to cut a release. The tool will:

1. Look up the last release tag and validate that the new version is strictly greater (e.g. `1.2.0` > `1.1.3`)
2. Generate a dated changelog entry (`## [1.2.0] - 2026-02-22`)
3. Prepend it to `CHANGELOG.md` in the repo (creating the file with a standard header if it doesn't exist)
4. Commit `CHANGELOG.md` with the message `Release 1.2.0`
5. Create an annotated git tag pointing at that commit
6. Print the `git push` commands to finish

```bash
changelog-generator --api-key {ANTHROPIC_TOKEN} --version 1.2.0

# stderr output:
# info: last release tag: 1.1.3
# info: including full diff (87 lines changed)
# info: updated CHANGELOG.md
# info: committed CHANGELOG.md
# info: created tag 1.2.0
# next: git push && git push --tags
```

### First release

If the repo has no tags yet, the tool diffs the entire history and accepts any valid semver version:

```bash
changelog-generator --api-key {ANTHROPIC_TOKEN} --version 1.0.0
```

### Version validation

The tool enforces [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH` format) and rejects versions that are not strictly greater than the last tag:

```bash
changelog-generator --version 1.1.0  # error if last tag is 1.2.0
changelog-generator --version 1.2.0  # error if last tag is 1.2.0 (equal)
changelog-generator --version release-2  # error: not valid semver
```

## Preview mode

Run without `--version` to preview the changelog without writing anything or creating a tag:

```bash
# Preview to stdout
changelog-generator --api-key {ANTHROPIC_TOKEN}

# Save preview to a file
changelog-generator --api-key {ANTHROPIC_TOKEN} --output preview.md
```

## Diff strategy

By default, if the total lines changed is **≤ 2000**, the full diff is sent to the model for more accurate output. For larger changesets, only the `git diff --stat` summary is used. Adjust the threshold with `--max-diff`.

Diagnostic messages go to stderr; changelog content goes to stdout — so piping works cleanly:

```bash
changelog-generator --api-key {ANTHROPIC_TOKEN} | less
```
