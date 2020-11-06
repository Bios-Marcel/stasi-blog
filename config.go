package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
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

	if deleteError := os.RemoveAll(absoluteOutputPath); deleteError != nil && !os.IsNotExist(deleteError) {
		panic(deleteError)
	}

	createEmptyDirectory(absoluteOutputPath)
	createEmptyDirectory(filepath.Join(absoluteOutputPath, outputCustomPages))
	createEmptyDirectory(filepath.Join(absoluteOutputPath, outputArticles))
}
