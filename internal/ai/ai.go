package ai

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Request holds all parameters for changelog generation.
type Request struct {
	APIKey        string
	Model         string
	From          string
	To            string
	VersionHeader string // e.g. "## [v1.2.0] - 2026-02-22" or "## [Unreleased]"
	Commits       []string
	DiffStat      string
	FullDiff      string // empty means stat-only mode
	Out           io.Writer
}

const systemPrompt = `You are a technical writer that generates git release changelogs in Keep a Changelog format (https://keepachangelog.com/).

Rules:
- Use the exact version header provided in the request
- Use these H3 sections (only include non-empty ones): ### Added, ### Changed, ### Deprecated, ### Removed, ### Fixed, ### Security
- Each item is a bullet point written in past tense (e.g., "Added support for X", "Fixed bug in Y")
- Be concise and factual — do not invent or hallucinate changes not present in the provided information
- No preamble, commentary, or text outside the changelog structure
- Output only the changelog markdown, nothing else`

// GenerateChangelog streams a Keep a Changelog formatted entry to req.Out.
func GenerateChangelog(ctx context.Context, req Request) error {
	client := anthropic.NewClient(option.WithAPIKey(req.APIKey))

	var sb strings.Builder
	sb.WriteString("Generate a changelog for the changes from `")
	sb.WriteString(req.From)
	sb.WriteString("` to `")
	sb.WriteString(req.To)
	sb.WriteString("`.\n\nVersion header to use: ")
	sb.WriteString(req.VersionHeader)
	sb.WriteString("\n\n")

	if len(req.Commits) > 0 {
		sb.WriteString("## Commit Messages\n\n")
		for _, c := range req.Commits {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if req.DiffStat != "" {
		sb.WriteString("## Diff Statistics\n\n```\n")
		sb.WriteString(req.DiffStat)
		sb.WriteString("\n```\n\n")
	}

	if req.FullDiff != "" {
		sb.WriteString("## Full Diff\n\n```diff\n")
		sb.WriteString(req.FullDiff)
		sb.WriteString("\n```\n")
	}

	stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
		},
	})

	for stream.Next() {
		event := stream.Current()
		switch ev := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch d := ev.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				if _, err := fmt.Fprint(req.Out, d.Text); err != nil {
					return err
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("streaming error: %w", err)
	}

	// Ensure trailing newline.
	_, _ = fmt.Fprintln(req.Out)
	return nil
}
