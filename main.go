package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var verbose = new(bool)

func main() {
	rootCmd := cobra.Command{}
	rootCmd.PersistentFlags().BoolVarP(verbose, "verbose", "v", false, "Decides whether additional, potentially unnecessary extra information, is printed to the terminal.")
	rootCmd.AddCommand(generateBuildCmd())
	rootCmd.AddCommand(generateServeCmd())
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
			fmt.Println("Error: Source and output can't be the same.")
		} else {
			build(source, *output, *config, *minifyOutput)
		}
	}

	return buildCmd
}

func generateServeCmd() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:        "serve <directory>",
		Short:      "Serves the directory via HTTP using a basic webserver.",
		Example:    "serve ./example-output",
		SuggestFor: []string{"run"},
		Args:       cobra.ExactArgs(1),
	}
	basepath := serveCmd.Flags().StringP("basepath", "b", "", "Defines the path at which the directory is served. (For example /hello for http://localhost:8080/hello).")
	port := serveCmd.Flags().IntP("port", "p", 8080, "Decides which port the HTTP server is run on.")
	serveCmd.Run = func(cmd *cobra.Command, args []string) {
		serve(args[0], *basepath, *port)
	}

	return serveCmd
}
