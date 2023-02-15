package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func serve(directoryToServe, basepath string, port int) error {
	//Example in my case.
	//go run . --input="../blog-test-source" --output="../blog-test" & go run demo/server.go --dir="../blog-test" --basepath="/blog-test/"

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
		<-sc
		os.Exit(0)
	}()

	log.Printf("Serving %s at localhost:%d%s", directoryToServe, port, basepath)
	log.Println("Please remember to only use this command to serve your website in a development scenario.")
	portString := fmt.Sprintf(":%d", port)

	dir := dirWith404Handler{http.Dir(directoryToServe)}
	if basepath == "" {
		return http.ListenAndServe(portString, http.FileServer(dir))
	}
	//Making sure there's not too many or too little slashes ;)
	basepath = "/" + strings.Trim(basepath, "/\\") + "/"
	http.Handle(basepath, http.StripPrefix(basepath, http.FileServer(dir)))
	return http.ListenAndServe(portString, nil)
}

type dirWith404Handler struct {
	dir http.Dir
}

// Open implements FileSystem using os.Open, opening files for reading rooted
// and relative to the directory d. If a file can't be found, we return a 404
// page instead.
func (d dirWith404Handler) Open(name string) (http.File, error) {
	file, err := d.dir.Open(name)
	if os.IsNotExist(err) {
		file404, err := d.dir.Open("404.html")
		//Technically we'd need the old error to indicate 404 to the
		//browser, but for demo/test purposes, this'll do.
		return file404, err
	}
	return file, err
}
