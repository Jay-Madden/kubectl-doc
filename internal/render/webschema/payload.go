package webschema

import (
	"fmt"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/fielddetail"
	"github.com/sttts/kubectl-doc/internal/render/tree"
)

const DefaultFullExpandDepth = 1000

type DescriptionMode = tree.DescriptionMode

type Options struct {
	ExpandDepth    int
	FullDepth      int
	Descriptions   DescriptionMode
	Columns        int
	RenderStatus   bool
	RenderMetadata bool
}

type DocumentPayload struct {
	APIVersion string         `json:"apiVersion"`
	Group      string         `json:"group"`
	Version    string         `json:"version"`
	Kind       string         `json:"kind"`
	Resource   string         `json:"resource,omitempty"`
	Complete   bool           `json:"complete"`
	FullURL    string         `json:"fullPayloadURL,omitempty"`
	Lines      []LinePayload  `json:"lines"`
	Fields     []FieldPayload `json:"fields"`
}

type LinePayload struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	Depth     int    `json:"depth"`
	Field     string `json:"field,omitempty"`
	Path      string `json:"path,omitempty"`
	Code      bool   `json:"code,omitempty"`
	Metadata  bool   `json:"metadata,omitempty"`
	Required  bool   `json:"required,omitempty"`
	Foldable  bool   `json:"foldable,omitempty"`
	Collapsed bool   `json:"collapsed,omitempty"`
	DetailID  string `json:"detailId,omitempty"`
}

type FieldPayload struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Metadata    []string `json:"metadata,omitempty"`
}

func Build(doc *crd.Document, opts Options) DocumentPayload {
	fullDepth := opts.FullDepth
	if fullDepth <= 0 {
		fullDepth = DefaultFullExpandDepth
	}

	lines := tree.WithCollapsed(tree.Build(doc, tree.Options{
		ExpandDepth:    fullDepth,
		Descriptions:   opts.Descriptions,
		Columns:        opts.Columns,
		RenderStatus:   opts.RenderStatus,
		RenderMetadata: opts.RenderMetadata,
	}), initialExpandDepth(opts.ExpandDepth))
	details := fielddetail.ByPath(doc)

	payload := DocumentPayload{
		APIVersion: APIVersion(doc.Group, doc.Version),
		Group:      doc.Group,
		Version:    doc.Version,
		Kind:       doc.Kind,
		Resource:   doc.Plural,
		Complete:   true,
		Fields:     FieldPayloads(fielddetail.Collect(doc)),
	}
	for _, line := range lines {
		payload.Lines = append(payload.Lines, LinePayloadForTreeLine(line, details))
	}
	return payload
}

func Shallow(full DocumentPayload, fullURL string) DocumentPayload {
	shallow := full
	shallow.Complete = false
	shallow.FullURL = fullURL
	shallow.Lines = VisibleLines(full.Lines)
	shallow.Fields = ReferencedFields(full.Fields, shallow.Lines)
	return shallow
}

func VisibleLines(lines []LinePayload) []LinePayload {
	var visible []LinePayload
	var collapsedDepths []int
	for _, line := range lines {
		if strings.TrimSpace(line.Text) == "" && len(collapsedDepths) > 0 {
			continue
		}
		for len(collapsedDepths) > 0 && line.Depth <= collapsedDepths[len(collapsedDepths)-1] {
			collapsedDepths = collapsedDepths[:len(collapsedDepths)-1]
		}
		if len(collapsedDepths) == 0 {
			visible = append(visible, line)
		}
		if line.Foldable && line.Collapsed {
			collapsedDepths = append(collapsedDepths, line.Depth)
		}
	}
	return visible
}

func ReferencedFields(fields []FieldPayload, lines []LinePayload) []FieldPayload {
	referenced := map[string]bool{}
	for _, line := range lines {
		if line.DetailID != "" {
			referenced[line.DetailID] = true
		}
	}
	var out []FieldPayload
	for _, field := range fields {
		if referenced[field.ID] {
			out = append(out, field)
		}
	}
	return out
}

func LinePayloadForTreeLine(line tree.Line, details map[string]fielddetail.Field) LinePayload {
	payload := LinePayload{
		Index:     line.Index,
		Text:      line.Text,
		Depth:     line.Depth,
		Field:     line.Field,
		Path:      line.Path,
		Code:      line.Code,
		Metadata:  line.Metadata,
		Required:  line.Required,
		Foldable:  line.Foldable,
		Collapsed: line.Collapsed,
	}
	if detail, ok := details[line.Path]; ok {
		payload.DetailID = detail.ID
	} else {
		payload.DetailID = fmt.Sprintf("line-%d", line.Index)
	}
	return payload
}

func FieldPayloads(fields []fielddetail.Field) []FieldPayload {
	payloads := make([]FieldPayload, 0, len(fields))
	for _, field := range fields {
		payloads = append(payloads, FieldPayload{
			ID:          field.ID,
			Path:        field.Path,
			Type:        field.Type,
			Required:    field.Required,
			Description: field.Description,
			Metadata:    field.Metadata,
		})
	}
	return payloads
}

func APIVersion(group, version string) string {
	if group == "" || group == "core" {
		return version
	}
	return group + "/" + version
}

func initialExpandDepth(depth int) int {
	if depth < 0 {
		return 0
	}
	return depth
}
