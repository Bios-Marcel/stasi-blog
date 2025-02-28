package main

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var verbose = new(bool)

func main() {
	log.SetFlags(log.Flags() | log.Lmicroseconds)
	rootCmd := cobra.Command{Use: "stasi-blog"}
	rootCmd.PersistentFlags().BoolVarP(verbose, "verbose", "v", false, "Decides whether additional, potentially unnecessary extra information, is printed to the terminal.")
	rootCmd.AddCommand(generateBuildCmd())
	rootCmd.AddCommand(generateLiveCmd())
	rootCmd.AddCommand(generateServeCmd())
	rootCmd.Execute()
}

func generateLiveCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:     "dev directory",
		Short:   "dev serves the specified source directory and reflects changes instantly (debounced)",
		Aliases: []string{"develop", "live"},
		Args:    cobra.ExactArgs(1),
	}
	minifyOutput := buildCmd.Flags().BoolP("minify", "m", false, "Decides whether css and html files will be minified (reduces file size).")
	draft := buildCmd.Flags().BoolP("draft", "d", true, "Decides whether draft files are included in the build output.")
	config := buildCmd.Flags().StringP("config", "c", "", "Defines where the config is. If left empty, the config will be assumed in the source directory.")
	basepath := buildCmd.Flags().StringP("basepath", "b", "", "Defines the path at which the directory is served. (For example /hello for http://localhost:8080/hello).")
	port := buildCmd.Flags().IntP("port", "p", 8080, "Decides which port the HTTP server is run on.")
	buildCmd.Run = func(cmd *cobra.Command, args []string) {
		if err := live(args[0], *basepath, *config, *port, *minifyOutput, *draft); err != nil {
			log.Println("Error serving files in dev mode:")
			log.Println(err)
		}
	}

	return buildCmd
}

func generateBuildCmd() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:        "build directory",
		Short:      "Assembles the source directory and delivers a deployable website.",
		Example:    "build ./example",
		SuggestFor: []string{"make", "assemble", "compile"},
		Args:       cobra.ExactArgs(1),
	}
	minifyOutput := buildCmd.Flags().BoolP("minify", "m", false, "Decides whether css and html files will be minified (reduces file size).")
	includeDrafts := buildCmd.Flags().BoolP("draft", "d", false, "Decides whether draft files are included in the build output.")
	config := buildCmd.Flags().StringP("config", "c", "", "Defines where the config is. If left empty, the config will be assumed in the source directory.")
	output := buildCmd.Flags().StringP("output", "o", "output", "Defines the directory where the build result will be written to.")
	buildCmd.RunE = func(cmd *cobra.Command, args []string) error {
		source := args[0]
		if source == *output {
			return fmt.Errorf("source and output can't be the same")
		}

		builder, err := NewBuilder()
		if err != nil {
			return fmt.Errorf("error constructing builder: %w", err)
		}
		if err := builder.Build(source, *output, *config, *minifyOutput, *includeDrafts); err != nil {
			return fmt.Errorf("error executing build: %w", err)
		}
		return nil
	}

	return buildCmd
}

func generateServeCmd() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:        "serve directory",
		Short:      "Serves the directory via HTTP using a basic webserver.",
		Example:    "serve ./example-output",
		SuggestFor: []string{"run"},
		Args:       cobra.ExactArgs(1),
	}
	basepath := serveCmd.Flags().StringP("basepath", "b", "", "Defines the path at which the directory is served. (For example /hello for http://localhost:8080/hello).")
	port := serveCmd.Flags().IntP("port", "p", 8080, "Decides which port the HTTP server is run on.")
	serveCmd.Run = func(cmd *cobra.Command, args []string) {
		if err := serve(args[0], *basepath, *port); err != nil {
			log.Println("Error serving directory:", err)
		}
	}

	return serveCmd
}
