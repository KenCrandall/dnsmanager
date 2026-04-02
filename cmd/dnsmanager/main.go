package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"dnsmanager/internal/cli"
	"dnsmanager/internal/client"
	"dnsmanager/internal/revision"

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
	rootCmd.AddCommand(configCommand(&baseURL, &token, &output))

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

func configCommand(baseURL, token, output *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage staged config revisions",
	}

	cmd.AddCommand(configCurrentCommand(baseURL, token, output))
	cmd.AddCommand(configListCommand(baseURL, token, output))
	cmd.AddCommand(configDraftCommand(baseURL, token, output))
	cmd.AddCommand(configValidateCommand(baseURL, token, output))
	cmd.AddCommand(configApplyCommand(baseURL, token, output))
	cmd.AddCommand(configRollbackCommand(baseURL, token, output))

	return cmd
}

func configCurrentCommand(baseURL, token, output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the currently active or most recent revision",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			api := client.New(*baseURL, *token)
			current, err := api.CurrentRevision(ctx)
			if err != nil {
				return err
			}

			return printRevision(cmd.OutOrStdout(), *output, current)
		},
	}
}

func configListCommand(baseURL, token, output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known config revisions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			api := client.New(*baseURL, *token)
			revisions, err := api.ListRevisions(ctx)
			if err != nil {
				return err
			}

			if *output == "json" {
				return writeJSON(cmd.OutOrStdout(), revisions)
			}

			for _, item := range revisions {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d\t%s\t%s\t%s\n", item.ID, item.State, item.ValidationStatus, item.Summary)
			}
			return nil
		},
	}
}

func configDraftCommand(baseURL, token, output *string) *cobra.Command {
	var (
		summary   string
		filePath  string
		createdBy string
	)

	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Create a draft config revision from file or stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			renderedConfig, err := readDraftInput(filePath, cmd.InOrStdin())
			if err != nil {
				return err
			}

			api := client.New(*baseURL, *token)
			created, err := api.CreateDraft(ctx, revision.CreateInput{
				Summary:        summary,
				RenderedConfig: renderedConfig,
				CreatedBy:      createdBy,
			})
			if err != nil {
				return err
			}

			return printRevision(cmd.OutOrStdout(), *output, created)
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "human-readable summary for the draft")
	cmd.Flags().StringVar(&filePath, "file", "", "path to a rendered dnsmasq fragment; defaults to stdin")
	cmd.Flags().StringVar(&createdBy, "created-by", "cli", "actor label recorded on the revision")

	return cmd
}

func configValidateCommand(baseURL, token, output *string) *cobra.Command {
	return revisionActionCommand(baseURL, token, output, "validate", "Run dnsmasq validation for a revision")
}

func configApplyCommand(baseURL, token, output *string) *cobra.Command {
	return revisionActionCommand(baseURL, token, output, "apply", "Apply a revision to the shared generated config")
}

func configRollbackCommand(baseURL, token, output *string) *cobra.Command {
	return revisionActionCommand(baseURL, token, output, "rollback", "Create and apply a rollback revision from a prior revision")
}

func revisionActionCommand(baseURL, token, output *string, action, short string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <revision-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			revisionID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return errors.New("revision id must be an integer")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			api := client.New(*baseURL, *token)
			var updated revision.Revision
			switch action {
			case "validate":
				updated, err = api.ValidateRevision(ctx, revisionID)
			case "apply":
				updated, err = api.ApplyRevision(ctx, revisionID)
			case "rollback":
				updated, err = api.RollbackRevision(ctx, revisionID)
			default:
				err = fmt.Errorf("unsupported action %s", action)
			}
			if err != nil {
				return err
			}

			return printRevision(cmd.OutOrStdout(), *output, updated)
		},
	}
}

func printRevision(w io.Writer, output string, item revision.Revision) error {
	if output == "json" {
		return writeJSON(w, item)
	}

	fmt.Fprintf(w, "Revision: %d\n", item.ID)
	fmt.Fprintf(w, "State: %s\n", item.State)
	fmt.Fprintf(w, "Summary: %s\n", item.Summary)
	fmt.Fprintf(w, "Validation: %s\n", item.ValidationStatus)
	if item.CreatedBy != "" {
		fmt.Fprintf(w, "Created by: %s\n", item.CreatedBy)
	}
	fmt.Fprintf(w, "Created at: %s\n", item.CreatedAt.Format(time.RFC3339))
	if item.AppliedAt != nil {
		fmt.Fprintf(w, "Applied at: %s\n", item.AppliedAt.Format(time.RFC3339))
	}
	if item.SourceRevisionID != nil {
		fmt.Fprintf(w, "Source revision: %d\n", *item.SourceRevisionID)
	}
	fmt.Fprintf(w, "Diff:\n%s\n", strings.TrimSpace(item.DiffText))
	fmt.Fprintf(w, "Validation output:\n%s\n", strings.TrimSpace(item.ValidationOutput))
	return nil
}

func readDraftInput(filePath string, stdin io.Reader) (string, error) {
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	content, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	if len(content) == 0 {
		return "", errors.New("no draft content provided; pass --file or pipe config on stdin")
	}

	return string(content), nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
