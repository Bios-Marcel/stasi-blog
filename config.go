package main

import (
	"flag"
	"log"
)

var input, output, config *string
var minifyOutput *bool

func init() {
	output = flag.String("output", "output", "defines the output folder")
	input = flag.String("input", "", "defines the input folder")
	config = flag.String("config", "", "defines the config file location")
	minifyOutput = flag.Bool("minify", false, "defines whether css and html will be minified")
	flag.Parse()

	if *output == *input {
		log.Fatalln("Output and input can't be the same.")
	}
}
