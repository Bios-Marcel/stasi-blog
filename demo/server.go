package main

import (
	"flag"
	"log"
	"net/http"
)

var output, basepath *string

func init() {
	output = flag.String("output", "output", "defines the output folder")
	basepath = flag.String("basepath", "", "specify basepath to simulate remote setup where files aren't at root")
	flag.Parse()
}

func main() {
	//Example in my case.
	//go run . --input="../blog-test-source" --output="../blog-test" & go run demo/server.go --output="../blog-test" --basepath="/blog-test/"

	if *basepath == "" {
		log.Println("Serving " + *output + " at localhost:8080")
		fs := http.FileServer(http.Dir(*output))
		log.Fatal(http.ListenAndServe(":8080", fs))
	} else {
		log.Println("Serving " + *output + " at localhost:8080" + *basepath)
		http.Handle(*basepath, http.StripPrefix(*basepath, http.FileServer(http.Dir(*output))))
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
