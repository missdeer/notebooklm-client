package cli

import (
	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

var homeDir string

var rootCmd = &cobra.Command{
	Use:   "notebooklm",
	Short: "NotebookLM CLI — generate podcasts, flashcards, reports via Google NotebookLM",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&homeDir, "home", "", "Config directory (default: ~/.notebooklm)")
	cobra.OnInitialize(func() {
		if homeDir != "" {
			rpc.SetHomeDir(homeDir)
		}
	})

	rootCmd.AddCommand(exportSessionCmd)
	rootCmd.AddCommand(importSessionCmd)
	rootCmd.AddCommand(refreshSessionCmd)
	rootCmd.AddCommand(audioCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(videoCmd)
	rootCmd.AddCommand(quizCmd)
	rootCmd.AddCommand(flashcardsCmd)
	rootCmd.AddCommand(infographicCmd)
	rootCmd.AddCommand(slidesCmd)
	rootCmd.AddCommand(dataTableCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(detailCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(sourceCmd)
	rootCmd.AddCommand(diagnoseCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
