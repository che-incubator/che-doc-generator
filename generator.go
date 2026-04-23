package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"
)

type Generator struct {
	Timeout time.Duration
}

func (g *Generator) BuildPrompt(prURL string) string {
	return fmt.Sprintf(`You are an automated documentation generator. Follow these steps exactly:

1. Using che-mcp-server, start a DevWorkspace.
2. Inject Claude code in the DevWorkspace. 
2. In the DevWorkspace, clone git@github.com:eclipse-che/che-docs.git
3. Add marketplace: /plugin marketplace add tolusha/claude-plugins and install plugin /plugin install che-docs-from-pr@claude-plugins
4. Add marketplace: /plugin marketplace add redhat-documentation/redhat-docs-agent-tools and install the plugin /plugin install cqa-tools@redhat-docs-agent-tools
5. Using the che-docs-from-pr skill, generate documentation for this PR: %s
6. Return ONLY the created documentation PR URL on a line by itself.
7. Delete the DevWorkspace using che-mcp-server.`, prURL)
}

func (g *Generator) Run(ctx context.Context, prURL string) (string, error) {
	prompt := g.BuildPrompt(prURL)

	ctx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"-p", prompt,
		"--output-format", "json",
	)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out after %v", g.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("claude exited with error: %w\noutput: %s", err, string(output))
	}

	return parseDocPRURL(string(output))
}

var prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/pull/\d+`)

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}
	return match, nil
}
