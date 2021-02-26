package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := cobra.Command{}
	rootCmd.AddCommand(generateBuildCmd())
	rootCmd.Execute()
}

func generateBuildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:        "build <source directory>",
		Short:      "Assembles the source directory and delivers a deployable website.",
		Example:    "build ./example",
		SuggestFor: []string{"make", "assemble", "compile"},
		Args:       cobra.ExactArgs(1),
	}
	minifyOutput := buildCmd.Flags().BoolP("minify", "m", false, "Decides whether css and html files will be minified (reduces file size).")
	config := buildCmd.Flags().StringP("config", "c", "", "Defines where the config is. If left empty, the config will be assumed in the source directory.")
	output := buildCmd.Flags().StringP("output", "o", "output", "Defines the directory where the build result will be written to.")
	buildCmd.Run = func(cmd *cobra.Command, args []string) {
		source := args[0]
		if source == *output {
			fmt.Println("Source and output can't be the same.")
		} else {
			build(source, *output, *config, *minifyOutput)
		}
	}

	return buildCmd
}
