package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	minify "github.com/tdewolff/minify/v2"
	cssminify "github.com/tdewolff/minify/v2/css"
	htmlminify "github.com/tdewolff/minify/v2/html"
)

var minifier = minify.New()

func init() {
	minifier.AddFunc("text/css", cssminify.Minify)
	minifier.Add("text/html", &htmlminify.Minifier{
		KeepDocumentTags: true,
		KeepQuotes:       true,
		KeepEndTags:      true,
	})
}

func createFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
}

func writeTemplateToFile(
	template *template.Template,
	templateData any,
	outputDir, path string,
	minifyOutput bool,
) error {
	file, err := createFile(filepath.Join(outputDir, path))
	if err != nil {
		return err
	}
	defer file.Close()

	if minifyOutput {
		// minify.Writer sadly doesn't work, the files end up empty.
		templateBuffer := &bytes.Buffer{}
		if err := template.Execute(templateBuffer, templateData); err != nil {
			return fmt.Errorf("error executing template '%s': %w", template.Name(), err)
		}

		if err := minifier.Minify("text/html", file, templateBuffer); err != nil {
			return fmt.Errorf("error minifying template '%s': %w", template.Name(), err)
		}
	} else {
		if err := template.Execute(file, templateData); err != nil {
			return fmt.Errorf("error executing template '%s': %w", template.Name(), err)
		}
	}

	return nil
}

func copyDataIntoFile(source io.Reader, targetPath string) error {
	target, err := createFile(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

func copyFileByPath(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	return copyDataIntoFile(source, targetPath)
}

func createDirectories(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func removeAll(paths ...string) error {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}
