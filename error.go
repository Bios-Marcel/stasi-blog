package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

func exitWithError(message, reason string) {
	fmt.Printf("Error: %s:\n\t%s\n", message, reason)
	fmt.Println("For more information, execute the previous command with --verbose")
	fmt.Println("Please include that information in a bug report if necessary.")
	if *verbose {
		fmt.Println(string(debug.Stack()))
	}
	os.Exit(1)
}

func showWarning(message string) {
	fmt.Printf("Warning: %s\n", message)
}

func showInfo(message string) {
	fmt.Printf("Info: %s\n", message)
}
