package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"dnsmanager/internal/cli"
	"dnsmanager/internal/client"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		baseURL string
		token   string
		output  string
	)

	rootCmd := &cobra.Command{
		Use:           "dnsmanager",
		Short:         "Remote CLI for the dnsmanager API",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&baseURL, "server", cli.DefaultServerURL(), "dnsmanager API base URL")
	rootCmd.PersistentFlags().StringVar(&token, "token", os.Getenv("DNSMANAGER_TOKEN"), "API token")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format: table or json")

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(statusCommand(&baseURL, &token, &output))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("dnsmanager CLI foundation build")
		},
	}
}

func statusCommand(baseURL, token, output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Fetch backend status and shared-volume layout",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			api := client.New(*baseURL, *token)
			status, err := api.Status(ctx)
			if err != nil {
				return err
			}

			if *output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Service: %s\n", status.Service)
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", status.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "HTTP: %s\n", status.HTTPAddr)
			fmt.Fprintf(cmd.OutOrStdout(), "Started: %s\n", status.StartedAt.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "Config root: %s\n", status.Paths.ConfigDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Data root: %s\n", status.Paths.DataDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Content root: %s\n", status.Paths.ContentDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Managed config: %s\n", status.Paths.ManagedDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Manual config: %s\n", status.Paths.ManualDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Generated config: %s\n", status.Paths.GeneratedDir)
			return nil
		},
	}
}
