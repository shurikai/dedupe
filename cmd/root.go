package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "dedupe",
	Short: "A photo deduplication and organization tool",
}

func Execute() error {
	return rootCmd.Execute()
}
