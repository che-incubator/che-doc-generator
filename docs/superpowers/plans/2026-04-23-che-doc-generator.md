# che-doc-generator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go program that polls GitHub PRs for `@generate-che-doc` comments and invokes Claude Code CLI to generate documentation PRs against eclipse-che/che-docs.

**Architecture:** A long-running Go process with a polling loop that scans configured repos for trigger comments. Deduplication via GitHub emoji reactions. Doc generation dispatched asynchronously via Claude Code CLI subprocess, which uses che-mcp-server to manage DevWorkspaces.

**Tech Stack:** Go 1.22+, google/go-github v68, golang.org/x/oauth2, Claude Code CLI

---

## File Structure

```
che-doc-generator/
├── main.go          # Entry point: env config parsing, polling loop, signal handling, graceful shutdown
├── github.go        # GitHub API wrapper: list PRs, list/check comments, reactions, post comments
├── github_test.go   # Tests for GitHub API wrapper (mocked HTTP)
├── generator.go     # Claude Code CLI invocation: prompt assembly, subprocess exec, JSON output parsing
├── generator_test.go# Tests for prompt assembly and output parsing
├── Dockerfile       # Container image for K8s pod deployment
├── go.mod
└── go.sum
```

---

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /home/tolusha/projects/tolusha/che-doc-generator
go mod init github.com/tolusha/che-doc-generator
```

- [ ] **Step 2: Create minimal main.go**

```go
package main

import "fmt"

func main() {
	fmt.Println("che-doc-generator starting")
}
```

- [ ] **Step 3: Verify it compiles and runs**

Run: `go run main.go`
Expected: `che-doc-generator starting`

- [ ] **Step 4: Commit**

```bash
git add main.go go.mod
git commit -m "feat: initialize go module with minimal main"
```

---

### Task 2: Configuration Parsing

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Write the config parsing test**

Create `main_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestParseConfig_Defaults(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1,org/repo2")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.WatchRepos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.WatchRepos))
	}
	if cfg.WatchRepos[0] != "org/repo1" {
		t.Errorf("expected org/repo1, got %s", cfg.WatchRepos[0])
	}
	if cfg.WatchRepos[1] != "org/repo2" {
		t.Errorf("expected org/repo2, got %s", cfg.WatchRepos[1])
	}
	if cfg.PollInterval != 10*time.Minute {
		t.Errorf("expected 10m default, got %v", cfg.PollInterval)
	}
	if cfg.GenerationTimeout != 30*time.Minute {
		t.Errorf("expected 30m default, got %v", cfg.GenerationTimeout)
	}
	if cfg.MaxConcurrent != 1 {
		t.Errorf("expected 1 default, got %d", cfg.MaxConcurrent)
	}
}

func TestParseConfig_CustomValues(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1")
	t.Setenv("POLL_INTERVAL", "5m")
	t.Setenv("GENERATION_TIMEOUT", "1h")
	t.Setenv("MAX_CONCURRENT", "3")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("expected 5m, got %v", cfg.PollInterval)
	}
	if cfg.GenerationTimeout != time.Hour {
		t.Errorf("expected 1h, got %v", cfg.GenerationTimeout)
	}
	if cfg.MaxConcurrent != 3 {
		t.Errorf("expected 3, got %d", cfg.MaxConcurrent)
	}
}

func TestParseConfig_MissingRepos(t *testing.T) {
	_, err := parseConfig()
	if err == nil {
		t.Fatal("expected error when WATCH_REPOS is not set")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestParseConfig`
Expected: FAIL — `parseConfig` not defined

- [ ] **Step 3: Implement config parsing in main.go**

Replace `main.go` with:

```go
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WatchRepos        []string
	PollInterval      time.Duration
	GenerationTimeout time.Duration
	MaxConcurrent     int
}

func parseConfig() (Config, error) {
	repos := os.Getenv("WATCH_REPOS")
	if repos == "" {
		return Config{}, fmt.Errorf("WATCH_REPOS environment variable is required")
	}

	pollInterval := 10 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
		}
		pollInterval = d
	}

	genTimeout := 30 * time.Minute
	if v := os.Getenv("GENERATION_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid GENERATION_TIMEOUT: %w", err)
		}
		genTimeout = d
	}

	maxConcurrent := 1
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MAX_CONCURRENT: %w", err)
		}
		maxConcurrent = n
	}

	var repoList []string
	for _, r := range strings.Split(repos, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			repoList = append(repoList, r)
		}
	}

	return Config{
		WatchRepos:        repoList,
		PollInterval:      pollInterval,
		GenerationTimeout: genTimeout,
		MaxConcurrent:     maxConcurrent,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	log.Printf("watching repos: %v, poll interval: %v", cfg.WatchRepos, cfg.PollInterval)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestParseConfig`
Expected: PASS (all 3 tests)

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: add environment variable config parsing"
```

---

### Task 3: GitHub API Client — List PRs and Comments

**Files:**
- Create: `github.go`
- Create: `github_test.go`

- [ ] **Step 1: Add go-github dependency**

Run:
```bash
cd /home/tolusha/projects/tolusha/che-doc-generator
go get github.com/google/go-github/v68@latest
go get golang.org/x/oauth2
```

- [ ] **Step 2: Write the test for finding trigger comments**

Create `github_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v68/github"
)

func TestFindTriggerComments_FindsUnprocessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*github.PullRequest{
			{Number: github.Ptr(1)},
		}
		json.NewEncoder(w).Encode(prs)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*github.IssueComment{
			{
				ID:   github.Ptr(int64(100)),
				Body: github.Ptr("please @generate-che-doc for this PR"),
			},
			{
				ID:   github.Ptr(int64(101)),
				Body: github.Ptr("just a regular comment"),
			},
		}
		json.NewEncoder(w).Encode(comments)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*github.Reaction{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newGitHubClient("fake-token", srv.URL)
	triggers, err := client.FindTriggerComments("org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].CommentID != 100 {
		t.Errorf("expected comment ID 100, got %d", triggers[0].CommentID)
	}
	if triggers[0].PRNumber != 1 {
		t.Errorf("expected PR number 1, got %d", triggers[0].PRNumber)
	}
}

func TestFindTriggerComments_SkipsProcessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*github.PullRequest{
			{Number: github.Ptr(1)},
		}
		json.NewEncoder(w).Encode(prs)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*github.IssueComment{
			{
				ID:   github.Ptr(int64(100)),
				Body: github.Ptr("@generate-che-doc"),
			},
		}
		json.NewEncoder(w).Encode(comments)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		reactions := []*github.Reaction{
			{Content: github.Ptr("eyes")},
		}
		json.NewEncoder(w).Encode(reactions)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newGitHubClient("fake-token", srv.URL)
	triggers, err := client.FindTriggerComments("org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers (already processed), got %d", len(triggers))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test -v -run TestFindTriggerComments`
Expected: FAIL — types and functions not defined

- [ ] **Step 4: Implement the GitHub client**

Create `github.go`:

```go
package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

const triggerPhrase = "@generate-che-doc"

type TriggerComment struct {
	Owner     string
	Repo      string
	PRNumber  int
	CommentID int64
	PRURL     string
}

type GitHubClient struct {
	client *github.Client
}

func newGitHubClient(token string, baseURL ...string) *GitHubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	client := github.NewClient(httpClient)
	if len(baseURL) > 0 && baseURL[0] != "" {
		client, _ = client.WithEnterpriseURLs(baseURL[0], baseURL[0])
	}

	return &GitHubClient{client: client}
}

func (g *GitHubClient) FindTriggerComments(owner, repo string) ([]TriggerComment, error) {
	ctx := context.Background()
	var triggers []TriggerComment

	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return nil, err
	}

	for _, pr := range prs {
		comments, _, err := g.client.Issues.ListComments(ctx, owner, repo, pr.GetNumber(), nil)
		if err != nil {
			return nil, err
		}

		for _, comment := range comments {
			if !strings.Contains(comment.GetBody(), triggerPhrase) {
				continue
			}

			processed, err := g.hasEyesReaction(ctx, owner, repo, comment.GetID())
			if err != nil {
				return nil, err
			}
			if processed {
				continue
			}

			triggers = append(triggers, TriggerComment{
				Owner:     owner,
				Repo:      repo,
				PRNumber:  pr.GetNumber(),
				CommentID: comment.GetID(),
				PRURL:     pr.GetHTMLURL(),
			})
		}
	}

	return triggers, nil
}

func (g *GitHubClient) hasEyesReaction(ctx context.Context, owner, repo string, commentID int64) (bool, error) {
	reactions, _, err := g.client.Reactions.ListIssueCommentReactions(ctx, owner, repo, commentID, nil)
	if err != nil {
		return false, err
	}
	for _, r := range reactions {
		if r.GetContent() == "eyes" {
			return true, nil
		}
	}
	return false, nil
}

func (g *GitHubClient) AddEyesReaction(ctx context.Context, owner, repo string, commentID int64) error {
	_, _, err := g.client.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, "eyes")
	return err
}

func (g *GitHubClient) PostComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	_, _, err := g.client.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{
		Body: github.Ptr(body),
	})
	return err
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v -run TestFindTriggerComments`
Expected: PASS (both tests)

- [ ] **Step 6: Run go mod tidy**

Run: `go mod tidy`

- [ ] **Step 7: Commit**

```bash
git add github.go github_test.go go.mod go.sum
git commit -m "feat: add GitHub client with trigger comment detection"
```

---

### Task 4: Claude Code Generator — Prompt Assembly and Output Parsing

**Files:**
- Create: `generator.go`
- Create: `generator_test.go`

- [ ] **Step 1: Write tests for prompt building and output parsing**

Create `generator_test.go`:

```go
package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPrompt(t *testing.T) {
	gen := &Generator{Timeout: 30 * time.Minute}
	prompt := gen.BuildPrompt("https://github.com/org/repo/pull/42")

	if !strings.Contains(prompt, "https://github.com/org/repo/pull/42") {
		t.Error("prompt should contain the PR URL")
	}
	if !strings.Contains(prompt, "che-mcp-server") {
		t.Error("prompt should reference che-mcp-server")
	}
	if !strings.Contains(prompt, "che-docs.git") {
		t.Error("prompt should reference che-docs repo")
	}
	if !strings.Contains(prompt, "tolusha/claude-plugins") {
		t.Error("prompt should reference tolusha/claude-plugins")
	}
	if !strings.Contains(prompt, "redhat-docs-agent-tools") {
		t.Error("prompt should reference redhat-docs-agent-tools marketplace")
	}
	if !strings.Contains(prompt, "Delete the DevWorkspace") {
		t.Error("prompt should include workspace cleanup step")
	}
}

func TestParseDocPRURL_Success(t *testing.T) {
	output := `{"result": "I created the documentation PR at https://github.com/eclipse-che/che-docs/pull/99. The workspace has been deleted."}`

	url, err := parseDocPRURL(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://github.com/eclipse-che/che-docs/pull/99" {
		t.Errorf("expected che-docs PR URL, got %s", url)
	}
}

func TestParseDocPRURL_NoPR(t *testing.T) {
	output := `{"result": "I tried but failed to generate docs."}`

	_, err := parseDocPRURL(output)
	if err == nil {
		t.Fatal("expected error when no PR URL in output")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestBuildPrompt|TestParseDocPRURL"`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement the generator**

Create `generator.go`:

```go
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
2. In the DevWorkspace, clone git@github.com:eclipse-che/che-docs.git
3. Install plugins: /plugin install https://github.com/tolusha/claude-plugins
4. Add marketplace: /plugin marketplace add https://github.com/redhat-documentation/redhat-docs-agent-tools.git
5. Install the plugin from redhat-docs-agent-tools marketplace.
6. Using the che-docs-from-pr skill, generate documentation for this PR: %s
7. Return ONLY the created documentation PR URL on a line by itself.
8. Delete the DevWorkspace using che-mcp-server.`, prURL)
}

func (g *Generator) Run(prURL string) (string, error) {
	prompt := g.BuildPrompt(prURL)

	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
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

var prURLPattern = regexp.MustCompile(`https://github\.com/[^\s"]+/pull/\d+`)

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}
	return match, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestBuildPrompt|TestParseDocPRURL"`
Expected: PASS (all 3 tests)

- [ ] **Step 5: Commit**

```bash
git add generator.go generator_test.go
git commit -m "feat: add Claude Code prompt assembly and output parsing"
```

---

### Task 5: Polling Loop with Async Dispatch and Signal Handling

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Write test for repo string splitting**

Add to `main_test.go`:

```go
func TestParseConfig_TrimsWhitespace(t *testing.T) {
	t.Setenv("WATCH_REPOS", " org/repo1 , org/repo2 ")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.WatchRepos[0] != "org/repo1" {
		t.Errorf("expected trimmed org/repo1, got %q", cfg.WatchRepos[0])
	}
	if cfg.WatchRepos[1] != "org/repo2" {
		t.Errorf("expected trimmed org/repo2, got %q", cfg.WatchRepos[1])
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -v -run TestParseConfig_TrimsWhitespace`
Expected: PASS (already handled in parseConfig)

- [ ] **Step 3: Implement the full main.go with polling loop**

Replace `main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	WatchRepos        []string
	PollInterval      time.Duration
	GenerationTimeout time.Duration
	MaxConcurrent     int
}

func parseConfig() (Config, error) {
	repos := os.Getenv("WATCH_REPOS")
	if repos == "" {
		return Config{}, fmt.Errorf("WATCH_REPOS environment variable is required")
	}

	pollInterval := 10 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
		}
		pollInterval = d
	}

	genTimeout := 30 * time.Minute
	if v := os.Getenv("GENERATION_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid GENERATION_TIMEOUT: %w", err)
		}
		genTimeout = d
	}

	maxConcurrent := 1
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MAX_CONCURRENT: %w", err)
		}
		maxConcurrent = n
	}

	var repoList []string
	for _, r := range strings.Split(repos, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			repoList = append(repoList, r)
		}
	}

	return Config{
		WatchRepos:        repoList,
		PollInterval:      pollInterval,
		GenerationTimeout: genTimeout,
		MaxConcurrent:     maxConcurrent,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	ghClient := newGitHubClient(ghToken)
	gen := &Generator{Timeout: cfg.GenerationTimeout}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sem := make(chan struct{}, cfg.MaxConcurrent)
	var wg sync.WaitGroup

	log.Printf("starting che-doc-generator: watching %v, poll every %v", cfg.WatchRepos, cfg.PollInterval)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	poll := func() {
		for _, repo := range cfg.WatchRepos {
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) != 2 {
				log.Printf("invalid repo format: %s (expected owner/repo)", repo)
				continue
			}
			owner, repoName := parts[0], parts[1]

			triggers, err := ghClient.FindTriggerComments(owner, repoName)
			if err != nil {
				log.Printf("error scanning %s: %v", repo, err)
				continue
			}

			for _, trigger := range triggers {
				if err := ghClient.AddEyesReaction(ctx, trigger.Owner, trigger.Repo, trigger.CommentID); err != nil {
					log.Printf("error adding reaction to comment %d: %v", trigger.CommentID, err)
					continue
				}

				wg.Add(1)
				sem <- struct{}{}
				go func(t TriggerComment) {
					defer wg.Done()
					defer func() { <-sem }()

					log.Printf("generating docs for %s/%s#%d", t.Owner, t.Repo, t.PRNumber)
					docPRURL, err := gen.Run(t.PRURL)
					if err != nil {
						log.Printf("generation failed for %s/%s#%d: %v", t.Owner, t.Repo, t.PRNumber, err)
						msg := fmt.Sprintf("Failed to generate documentation: %v", err)
						if commentErr := ghClient.PostComment(ctx, t.Owner, t.Repo, t.PRNumber, msg); commentErr != nil {
							log.Printf("error posting failure comment: %v", commentErr)
						}
						return
					}

					log.Printf("docs generated for %s/%s#%d: %s", t.Owner, t.Repo, t.PRNumber, docPRURL)
					msg := fmt.Sprintf("Documentation PR created: %s", docPRURL)
					if commentErr := ghClient.PostComment(ctx, t.Owner, t.Repo, t.PRNumber, msg); commentErr != nil {
						log.Printf("error posting success comment: %v", commentErr)
					}
				}(trigger)
			}
		}
	}

	poll()

	for {
		select {
		case <-ticker.C:
			poll()
		case <-sigCh:
			log.Println("shutdown signal received, waiting for in-progress generations...")
			cancel()
			wg.Wait()
			log.Println("shutdown complete")
			return
		}
	}
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Run all tests**

Run: `go test -v ./...`
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: add polling loop with async dispatch and graceful shutdown"
```

---

### Task 6: Dockerfile

**Files:**
- Create: `Dockerfile`

- [ ] **Step 1: Create the Dockerfile**

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o che-doc-generator .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl git bash \
    && curl -fsSL https://claude.ai/install.sh | bash

COPY --from=builder /app/che-doc-generator /usr/local/bin/che-doc-generator

ENTRYPOINT ["che-doc-generator"]
```

- [ ] **Step 2: Verify the Docker build (dry run)**

Run: `cd /home/tolusha/projects/tolusha/che-doc-generator && docker build --check .` or just verify the Dockerfile syntax is correct:
Run: `cat Dockerfile`
Expected: valid Dockerfile with multi-stage build

- [ ] **Step 3: Commit**

```bash
git add Dockerfile
git commit -m "feat: add Dockerfile for K8s pod deployment"
```

---

### Task 7: Integration Smoke Test

**Files:**
- Modify: `generator_test.go`

- [ ] **Step 1: Add test for prompt containing all required steps**

Add to `generator_test.go`:

```go
func TestBuildPrompt_ContainsAllSteps(t *testing.T) {
	gen := &Generator{Timeout: 30 * time.Minute}
	prompt := gen.BuildPrompt("https://github.com/org/repo/pull/1")

	required := []string{
		"start a DevWorkspace",
		"clone git@github.com:eclipse-che/che-docs.git",
		"/plugin install https://github.com/tolusha/claude-plugins",
		"/plugin marketplace add https://github.com/redhat-documentation/redhat-docs-agent-tools.git",
		"redhat-docs-agent-tools marketplace",
		"che-docs-from-pr",
		"Delete the DevWorkspace",
	}

	for _, r := range required {
		if !strings.Contains(prompt, r) {
			t.Errorf("prompt missing required content: %q", r)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -v -run TestBuildPrompt_ContainsAllSteps`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `go test -v -count=1 ./...`
Expected: all tests PASS

- [ ] **Step 4: Commit**

```bash
git add generator_test.go
git commit -m "test: add integration smoke test for prompt completeness"
```

---

### Task 8: Final Cleanup and Verify

**Files:**
- All files

- [ ] **Step 1: Run go vet**

Run: `cd /home/tolusha/projects/tolusha/che-doc-generator && go vet ./...`
Expected: no issues

- [ ] **Step 2: Run go mod tidy**

Run: `go mod tidy`
Expected: clean module

- [ ] **Step 3: Run full test suite one final time**

Run: `go test -v -count=1 ./...`
Expected: all tests PASS

- [ ] **Step 4: Commit any tidying changes**

```bash
git add -A
git commit -m "chore: tidy go module and final cleanup"
```
