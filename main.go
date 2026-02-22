package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nealwashere/ai-changelog-generator/internal/ai"
	"github.com/nealwashere/ai-changelog-generator/internal/git"
)

const defaultModel = "claude-sonnet-4-6"

type config struct {
	Repo    string
	Model   string
	Output  string
	Version string
	MaxDiff int
	APIKey  string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var cfg config

	flag.StringVar(&cfg.Repo, "repo", ".", "Path to git repo")
	flag.StringVar(&cfg.Repo, "r", ".", "Path to git repo (shorthand)")
	flag.StringVar(&cfg.Model, "model", defaultModel, "Anthropic model ID")
	flag.StringVar(&cfg.Model, "m", defaultModel, "Anthropic model ID (shorthand)")
	flag.StringVar(&cfg.Output, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&cfg.Output, "o", "", "Output file path (shorthand)")
	flag.StringVar(&cfg.Version, "version", "", "Release version (e.g. v1.2.0); updates CHANGELOG.md and creates a git tag")
	flag.StringVar(&cfg.Version, "v", "", "Release version (shorthand)")
	flag.IntVar(&cfg.MaxDiff, "max-diff", 2000, "Line threshold for full diff inclusion")
	flag.StringVar(&cfg.APIKey, "api-key", "", "Anthropic API key (default: $ANTHROPIC_API_KEY)")
	flag.Parse()

	// Resolve API key: flag > env var.
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("no API key provided; set --api-key or $ANTHROPIC_API_KEY")
	}

	// Validate repo path.
	if _, err := os.Stat(cfg.Repo); err != nil {
		return fmt.Errorf("repo path %q not accessible: %w", cfg.Repo, err)
	}

	// Get the last release tag. Returns "" when no tags exist yet.
	lastTag, err := git.LastReleaseTag(cfg.Repo)
	if err != nil {
		return fmt.Errorf("getting last release tag: %w", err)
	}

	if lastTag == "" {
		fmt.Fprintln(os.Stderr, "info: no prior release tags found — will diff entire history")
	} else {
		fmt.Fprintf(os.Stderr, "info: last release tag: %s\n", lastTag)
	}

	// Validate the requested version against the last tag.
	if cfg.Version != "" {
		if err := validateNewVersion(cfg.Version, lastTag); err != nil {
			return err
		}
	}

	// fromGit is empty when there are no prior tags (git functions handle this).
	// fromDesc is a human-readable label used in the AI prompt.
	fromGit := lastTag
	fromDesc := lastTag
	if lastTag == "" {
		fromDesc = "the beginning of the repository"
	}

	// Gather git data.
	commits, err := git.CommitLog(cfg.Repo, fromGit, "HEAD")
	if err != nil {
		return fmt.Errorf("getting commit log: %w", err)
	}

	stat, err := git.DiffStat(cfg.Repo, fromGit, "HEAD")
	if err != nil {
		return fmt.Errorf("getting diff stat: %w", err)
	}

	// Decide diff strategy.
	var fullDiff string
	totalChanged := git.ParseTotalChangedLines(stat)
	if totalChanged <= cfg.MaxDiff {
		fullDiff, err = git.FullDiff(cfg.Repo, fromGit, "HEAD")
		if err != nil {
			return fmt.Errorf("getting full diff: %w", err)
		}
		fmt.Fprintf(os.Stderr, "info: including full diff (%d lines changed)\n", totalChanged)
	} else {
		fmt.Fprintf(os.Stderr, "info: stat-only mode (%d lines changed, threshold %d)\n", totalChanged, cfg.MaxDiff)
	}

	// Build the version header the AI will use.
	versionHeader := "## [Unreleased]"
	if cfg.Version != "" {
		versionHeader = fmt.Sprintf("## [%s] - %s", cfg.Version, time.Now().Format("2006-01-02"))
	}

	req := ai.Request{
		APIKey:        cfg.APIKey,
		Model:         cfg.Model,
		From:          fromDesc,
		To:            "HEAD",
		VersionHeader: versionHeader,
		Commits:       commits,
		DiffStat:      stat,
		FullDiff:      fullDiff,
	}

	if cfg.Version != "" {
		// Release mode: buffer output → prepend to CHANGELOG.md → create tag.
		var buf bytes.Buffer
		req.Out = &buf
		if err := ai.GenerateChangelog(context.Background(), req); err != nil {
			return err
		}

		changelogPath := filepath.Join(cfg.Repo, "CHANGELOG.md")
		if cfg.Output != "" {
			changelogPath = cfg.Output
		}
		if err := updateChangelogFile(changelogPath, buf.String()); err != nil {
			return fmt.Errorf("updating %s: %w", changelogPath, err)
		}
		fmt.Fprintf(os.Stderr, "info: updated %s\n", changelogPath)

		if err := git.Commit(cfg.Repo, "Release "+cfg.Version, changelogPath); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "info: committed %s\n", changelogPath)

		if err := git.CreateTag(cfg.Repo, cfg.Version, "Release "+cfg.Version); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "info: created tag %s\n", cfg.Version)
		fmt.Fprintf(os.Stderr, "next: git push && git push --tags\n")
		return nil
	}

	// Preview mode: stream directly to stdout or --output file.
	var out io.Writer = os.Stdout
	if cfg.Output != "" {
		f, err := os.Create(cfg.Output)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		defer f.Close()
		out = f
	}
	req.Out = out
	return ai.GenerateChangelog(context.Background(), req)
}

// semver holds a parsed semantic version.
type semver struct{ major, minor, patch int }

func parseSemver(v string) (semver, error) {
	stripped := strings.TrimPrefix(v, "v")
	parts := strings.SplitN(stripped, ".", 3)
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("version %q must be in vMAJOR.MINOR.PATCH format (e.g. v1.2.0)", v)
	}
	var sv semver
	var err error
	if sv.major, err = strconv.Atoi(parts[0]); err != nil {
		return semver{}, fmt.Errorf("version %q: invalid major component", v)
	}
	if sv.minor, err = strconv.Atoi(parts[1]); err != nil {
		return semver{}, fmt.Errorf("version %q: invalid minor component", v)
	}
	if sv.patch, err = strconv.Atoi(parts[2]); err != nil {
		return semver{}, fmt.Errorf("version %q: invalid patch component", v)
	}
	return sv, nil
}

func (a semver) greaterThan(b semver) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	return a.patch > b.patch
}

// validateNewVersion ensures newVersion is valid semver and strictly greater
// than lastTag (if one exists).
func validateNewVersion(newVersion, lastTag string) error {
	newSV, err := parseSemver(newVersion)
	if err != nil {
		return err
	}
	if lastTag == "" {
		return nil // first release — any valid semver is fine
	}
	lastSV, err := parseSemver(lastTag)
	if err != nil {
		return fmt.Errorf("last tag %q is not valid semver; cannot compare versions", lastTag)
	}
	if !newSV.greaterThan(lastSV) {
		return fmt.Errorf("version %s must be greater than the last release tag %s", newVersion, lastTag)
	}
	return nil
}

// updateChangelogFile prepends entry to the Keep a Changelog file at path,
// creating the file with a standard header if it does not yet exist.
func updateChangelogFile(path, entry string) error {
	const fileHeader = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\nThe format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),\nand this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n"

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	entry = strings.TrimRight(entry, "\n")

	var result string
	if len(existing) == 0 {
		result = fileHeader + "\n" + entry + "\n"
	} else {
		content := string(existing)
		// Find the first "## [" section to insert before.
		idx := strings.Index(content, "\n## [")
		if idx == -1 {
			result = strings.TrimRight(content, "\n") + "\n\n" + entry + "\n"
		} else {
			before := strings.TrimRight(content[:idx], "\n")
			after := content[idx+1:] // starts at "## ["
			result = before + "\n\n" + entry + "\n\n" + after
			if !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
		}
	}

	return os.WriteFile(path, []byte(result), 0644)
}
