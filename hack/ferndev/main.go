package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	"github.com/sttts/kubectl-doc/internal/render/webschema"
)

type manifest struct {
	Title   string            `json:"title"`
	Schemas []schemaReference `json:"schemas"`
}

type schemaReference struct {
	Label string                    `json:"label"`
	Data  webschema.DocumentPayload `json:"data"`
}

func main() {
	crdPath := flag.String("crd", "", "CRD manifest used for the local Fern preview fixture")
	outDir := flag.String("out", "", "directory for local Fern preview schema files")
	flag.Parse()

	if *crdPath == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "--crd and --out are required")
		os.Exit(2)
	}
	if err := run(*crdPath, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "generate Fern dev fixture: %v\n", err)
		os.Exit(1)
	}
}

func run(crdPath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	docs, err := crd.LoadAllVersions([]string{crdPath})
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return fmt.Errorf("CRD has no served versions")
	}

	out := manifest{
		Title: docs[0].Kind,
	}
	for i, doc := range docs {
		full := webschema.Build(doc, webschema.Options{
			ExpandDepth:    3,
			FullDepth:      webschema.DefaultFullExpandDepth,
			Descriptions:   tree.DescriptionTrue,
			Columns:        100,
			RenderStatus:   true,
			RenderMetadata: true,
		})
		filename := fmt.Sprintf("%s-schema-%d-full.json", slug(doc.Kind), i)
		if err := os.WriteFile(filepath.Join(outDir, filename), jsonCompact(full), 0o644); err != nil {
			return err
		}

		out.Schemas = append(out.Schemas, schemaReference{
			Label: webschema.APIVersion(doc.Group, doc.Version),
			Data:  webschema.Shallow(full, "./schemas/"+filename),
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "manifest.json"), append(data, '\n'), 0o644)
}

func jsonCompact(value interface{}) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("null")
	}
	return data
}

func slug(value string) string {
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if out.Len() > 0 && unicode.IsUpper(r) && !lastDash {
				out.WriteByte('-')
			}
			out.WriteRune(unicode.ToLower(r))
			lastDash = false
			continue
		}
		if out.Len() > 0 && !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
