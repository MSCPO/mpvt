// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"mpvt/internal/auth"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

var (
	loginToken         string
	loginTokenStdin    bool
	loginEndpoint      string
	loginAuthURL       string
	loginSiteKey       string
	loginInternalToken string
	loginUserId        string
	loginUserEmail     string
	loginUserRole      string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials and connection settings",
}

var authStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"whoami"},
	Short:   "View current authentication status and server connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint := activeConfig.GetEffectiveEndpoint()
		ui.Infof("Checking server status at %s ...", ui.Bold(endpoint))

		health, err := apiClient.Health(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to connect to ProjectVitals backend: %w", err)
		}

		if flagJSON {
			redactedAuth := activeConfig.Global
			if redactedAuth.Token != "" {
				redactedAuth.Token = "[REDACTED]"
			}
			if redactedAuth.RefreshToken != "" {
				redactedAuth.RefreshToken = "[REDACTED]"
			}
			if redactedAuth.InternalToken != "" {
				redactedAuth.InternalToken = "[REDACTED]"
			}
			return outputJSON(map[string]any{
				"health": health,
				"auth":   redactedAuth,
			})
		}

		ui.Successf("Connected to %s (v%s, status: %s)", health.Service, health.Version, health.Status)
		fmt.Println()

		fmt.Printf("● API Endpoint:   %s\n", ui.Cyan(endpoint))
		if activeConfig.Global.AuthEndpoint != "" {
			fmt.Printf("● Auth Server:    %s\n", ui.Dim(activeConfig.Global.AuthEndpoint))
		}
		if activeConfig.Global.InternalToken != "" {
			fmt.Printf("● Auth Mode:      %s (Internal Gateway Proxy)\n", ui.Green("Gateway Headers"))
			fmt.Printf("● User ID:        %s\n", activeConfig.Global.UserId)
			fmt.Printf("● User Email:     %s\n", activeConfig.Global.UserEmail)
			fmt.Printf("● User Role:      %s\n", activeConfig.Global.UserRole)
		} else if activeConfig.Global.Token != "" {
			tokenDisplay := "********"
			if len(activeConfig.Global.Token) > 8 {
				tokenDisplay = activeConfig.Global.Token[:4] + "..." + activeConfig.Global.Token[len(activeConfig.Global.Token)-4:]
			}
			fmt.Printf("● Auth Mode:      %s (Bearer JWT)\n", ui.Green("Bearer Token"))
			if activeConfig.Global.UserName != "" || activeConfig.Global.UserEmail != "" {
				nameStr := activeConfig.Global.UserName
				if nameStr != "" {
					nameStr += " "
				}
				fmt.Printf("● User:           %s<%s>\n", nameStr, activeConfig.Global.UserEmail)
			}
			if activeConfig.Global.UserRole != "" {
				fmt.Printf("● Role:           %s\n", activeConfig.Global.UserRole)
			}
			fmt.Printf("● Token:          %s\n", ui.Dim(tokenDisplay))
		} else {
			fmt.Printf("● Auth Mode:      %s (No credentials configured)\n", ui.Yellow("Anonymous"))
			fmt.Println("  Run 'mpvt auth login' to authenticate.")
		}

		return nil
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with MSCPO account via browser or configure credentials",
	Long: `Authenticate mpvt CLI with your MSCPO account.
By default, this command launches a temporary local server, opens your web browser to render the MSCPO login page with hCaptcha, and securely captures the access token upon successful verification.

Alternatively, you can manually supply a Bearer token via --token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 0. Support reading token from stdin if --token-stdin is specified
		if loginTokenStdin {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				loginToken = strings.TrimSpace(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read token from stdin: %w", err)
			}
			if loginToken == "" {
				return fmt.Errorf("token read from stdin was empty")
			}
		}

		// 1. Manual token/gateway mode if provided via flags
		if loginToken != "" || loginInternalToken != "" {
			if cmd.Flags().Changed("token") || cmd.Flags().Changed("internal-token") {
				ui.Warning("Security notice: Passing tokens via command-line flags exposes them in process listings (ps aux) and shell history. Consider using environment variables (MPVT_TOKEN) or --token-stdin.")
			}
			if loginEndpoint != "" {
				activeConfig.Global.Endpoint = strings.TrimRight(loginEndpoint, "/")
			}
			if loginToken != "" {
				activeConfig.Global.Token = loginToken
				if loginInternalToken == "" {
					activeConfig.Global.InternalToken = "" // Clear internal token so Bearer works
				}
				if loginUserId == "" {
					activeConfig.Global.UserId = ""
				}
				if loginUserEmail == "" {
					activeConfig.Global.UserEmail = ""
				}
				if loginUserRole == "" {
					activeConfig.Global.UserRole = ""
				}
				activeConfig.Global.UserName = ""
			}
			if loginInternalToken != "" {
				activeConfig.Global.InternalToken = loginInternalToken
				if loginToken == "" {
					activeConfig.Global.Token = "" // Clear bearer token
					activeConfig.Global.RefreshToken = ""
				}
			}
			if loginUserId != "" {
				activeConfig.Global.UserId = loginUserId
			}
			if loginUserEmail != "" {
				activeConfig.Global.UserEmail = loginUserEmail
			}
			if loginUserRole != "" {
				activeConfig.Global.UserRole = loginUserRole
			}

			if err := config.SaveGlobalConfig(activeConfig.Global); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}
			cfgPath, _ := config.GetGlobalConfigPath()
			ui.Successf("Authentication credentials saved to %s", cfgPath)
			return nil
		}

		// 2. Browser-based login flow with hCaptcha
		authURL := loginAuthURL
		if authURL == "" {
			if activeConfig.Global.AuthEndpoint != "" {
				authURL = activeConfig.Global.AuthEndpoint
			} else {
				authURL = auth.DefaultAuthBaseURL
			}
		}

		siteKey := loginSiteKey
		if siteKey == "" {
			siteKey = auth.DefaultHCaptchaSiteKey
		}

		loginRes, err := auth.StartBrowserLogin(cmd.Context(), authURL, siteKey)
		if err != nil {
			return fmt.Errorf("browser login failed: %w", err)
		}

		// Save token and user details to global config
		activeConfig.Global.Token = loginRes.AccessToken
		activeConfig.Global.RefreshToken = loginRes.RefreshToken
		activeConfig.Global.InternalToken = "" // Clear any lingering internal gateway token so Bearer is used
		activeConfig.Global.AuthEndpoint = authURL
		activeConfig.Global.UserId = loginRes.User.Id
		activeConfig.Global.UserEmail = loginRes.User.Email
		if loginRes.User.Name != nil {
			activeConfig.Global.UserName = *loginRes.User.Name
		} else {
			activeConfig.Global.UserName = ""
		}
		activeConfig.Global.UserRole = loginRes.User.Role

		if loginEndpoint != "" {
			activeConfig.Global.Endpoint = strings.TrimRight(loginEndpoint, "/")
		}

		if err := config.SaveGlobalConfig(activeConfig.Global); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Println()
		ui.Success("登录成功！MSCPO 开发者身份已授权至 CLI")
		userName := loginRes.User.Email
		if loginRes.User.Name != nil && *loginRes.User.Name != "" {
			userName = fmt.Sprintf("%s <%s>", *loginRes.User.Name, loginRes.User.Email)
		}
		fmt.Printf("● 登录用户:   %s\n", ui.Bold(userName))
		fmt.Printf("● 用户权限:   %s\n", ui.Cyan(loginRes.User.Role))
		cfgPath, _ := config.GetGlobalConfigPath()
		fmt.Printf("● 配置文件:   %s\n", ui.Dim(cfgPath))
		fmt.Println()
		ui.Infof("现在您可以在代码目录中执行 '%s' 或 '%s' 提交修改记录。", ui.Bold("mpvt link"), ui.Bold("mpvt commit"))

		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved authentication credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		activeConfig.Global.Token = ""
		activeConfig.Global.RefreshToken = ""
		activeConfig.Global.InternalToken = ""
		activeConfig.Global.UserId = ""
		activeConfig.Global.UserEmail = ""
		activeConfig.Global.UserRole = ""
		activeConfig.Global.UserName = ""

		if err := config.SaveGlobalConfig(activeConfig.Global); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		ui.Success("Logged out. Saved credentials cleared.")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVarP(&loginEndpoint, "endpoint", "e", "", "ProjectVitals API Endpoint URL")
	authLoginCmd.Flags().StringVarP(&loginAuthURL, "auth-url", "a", auth.DefaultAuthBaseURL, "MSCPO Auth Server URL")
	authLoginCmd.Flags().StringVar(&loginSiteKey, "sitekey", auth.DefaultHCaptchaSiteKey, "hCaptcha sitekey")
	authLoginCmd.Flags().StringVarP(&loginToken, "token", "t", "", "Direct Bearer JWT Token (bypasses browser)")
	authLoginCmd.Flags().BoolVar(&loginTokenStdin, "token-stdin", false, "Read Bearer token from standard input")
	authLoginCmd.Flags().StringVar(&loginInternalToken, "internal-token", "", "Internal Gateway Token (For development/microservice mode)")
	authLoginCmd.Flags().StringVar(&loginUserId, "user-id", "", "User UUID (For gateway mode)")
	authLoginCmd.Flags().StringVar(&loginUserEmail, "user-email", "", "User Email (For gateway mode)")
	authLoginCmd.Flags().StringVar(&loginUserRole, "user-role", "USER", "User Role (USER/ADMIN)")

	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)

	rootCmd.AddCommand(authCmd)

	// Top-level 'mpvt login' shortcut
	topLoginCmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with MSCPO account (Shortcut for 'mpvt auth login')",
		RunE:  authLoginCmd.RunE,
	}
	topLoginCmd.Flags().AddFlagSet(authLoginCmd.Flags())
	rootCmd.AddCommand(topLoginCmd)
}
