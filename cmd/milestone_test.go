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

func TestMilestoneList_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-ms/milestones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		date := "2026-12-31"
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Milestone]{
			Data: []api.Milestone{
				{
					Id:         "ms-1",
					Title:      "Launch Beta",
					Type:       "ROADMAP",
					Status:     "IN_PROGRESS",
					TargetDate: &date,
				},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-ms", DefaultMilestoneId: "ms-1"},
	}
	apiClient = api.NewClient(activeConfig)

	if err := milestoneListCmd.RunE(milestoneListCmd, []string{}); err != nil {
		t.Fatalf("milestoneListCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := milestoneListCmd.RunE(milestoneListCmd, []string{}); err != nil {
		t.Fatalf("milestoneListCmd JSON mode failed: %v", err)
	}
}

func TestMilestoneCreate_WithFlags(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-ms/milestones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req api.CreateMilestoneRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(api.ApiResponse[api.Milestone]{
				Data: api.Milestone{
					Id:     "ms-created-01",
					Title:  req.Title,
					Type:   req.Type,
					Status: req.Status,
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
		Local:  &config.LocalConfig{ProjectId: "proj-ms"},
	}
	apiClient = api.NewClient(activeConfig)

	msCreateTitle = "v2.0 Milestone"
	msCreateType = "ROADMAP"
	msCreateDescription = "Major release roadmap"
	msCreateTargetDate = "2026-12-31"
	msCreateStatus = "PLANNED"
	defer func() {
		msCreateTitle = ""
		msCreateType = ""
		msCreateDescription = ""
		msCreateTargetDate = ""
		msCreateStatus = ""
	}()

	if err := milestoneCreateCmd.RunE(milestoneCreateCmd, []string{}); err != nil {
		t.Fatalf("milestoneCreateCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := milestoneCreateCmd.RunE(milestoneCreateCmd, []string{}); err != nil {
		t.Fatalf("milestoneCreateCmd JSON mode failed: %v", err)
	}
}

func TestMilestoneBind(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/records/rec-bind-1/bind", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msId, _ := body["milestoneId"].(string)
		json.NewEncoder(w).Encode(api.ApiResponse[api.Record]{
			Data: api.Record{
				Id:          "rec-bind-1",
				MilestoneId: &msId,
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	// Bind to a milestone
	if err := milestoneBindCmd.RunE(milestoneBindCmd, []string{"rec-bind-1", "ms-target"}); err != nil {
		t.Fatalf("milestoneBindCmd failed: %v", err)
	}

	// Unbind (pass "null")
	if err := milestoneBindCmd.RunE(milestoneBindCmd, []string{"rec-bind-1", "null"}); err != nil {
		t.Fatalf("milestoneBindCmd unbind failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := milestoneBindCmd.RunE(milestoneBindCmd, []string{"rec-bind-1", "ms-target"}); err != nil {
		t.Fatalf("milestoneBindCmd JSON mode failed: %v", err)
	}
}
