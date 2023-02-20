package main

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed example
var exampleFS embed.FS

func initDir(directory string) error {
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}

	return fs.WalkDir(exampleFS, "example", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "example" {
			return nil
		}

		destPath := filepath.Join(directory, strings.TrimPrefix(path, "example"))

		if d.IsDir() {
			return os.Mkdir(destPath, 0o700)
		}

		file, err := exampleFS.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer dest.Close()

		_, err = io.Copy(dest, file)
		if err != nil {
			return err
		}

		return nil
	})
}
