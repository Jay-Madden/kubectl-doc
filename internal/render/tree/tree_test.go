package tree

import (
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestBuildLinesCarrySchemaMetadata(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Spec configures the widget.",
					},
					Properties: map[string]docschema.Structural{
						"management_clusters": {
							Generic: docschema.Generic{Type: "array"},
							Items: &docschema.Structural{
								Generic: docschema.Generic{Type: "object"},
								Properties: map[string]docschema.Structural{
									"cluster_location": {
										Generic: docschema.Generic{Type: "string"},
									},
									"cluster_name": {
										Generic: docschema.Generic{Type: "string"},
									},
								},
							},
						},
						"template": {
							Generic: docschema.Generic{Type: "object"},
							Properties: map[string]docschema.Structural{
								"image": {
									Generic: docschema.Generic{Type: "string"},
								},
							},
							ValueValidation: &docschema.ValueValidation{
								Required: []string{"image"},
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{
						Required: []string{"template"},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  3,
		Descriptions: DescriptionTrue,
	})
	spec, ok := findLine(lines, "spec")
	if !ok {
		t.Fatalf("expected spec line, got %#v", lines)
	}
	if !spec.Foldable || !spec.Required {
		t.Fatalf("expected required foldable spec line, got %#v", spec)
	}

	specDescription, ok := findText(lines, "# Spec configures the widget.")
	if !ok {
		t.Fatalf("expected spec description line, got %#v", lines)
	}
	if specDescription.Path != "spec" || specDescription.Field != "" {
		t.Fatalf("expected description to share spec path without becoming a field, got %#v", specDescription)
	}
	if specDescription.Description != "Spec configures the widget." {
		t.Fatalf("expected structured description metadata, got %#v", specDescription)
	}

	location, ok := findLine(lines, "spec.management_clusters[].cluster_location")
	if !ok {
		t.Fatalf("expected cluster_location line, got %#v", lines)
	}
	name, ok := findLine(lines, "spec.management_clusters[].cluster_name")
	if !ok {
		t.Fatalf("expected cluster_name line, got %#v", lines)
	}
	if location.Field != "cluster_location" || !strings.Contains(location.Text, `# - cluster_location: "<string>"`) {
		t.Fatalf("expected commented array item field, got %#v", location)
	}
	if location.Foldable {
		t.Fatalf("did not expect scalar array item field to be foldable, got %#v", location)
	}
	if location.Depth != name.Depth {
		t.Fatalf("expected array item sibling depths to match, got %d and %d", location.Depth, name.Depth)
	}
}

func TestWrapInlineCommentTextAlignsContinuationComment(t *testing.T) {
	wrapped := WrapInlineCommentText(`stopSignal: "SIGABRT" # enum: "SIGALRM" | "SIGBUS" | "SIGCHLD"`, true, 45)
	if len(wrapped) < 2 {
		t.Fatalf("expected inline comment to wrap, got %#v", wrapped)
	}
	firstHash := strings.Index(wrapped[0].Text, "#")
	secondHash := strings.Index(wrapped[1].Text, "#")
	if firstHash < 0 || secondHash < 0 || firstHash != secondHash {
		t.Fatalf("expected continuation # column %d to match first # column %d, got %#v", secondHash, firstHash, wrapped)
	}
	if !wrapped[0].Code || wrapped[0].Metadata {
		t.Fatalf("expected first wrapped line to stay code, got %#v", wrapped[0])
	}
	if wrapped[1].Code || !wrapped[1].Metadata {
		t.Fatalf("expected continuation line to be metadata comment, got %#v", wrapped[1])
	}
}

func TestBuildRendersMetadataFieldDescriptions(t *testing.T) {
	doc := &crd.Document{
		Group:      "example.io",
		Version:    "v1",
		Kind:       "Widget",
		Namespaced: true,
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:    2,
		Descriptions:   DescriptionTrue,
		RenderMetadata: true,
	})
	name, ok := findLine(lines, "metadata.name")
	if !ok {
		t.Fatalf("expected metadata.name field, got %#v", Texts(lines))
	}
	if name.Description != "Name must be unique within a namespace." {
		t.Fatalf("expected structured metadata description, got %#v", name)
	}
	if _, ok := findText(lines, "# Name must be unique within a namespace."); !ok {
		t.Fatalf("expected metadata child field descriptions in YAML text, got %#v", Texts(lines))
	}
	metadata, ok := findLine(lines, "metadata")
	if !ok {
		t.Fatalf("expected metadata field, got %#v", Texts(lines))
	}
	if metadata.Description != "Standard Kubernetes object metadata." {
		t.Fatalf("expected metadata details description, got %#v", metadata)
	}
	for _, text := range Texts(lines) {
		if strings.TrimSpace(text) == "# Standard Kubernetes object metadata." {
			t.Fatalf("top metadata description should be details-only in YAML text, got %#v", Texts(lines))
		}
	}
}

func TestBuildRendersRootDescriptionBeforeTypeMeta(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Generic: docschema.Generic{
				Type:        "object",
				Description: "Widget describes the root object.",
			},
			Properties: map[string]docschema.Structural{},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  2,
		Descriptions: DescriptionTrue,
	})
	texts := Texts(lines)
	if len(texts) < 2 {
		t.Fatalf("expected root description and type metadata, got %#v", texts)
	}
	if texts[0] != "# Widget describes the root object." || !strings.HasPrefix(texts[1], "apiVersion: example.io/v1") {
		t.Fatalf("expected root description above apiVersion, got %#v", texts[:min(len(texts), 3)])
	}
	if lines[0].Path != "" || lines[0].Field != "" || lines[0].Description != "Widget describes the root object." || !lines[0].RootDescription {
		t.Fatalf("expected root description to stay pathless metadata, got %#v", lines[0])
	}
}

func TestBuildMarksDescriptionCommentParagraphs(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type: "object",
						Description: "" +
							"First paragraph has enough words to wrap across several generated comment lines.\n\n" +
							"Second paragraph also wraps and must not share the first paragraph group.",
					},
				},
			},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  1,
		Descriptions: DescriptionTrue,
		Columns:      34,
	})

	var specComments []Line
	for _, line := range lines {
		if line.Path == "spec" && line.Field == "" {
			specComments = append(specComments, line)
		}
	}
	if len(specComments) < 5 {
		t.Fatalf("expected wrapped comments for two paragraphs, got %#v", specComments)
	}
	if specComments[0].CommentGroup == "" || specComments[0].CommentGroup != specComments[1].CommentGroup {
		t.Fatalf("expected first paragraph lines to share a comment group, got %#v", specComments[:2])
	}
	blankIndex := -1
	for i, line := range specComments {
		if line.Text == "#" {
			blankIndex = i
			break
		}
	}
	if blankIndex < 2 || blankIndex >= len(specComments)-1 || specComments[blankIndex].CommentGroup != "" {
		t.Fatalf("expected blank comment line to split paragraphs, got index=%d comments=%#v", blankIndex, specComments)
	}
	if specComments[blankIndex+1].CommentGroup == "" || specComments[blankIndex+1].CommentGroup == specComments[0].CommentGroup {
		t.Fatalf("expected second paragraph to have a distinct comment group, got %#v", specComments)
	}
}

func TestBuildHidesRootDescriptionWhenDescriptionsDisabled(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Generic: docschema.Generic{
				Type:        "object",
				Description: "Widget describes the root object.",
			},
			Properties: map[string]docschema.Structural{},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  2,
		Descriptions: DescriptionFalse,
	})
	texts := Texts(lines)
	if len(texts) == 0 || !strings.HasPrefix(texts[0], "apiVersion: example.io/v1") {
		t.Fatalf("expected apiVersion first when descriptions are disabled, got %#v", texts[:min(len(texts), 3)])
	}
	if _, ok := findText(lines, "# Widget describes the root object."); ok {
		t.Fatalf("did not expect root description when descriptions are disabled, got %#v", texts)
	}
}

func TestBuildRendersTypeMetaAsSelectableTopFields(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  2,
		Descriptions: DescriptionTrue,
	})
	apiVersion, ok := findLine(lines, "apiVersion")
	if !ok {
		t.Fatalf("expected selectable apiVersion field, got %#v", Texts(lines))
	}
	if !strings.Contains(apiVersion.Text, "apiVersion: example.io/v1") {
		t.Fatalf("expected apiVersion to render the document version, got %#v", apiVersion)
	}
	if apiVersion.Description == "" || !apiVersion.Required {
		t.Fatalf("expected apiVersion details metadata, got %#v", apiVersion)
	}

	kind, ok := findLine(lines, "kind")
	if !ok {
		t.Fatalf("expected selectable kind field, got %#v", Texts(lines))
	}
	if kind.Index != apiVersion.Index+1 || !strings.Contains(kind.Text, "kind: Widget") {
		t.Fatalf("expected kind to immediately follow apiVersion, got apiVersion=%#v kind=%#v", apiVersion, kind)
	}
	if kind.Description == "" || !kind.Required {
		t.Fatalf("expected kind details metadata, got %#v", kind)
	}
	for _, text := range Texts(lines) {
		if strings.Contains(text, "APIVersion defines") || strings.Contains(text, "Kind is a string") {
			t.Fatalf("type metadata descriptions should be details-only in YAML text, got %#v", Texts(lines))
		}
	}
}

func findLine(lines []Line, path string) (Line, bool) {
	for _, line := range lines {
		if line.Path == path && line.Field != "" {
			return line, true
		}
	}
	return Line{}, false
}

func findText(lines []Line, text string) (Line, bool) {
	for _, line := range lines {
		if strings.TrimSpace(line.Text) == text {
			return line, true
		}
	}
	return Line{}, false
}
