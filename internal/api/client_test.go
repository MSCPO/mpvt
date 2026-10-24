// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestUserDisplayName(t *testing.T) {
	var nilUser *api.User
	if got := nilUser.DisplayName(); got != "Unknown" {
		t.Errorf("expected 'Unknown', got '%s'", got)
	}

	name := "Alice Wonder"
	userWithName := &api.User{
		ID:    "user-1234567890",
		Email: "alice@example.com",
		Name:  &name,
	}
	if got := userWithName.DisplayName(); got != "Alice Wonder" {
		t.Errorf("expected 'Alice Wonder', got '%s'", got)
	}

	emptyName := ""
	userWithEmailOnly := &api.User{
		ID:    "user-1234567890",
		Email: "bob@example.com",
		Name:  &emptyName,
	}
	if got := userWithEmailOnly.DisplayName(); got != "bob@example.com" {
		t.Errorf("expected 'bob@example.com', got '%s'", got)
	}

	userWithLongID := &api.User{
		ID: "1234567890abcdef",
	}
	if got := userWithLongID.DisplayName(); got != "12345678" {
		t.Errorf("expected '12345678', got '%s'", got)
	}

	userWithShortID := &api.User{
		ID: "short",
	}
	if got := userWithShortID.DisplayName(); got != "short" {
		t.Errorf("expected 'short', got '%s'", got)
	}
}

func TestAPIErrorFormat(t *testing.T) {
	errWithCode := &api.APIError{
		StatusCode: 404,
		Code:       "NOT_FOUND",
		Message:    "project not found",
	}
	expectedCodeStr := "[NOT_FOUND] project not found (HTTP 404)"
	if errWithCode.Error() != expectedCodeStr {
		t.Errorf("expected %q, got %q", expectedCodeStr, errWithCode.Error())
	}

	errNoCode := &api.APIError{
		StatusCode: 500,
		Message:    "internal failure",
	}
	expectedNoCodeStr := "server error HTTP 500: internal failure"
	if errNoCode.Error() != expectedNoCodeStr {
		t.Errorf("expected %q, got %q", expectedNoCodeStr, errNoCode.Error())
	}
}

func TestClientEndpoints(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.HealthResponse{
			Status:  "healthy",
			Service: "project-vitals",
			Version: "1.0.0",
		})
	})

	// 2. Projects
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Project]{
				Data: []api.Project{
					{
						Id:         "proj-1",
						Title:      "Vitals Core",
						Slug:       "vitals-core",
						Status:     "IN_DEVELOPMENT",
						Visibility: "PUBLIC",
					},
				},
				Meta: &api.Meta{
					Total:      1,
					Page:       1,
					Limit:      20,
					TotalPages: 1,
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			var req api.CreateProjectRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
				Data: api.Project{
					Id:         "proj-new",
					Title:      req.Title,
					Slug:       req.Slug,
					Status:     req.Status,
					Visibility: req.Visibility,
				},
			})
			return
		}
	})

	// 3. Project detail & patch
	mux.HandleFunc("/api/v1/projects/vitals-core", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
				Data: api.Project{
					Id:         "proj-1",
					Title:      "Vitals Core",
					Slug:       "vitals-core",
					Status:     "IN_DEVELOPMENT",
					Visibility: "PUBLIC",
				},
			})
			return
		}
		if r.Method == http.MethodPatch {
			var req api.UpdateProjectRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
				Data: api.Project{
					Id:     "proj-1",
					Title:  req.Title,
					Status: req.Status,
				},
			})
			return
		}
	})

	// 4. Milestones
	mux.HandleFunc("/api/v1/projects/proj-1/milestones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Milestone]{
				Data: []api.Milestone{
					{
						Id:        "ms-1",
						ProjectId: "proj-1",
						Title:     "v1.0 Milestone",
						Status:    "IN_PROGRESS",
					},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			var req api.CreateMilestoneRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Milestone]{
				Data: api.Milestone{
					Id:        "ms-created",
					ProjectId: "proj-1",
					Title:     req.Title,
					Type:      req.Type,
					Status:    req.Status,
				},
			})
			return
		}
	})

	mux.HandleFunc("/api/v1/projects/proj-1/milestones/ms-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[api.Milestone]{
				Data: api.Milestone{
					Id:        "ms-1",
					ProjectId: "proj-1",
					Title:     "v1.0 Milestone",
				},
			})
			return
		}
		if r.Method == http.MethodPatch {
			var req api.UpdateMilestoneRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Milestone]{
				Data: api.Milestone{
					Id:        "ms-1",
					ProjectId: "proj-1",
					Title:     req.Title,
				},
			})
			return
		}
	})

	// 5. Records
	mux.HandleFunc("/api/v1/projects/proj-1/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
				Data: []api.Record{
					{
						Id:        "rec-1",
						ProjectId: "proj-1",
						Title:     "Initial commit",
					},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			// Check auth header
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(api.ErrorResponse{
					Error: api.APIError{Code: "UNAUTHORIZED", Message: "Missing valid token"},
				})
				return
			}

			var req api.CreateRecordRequest
			json.NewDecoder(r.Body).Decode(&req)

			json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
				Data: api.Record{
					Id:        "rec-100",
					ProjectId: "proj-1",
					Title:     req.Title,
				},
			})
			return
		}
	})

	mux.HandleFunc("/api/v1/projects/proj-1/records/rec-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:        "rec-1",
				ProjectId: "proj-1",
				Title:     "Initial commit",
			},
		})
	})

	// 6. Bind record
	mux.HandleFunc("/api/v1/records/rec-1/bind", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msId, _ := body["milestoneId"].(string)
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:          "rec-1",
				ProjectId:   "proj-1",
				MilestoneId: &msId,
			},
		})
	})

	// 7. Proposals / Concept PRs
	mux.HandleFunc("/api/v1/projects/proj-1/proposals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.ConceptPR]{
				Data: []api.ConceptPR{
					{
						Id:        "pr-1",
						ProjectId: "proj-1",
						Title:     "Update architecture",
						Status:    "OPEN",
					},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			var req api.CreateConceptPRRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptPR]{
				Data: api.ConceptPR{
					Id:        "pr-created",
					ProjectId: "proj-1",
					Title:     req.Title,
					Status:    "OPEN",
				},
			})
			return
		}
	})

	mux.HandleFunc("/api/v1/projects/proj-1/proposals/pr-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptPR]{
			Data: api.ConceptPR{
				Id:        "pr-1",
				ProjectId: "proj-1",
				Title:     "Update architecture",
				Status:    "OPEN",
			},
		})
	})

	// 8. Concept blame
	mux.HandleFunc("/api/v1/projects/proj-1/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.ConceptBlame]{
			Data: []api.ConceptBlame{
				{
					Id:   "block-1",
					Type: "paragraph",
					Text: "Architecture overview block",
				},
			},
		})
	})

	// 9. Error endpoint
	mux.HandleFunc("/api/v1/projects/error-endpoint", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ErrorResponse{
			Error: api.APIError{
				Code:    "INVALID_ARGUMENT",
				Message: "slug is malformed",
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			Endpoint: server.URL,
			Token:    "test-token",
		},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// Verify transport pool settings
	transport, ok := client.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.HTTPClient.Transport)
	}
	if transport.MaxIdleConns != 50 {
		t.Errorf("expected MaxIdleConns=50, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected MaxIdleConnsPerHost=10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected IdleConnTimeout=90s, got %v", transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("expected TLSHandshakeTimeout=10s, got %v", transport.TLSHandshakeTimeout)
	}

	// Test Health
	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if health.Status != "healthy" {
		t.Errorf("expected healthy, got %s", health.Status)
	}

	// Test ListProjects
	projects, err := client.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 1 || projects[0].Id != "proj-1" {
		t.Errorf("unexpected projects: %+v", projects)
	}

	// Test GetProject
	proj, err := client.GetProject(ctx, "vitals-core")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if proj.Id != "proj-1" || proj.Slug != "vitals-core" {
		t.Errorf("unexpected project: %+v", proj)
	}

	// Test CreateProject
	newProj, err := client.CreateProject(ctx, api.CreateProjectRequest{
		Title:      "New Project",
		Slug:       "new-proj",
		Status:     "PLANNING",
		Visibility: "PRIVATE",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if newProj.Id != "proj-new" || newProj.Title != "New Project" {
		t.Errorf("unexpected created project: %+v", newProj)
	}

	// Test UpdateProject
	updatedProj, err := client.UpdateProject(ctx, "vitals-core", api.UpdateProjectRequest{
		Title:  "Updated Title",
		Status: "PUBLISHED",
	})
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if updatedProj.Title != "Updated Title" {
		t.Errorf("unexpected updated project: %+v", updatedProj)
	}

	// Test ListMilestones & GetMilestone & CreateMilestone & UpdateMilestone
	milestones, err := client.ListMilestones(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListMilestones failed: %v", err)
	}
	if len(milestones) != 1 || milestones[0].Id != "ms-1" {
		t.Errorf("unexpected milestones: %+v", milestones)
	}

	ms, err := client.GetMilestone(ctx, "proj-1", "ms-1")
	if err != nil {
		t.Fatalf("GetMilestone failed: %v", err)
	}
	if ms.Id != "ms-1" {
		t.Errorf("unexpected milestone: %+v", ms)
	}

	createdMS, err := client.CreateMilestone(ctx, "proj-1", api.CreateMilestoneRequest{
		Title: "v2.0",
		Type:  "ROADMAP",
	})
	if err != nil {
		t.Fatalf("CreateMilestone failed: %v", err)
	}
	if createdMS.Id != "ms-created" {
		t.Errorf("unexpected created milestone: %+v", createdMS)
	}

	updatedMS, err := client.UpdateMilestone(ctx, "proj-1", "ms-1", api.UpdateMilestoneRequest{
		Title: "v1.1 Updated",
	})
	if err != nil {
		t.Fatalf("UpdateMilestone failed: %v", err)
	}
	if updatedMS.Title != "v1.1 Updated" {
		t.Errorf("unexpected updated milestone: %+v", updatedMS)
	}

	// Test ListRecords & GetRecord & CreateRecord & BindRecordMilestone
	records, err := client.ListRecords(ctx, "proj-1", 10, 0)
	if err != nil {
		t.Fatalf("ListRecords failed: %v", err)
	}
	if len(records) != 1 || records[0].Id != "rec-1" {
		t.Errorf("unexpected records: %+v", records)
	}

	rec, err := client.GetRecord(ctx, "proj-1", "rec-1")
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}
	if rec.Id != "rec-1" {
		t.Errorf("unexpected record: %+v", rec)
	}

	createdRec, err := client.CreateRecord(ctx, "proj-1", api.CreateRecordRequest{
		Title:       "Optimized network pool",
		Description: "improved throughput by 35%",
	})
	if err != nil {
		t.Fatalf("CreateRecord failed: %v", err)
	}
	if createdRec.Id != "rec-100" || createdRec.Title != "Optimized network pool" {
		t.Errorf("unexpected record: %+v", createdRec)
	}

	if err := client.BindRecordMilestone(ctx, "proj-1", "rec-1", "ms-1"); err != nil {
		t.Fatalf("BindRecordMilestone failed: %v", err)
	}

	// Test ListConceptPRs & GetConceptPR & CreateConceptPR
	prs, err := client.ListConceptPRs(ctx, "proj-1", "OPEN")
	if err != nil {
		t.Fatalf("ListConceptPRs failed: %v", err)
	}
	if len(prs) != 1 || prs[0].Id != "pr-1" {
		t.Errorf("unexpected PRs: %+v", prs)
	}

	pr, err := client.GetConceptPR(ctx, "proj-1", "pr-1")
	if err != nil {
		t.Fatalf("GetConceptPR failed: %v", err)
	}
	if pr.Id != "pr-1" {
		t.Errorf("unexpected PR: %+v", pr)
	}

	createdPR, err := client.CreateConceptPR(ctx, "proj-1", api.CreateConceptPRRequest{
		Title: "Refactor architecture",
	})
	if err != nil {
		t.Fatalf("CreateConceptPR failed: %v", err)
	}
	if createdPR.Id != "pr-created" {
		t.Errorf("unexpected created PR: %+v", createdPR)
	}

	// Test GetConceptBlame
	blames, err := client.GetConceptBlame(ctx, "proj-1", "")
	if err != nil {
		t.Fatalf("GetConceptBlame failed: %v", err)
	}
	if len(blames) != 1 || blames[0].Id != "block-1" {
		t.Errorf("unexpected blame items: %+v", blames)
	}

	// Test typed APIError handling
	_, err = client.GetProject(ctx, "error-endpoint")
	if err == nil {
		t.Fatalf("expected error from error-endpoint, got nil")
	}
	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("expected StatusCode=400, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "INVALID_ARGUMENT" {
		t.Errorf("expected Code='INVALID_ARGUMENT', got %s", apiErr.Code)
	}

	// Test Context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err = client.Health(cancelCtx)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

func TestCheckRedirectSensitiveHeaders(t *testing.T) {
	var secondHostReceivedHeaders http.Header

	// Destination server (different host)
	destServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHostReceivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer destServer.Close()

	// Initial server that redirects to destServer
	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destServer.URL+"/redirected", http.StatusFound)
	}))
	defer srcServer.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			Endpoint:      srcServer.URL,
			InternalToken: "secret-internal-token",
			UserId:        "user-uuid-123",
			UserRole:      "ADMIN",
			UserEmail:     "admin@mscpo.org",
		},
	}
	client := api.NewClient(cfg)

	// Issue request to srcServer which redirects to destServer
	_, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if val := secondHostReceivedHeaders.Get("x-internal-token"); val != "" {
		t.Errorf("expected x-internal-token to be stripped on cross-domain redirect, got %s", val)
	}
	if val := secondHostReceivedHeaders.Get("x-user-id"); val != "" {
		t.Errorf("expected x-user-id to be stripped, got %s", val)
	}
	if val := secondHostReceivedHeaders.Get("x-user-role"); val != "" {
		t.Errorf("expected x-user-role to be stripped, got %s", val)
	}
	if val := secondHostReceivedHeaders.Get("x-user-email"); val != "" {
		t.Errorf("expected x-user-email to be stripped, got %s", val)
	}
}

func TestRemainingClientEndpoints(t *testing.T) {
	mux := http.NewServeMux()

	// Users search
	mux.HandleFunc("/api/v1/users/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.User]{
			Data: []api.User{
				{ID: "u1", Email: "u1@example.com"},
			},
		})
	})

	// Project status
	mux.HandleFunc("/api/v1/projects/proj-1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{Id: "proj-1", Status: "ARCHIVED"},
		})
	})

	// Members
	mux.HandleFunc("/api/v1/projects/proj-1/members", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Member]{
			Data: []api.Member{
				{Id: "m1", Role: "OWNER"},
			},
		})
	})

	// Delete record
	mux.HandleFunc("/api/v1/records/rec-del", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})

	// Concept
	mux.HandleFunc("/api/v1/projects/proj-1/concept", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptDoc]{
			Data: api.ConceptDoc{Version: 2},
		})
	})

	// Proposals merge / close
	mux.HandleFunc("/api/v1/proposals/prop-1/merge", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/v1/proposals/prop-1/close", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})

	// Projects paged
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Project]{
			Data: []api.Project{{Id: "proj-1"}},
		})
	})

	// Records filter
	mux.HandleFunc("/api/v1/projects/proj-1/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
			Data: []api.Record{{Id: "rec-1"}},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			Endpoint:      server.URL,
			InternalToken: "internal-tok",
			UserId:        "user-1",
			UserRole:      "ADMIN",
			UserEmail:     "admin@example.com",
		},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// 1. SearchUsers
	users, err := client.SearchUsers(ctx, "test")
	if err != nil || len(users) != 1 {
		t.Errorf("SearchUsers failed: %v", err)
	}

	// 2. UpdateProjectStatus
	p, err := client.UpdateProjectStatus(ctx, "proj-1", "ARCHIVED")
	if err != nil || p.Status != "ARCHIVED" {
		t.Errorf("UpdateProjectStatus failed: %v", err)
	}

	// 3. ListMembers
	members, err := client.ListMembers(ctx, "proj-1")
	if err != nil || len(members) != 1 {
		t.Errorf("ListMembers failed: %v", err)
	}

	// 4. DeleteRecord
	if err := client.DeleteRecord(ctx, "rec-del"); err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}

	// 5. GetConcept
	concept, err := client.GetConcept(ctx, "proj-1")
	if err != nil || concept.Version != 2 {
		t.Errorf("GetConcept failed: %v", err)
	}

	// 6. MergeProposal & CloseProposal
	if err := client.MergeProposal(ctx, "prop-1"); err != nil {
		t.Errorf("MergeProposal failed: %v", err)
	}
	if err := client.CloseProposal(ctx, "prop-1"); err != nil {
		t.Errorf("CloseProposal failed: %v", err)
	}

	// 7. ListProjectsPaged & ListRecordsWithFilter
	paged, err := client.ListProjectsPaged(ctx, 1, 10, "IN_DEV", "proj")
	if err != nil {
		t.Errorf("ListProjectsPaged failed: %v", err)
	}
	_ = paged

	recordsFiltered, err := client.ListRecordsWithFilter(ctx, "proj-1", "ms-1", "author-1", 1, 10)
	if err != nil {
		t.Errorf("ListRecordsWithFilter failed: %v", err)
	}
	_ = recordsFiltered
}

func TestGetConceptBlame_EmptyArray(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-empty/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": []}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			Endpoint: server.URL,
		},
	}
	client := api.NewClient(cfg)
	blames, err := client.GetConceptBlame(context.Background(), "proj-empty", "")
	if err != nil {
		t.Fatalf("expected nil error on empty blame array, got: %v", err)
	}
	if len(blames) != 0 {
		t.Errorf("expected 0 blames, got %d", len(blames))
	}
}

func TestConceptDoc_NextBlockID(t *testing.T) {
	var nilDoc *api.ConceptDoc
	if id := nilDoc.NextBlockID(); id != "b-1" {
		t.Errorf("expected b-1 for nil doc, got %s", id)
	}

	doc := &api.ConceptDoc{
		Blocks: []api.ConceptBlock{
			{Id: "b-1"},
			{Id: "b-2"},
			{Id: "custom-id"},
		},
	}
	if id := doc.NextBlockID(); id != "b-3" {
		t.Errorf("expected b-3, got %s", id)
	}

	docWithCollision := &api.ConceptDoc{
		Blocks: []api.ConceptBlock{
			{Id: "b-1"},
			{Id: "b-3"},
		},
	}
	if id := docWithCollision.NextBlockID(); id != "b-4" {
		t.Errorf("expected b-4, got %s", id)
	}
}

func TestGetConceptDocBlame_Polymorphic(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Array response
	mux.HandleFunc("/api/v1/projects/proj-arr/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": [{"id": "b-arr-1", "type": "paragraph", "text": "Array text"}]}`))
	})

	// 2. Object response
	mux.HandleFunc("/api/v1/projects/proj-obj/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"version": 3, "blocks": [{"id": "b-obj-1", "type": "paragraph", "text": "Object text"}]}}`))
	})

	// 3. Null response
	mux.HandleFunc("/api/v1/projects/proj-null/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": null}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// Test Array payload
	docArr, err := client.GetConceptDocBlame(ctx, "proj-arr")
	if err != nil {
		t.Fatalf("GetConceptDocBlame with array payload failed: %v", err)
	}
	if len(docArr.Blocks) != 1 || docArr.Blocks[0].Id != "b-arr-1" {
		t.Errorf("unexpected blocks from array: %+v", docArr.Blocks)
	}

	// Test Object payload
	docObj, err := client.GetConceptDocBlame(ctx, "proj-obj")
	if err != nil {
		t.Fatalf("GetConceptDocBlame with object payload failed: %v", err)
	}
	if docObj.Version != 3 || len(docObj.Blocks) != 1 || docObj.Blocks[0].Id != "b-obj-1" {
		t.Errorf("unexpected doc from object: %+v", docObj)
	}

	// Test Null payload
	docNull, err := client.GetConceptDocBlame(ctx, "proj-null")
	if err != nil {
		t.Fatalf("GetConceptDocBlame with null payload failed: %v", err)
	}
	if len(docNull.Blocks) != 0 {
		t.Errorf("expected empty blocks for null payload, got %d", len(docNull.Blocks))
	}
}

func TestTypeIDMethods(t *testing.T) {
	var nilProj *api.Project
	if got := nilProj.ID(); got != "" {
		t.Errorf("expected empty string for nil project, got %q", got)
	}
	proj := &api.Project{Id: "p-123"}
	if got := proj.ID(); got != "p-123" {
		t.Errorf("expected p-123, got %q", got)
	}

	var nilMs *api.Milestone
	if got := nilMs.ID(); got != "" {
		t.Errorf("expected empty string for nil milestone, got %q", got)
	}
	ms := &api.Milestone{Id: "m-456"}
	if got := ms.ID(); got != "m-456" {
		t.Errorf("expected m-456, got %q", got)
	}

	var nilRec *api.Record
	if got := nilRec.ID(); got != "" {
		t.Errorf("expected empty string for nil record, got %q", got)
	}
	rec := &api.Record{Id: "r-789"}
	if got := rec.ID(); got != "r-789" {
		t.Errorf("expected r-789, got %q", got)
	}

	var nilProp *api.Proposal
	if got := nilProp.ID(); got != "" {
		t.Errorf("expected empty string for nil proposal, got %q", got)
	}
	prop := &api.Proposal{Id: "pr-101"}
	if got := prop.ID(); got != "pr-101" {
		t.Errorf("expected pr-101, got %q", got)
	}
}

func TestClient_RecordBindAndDelete(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/records/rec-1/bind", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "bad method", http.StatusBadRequest)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		msID, _ := body["milestoneId"].(string)
		var msPtr *string
		if msID != "" {
			msPtr = &msID
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:          "rec-1",
				Title:       "Updated Record",
				MilestoneId: msPtr,
			},
		})
	})

	mux.HandleFunc("/api/v1/records/rec-del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "bad method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// 1. Bind to milestone
	recBound, err := client.BindRecord(ctx, "rec-1", "ms-target")
	if err != nil {
		t.Fatalf("BindRecord failed: %v", err)
	}
	if recBound.MilestoneId == nil || *recBound.MilestoneId != "ms-target" {
		t.Errorf("expected milestoneId ms-target, got %+v", recBound.MilestoneId)
	}

	// 2. Unbind (empty milestoneID)
	recUnbound, err := client.BindRecord(ctx, "rec-1", "")
	if err != nil {
		t.Fatalf("BindRecord unbind failed: %v", err)
	}
	if recUnbound.MilestoneId != nil {
		t.Errorf("expected nil milestoneId after unbind, got %v", *recUnbound.MilestoneId)
	}

	// 3. Delete record
	if err := client.DeleteRecord(ctx, "rec-del"); err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}
}

func TestClient_ProposalsAndConcept(t *testing.T) {
	mux := http.NewServeMux()

	// Proposals
	mux.HandleFunc("/api/v1/projects/p-1/proposals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ApiResponse[[]api.Proposal]{
				Data: []api.Proposal{
					{Id: "prop-1", Title: "Proposal 1", Status: "OPEN"},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(api.ApiResponse[api.Proposal]{
				Data: api.Proposal{
					Id:     "prop-new",
					Title:  "New Proposal",
					Status: "OPEN",
				},
			})
			return
		}
	})

	mux.HandleFunc("/api/v1/proposals/prop-1/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})

	mux.HandleFunc("/api/v1/proposals/prop-1/close", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})

	// Concept doc
	mux.HandleFunc("/api/v1/projects/p-1/concept", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptDoc]{
			Data: api.ConceptDoc{
				Version: 2,
				Blocks: []api.ConceptBlock{
					{Id: "b-1", Type: "paragraph", Text: "Hello Concept"},
				},
			},
		})
	})

	// Concept Blame with query param
	mux.HandleFunc("/api/v1/projects/p-1/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("file") == "incompatible" {
			w.Write([]byte(`{"data": "invalid-string"}`))
			return
		}
		if r.URL.Query().Get("file") == "as_doc" {
			w.Write([]byte(`{"data": {"version": 1, "blocks": [{"id": "b-doc-1", "type": "header", "text": "Header"}]}}`))
			return
		}
		if r.URL.Query().Get("file") == "null_payload" {
			w.Write([]byte(`{"data": null}`))
			return
		}
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.ConceptBlame]{
			Data: []api.ConceptBlame{
				{Id: "b-1", Type: "paragraph", Text: "Blamed text"},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// 1. ListProposals
	props, err := client.ListProposals(ctx, "p-1", "OPEN")
	if err != nil {
		t.Fatalf("ListProposals failed: %v", err)
	}
	if len(props) != 1 || props[0].Id != "prop-1" {
		t.Errorf("unexpected proposals: %+v", props)
	}

	// 2. CreateProposal
	newProp, err := client.CreateProposal(ctx, "p-1", "New Title", "Desc", api.PatchData{})
	if err != nil {
		t.Fatalf("CreateProposal failed: %v", err)
	}
	if newProp.Id != "prop-new" {
		t.Errorf("expected prop-new, got %s", newProp.Id)
	}

	// 3. MergeProposal & CloseProposal
	if err := client.MergeProposal(ctx, "prop-1"); err != nil {
		t.Fatalf("MergeProposal failed: %v", err)
	}
	if err := client.CloseProposal(ctx, "prop-1"); err != nil {
		t.Fatalf("CloseProposal failed: %v", err)
	}

	// 4. GetConcept
	doc, err := client.GetConcept(ctx, "p-1")
	if err != nil {
		t.Fatalf("GetConcept failed: %v", err)
	}
	if doc.Version != 2 || len(doc.Blocks) != 1 {
		t.Errorf("unexpected concept doc: %+v", doc)
	}

	// 5. GetConceptBlame (array format)
	blames, err := client.GetConceptBlame(ctx, "p-1", "README.md")
	if err != nil {
		t.Fatalf("GetConceptBlame failed: %v", err)
	}
	if len(blames) != 1 || blames[0].Id != "b-1" {
		t.Errorf("unexpected blames: %+v", blames)
	}

	// 6. GetConceptBlame (doc format fallback)
	blamesDoc, err := client.GetConceptBlame(ctx, "p-1", "as_doc")
	if err != nil {
		t.Fatalf("GetConceptBlame doc format failed: %v", err)
	}
	if len(blamesDoc) != 1 || blamesDoc[0].Id != "b-doc-1" {
		t.Errorf("unexpected blames from doc: %+v", blamesDoc)
	}

	// 7. GetConceptBlame (incompatible format)
	_, err = client.GetConceptBlame(ctx, "p-1", "incompatible")
	if err == nil {
		t.Fatalf("expected error for incompatible payload, got nil")
	}

	// 8. GetConceptBlame (null payload)
	blamesNull, err := client.GetConceptBlame(ctx, "p-1", "null_payload")
	if err != nil {
		t.Fatalf("GetConceptBlame null payload failed: %v", err)
	}
	if blamesNull != nil {
		t.Errorf("expected nil blames, got %+v", blamesNull)
	}
}

func TestNextBlockID_EdgeCases(t *testing.T) {
	var nilDoc *api.ConceptDoc
	if id := nilDoc.NextBlockID(); id != "b-1" {
		t.Errorf("expected b-1 for nil doc, got %s", id)
	}

	emptyDoc := &api.ConceptDoc{Blocks: []api.ConceptBlock{}}
	if id := emptyDoc.NextBlockID(); id != "b-1" {
		t.Errorf("expected b-1 for empty blocks, got %s", id)
	}

	docWithBlocks := &api.ConceptDoc{
		Blocks: []api.ConceptBlock{
			{Id: "b-1"},
			{Id: "b-2"},
			{Id: "custom-id"},
		},
	}
	if id := docWithBlocks.NextBlockID(); id != "b-3" {
		t.Errorf("expected b-3, got %s", id)
	}
}

func TestClient_AllMethods_ServerError(t *testing.T) {
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code": "INTERNAL_ERROR", "message": "server error"}`))
	}))
	defer errServer.Close()

	cfg := &config.Config{
		Global: config.GlobalConfig{Endpoint: errServer.URL},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// Verify that every method returns error cleanly when backend fails
	if _, err := client.Health(ctx); err == nil {
		t.Errorf("expected error from Health")
	}
	if _, err := client.SearchUsers(ctx, "query"); err == nil {
		t.Errorf("expected error from SearchUsers")
	}
	if _, err := client.ListProjects(ctx); err == nil {
		t.Errorf("expected error from ListProjects")
	}
	if _, err := client.ListProjectsPaged(ctx, 1, 10, "", ""); err == nil {
		t.Errorf("expected error from ListProjectsPaged")
	}
	if _, err := client.GetProject(ctx, "p1"); err == nil {
		t.Errorf("expected error from GetProject")
	}
	if _, err := client.CreateProject(ctx, api.CreateProjectRequest{Title: "P"}); err == nil {
		t.Errorf("expected error from CreateProject")
	}
	if _, err := client.UpdateProject(ctx, "p1", api.UpdateProjectRequest{}); err == nil {
		t.Errorf("expected error from UpdateProject")
	}
	if _, err := client.UpdateProjectStatus(ctx, "p1", "IN_DEVELOPMENT"); err == nil {
		t.Errorf("expected error from UpdateProjectStatus")
	}
	if _, err := client.ListMembers(ctx, "p1"); err == nil {
		t.Errorf("expected error from ListMembers")
	}
	if _, err := client.ListMilestones(ctx, "p1"); err == nil {
		t.Errorf("expected error from ListMilestones")
	}
	if _, err := client.GetMilestone(ctx, "p1", "m1"); err == nil {
		t.Errorf("expected error from GetMilestone")
	}
	if _, err := client.CreateMilestone(ctx, "p1", api.CreateMilestoneRequest{Title: "M"}); err == nil {
		t.Errorf("expected error from CreateMilestone")
	}
	if _, err := client.UpdateMilestone(ctx, "p1", "m1", api.UpdateMilestoneRequest{}); err == nil {
		t.Errorf("expected error from UpdateMilestone")
	}
	if _, err := client.ListRecords(ctx, "p1", 1, 10); err == nil {
		t.Errorf("expected error from ListRecords")
	}
	if _, err := client.ListRecordsWithFilter(ctx, "p1", "q", "m1", 1, 10); err == nil {
		t.Errorf("expected error from ListRecordsWithFilter")
	}
	if _, err := client.GetRecord(ctx, "p1", "r1"); err == nil {
		t.Errorf("expected error from GetRecord")
	}
	if _, err := client.CreateRecord(ctx, "p1", api.CreateRecordRequest{Title: "R"}); err == nil {
		t.Errorf("expected error from CreateRecord")
	}
	if err := client.BindRecordMilestone(ctx, "p1", "r1", "m1"); err == nil {
		t.Errorf("expected error from BindRecordMilestone")
	}
	if _, err := client.BindRecord(ctx, "r1", "m1"); err == nil {
		t.Errorf("expected error from BindRecord")
	}
	if err := client.DeleteRecord(ctx, "r1"); err == nil {
		t.Errorf("expected error from DeleteRecord")
	}
	if _, err := client.GetConcept(ctx, "p1"); err == nil {
		t.Errorf("expected error from GetConcept")
	}
	if _, err := client.GetConceptDocBlame(ctx, "p1"); err == nil {
		t.Errorf("expected error from GetConceptDocBlame")
	}
	if _, err := client.ListConceptPRs(ctx, "p1", "OPEN"); err == nil {
		t.Errorf("expected error from ListConceptPRs")
	}
	if _, err := client.GetConceptPR(ctx, "p1", "pr1"); err == nil {
		t.Errorf("expected error from GetConceptPR")
	}
	if _, err := client.CreateConceptPR(ctx, "p1", api.CreateConceptPRRequest{Title: "PR"}); err == nil {
		t.Errorf("expected error from CreateConceptPR")
	}
	if err := client.MergeProposal(ctx, "pr1"); err == nil {
		t.Errorf("expected error from MergeProposal")
	}
	if err := client.CloseProposal(ctx, "pr1"); err == nil {
		t.Errorf("expected error from CloseProposal")
	}
}

func TestClient_NewRequestAndErrorParsing(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Direct APIError response
	mux.HandleFunc("/api/v1/projects/direct-err", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"code": "FORBIDDEN", "message": "access denied"}`))
	})

	// 2. Plaintext 502 Bad Gateway
	mux.HandleFunc("/api/v1/projects/plain-err", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("502 Bad Gateway"))
	})

	// 3. API endpoint with /api/v1 suffix handling
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": []}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Client with Gateway internal token but no user role (defaults to USER)
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Endpoint:      server.URL + "/api/v1",
			InternalToken: "gw-token",
			UserId:        "u-gw-1",
			UserEmail:     "gw@example.com",
		},
	}
	client := api.NewClient(cfg)
	ctx := context.Background()

	// Verify /projects resolution with /api/v1 BaseURL suffix
	if _, err := client.ListProjects(ctx); err != nil {
		t.Fatalf("ListProjects with /api/v1 suffix failed: %v", err)
	}

	// Test direct APIError response
	if _, err := client.GetProject(ctx, "direct-err"); err == nil {
		t.Errorf("expected error for direct-err")
	}

	// Test plaintext error response
	if _, err := client.GetProject(ctx, "plain-err"); err == nil {
		t.Errorf("expected error for plain-err")
	}
}
