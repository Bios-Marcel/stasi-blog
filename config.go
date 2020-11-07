package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var input, output *string

func init() {
	output = flag.String("output", "output", "defines the output folder")
	input = flag.String("input", "input", "defines the input folder")
	flag.Parse()

	if *output == *input {
		log.Fatalln("Output and input can't be the same.")
	}

	cleanAndPrepareOutputDirectory()
}

func cleanAndPrepareOutputDirectory() {
	absoluteOutputPath, absError := filepath.Abs(*output)
	if absError != nil {
		panic(absError)
	}

	_, statError := os.Stat(absoluteOutputPath)
	if statError != nil && !os.IsNotExist(statError) {
		filepath.Walk(absoluteOutputPath, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			//Dotfiles are generally being ignored.
			if strings.Contains(path, "/.") {
				return nil
			}

			return os.Remove(path)
		})
	}

	createEmptyDirectory(absoluteOutputPath)
	createEmptyDirectory(filepath.Join(absoluteOutputPath, outputCustomPages))
	createEmptyDirectory(filepath.Join(absoluteOutputPath, outputArticles))
}
