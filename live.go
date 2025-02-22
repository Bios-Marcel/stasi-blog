package main

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

func live(sourceDir, basepath, configPath string, port int, minifyOutput, includeDrafts bool) error {
	// Initial build
	target := "./.tmp"

	builder, err := NewBuilder()
	if err != nil {
		return fmt.Errorf("error constructing builder: %w", err)
	}

	build := func() error {
		return builder.Build(sourceDir, target, configPath, minifyOutput, includeDrafts)
	}
	if err := build(); err != nil {
		// We don't return an error here, since the user can simply try
		// fixing the issue, causing the watcher to automatically rebuild.
		log.Println("Error running initial build:", err)
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

					buildStartTime := time.Now()
					if err := build(); err != nil {
						log.Println("Error rebuilding:", err)
						return
					}

					log.Printf("Rebuild successful. (%s)\n", time.Since(buildStartTime).String())
				})
			case err, channelOpen := <-watcher.Errors:
				if !channelOpen {
					return
				}
				log.Println("watcher error:", err)
			}
		}
	}()

	err = filepath.WalkDir(
		sourceDir,
		func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !dirEntry.IsDir() {
				return nil
			}

			return watcher.Add(path)
		})
	if err != nil {
		return err
	}

	return serve(target, basepath, port)
}
