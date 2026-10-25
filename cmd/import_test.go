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
	"sync"
	"testing"
	"time"

	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

func setupTestGitRepoWithCommits(t *testing.T, commitCount int) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Tester Bot")
	runGit("config", "user.email", "bot@example.com")

	for i := 1; i <= commitCount; i++ {
		msg := "Commit number " + strings.Repeat("A", i)
		body := "Description detail for commit " + strings.Repeat("B", i)
		runGit("commit", "--allow-empty", "-m", msg+"\n\n"+body)
	}

	return dir
}

func TestImportGit_UnboundProjectError(t *testing.T) {
	resetImportFlags()
	activeConfig = &config.Config{
		Local: nil,
	}

	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Fatal("expected error when project is not bound, got nil")
	}
	if !strings.Contains(err.Error(), "no linked project found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestImportGit_NonRepoError(t *testing.T) {
	resetImportFlags()
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(dir)

	activeConfig = &config.Config{
		Local: &config.LocalConfig{
			ProjectId:   "proj-test",
			ProjectSlug: "slug-test",
		},
	}

	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Fatal("expected error when running in non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "current directory is not a git repository") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestImportGit_SuccessAndDeduplication(t *testing.T) {
	resetImportFlags()
	repoDir := setupTestGitRepoWithCommits(t, 3)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(repoDir)

	var mu sync.Mutex
	var receivedRequests []api.CreateRecordRequest

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-import/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// Initially no existing records
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
				Data: []api.Record{},
				Meta: &api.Meta{Page: 1, TotalPages: 1, Total: 0},
			})
			return
		}
		if r.Method == http.MethodPost {
			var req api.CreateRecordRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			receivedRequests = append(receivedRequests, req)
			idx := len(receivedRequests)
			mu.Unlock()

			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
				Data: api.Record{
					Id:          "rec-" + string(rune('0'+idx)),
					ProjectId:   "proj-import",
					Title:       req.Title,
					Description: &req.Description,
					MilestoneId: &req.MilestoneID,
					CommittedAt: *req.CommittedAt,
					CreatedAt:   time.Now(),
				},
			})
			return
		}
		http.NotFound(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:          "proj-import",
			ProjectSlug:        "import-proj",
			DefaultMilestoneId: "ms-default",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// 1. Dry Run First
	importDryRun = true
	importYes = true
	err := importGitCmd.RunE(importGitCmd, []string{})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if len(receivedRequests) != 0 {
		t.Fatalf("expected 0 requests during dry run, got %d", len(receivedRequests))
	}

	// 2. Actual Import with IncludeHash & IncludeAuthor
	resetImportFlags()
	importYes = true
	importIncludeHash = true
	importIncludeAuthor = true

	cmd := importCmd
	cmd.SetContext(context.Background())
	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	mu.Lock()
	count := len(receivedRequests)
	mu.Unlock()
	if count != 3 {
		t.Fatalf("expected 3 commits imported, got %d", count)
	}

	// Verify reverse order: commit 1 was oldest, so it was imported first
	if !strings.Contains(receivedRequests[0].Title, "Commit number A") {
		t.Errorf("expected first imported commit to be oldest 'Commit number A', got %q", receivedRequests[0].Title)
	}
	if receivedRequests[0].MilestoneID != "ms-default" {
		t.Errorf("expected milestone 'ms-default', got %q", receivedRequests[0].MilestoneID)
	}
	if !strings.Contains(receivedRequests[0].Description, "Git-Commit:") {
		t.Errorf("expected description to contain Git-Commit:, got %s", receivedRequests[0].Description)
	}
	if !strings.Contains(receivedRequests[0].Description, "Git-Author: Tester Bot <bot@example.com>") {
		t.Errorf("expected description to contain Git-Author:, got %s", receivedRequests[0].Description)
	}

	// 3. Second run with skip-existing: all 3 should be skipped
	// Reconfigure mock server to return the 3 imported records on GET
	existingRecs := make([]api.Record, 3)
	for i, req := range receivedRequests {
		existingRecs[i] = api.Record{
			Id:          "rec-" + string(rune('0'+i+1)),
			Title:       req.Title,
			Description: &req.Description,
			CommittedAt: *req.CommittedAt,
		}
	}

	mux2 := http.NewServeMux()
	var newPosts int
	mux2.HandleFunc("/api/v1/projects/proj-import/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
				Data: existingRecs,
				Meta: &api.Meta{Page: 1, TotalPages: 1, Total: len(existingRecs)},
			})
			return
		}
		if r.Method == http.MethodPost {
			newPosts++
			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
				Data: api.Record{Id: "rec-new", Title: "New"},
			})
			return
		}
		http.NotFound(w, r)
	})
	server2 := httptest.NewServer(mux2)
	defer server2.Close()

	activeConfig.Global.Endpoint = server2.URL
	apiClient = api.NewClient(activeConfig)

	resetImportFlags()
	importYes = true
	importSkipExisting = true
	flagJSON = true
	defer func() { flagJSON = false }()

	err = importGitCmd.RunE(importGitCmd, []string{})
	if err != nil {
		t.Fatalf("second import with skip-existing failed: %v", err)
	}
	if newPosts != 0 {
		t.Errorf("expected 0 new posts because all 3 were existing, got %d", newPosts)
	}
}

func TestCommitCmd_AllFlagDelegatesToImport(t *testing.T) {
	resetCommitFlags()
	resetImportFlags()
	repoDir := setupTestGitRepoWithCommits(t, 2)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(repoDir)

	var postCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-commit-all/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
				Data: []api.Record{},
			})
			return
		}
		if r.Method == http.MethodPost {
			postCount++
			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
				Data: api.Record{Id: "rec-post"},
			})
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:          "proj-commit-all",
			ProjectSlug:        "commit-all-proj",
			DefaultMilestoneId: "ms-1",
		},
	}
	apiClient = api.NewClient(activeConfig)

	commitAll = true
	importYes = true
	defer func() { resetCommitFlags(); resetImportFlags() }()

	cmd := commitCmd
	cmd.SetContext(context.Background())
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("commit --all failed: %v", err)
	}

	if postCount != 2 {
		t.Fatalf("expected 2 commits imported via commit --all, got %d", postCount)
	}
}

func TestImportGit_InteractiveCancellation(t *testing.T) {
	resetImportFlags()
	repoDir := setupTestGitRepoWithCommits(t, 1)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(repoDir)

	mux := http.NewServeMux()
	var postCount int
	mux.HandleFunc("/api/v1/projects/proj-cancel/records", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{Data: []api.Record{}})
			return
		}
		if r.Method == http.MethodPost {
			postCount++
			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{Data: api.Record{Id: "rec-cancel"}})
			return
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:   "proj-cancel",
			ProjectSlug: "cancel-proj",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// Simulate user typing 'n' to cancel
	ui.SetReader(strings.NewReader("n\n"))
	defer ui.SetReader(nil)

	importYes = false
	err := importGitCmd.RunE(importGitCmd, []string{})
	if err != nil {
		t.Fatalf("expected nil error on user cancellation, got %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected 0 commits created when user cancels, got %d", postCount)
	}
}

func TestImportGit_LimitAndMilestoneOverride(t *testing.T) {
	resetImportFlags()
	repoDir := setupTestGitRepoWithCommits(t, 4)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(repoDir)

	var posts []api.CreateRecordRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-filter/records", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{Data: []api.Record{}})
			return
		}
		if r.Method == http.MethodPost {
			var req api.CreateRecordRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			posts = append(posts, req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{Data: api.Record{Id: "rec-ok"}})
			return
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:          "proj-filter",
			ProjectSlug:        "filter-proj",
			DefaultMilestoneId: "ms-default",
		},
	}
	apiClient = api.NewClient(activeConfig)

	importLimit = 2
	importMilestone = "ms-override"
	importYes = true
	defer resetImportFlags()

	err := importCmd.RunE(importCmd, []string{})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 commits imported with --limit 2, got %d", len(posts))
	}
	if posts[0].MilestoneID != "ms-override" {
		t.Errorf("expected milestone 'ms-override', got %q", posts[0].MilestoneID)
	}
}
