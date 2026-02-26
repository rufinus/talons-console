package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rufinus/talons-console/internal/version"
)

var updateCheckCmd = &cobra.Command{
	Use:   "update-check",
	Short: "Check for talons updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		info, err := version.CheckUpdate(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
			os.Exit(0)
			return nil
		}

		if info.UpToDate {
			fmt.Printf("You're up to date (v%s)\n", info.Current)
		} else {
			fmt.Printf("You're running v%s. Latest is v%s.\n", info.Current, info.Latest)
			if info.DownloadURL != "" {
				fmt.Printf("Download: %s\n", info.DownloadURL)
			}
		}
		return nil
	},
}
