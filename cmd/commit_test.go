// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

func TestCommit_NotBoundReturnsError(t *testing.T) {
	resetCommitFlags()
	activeConfig = &config.Config{
		Local: nil,
	}

	err := commitCmd.RunE(commitCmd, []string{})
	if err == nil {
		t.Fatal("expected error when project is not bound, got nil")
	}
}

func TestCommit_WithFlagsAndMockAPI(t *testing.T) {
	resetCommitFlags()

	var receivedReq api.CreateRecordRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-123/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&receivedReq)
			now := time.Now()
			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
				Data: api.Record{
					Id:          "record-001",
					ProjectId:   "proj-123",
					Title:       receivedReq.Title,
					Description: &receivedReq.Description,
					MilestoneId: &receivedReq.MilestoneID,
					CommittedAt: now,
					CreatedAt:   now,
				},
			})
			return
		}
		http.NotFound(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{
			Endpoint: server.URL,
			Token:    "mock-token",
		},
		Local: &config.LocalConfig{
			ProjectId:          "proj-123",
			ProjectSlug:        "my-proj",
			DefaultMilestoneId: "ms-default",
		},
	}
	apiClient = api.NewClient(activeConfig)

	commitTitle = "Implemented user authentication"
	commitDescription = "Added OAuth2 support and unit tests"
	commitMilestone = "ms-custom"
	commitTime = "2026-10-24T12:00:00Z"
	flagJSON = true
	defer func() { flagJSON = false; resetCommitFlags() }()

	err := commitCmd.RunE(commitCmd, []string{})
	if err != nil {
		t.Fatalf("commitCmd failed: %v", err)
	}

	if receivedReq.Title != "Implemented user authentication" {
		t.Errorf("expected title 'Implemented user authentication', got %q", receivedReq.Title)
	}
	if receivedReq.Description != "Added OAuth2 support and unit tests" {
		t.Errorf("expected description 'Added OAuth2 support and unit tests', got %q", receivedReq.Description)
	}
	if receivedReq.MilestoneID != "ms-custom" {
		t.Errorf("expected milestone 'ms-custom', got %q", receivedReq.MilestoneID)
	}
	if receivedReq.CommittedAt == nil {
		t.Fatal("expected non-nil CommittedAt timestamp")
	}
}

func TestCommit_FromGit(t *testing.T) {
	resetCommitFlags()

	// Setup a real git repo in temp dir
	dir := t.TempDir()
	runGit := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v: %s", args, err, string(out))
		}
	}
	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "test@example.com")
	runGit("commit", "--allow-empty", "-m", "feat: git commit subject\n\nGit commit body description")

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(dir)

	var receivedReq api.CreateRecordRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-git/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:          "record-git-01",
				Title:       receivedReq.Title,
				CommittedAt: time.Now(),
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-git", ProjectSlug: "git-proj"},
	}
	apiClient = api.NewClient(activeConfig)

	commitFromGit = true
	defer func() { resetCommitFlags() }()

	cmd := commitCmd
	cmd.SetContext(context.Background())
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("commitCmd with --from-git failed: %v", err)
	}

	if receivedReq.Title != "feat: git commit subject" {
		t.Errorf("expected title from git 'feat: git commit subject', got %q", receivedReq.Title)
	}
	if receivedReq.Description != "Git commit body description" {
		t.Errorf("expected description 'Git commit body description', got %q", receivedReq.Description)
	}
}

func TestParseCommitTimestamp(t *testing.T) {
	// 1. Empty string
	res, err := parseCommitTimestamp("")
	if err != nil || res != nil {
		t.Errorf("expected nil for empty timestamp, got %v, %v", res, err)
	}

	// 2. RFC3339
	res, err = parseCommitTimestamp("2026-10-24T15:00:00Z")
	if err != nil || res == nil {
		t.Fatalf("expected valid parsed RFC3339, got err: %v", err)
	}

	// 3. YYYY-MM-DD HH:MM:SS
	res, err = parseCommitTimestamp("2026-10-24 15:00:00")
	if err != nil || res == nil {
		t.Fatalf("expected valid parsed local date, got err: %v", err)
	}

	// 4. Invalid
	_, err = parseCommitTimestamp("invalid-date-string")
	if err == nil {
		t.Errorf("expected error for invalid date, got nil")
	}
}

func TestCommit_ExplicitMilestoneNone(t *testing.T) {
	resetCommitFlags()

	var receivedReq api.CreateRecordRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-ms-none/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:          "record-ms-none",
				Title:       receivedReq.Title,
				CommittedAt: time.Now(),
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:          "proj-ms-none",
			ProjectSlug:        "ms-none-proj",
			DefaultMilestoneId: "default-ms-to-override",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// Simulate user explicitly passing --milestone "none"
	commitTitle = "Commit without milestone"
	commitMilestone = "none"
	defer func() { resetCommitFlags() }()

	cmd := commitCmd
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("commitCmd failed: %v", err)
	}

	if receivedReq.MilestoneID != "" {
		t.Errorf("expected empty milestone ID when 'none' specified, got %q", receivedReq.MilestoneID)
	}
}

func TestPromptCommitDetails(t *testing.T) {
	resetCommitFlags()
	defer resetCommitFlags()

	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(tmpDir)

	cmd := commitCmd
	cmd.SetContext(context.Background())

	// 1. Success case: user enters title and multiline description
	ui.SetReader(strings.NewReader("Implemented OAuth login\nAdded token exchange\nHandled refresh\n\n"))
	commitTitle = ""
	commitDescription = ""

	if err := promptCommitDetails(cmd); err != nil {
		t.Fatalf("promptCommitDetails failed: %v", err)
	}
	if commitTitle != "Implemented OAuth login" {
		t.Errorf("expected title 'Implemented OAuth login', got %q", commitTitle)
	}
	if !strings.Contains(commitDescription, "Added token exchange") {
		t.Errorf("expected description to contain 'Added token exchange', got %q", commitDescription)
	}

	// 2. Empty title returns error
	ui.SetReader(strings.NewReader("\n\n"))
	commitTitle = ""
	commitDescription = ""

	err := promptCommitDetails(cmd)
	if err == nil {
		t.Fatalf("expected error for empty commit title, got nil")
	}
}

func TestResolveMilestone_Interactive(t *testing.T) {
	resetCommitFlags()
	defer resetCommitFlags()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-ms-inter/milestones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Milestone]{
			Data: []api.Milestone{
				{Id: "ms-alpha", Title: "Milestone Alpha", Type: "ROADMAP", Status: "IN_PROGRESS"},
				{Id: "ms-beta", Title: "Milestone Beta", Type: "FUTURE_MILESTONE", Status: "PLANNED"},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-ms-inter"},
	}
	apiClient = api.NewClient(activeConfig)

	cmd := commitCmd
	cmd.SetContext(context.Background())
	commitTitle = "Valid progress"

	// User selects item 1 (Milestone Alpha, index 1 in options: 0 is None, 1 is Alpha, 2 is Beta)
	ui.SetReader(strings.NewReader("2\n"))
	selected := resolveMilestone(cmd, activeConfig.Local, true)
	if selected != "ms-alpha" {
		t.Errorf("expected selected milestone 'ms-alpha', got %q", selected)
	}
}

func TestPromptCommitDetails_GitSuggestionAccepted(t *testing.T) {
	resetCommitFlags()
	defer resetCommitFlags()

	dir := t.TempDir()
	runGit := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v: %s", args, err, string(out))
		}
	}
	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "test@example.com")
	runGit("commit", "--allow-empty", "-m", "fix: bug fixed\n\nFix body details")

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(dir)

	cmd := commitCmd
	cmd.SetContext(context.Background())

	// User confirms "y" to use git commit message
	ui.SetReader(strings.NewReader("y\n"))
	commitTitle = ""
	commitDescription = ""

	if err := promptCommitDetails(cmd); err != nil {
		t.Fatalf("promptCommitDetails failed: %v", err)
	}
	if commitTitle != "fix: bug fixed" {
		t.Errorf("expected title 'fix: bug fixed', got %q", commitTitle)
	}
	if commitDescription != "Fix body details" {
		t.Errorf("expected description 'Fix body details', got %q", commitDescription)
	}
}

func TestResolveFromGit_Errors(t *testing.T) {
	resetCommitFlags()
	defer resetCommitFlags()

	// 1. In a non-git directory
	nonGitDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(nonGitDir)

	cmd := commitCmd
	cmd.SetContext(context.Background())

	err := resolveFromGit(cmd)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' error, got %v", err)
	}

	// 2. In an empty git repo (no commits)
	emptyGitDir := t.TempDir()
	c := exec.Command("git", "init")
	c.Dir = emptyGitDir
	_ = c.Run()

	_ = os.Chdir(emptyGitDir)
	err = resolveFromGit(cmd)
	if err == nil || !strings.Contains(err.Error(), "no commits yet") {
		t.Errorf("expected 'no commits yet' error, got %v", err)
	}
}

func TestPrintCommitSuccess_Branches(t *testing.T) {
	desc := "Multi\nLine\nDescription"
	ms := "ms-test-123"

	// 1. With Description and Milestone
	recFull := &api.Record{
		Id:          "rec-full-01",
		Title:       "Full Record",
		Description: &desc,
		MilestoneId: &ms,
		CommittedAt: time.Now(),
	}
	printCommitSuccess(recFull)

	// 2. Without Description and Milestone
	recEmpty := &api.Record{
		Id:          "rec-empty-01",
		Title:       "Empty Record",
		CommittedAt: time.Now(),
	}
	printCommitSuccess(recEmpty)
}
