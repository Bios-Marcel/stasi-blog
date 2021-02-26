package main

import (
	"bytes"
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

func createFile(path string) *os.File {
	_, statError := os.Stat(path)
	if statError == nil {
		os.Remove(path)
	}
	file, fileError := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0400)
	if fileError != nil {
		panic(fileError)
	}

	return file
}

func writeTemplateToFile(sourceTemplate *template.Template, data interface{}, outputFolder, path string, minifyOutput bool) {
	var file io.Writer = createFile(filepath.Join(outputFolder, path))
	if minifyOutput {
		//minify.Writer sadly doesn't work, the files end up empty.
		templateBuffer := &bytes.Buffer{}
		executeError := sourceTemplate.Execute(templateBuffer, data)
		if executeError != nil {
			panic(executeError)
		}

		minifyError := minifier.Minify("text/html", file, templateBuffer)
		if minifyError != nil {
			panic(minifyError)
		}
	} else {
		executeError := sourceTemplate.Execute(file, data)
		if executeError != nil {
			panic(executeError)
		}
	}
}

func copyDataIntoFile(source io.Reader, targetPath string) {
	target := createFile(targetPath)
	defer target.Close()

	_, copyError := io.Copy(target, source)
	if copyError != nil {
		panic(copyError)
	}
	target.Close()
}

func copyFileByPath(sourcePath, targetPath string) {
	source, openError := os.Open(sourcePath)
	if openError != nil {
		panic(openError)
	}
	defer source.Close()
	copyDataIntoFile(source, targetPath)
}

func createDirectory(path string) {
	_, statError := os.Stat(path)
	if statError != nil {
		if os.IsNotExist(statError) {
			articlesMkdirError := os.MkdirAll(path, 0755)
			if articlesMkdirError != nil {
				panic(articlesMkdirError)
			}
		} else {
			panic(statError)
		}
	}
}
