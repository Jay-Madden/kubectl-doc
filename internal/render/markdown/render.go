package markdownrender

import (
	"bytes"
	"fmt"
	"io"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

type Dialect string

const (
	DialectGitHub Dialect = "markdown-github"
	DialectFern   Dialect = "markdown-fern"
)

type Renderer struct {
	Dialect      Dialect
	ExpandDepth  int
	Descriptions yamlrender.DescriptionMode
	Columns      int
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	var yaml bytes.Buffer
	if err := (yamlrender.Renderer{
		ExpandDepth:  r.ExpandDepth,
		Descriptions: r.Descriptions,
		Columns:      r.Columns,
	}).Render(&yaml, doc); err != nil {
		return err
	}

	dialect := r.Dialect
	if dialect == "" {
		dialect = DialectGitHub
	}
	if dialect == DialectFern {
		if _, err := fmt.Fprintf(out, "---\ntitle: %s\n---\n\n", doc.Kind); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "# %s\n\n", doc.Kind); err != nil {
		return err
	}
	if err := renderMetadata(out, doc); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "\n## YAML\n\n```yaml\n%s```\n", yaml.String())
	return err
}

func renderMetadata(out io.Writer, doc *crd.Document) error {
	rows := [][2]string{
		{"API Version", apiVersion(doc.Group, doc.Version)},
		{"Kind", doc.Kind},
	}
	if doc.Plural != "" {
		rows = append(rows, [2]string{"Resource", doc.Plural})
	}

	if _, err := fmt.Fprintln(out, "| Field | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "| --- | --- |"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "| %s | `%s` |\n", row[0], row[1]); err != nil {
			return err
		}
	}
	return nil
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}
