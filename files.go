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

func createFile(path string) *os.File {
	_, statError := os.Stat(path)
	if statError == nil {
		os.Remove(path)
	}
	file, fileError := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0400)
	if fileError != nil {
		exitWithError(fmt.Sprintf("Couldn't create file '%s'", path), fileError.Error())
	}

	return file
}

func writeTemplateToFile(sourceTemplate *template.Template, data interface{}, outputFolder, path string, minifyOutput bool) {
	filePath := filepath.Join(outputFolder, path)
	var file io.Writer = createFile(filePath)
	if minifyOutput {
		//minify.Writer sadly doesn't work, the files end up empty.
		templateBuffer := &bytes.Buffer{}
		executeError := sourceTemplate.Execute(templateBuffer, data)
		if executeError != nil {
			exitWithError(fmt.Sprintf("Couldn't execute template '%s'", sourceTemplate.Name()), executeError.Error())
		}

		minifyError := minifier.Minify("text/html", file, templateBuffer)
		if minifyError != nil {
			exitWithError(fmt.Sprintf("Couldn't minify file '%s'", filePath), minifyError.Error())
		}
	} else {
		executeError := sourceTemplate.Execute(file, data)
		if executeError != nil {
			exitWithError(fmt.Sprintf("Couldn't execute template '%s'", sourceTemplate.Name()), executeError.Error())
		}
	}
}

func copyDataIntoFile(source io.Reader, targetPath string) {
	target := createFile(targetPath)
	defer target.Close()

	_, copyError := io.Copy(target, source)
	if copyError != nil {
		exitWithError(fmt.Sprintf("Couldn't copy data into file '%s'", targetPath), copyError.Error())
	}
	target.Close()
}

func copyFileByPath(sourcePath, targetPath string) {
	source, openError := os.Open(sourcePath)
	if openError != nil {
		exitWithError(fmt.Sprintf("Couldn't copy file '%s' to '%s'", sourcePath, targetPath), openError.Error())
	}
	defer source.Close()
	copyDataIntoFile(source, targetPath)
}

func createDirectory(path string) {
	_, statError := os.Stat(path)
	if statError != nil {
		if os.IsNotExist(statError) {
			mkDirError := os.MkdirAll(path, 0755)
			if mkDirError != nil {
				exitWithError(fmt.Sprintf("Couldn't create directory '%s'", path), mkDirError.Error())
			}
		} else {
			exitWithError(fmt.Sprintf("Couldn't create directory '%s'", path), statError.Error())
		}
	}
}
