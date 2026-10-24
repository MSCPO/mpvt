// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestProjectList_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Project]{
			Data: []api.Project{
				{
					Id:          "p-1",
					Title:       "Alpha",
					Slug:        "alpha",
					Status:      "IN_DEVELOPMENT",
					Visibility:  "PUBLIC",
					MemberCount: 3,
				},
			},
			Meta: &api.Meta{Page: 1, TotalPages: 1, Total: 1},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	if err := projectListCmd.RunE(projectListCmd, []string{}); err != nil {
		t.Fatalf("projectListCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := projectListCmd.RunE(projectListCmd, []string{}); err != nil {
		t.Fatalf("projectListCmd JSON mode failed: %v", err)
	}
}

func TestProjectView_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/alpha", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		desc := "Alpha project description"
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{
				Id:          "p-1",
				Title:       "Alpha",
				Slug:        "alpha",
				Status:      "IN_DEVELOPMENT",
				Visibility:  "PUBLIC",
				Description: &desc,
			},
		})
	})
	mux.HandleFunc("/api/v1/projects/p-1/members", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Member]{
			Data: []api.Member{
				{
					Id:     "m-1",
					Role:   "OWNER",
					UserId: "u-1",
					User:   &api.User{ID: "u-1", Email: "owner@example.com"},
				},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	if err := projectViewCmd.RunE(projectViewCmd, []string{"alpha"}); err != nil {
		t.Fatalf("projectViewCmd failed: %v", err)
	}
}

func TestProjectCreate_WithArgs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req api.CreateProjectRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
				Data: api.Project{
					Id:         "proj-created-01",
					Title:      req.Title,
					Slug:       req.Slug,
					Status:     req.Status,
					Visibility: req.Visibility,
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
			ProjectId: "proj-already-bound",
		},
	}
	apiClient = api.NewClient(activeConfig)

	projectCreateSlug = "beta-slug"
	projectCreateDesc = "Beta description"
	defer func() {
		projectCreateSlug = ""
		projectCreateDesc = ""
	}()

	if err := projectCreateCmd.RunE(projectCreateCmd, []string{"Beta Project"}); err != nil {
		t.Fatalf("projectCreateCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := projectCreateCmd.RunE(projectCreateCmd, []string{"Beta Project 2"}); err != nil {
		t.Fatalf("projectCreateCmd in JSON mode failed: %v", err)
	}
}

func TestProjectSetStatus_WithArgs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/alpha-slug", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{
				Id:     "p-1",
				Slug:   "alpha-slug",
				Status: "IN_DEVELOPMENT",
			},
		})
	})
	mux.HandleFunc("/api/v1/projects/p-1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{
				Id:     "p-1",
				Status: "PUBLISHED",
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	if err := projectStateCmd.RunE(projectStateCmd, []string{"PUBLISHED", "alpha-slug"}); err != nil {
		t.Fatalf("projectStateCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := projectStateCmd.RunE(projectStateCmd, []string{"PUBLISHED", "alpha-slug"}); err != nil {
		t.Fatalf("projectStateCmd JSON mode failed: %v", err)
	}
}
