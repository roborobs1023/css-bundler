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

	// Walk the directory recursively
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process files ending in .css and skip the output file if it's in the same dir
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".css") && info.Name() != outputFile {
			fmt.Printf("📦 Bundling: %s\n", path)

			// Write a comment header to the bundle for easier debugging
			header := fmt.Sprintf("\n/* --- Source: %s --- */\n", path)
			if _, err := out.WriteString(header); err != nil {
				return err
			}

			// Open the source CSS file
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			// Stream the content directly to the output file
			if _, err := io.Copy(out, f); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path: %v\n", err)
		return
	}

	fmt.Println("\n✨ Success! Your raw CSS is bundled and ready to go.")
}
