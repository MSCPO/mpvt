// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestInitAndUnlink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/my-init-slug", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{
				Id:         "proj-init-123",
				Title:      "My Init Project",
				Slug:       "my-init-slug",
				Status:     "IN_DEVELOPMENT",
				Visibility: "PUBLIC",
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	flagNoGitIgnore = true
	defer func() { flagNoGitIgnore = false }()

	// 1. Run init
	if err := initCmd.RunE(initCmd, []string{"my-init-slug"}); err != nil {
		t.Fatalf("initCmd failed: %v", err)
	}

	cfgFile := filepath.Join(tmpDir, ".mpvt", "config.json")
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected config file at %s, got: %v", cfgFile, err)
	}

	// 2. Run unlink
	if err := unlinkCmd.RunE(unlinkCmd, []string{}); err != nil {
		t.Fatalf("unlinkCmd failed: %v", err)
	}

	if _, err := os.Stat(cfgFile); !os.IsNotExist(err) {
		t.Errorf("expected config file to be removed after unlink, but it still exists")
	}
}
