package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var output, basepath *string

func init() {
	output = flag.String("dir", "dir", "defines the directory that is being served")
	basepath = flag.String("basepath", "", "specify basepath to simulate remote setup where files aren't at root")
	flag.Parse()
}

func main() {
	//Example in my case.
	//go run . --input="../blog-test-source" --output="../blog-test" & go run demo/server.go --dir="../blog-test" --basepath="/blog-test/"

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-sc
		os.Exit(0)
	}()

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
