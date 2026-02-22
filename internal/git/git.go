package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// emptyTreeSHA is a well-known git object representing an empty tree,
// used to diff from "nothing" when there is no prior commit to compare against.
const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// LastReleaseTag returns the most recent tag reachable from HEAD.
// Returns ("", nil) when the repository has no tags at all.
func LastReleaseTag(repoPath string) (string, error) {
	out, err := runGit(repoPath, "tag", "-l")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "", nil // no tags exist yet
	}
	return runGit(repoPath, "describe", "--tags", "--abbrev=0")
}

// CommitLog returns one-line commit messages from from..to, excluding merges.
// When from is empty, all commits reachable from to are returned.
func CommitLog(repoPath, from, to string) ([]string, error) {
	var out string
	var err error
	if from == "" {
		out, err = runGit(repoPath, "log", "--oneline", "--no-merges", to)
	} else {
		out, err = runGit(repoPath, "log", "--oneline", "--no-merges", from+".."+to)
	}
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// DiffStat returns the --stat output for from..to.
// When from is empty, diffs from the empty tree (i.e. all content is "added").
func DiffStat(repoPath, from, to string) (string, error) {
	if from == "" {
		from = emptyTreeSHA
	}
	return runGit(repoPath, "diff", "--stat", from+".."+to)
}

// FullDiff returns the full diff for from..to without ANSI color codes.
// When from is empty, diffs from the empty tree.
func FullDiff(repoPath, from, to string) (string, error) {
	if from == "" {
		from = emptyTreeSHA
	}
	return runGit(repoPath, "diff", "--no-color", from+".."+to)
}

// Commit stages the given files and creates a commit with the provided message.
func Commit(repoPath, message string, files ...string) error {
	addArgs := append([]string{"add"}, files...)
	if _, err := runGit(repoPath, addArgs...); err != nil {
		return fmt.Errorf("staging files: %w", err)
	}
	if _, err := runGit(repoPath, "commit", "-m", message); err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}
	return nil
}

// CreateTag creates an annotated git tag at HEAD.
func CreateTag(repoPath, tag, message string) error {
	_, err := runGit(repoPath, "tag", "-a", tag, "-m", message)
	if err != nil {
		return fmt.Errorf("creating tag %s: %w", tag, err)
	}
	return nil
}

var changedLinesRe = regexp.MustCompile(`(\d+) insertion|(\d+) deletion`)

// ParseTotalChangedLines extracts the total number of inserted + deleted lines
// from a "git diff --stat" summary line.
func ParseTotalChangedLines(stat string) int {
	matches := changedLinesRe.FindAllStringSubmatch(stat, -1)
	total := 0
	for _, m := range matches {
		for _, g := range m[1:] {
			if g != "" {
				n, _ := strconv.Atoi(g)
				total += n
			}
		}
	}
	return total
}
