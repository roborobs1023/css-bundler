package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	srcPtr := flag.String("src", "./src/js", "Source directory")
	outPtr := flag.String("out", "./dist/bundle.js", "Output file path")
	// doMinify := flag.Bool("minify", false, "Minify the output")

	flag.Parse()

	// Clean paths to avoid trailing slash issues
	srcDir := filepath.Clean(*srcPtr)
	outputFile := filepath.Clean(*outPtr)

	fmt.Printf("🔍 Scanning: %s\n", srcDir)

	// Create or truncate the output file
	out, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating bundle file: %v\n", err)
		return
	}
	defer out.Close()

	priorityFile := "index.css" // The file that MUST be first

	// 1. Manually process the priority file first
	priorityPath := filepath.Join(srcDir, priorityFile)
	if _, err := os.Stat(priorityPath); err == nil {
		fmt.Printf("🔝 Priority First: %s\n", priorityPath)
		appendFile(priorityPath, out)
	}

	// 2. Walk the directory for everything else
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Conditions to bundle:
		// - Must be .css
		// - Must NOT be the output file
		// - Must NOT be the priority file we already handled
		isCSS := strings.HasSuffix(info.Name(), ".css")
		isNotOutput := info.Name() != outputFile
		isNotPriority := info.Name() != priorityFile

		if isCSS && isNotOutput && isNotPriority {
			fmt.Printf("📦 Bundling: %s\n", path)
			return appendFile(path, out)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Walk error: %v\n", err)
	}
}

// Helper function to keep the main loop clean
func appendFile(sourcePath string, destination *os.File) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer f.Close()

	header := fmt.Sprintf("\n/* --- Source: %s --- */\n", sourcePath)
	destination.WriteString(header)

	_, err = io.Copy(destination, f)
	return err
}
