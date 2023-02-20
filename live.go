package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

func live(sourceFolder, basepath string, config string, port int) error {
	// Initial build
	target := "./.tmp"
	if err := build(sourceFolder, target, config, false); err != nil {
		// We don't return an error here, since the user can simply try
		// fixing the issue, causing the watcher to automatically rebuild.
		log.Println("Error rebuilding:", err)
	}

	// Then watch for changes and rebuild.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Start listening for events.
	go func() {
		debouncer := debounce.New(100 * time.Millisecond)
		for {
			select {
			case event, channelOpen := <-watcher.Events:
				if !channelOpen {
					return
				}

				// Permission changes do not affect the build
				// outcome, therefore we ignore these types of events.
				if event.Op == fsnotify.Chmod {
					continue
				}

				// Some editors or IDEs migth write multiple times,
				// therefore we debounce, to prevent unnecessar lag.
				// Another scenario where this might be useful, are
				// for example reformats or search and replace invocations
				// on the whole codebase.
				debouncer(func() {
					log.Println("Rebuilding ...", event)
					now := time.Now()
					if err := build(sourceFolder, target, config, false); err != nil {
						log.Println("Error rebuilding:", err)
					} else {
						log.Printf("Rebuild successful. (%s)\n", time.Since(now).String())
					}
				})
			case err, channelOpen := <-watcher.Errors:
				if !channelOpen {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = filepath.WalkDir(
		sourceFolder,
		func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if dirEntry.IsDir() {
				if err := watcher.Add(path); err != nil {
					return err
				}
			}

			return nil
		})
	if err != nil {
		return err
	}

	return serve(target, basepath, port)
}
