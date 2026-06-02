package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN GENERATED GITHUB MARKDOWN EXAMPLE -->"
	endMarker   = "<!-- END GENERATED GITHUB MARKDOWN EXAMPLE -->"
)

func main() {
	readmePath := flag.String("readme", "README.md", "README file to update")
	examplePath := flag.String("example", "docs/examples/github-crontab.md", "generated Markdown example")
	flag.Parse()

	if err := updateReadme(*readmePath, *examplePath); err != nil {
		fmt.Fprintf(os.Stderr, "readmegen: %v\n", err)
		os.Exit(1)
	}
}

func updateReadme(readmePath, examplePath string) error {
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", readmePath, err)
	}
	example, err := os.ReadFile(examplePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", examplePath, err)
	}

	updated, err := replaceGeneratedBlock(string(readme), string(example))
	if err != nil {
		return err
	}
	if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", readmePath, err)
	}
	return nil
}

func replaceGeneratedBlock(readme, example string) (string, error) {
	begin := strings.Index(readme, beginMarker)
	if begin < 0 {
		return "", fmt.Errorf("missing marker %q", beginMarker)
	}

	afterBegin := begin + len(beginMarker)
	end := strings.Index(readme[afterBegin:], endMarker)
	if end < 0 {
		return "", fmt.Errorf("missing marker %q", endMarker)
	}
	end += afterBegin

	example = strings.TrimRight(example, "\n")
	updated := readme[:afterBegin] + "\n" + example + "\n" + readme[end:]
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return updated, nil
}
