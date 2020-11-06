package main

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
)

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

func writeTemplateToFile(sourceTemplate *template.Template, data interface{}, path string) {
	file := createFile(filepath.Join(*output, path))
	executeError := sourceTemplate.Execute(file, data)
	if executeError != nil {
		panic(executeError)
	}

}

func copyFile(sourcePath, targetPath string) {
	source, openError := os.Open(sourcePath)
	if openError != nil {
		panic(openError)
	}
	defer source.Close()

	target := createFile(targetPath)
	defer target.Close()

	_, copyError := io.Copy(target, source)
	if copyError != nil {
		panic(copyError)
	}
	target.Close()
}

func createEmptyDirectory(path string) {
	_, statError := os.Stat(path)
	if statError != nil {
		if os.IsNotExist(statError) {
			articlesMkdirError := os.Mkdir(path, 0755)
			if articlesMkdirError != nil {
				panic(articlesMkdirError)
			}
		} else {
			panic(statError)
		}
	}
}
