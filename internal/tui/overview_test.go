package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	"github.com/sttts/kubectl-doc/internal/render/termstyle"
)

func TestOverviewModelRendersGroupResourceVersionTree(t *testing.T) {
	model := NewOverviewModel(testOverview(), Config{Columns: 80})

	raw := model.view()
	if !strings.Contains(raw, termstyle.KeyStyle.Render("apps")) {
		t.Fatalf("expected overview groups to render with terminal key style, got:\n%s", raw)
	}
	if strings.Contains(raw, termstyle.KeyStyle.Render("deployments")) {
		t.Fatalf("expected overview resources to render without group key style, got:\n%s", raw)
	}
	if !strings.Contains(raw, termstyle.ScalarStyle.Render("v1beta1")) {
		t.Fatalf("expected overview versions to render with terminal scalar style, got:\n%s", raw)
	}

	view := stripANSI(raw)
	for _, expected := range []string{
		"Kubernetes resources",
		"core",
		"  pods  v1",
		"apps",
		"  deployments  v1",
		"  deployments  v1beta1",
	} {
		if !containsLine(view, expected) {
			t.Fatalf("expected overview to contain line %q, got:\n%s", expected, view)
		}
	}

	item := model.FocusedItem()
	if item.group != "" || item.resource != "pods" || item.version != "v1" {
		t.Fatalf("expected initial focus on core pods/v1, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyDown})
	item = model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1" {
		t.Fatalf("expected down to focus apps deployments/v1, got %#v", item)
	}
}

func TestOverviewModelUsesAltScreen(t *testing.T) {
	model := NewOverviewModel(testOverview(), Config{Columns: 80})
	if view := model.View(); !view.AltScreen {
		t.Fatalf("TUI overview should use the alternate screen")
	}
}

func TestOverviewModelOpensSchemaAndBackPreservesCursor(t *testing.T) {
	model := NewOverviewModel(testOverview(), Config{Columns: 100, ExpandDepth: 2})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 6})
	model = updated.(OverviewModel)
	model = pressOverview(model, tea.Key{Code: tea.KeyEnd})
	item := model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1beta1" {
		t.Fatalf("expected end to focus apps deployments/v1beta1, got %#v", item)
	}
	savedFocus := model.focus
	savedTop := model.top

	var loaded overviewItem
	model.loadDocument = func(ctx context.Context, group, version, resource string) (*crd.Document, error) {
		loaded = overviewItem{group: group, version: version, resource: resource}
		return testDocument(), nil
	}
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(OverviewModel)
	if cmd == nil {
		t.Fatal("expected enter to load the selected schema")
	}
	updated, _ = model.Update(cmd())
	model = updated.(OverviewModel)
	if !reflect.DeepEqual(loaded, item) {
		t.Fatalf("expected loader to receive focused item %#v, got %#v", item, loaded)
	}
	if model.schema == nil {
		t.Fatal("expected selected schema to be active")
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyEsc})
	if model.schema != nil {
		t.Fatal("expected escape to return to the overview")
	}
	if model.focus != savedFocus || model.top != savedTop {
		t.Fatalf("expected overview cursor/top to be preserved, focus %d/%d top %d/%d", model.focus, savedFocus, model.top, savedTop)
	}
}

func TestOverviewModelEscapeQuitsOverview(t *testing.T) {
	model := NewOverviewModel(testOverview(), Config{Columns: 80})

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected escape in overview to quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected escape in overview to return quit message")
	}
}

func TestOverviewModelHorizontalKeysJumpGroups(t *testing.T) {
	model := NewOverviewModel(&kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v1"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1", "v1beta1"}},
					{Name: "daemonsets", Versions: []string{"v1"}},
				},
			},
			{
				Name: "batch",
				Resources: []kube.Resource{
					{Name: "jobs", Versions: []string{"v1"}},
				},
			},
		},
	}, Config{Columns: 100})

	model = pressOverview(model, tea.Key{Code: tea.KeyRight})
	item := model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1" {
		t.Fatalf("right should jump to first version in apps group, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyDown})
	item = model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1beta1" {
		t.Fatalf("down should still move within the current group, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyRight})
	item = model.FocusedItem()
	if item.group != "batch" || item.resource != "jobs" || item.version != "v1" {
		t.Fatalf("right should jump from apps to first version in batch group, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyLeft})
	item = model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1" {
		t.Fatalf("left should jump back to first version in apps group, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyTab})
	item = model.FocusedItem()
	if item.group != "batch" || item.resource != "jobs" || item.version != "v1" {
		t.Fatalf("tab should jump to next group like right, got %#v", item)
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	item = model.FocusedItem()
	if item.group != "apps" || item.resource != "deployments" || item.version != "v1" {
		t.Fatalf("shift-tab should jump to previous group like left, got %#v", item)
	}

	model.top = 3
	model = pressOverview(model, tea.Key{Code: tea.KeyLeft})
	item = model.FocusedItem()
	if item.group != "" || item.resource != "pods" || item.version != "v1" {
		t.Fatalf("left should jump to first group, got %#v", item)
	}
	if model.top != 0 {
		t.Fatalf("jumping to the first resource in the first group should scroll to top, got top %d", model.top)
	}
}

func TestOverviewModelHomeScrollsToCoreHeader(t *testing.T) {
	model := NewOverviewModel(&kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "configmaps", Versions: []string{"v1"}},
					{Name: "pods", Versions: []string{"v1"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1"}},
					{Name: "statefulsets", Versions: []string{"v1"}},
				},
			},
		},
	}, Config{Columns: 80})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	model = updated.(OverviewModel)
	model = pressOverview(model, tea.Key{Code: tea.KeyEnd})
	if model.top == 0 {
		t.Fatalf("test setup should scroll away from the core header")
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyHome})
	item := model.FocusedItem()
	if item.group != "" || item.resource != "configmaps" || item.version != "v1" {
		t.Fatalf("home should focus the first core resource, got %#v", item)
	}
	if model.top != 0 {
		t.Fatalf("home should scroll back to the core header, got top %d", model.top)
	}
	if !containsLine(stripANSI(model.view()), "core") {
		t.Fatalf("expected core header to be visible after home, got:\n%s", stripANSI(model.view()))
	}
}

func TestOverviewModelFiltersByShortNameAndPreservesFocus(t *testing.T) {
	model := NewOverviewModel(&kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v1"}, ShortNames: []string{"po"}},
					{Name: "services", Versions: []string{"v1"}, ShortNames: []string{"svc"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1"}, ShortNames: []string{"deploy"}},
				},
			},
		},
	}, Config{Columns: 100})

	for _, r := range "svc" {
		model = pressOverviewText(model, string(r))
	}
	if model.filter.query != "svc" {
		t.Fatalf("expected overview filter query, got %q", model.filter.query)
	}
	item := model.FocusedItem()
	if item.resource != "services" || item.version != "v1" {
		t.Fatalf("shortname filter should focus services/v1, got %#v", item)
	}
	view := stripANSI(model.view())
	if !strings.Contains(view, "services  v1") || strings.Contains(view, "deployments") {
		t.Fatalf("overview filter should keep only matching resource rows, got:\n%s", view)
	}
	if !strings.Contains(model.view(), "48;5;214") {
		t.Fatalf("overview filter should highlight shortname-matched row, got:\n%s", model.view())
	}
	lines := stringsSplit(stripANSI(model.view()))
	if len(lines) < 2 {
		t.Fatalf("expected filter line in overview render, got:\n%s", stripANSI(model.view()))
	}
	if !strings.HasPrefix(lines[1], "filter: svc") || len(lines[1]) != model.width {
		t.Fatalf("filter line should reserve the full overview width, got %q in:\n%s", lines[1], stripANSI(model.view()))
	}

	model = pressOverview(model, tea.Key{Code: tea.KeyEsc})
	if model.filter.query != "" {
		t.Fatalf("escape should clear overview filter, got %q", model.filter.query)
	}
	item = model.FocusedItem()
	if item.resource != "services" || item.version != "v1" {
		t.Fatalf("escape should preserve logical overview focus, got %#v", item)
	}
}

func TestOverviewModelKeepsFooterSticky(t *testing.T) {
	model := NewOverviewModel(testOverview(), Config{Columns: 60})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 8})
	model = updated.(OverviewModel)

	view := stripANSI(model.view())
	lines := stringsSplit(view)
	if len(lines) != model.height {
		t.Fatalf("expected overview to keep terminal height %d, got %d:\n%s", model.height, len(lines), view)
	}
	if !strings.Contains(view, "left/right/tab group") || !strings.Contains(view, "q/F10/Ctrl-C quit") {
		t.Fatalf("expected sticky overview footer, got:\n%s", view)
	}
}

func pressOverview(model OverviewModel, key tea.Key) OverviewModel {
	updated, _ := model.Update(tea.KeyPressMsg(key))
	return updated.(OverviewModel)
}

func pressOverviewText(model OverviewModel, text string) OverviewModel {
	return pressOverview(model, tea.Key{Code: []rune(text)[0], Text: text})
}

func testOverview() *kube.Overview {
	return &kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v1"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1", "v1beta1"}},
				},
			},
		},
	}
}

func containsLine(text, expected string) bool {
	for _, line := range stringsSplit(text) {
		if strings.TrimRight(line, " ") == expected {
			return true
		}
	}
	return false
}

func stringsSplit(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
