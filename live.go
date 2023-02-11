package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

func live(sourceFolder, basepath string, config string, port int) {
	// Initial build
	target := "./.tmp"
	build(sourceFolder, target, config, false)

	// Then watch for changes and rebuild.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start listening for events.
	go func() {
		debouncer := debounce.New(100 * time.Millisecond)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op != fsnotify.Chmod {
					// Some editors or IDEs migth write multiple times,
					// therefore we debounce, to prevent unnecessar lag.
					// Another scenario where this might be useful, are
					// for example reformats or search and replace invocations
					// on the whole codebase.
					debouncer(func() {
						log.Println("Rebuilding ...", event)
						now := time.Now()
						build(sourceFolder, target, config, false)
						log.Printf("Rebuild done. (%s)\n", time.Since(now).String())
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = filepath.WalkDir(sourceFolder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}

	serve(target, basepath, port)

}
