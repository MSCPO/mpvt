// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"mpvt/internal/ui"
)

var (
	Version   = "1.0.0"
	BuildDate = "2026-10-18"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mpvt",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagJSON {
			return outputJSON(map[string]string{
				"version":   Version,
				"buildDate": BuildDate,
				"app":       "mpvt (MSCPO ProjectVitals CLI)",
			})
		}
		fmt.Printf("%s version %s (%s)\n", ui.Bold("mpvt"), ui.Cyan(Version), ui.Dim(BuildDate))
		fmt.Println("MSCPO ProjectVitals CLI - Transparent progress tracking tool")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
