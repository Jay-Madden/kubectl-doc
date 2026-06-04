package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestModelNavigatesFieldsAndFolds(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	if model.FocusPath() != "apiVersion" {
		t.Fatalf("expected initial focus on apiVersion, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyDown})
	if model.FocusPath() != "kind" {
		t.Fatalf("down should move to kind, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyDown})
	if model.FocusPath() != "metadata" {
		t.Fatalf("down should move to metadata, got %q", model.FocusPath())
	}
	if !model.IsCollapsed("metadata") {
		t.Fatalf("expected metadata to start collapsed")
	}

	model = press(model, tea.Key{Code: tea.KeyRight})
	if model.FocusPath() != "metadata" {
		t.Fatalf("right should keep focus while expanding collapsed metadata, got %q", model.FocusPath())
	}
	if model.IsCollapsed("metadata") {
		t.Fatalf("right should expand collapsed metadata")
	}

	model = press(model, tea.Key{Code: tea.KeyRight})
	if model.FocusPath() != "metadata.name" {
		t.Fatalf("right should focus first child from expanded metadata, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyDown})
	if model.FocusPath() != "metadata.namespace" {
		t.Fatalf("down should move to next visible field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyLeft})
	if model.FocusPath() != "metadata" {
		t.Fatalf("left should move to parent field, got %q", model.FocusPath())
	}
	if model.IsCollapsed("metadata") {
		t.Fatalf("left from a child should not collapse the parent")
	}

	model = press(model, tea.Key{Code: tea.KeyLeft})
	if !model.IsCollapsed("metadata") {
		t.Fatalf("left should collapse an expanded focused field")
	}

	model = press(model, tea.Key{Code: tea.KeyTab})
	if model.FocusPath() != "spec" {
		t.Fatalf("tab should jump to next visible foldable field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.FocusPath() != "metadata" {
		t.Fatalf("shift-tab should jump to previous visible foldable field, got %q", model.FocusPath())
	}
}

func TestModelPageKeysMoveHalfPage(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 10})
	model = updated.(Model)

	model = press(model, tea.Key{Code: tea.KeyPgDown})
	if model.FocusPath() != "spec.template.image" {
		t.Fatalf("page down should move half a page through visible fields, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyPgUp})
	if model.FocusPath() != "apiVersion" {
		t.Fatalf("page up should move half a page through visible fields, got %q", model.FocusPath())
	}
}

func TestModelHomeEndAndSearch(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	model = press(model, tea.Key{Code: tea.KeyEnd})
	if model.FocusPath() != "status" {
		t.Fatalf("end should focus last visible field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyHome})
	if model.FocusPath() != "apiVersion" {
		t.Fatalf("home should focus first visible field, got %q", model.FocusPath())
	}

	model = pressText(model, "/")
	for _, r := range "template" {
		model = pressText(model, string(r))
	}
	if model.SearchQuery() != "template" {
		t.Fatalf("expected search query to be recorded, got %q", model.SearchQuery())
	}
	if model.FocusPath() != "spec.template" {
		t.Fatalf("search should focus matching field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyEsc})
	model = pressText(model, "p")
	if model.FocusPath() != "spec.template" {
		t.Fatalf("single search match should stay focused on previous, got %q", model.FocusPath())
	}
}

func TestModelQuitKeysWorkInSearchMode(t *testing.T) {
	for name, key := range map[string]tea.Key{
		"q":      {Code: 'q', Text: "q"},
		"f10":    {Code: tea.KeyF10},
		"ctrl-c": {Code: 'c', Mod: tea.ModCtrl},
	} {
		t.Run(name, func(t *testing.T) {
			model := NewModel(testDocument(), Config{
				ExpandDepth:  2,
				Descriptions: tree.DescriptionTrue,
				Columns:      120,
			})
			model.search.active = true

			_, cmd := model.Update(tea.KeyPressMsg(key))
			if cmd == nil {
				t.Fatalf("expected quit command for %s", name)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("expected quit message for %s", name)
			}
		})
	}
}

func TestModelEscapeOnlyLeavesSearchMode(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("escape outside search must not quit")
	}

	model.search.active = true
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("escape in search must leave search mode without quitting")
	}
	if model.search.active {
		t.Fatalf("escape should leave search mode")
	}
}

func TestModelFiltersCollapsedDescendantsAndRestoresFocus(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  0,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	if !model.IsCollapsed("spec") {
		t.Fatalf("test setup expects spec to start collapsed")
	}

	for _, r := range "image" {
		model = pressText(model, string(r))
	}
	if model.FilterQuery() != "image" {
		t.Fatalf("expected filter query to be recorded, got %q", model.FilterQuery())
	}
	if model.FocusPath() != "spec.template.image" {
		t.Fatalf("filter should focus matching collapsed descendant, got %q", model.FocusPath())
	}
	view := stripANSI(model.schemaView(120, 20))
	for _, expected := range []string{"spec:", "template:", "image:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("filtered view should reveal ancestor path containing %q, got:\n%s", expected, view)
		}
	}
	if !strings.Contains(model.schemaView(120, 20), "\x1b[") || !strings.Contains(model.schemaView(120, 20), "48;5;214") {
		t.Fatalf("filtered view should highlight matches in strong orange, got:\n%s", model.schemaView(120, 20))
	}

	model = press(model, tea.Key{Code: tea.KeyEsc})
	if model.FilterQuery() != "" {
		t.Fatalf("escape should clear filter, got %q", model.FilterQuery())
	}
	if model.FocusPath() != "spec.template.image" {
		t.Fatalf("escape should preserve logical field focus, got %q", model.FocusPath())
	}
	if model.IsCollapsed("spec") || model.IsCollapsed("spec.template") {
		t.Fatalf("escape should expand ancestors so the focused field remains visible")
	}
}

func TestModelFilterEscapeRestoresUnfocusedRevealedBranches(t *testing.T) {
	model := NewModel(filterBranchDocument(), Config{
		ExpandDepth:  1,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	for _, path := range []string{"spec.left", "spec.right"} {
		if !model.IsCollapsed(path) {
			t.Fatalf("test setup expects %s to start collapsed", path)
		}
	}

	for _, r := range "needle" {
		model = pressText(model, string(r))
	}
	if model.FocusPath() != "spec.left.needle" {
		t.Fatalf("filter should focus first matching collapsed descendant, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyEsc})
	if model.FilterQuery() != "" {
		t.Fatalf("escape should clear filter, got %q", model.FilterQuery())
	}
	if model.FocusPath() != "spec.left.needle" {
		t.Fatalf("escape should preserve focused logical field, got %q", model.FocusPath())
	}
	if model.IsCollapsed("spec.left") {
		t.Fatalf("escape should keep focused path ancestors expanded")
	}
	if !model.IsCollapsed("spec.right") {
		t.Fatalf("escape should restore unfocused filter-revealed branches")
	}
	view := stripANSI(model.schemaView(120, 20))
	if !strings.Contains(view, "left:") || !strings.Contains(view, "needle:") {
		t.Fatalf("focused branch should remain visible after escape, got:\n%s", view)
	}
	if strings.Contains(view, "right:\n    needle:") {
		t.Fatalf("unfocused branch descendant should be hidden again after escape, got:\n%s", view)
	}
}

func TestModelFilterEnterKeepsRevealedBranchesExpanded(t *testing.T) {
	model := NewModel(filterBranchDocument(), Config{
		ExpandDepth:  1,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	for _, r := range "needle" {
		model = pressText(model, string(r))
	}
	model = press(model, tea.Key{Code: tea.KeyEnter})
	if model.FilterQuery() != "" {
		t.Fatalf("enter should clear filter after accepting it, got %q", model.FilterQuery())
	}
	for _, path := range []string{"spec.left", "spec.right"} {
		if model.IsCollapsed(path) {
			t.Fatalf("enter should keep filter-revealed branch %s expanded", path)
		}
	}
	view := stripANSI(model.schemaView(120, 30))
	for _, expected := range []string{"left:", "right:", "needle:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("accepted filter should keep %q visible, got:\n%s", expected, view)
		}
	}
}

func TestModelFilterParentDescriptionShowsDescendants(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  0,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	for _, r := range "configures" {
		model = pressText(model, string(r))
	}
	if model.FocusPath() != "spec" {
		t.Fatalf("parent description filter should focus spec, got %q", model.FocusPath())
	}
	view := stripANSI(model.schemaView(120, 30))
	for _, expected := range []string{"spec:", "template:", "image:", "replicas:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("parent description match should keep descendant %q visible, got:\n%s", expected, view)
		}
	}
}

func TestModelFilterDescriptionMatchHighlightsFieldName(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  0,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	for _, r := range "replica count" {
		model = pressText(model, string(r))
	}
	if model.FocusPath() != "spec.replicas" {
		t.Fatalf("description filter should focus replicas, got %q", model.FocusPath())
	}
	view := model.schemaView(120, 30)
	if !strings.Contains(view, filterHitStyle.Render("replicas")) {
		t.Fatalf("description-only match should highlight field name, got:\n%s", view)
	}
}

func TestModelFilterTabJumpsOnlyDirectMatches(t *testing.T) {
	model := NewModel(filterBranchDocument(), Config{
		ExpandDepth:  0,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	for _, r := range "branch marker" {
		model = pressText(model, string(r))
	}
	if model.FocusPath() != "spec.left" {
		t.Fatalf("filter should focus first direct parent-description match, got %q", model.FocusPath())
	}
	model = press(model, tea.Key{Code: tea.KeyTab})
	if model.FocusPath() != "spec.right" {
		t.Fatalf("tab should jump to next direct match, got %q", model.FocusPath())
	}
	model = press(model, tea.Key{Code: tea.KeyTab})
	if model.FocusPath() != "spec.left" {
		t.Fatalf("tab should wrap across direct matches, got %q", model.FocusPath())
	}
	model = press(model, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.FocusPath() != "spec.right" {
		t.Fatalf("shift-tab should jump to previous direct match, got %q", model.FocusPath())
	}
	if strings.Contains(model.FocusPath(), "needle") {
		t.Fatalf("tab must not jump to descendant visible only because a parent matched")
	}
}

func TestModelFocusDoesNotMoveRenderedText(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})

	view := stripANSI(model.schemaView(120, 5))
	if strings.Contains(view, "> ") {
		t.Fatalf("focus must be rendered as color only, got textual cursor:\n%s", view)
	}
	if !strings.Contains(view, "▶ metadata:") {
		t.Fatalf("expected focused foldable line to keep the same fixed gutter, got:\n%s", view)
	}
	if !strings.HasPrefix(firstLine(view, "metadata:"), "▶ metadata:") {
		t.Fatalf("focused line should keep the YAML text in place, got:\n%s", view)
	}
}

func TestModelCursorFillsSchemaPane(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})

	view := stripANSI(model.schemaView(40, 5))
	line := firstLine(view, "metadata:")
	if lipgloss.Width(line) != 40 {
		t.Fatalf("expected focused line to be padded to pane width, got width %d line %q", lipgloss.Width(line), line)
	}
	if !strings.HasPrefix(line, "▶ metadata:") {
		t.Fatalf("expected padding to keep field text stable, got %q", line)
	}
}

func TestFocusedSchemaLineKeepsSyntaxHighlighting(t *testing.T) {
	for _, line := range []string{
		`▼ # labels:`,
		`▼ # managedFields: # listType: atomic`,
	} {
		focused := colorFocusedSchemaLine(line, true, 40)
		if !strings.Contains(focused, cursorBackgroundANSI) {
			t.Fatalf("expected focused line background in %q, got %q", line, focused)
		}
		if !strings.Contains(focused, "\x1b[1;96m") {
			t.Fatalf("expected focused line to keep key highlighting in %q, got %q", line, focused)
		}
		if width := lipgloss.Width(stripANSI(focused)); width != 40 {
			t.Fatalf("expected focused line width 40, got %d: %q", width, stripANSI(focused))
		}
	}
}

func TestSchemaLineWrapsInlineMetadataAsCommentContinuation(t *testing.T) {
	line := tree.Line{
		Text: `stopSignal: "SIGABRT" # enum: "SIGALRM" | "SIGBUS" | "SIGCHLD"`,
		Code: true,
	}
	wrapped := wrapSchemaLine(line, "  "+line.Text, 45)
	if len(wrapped) < 2 {
		t.Fatalf("expected inline metadata to wrap, got %#v", wrapped)
	}
	firstHash := strings.Index(wrapped[0].Text, "#")
	secondHash := strings.Index(wrapped[1].Text, "#")
	if firstHash < 0 || secondHash < 0 || firstHash != secondHash {
		t.Fatalf("expected continuation # column %d to match first # column %d, got %#v", secondHash, firstHash, wrapped)
	}
	if !wrapped[0].Code {
		t.Fatalf("expected first visual row to stay code, got %#v", wrapped[0])
	}
	if wrapped[1].Code {
		t.Fatalf("expected continuation visual row to be comment-colored, got %#v", wrapped[1])
	}
}

func TestModelResizeKeepsFocusedFieldVisible(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyEnd})
	if model.FocusPath() != "status" {
		t.Fatalf("expected status focus before resize, got %q", model.FocusPath())
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 4})
	model = updated.(Model)
	view := stripANSI(model.schemaView(80, model.schemaHeight()))
	if !strings.Contains(view, "status:") {
		t.Fatalf("focused field should remain visible after resize, got:\n%s", view)
	}
}

func TestModelDownOnLastFieldKeepsCursorVisibleWithWrappedRows(t *testing.T) {
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Description = "Spec has a deliberately long description so it wraps into several physical terminal rows before the last visible field."
	labels := spec.Properties["labels"]
	labels.Description = "Labels also have a deliberately long description so scrolling has to account for wrapped rows, not only logical lines."
	spec.Properties["labels"] = labels
	doc.Schema.Properties["spec"] = spec

	model := NewModel(doc, Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	model = updated.(Model)
	model = press(model, tea.Key{Code: tea.KeyEnd})
	model = press(model, tea.Key{Code: tea.KeyDown})

	if model.FocusPath() != "status" {
		t.Fatalf("down on the last field should keep focus on status, got %q", model.FocusPath())
	}
	view := stripANSI(model.schemaView(model.schemaPaneWidth(), model.schemaHeight()))
	if !strings.Contains(view, "status:") {
		t.Fatalf("last focused field should remain visible after pressing down, got:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) > model.schemaHeight() {
		t.Fatalf("schema view should not exceed pane height %d, got %d:\n%s", model.schemaHeight(), len(lines), view)
	}
}

func TestModelHomeEndOnLargeWrappedSchema(t *testing.T) {
	properties := map[string]docschema.Structural{}
	for i := 0; i < 400; i++ {
		name := fmt.Sprintf("field%03d", i)
		properties[name] = docschema.Structural{
			Generic: docschema.Generic{
				Type:        "string",
				Description: "A deliberately long description that wraps in the TUI and used to make Home and End expensive.",
			},
		}
	}
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Large",
		Plural:  "larges",
		Schema: &docschema.Structural{
			Properties: properties,
		},
	}

	model := NewModel(doc, Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	model = updated.(Model)

	model = press(model, tea.Key{Code: tea.KeyEnd})
	if model.FocusPath() != "field399" {
		t.Fatalf("end should focus the last field, got %q", model.FocusPath())
	}
	view := stripANSI(model.schemaView(model.schemaPaneWidth(), model.schemaHeight()))
	if !strings.Contains(view, "field399:") {
		t.Fatalf("end should keep the last field visible, got:\n%s", view)
	}

	model = press(model, tea.Key{Code: tea.KeyHome})
	if model.FocusPath() != "apiVersion" {
		t.Fatalf("home should focus the first field, got %q", model.FocusPath())
	}
	view = stripANSI(model.schemaView(model.schemaPaneWidth(), model.schemaHeight()))
	if !strings.Contains(view, "apiVersion: example.io/v1") {
		t.Fatalf("home should keep the first field visible, got:\n%s", view)
	}
	for _, expected := range []string{"apiVersion: example.io/v1", "kind: Large"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("home should keep generated YAML header line %q visible, got:\n%s", expected, view)
		}
	}
}

func TestModelRendersDetailsAndResponsiveLayout(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})
	model = press(model, tea.Key{Code: tea.KeyTab})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	model = updated.(Model)
	wide := model.view()
	for _, expected := range []string{
		"PATH  spec",
		"Spec configures the widget.",
	} {
		if !strings.Contains(stripANSI(wide), expected) {
			t.Fatalf("expected wide view to contain %q, got:\n%s", expected, wide)
		}
	}
	if !strings.Contains(wide, "\x1b[") {
		t.Fatalf("expected styled details heading and syntax colors, got:\n%s", wide)
	}
	if strings.Contains(stripANSI(wide), "Widget example.io/v1") {
		t.Fatalf("expected wide view to omit sticky GVK header, got:\n%s", wide)
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	model = updated.(Model)
	narrow := model.view()
	if !strings.Contains(stripANSI(narrow), "\n\nDetails") {
		t.Fatalf("expected narrow view to place details below schema, got:\n%s", narrow)
	}
}

func TestModelShowsStatusLineOnlyForSearch(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 10})
	model = updated.(Model)

	normal := stripANSI(model.view())
	if strings.HasPrefix(normal, "Widget example.io/v1\n") || strings.HasPrefix(normal, "search: ") {
		t.Fatalf("normal view should not have a sticky header, got:\n%s", normal)
	}

	model = pressText(model, "/")
	model = pressText(model, "s")
	searching := stripANSI(model.view())
	if !strings.HasPrefix(searching, "search: /s\n") {
		t.Fatalf("search view should show conditional search status line, got:\n%s", searching)
	}
}

func TestModelDetailsShowSchemaInfoAndStickyFooter(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})
	model = press(model, tea.Key{Code: tea.KeyTab})

	details := stripANSI(model.detailsView(40, 18))
	for _, expected := range []string{
		"Details",
		"PATH  spec",
		"TYPE  object",
		"REQUIRED  yes",
		"DESCRIPTION",
		"Spec configures the widget.",
		"VALIDATION AND METADATA",
		"- required",
	} {
		if !strings.Contains(details, expected) {
			t.Fatalf("expected details to contain %q, got:\n%s", expected, details)
		}
	}
	for _, unwanted := range []string{"YAML:", "FOLDABLE", "COLLAPSED"} {
		if strings.Contains(details, unwanted) {
			t.Fatalf("details should not contain %q, got:\n%s", unwanted, details)
		}
	}
	lines := strings.Split(details, "\n")
	if len(lines) != 18 {
		t.Fatalf("expected details view to keep requested height, got %d:\n%s", len(lines), details)
	}
	if !strings.Contains(details, "up/down focus") || !strings.Contains(lines[len(lines)-1], "quit") {
		t.Fatalf("expected key help sticky at bottom, got:\n%s", details)
	}
}

func TestDescriptionLinesStripCommentAndSequenceMarkers(t *testing.T) {
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Properties["containers"] = docschema.Structural{
		Generic: docschema.Generic{
			Type:        "array",
			Description: "Container list.",
		},
		Items: &docschema.Structural{
			Generic: docschema.Generic{Type: "object"},
			Properties: map[string]docschema.Structural{
				"name": {
					Generic: docschema.Generic{
						Type:        "string",
						Description: "Unique container name.",
					},
				},
			},
		},
	}
	doc.Schema.Properties["spec"] = spec

	model := NewModel(doc, Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	descriptions := model.descriptionLines("spec.containers[].name")
	if len(descriptions) == 0 {
		t.Fatalf("expected array item field description")
	}
	for _, description := range descriptions {
		if strings.Contains(description, "#") || strings.HasPrefix(description, "-") {
			t.Fatalf("details description should not contain YAML comment markers, got %#v", descriptions)
		}
	}
}

func TestDetailsViewConstrainLinesToPaneWidth(t *testing.T) {
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Description = "A long details description includes https://example.com/a/very/long/unbroken/path/that/must/not/wrap/the/terminal."
	doc.Schema.Properties["spec"] = spec

	model := NewModel(doc, Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})
	model = press(model, tea.Key{Code: tea.KeyTab})

	const width = 30
	const height = 12
	details := stripANSI(model.detailsView(width, height))
	lines := strings.Split(details, "\n")
	if len(lines) != height {
		t.Fatalf("expected %d details lines, got %d:\n%s", height, len(lines), details)
	}
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			t.Fatalf("details line exceeds width %d with width %d: %q\n%s", width, lipgloss.Width(line), line, details)
		}
	}
}

func TestWideLayoutSeparatorSpansContentHeight(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 10})
	model = updated.(Model)

	view := stripANSI(model.view())
	if lines := strings.Split(view, "\n"); len(lines) != model.height {
		t.Fatalf("expected wide view to keep terminal height %d, got %d:\n%s", model.height, len(lines), view)
	}
	if count := strings.Count(view, "│"); count != model.contentHeight() {
		t.Fatalf("expected separator to span %d content rows, got %d:\n%s", model.contentHeight(), count, view)
	}
}

func TestWideLayoutShortSchemaStillFillsTerminalHeight(t *testing.T) {
	model := NewModel(&crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Tiny",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{},
		},
	}, Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	model = updated.(Model)

	view := stripANSI(model.view())
	lines := strings.Split(view, "\n")
	if len(lines) != model.height {
		t.Fatalf("expected short schema view to keep terminal height %d, got %d:\n%s", model.height, len(lines), view)
	}
	if count := strings.Count(view, "│"); count != model.contentHeight() {
		t.Fatalf("expected separator to span %d content rows for short schema, got %d:\n%s", model.contentHeight(), count, view)
	}
	if !strings.Contains(lines[len(lines)-1], "quit") {
		t.Fatalf("expected key hints at bottom for short schema, got:\n%s", view)
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	model = updated.(Model)
	view = stripANSI(model.view())
	lines = strings.Split(view, "\n")
	if len(lines) != model.height {
		t.Fatalf("expected resized short schema view to keep terminal height %d, got %d:\n%s", model.height, len(lines), view)
	}
	if count := strings.Count(view, "│"); count != model.contentHeight() {
		t.Fatalf("expected resized separator to span %d content rows, got %d:\n%s", model.contentHeight(), count, view)
	}
	if !strings.Contains(lines[len(lines)-1], "quit") {
		t.Fatalf("expected key hints at bottom after resize, got:\n%s", view)
	}
}

func TestWideLayoutCapsWrappedSchemaRows(t *testing.T) {
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Description = "This long schema comment wraps several times so wide layout must still keep the separator height aligned with the terminal height."
	doc.Schema.Properties["spec"] = spec

	model := NewModel(doc, Config{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	model = updated.(Model)

	view := stripANSI(model.view())
	lines := strings.Split(view, "\n")
	if len(lines) != model.height {
		t.Fatalf("expected wrapped schema wide view to keep terminal height %d, got %d:\n%s", model.height, len(lines), view)
	}
	if count := strings.Count(view, "│"); count != model.contentHeight() {
		t.Fatalf("expected separator to span %d content rows, got %d:\n%s", model.contentHeight(), count, view)
	}
}

func TestWideLayoutUsesThreeQuarterSchemaPane(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	schemaWidth, detailsWidth := model.widePaneWidths(120)
	if schemaWidth != 87 || detailsWidth != 30 {
		t.Fatalf("expected 75/25 split minus separator for 120 columns, got schema=%d details=%d", schemaWidth, detailsWidth)
	}
}

func press(model Model, key tea.Key) Model {
	updated, _ := model.Update(tea.KeyPressMsg(key))
	return updated.(Model)
}

func pressText(model Model, text string) Model {
	return press(model, tea.Key{Code: []rune(text)[0], Text: text})
}

func stripANSI(text string) string {
	return regexp.MustCompile(`\x1b\[[0-9;:]*m`).ReplaceAllString(text, "")
}

func firstLine(text, contains string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, contains) {
			return line
		}
	}
	return ""
}

func testDocument() *crd.Document {
	return &crd.Document{
		Group:      "example.io",
		Version:    "v1",
		Kind:       "Widget",
		Plural:     "widgets",
		Namespaced: true,
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Spec configures the widget.",
					},
					Properties: map[string]docschema.Structural{
						"labels": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Labels are copied to generated pods.",
							},
							AdditionalProperties: &docschema.StructuralOrBool{
								Structural: &docschema.Structural{
									Generic: docschema.Generic{Type: "string"},
								},
							},
						},
						"replicas": {
							Generic: docschema.Generic{
								Type:        "integer",
								Description: "Desired replica count.",
							},
						},
						"template": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Template for generated pods.",
							},
							Properties: map[string]docschema.Structural{
								"image": {
									Generic: docschema.Generic{
										Type:        "string",
										Description: "Container image.",
									},
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
				"status": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Observed widget state.",
					},
					Properties: map[string]docschema.Structural{
						"phase": {
							Generic: docschema.Generic{Type: "string"},
						},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}
}

func filterBranchDocument() *crd.Document {
	return &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Branchy",
		Plural:  "branchies",
		Schema: &docschema.Structural{
			Generic: docschema.Generic{Type: "object"},
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{Type: "object"},
					Properties: map[string]docschema.Structural{
						"left": {
							Generic: docschema.Generic{Type: "object", Description: "branch marker left"},
							Properties: map[string]docschema.Structural{
								"needle": {Generic: docschema.Generic{Type: "string"}},
							},
						},
						"right": {
							Generic: docschema.Generic{Type: "object", Description: "branch marker right"},
							Properties: map[string]docschema.Structural{
								"needle": {Generic: docschema.Generic{Type: "string"}},
							},
						},
					},
				},
			},
		},
	}
}
