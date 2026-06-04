package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	htmlrender "github.com/sttts/kubectl-doc/internal/render/html"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestRenderOverviewNavigationContract(t *testing.T) {
	var out bytes.Buffer
	renderOverview(&out, webTestOverview())
	rendered := out.String()

	assertContainsInOrder(t, rendered, []string{
		`<header class="kdoc-header"><h1>Kubernetes resources</h1><div class="kdoc-filter-overlay" data-kdoc-filter-overlay hidden></div></header>`,
		`<li class="kdoc-group" data-kdoc-overview-group data-group-name="core"><h2>core</h2>`,
		`<div class="kdoc-resource" data-kdoc-overview-resource data-resource-name="pods" data-shortnames="po"><span class="kdoc-resource-name">pods</span><span class="kdoc-version"><a href="/?resource=pods&amp;version=v1" data-kdoc-overview-item data-index="0" data-version="v1">v1</a></span>`,
		`<li class="kdoc-group" data-kdoc-overview-group data-group-name="apps"><h2>apps</h2>`,
		`<div class="kdoc-resource" data-kdoc-overview-resource data-resource-name="deployments" data-shortnames="deploy"><span class="kdoc-resource-name">deployments</span><span class="kdoc-version"><a href="/?group=apps&amp;resource=deployments&amp;version=v1" data-kdoc-overview-item data-index="1" data-version="v1">v1</a></span><span class="kdoc-version"><a href="/?group=apps&amp;resource=deployments&amp;version=v1beta1" data-kdoc-overview-item data-index="2" data-version="v1beta1">v1beta1</a></span>`,
		`<li class="kdoc-group" data-kdoc-overview-group data-group-name="batch"><h2>batch</h2>`,
		`<div class="kdoc-resource" data-kdoc-overview-resource data-resource-name="jobs" data-shortnames=""><span class="kdoc-resource-name">jobs</span><span class="kdoc-version"><a href="/?group=batch&amp;resource=jobs&amp;version=v1" data-kdoc-overview-item data-index="3" data-version="v1">v1</a></span>`,
	})
	for _, expected := range []string{
		`data-kdoc-filter-overlay hidden`,
		`.kdoc-filter-hit{background:#fb8500;`,
		`var storageKey = "kubectl-doc-overview-focus";`,
		`var filterQuery = "";`,
		`function applyOverviewFilter()`,
		`function applyOverviewHighlights()`,
		`function clearFilter()`,
		`data-shortnames`,
		`var selected = Number(storageGet());`,
		`function selectGroup(direction)`,
		`function firstItemInGroup(group)`,
		`case "ArrowLeft":`,
		`handled = selectGroup(-1);`,
		`case "ArrowRight":`,
		`handled = selectGroup(1);`,
		`case "Tab":`,
		`handled = selectGroup(event.shiftKey ? -1 : 1);`,
		`case "Enter":`,
		`if(scroll && selected === 0){`,
		`window.scrollTo({top:0, left:0});`,
		`if(scroll && selected === items.length - 1){`,
		`window.scrollTo({top:document.documentElement.scrollHeight, left:0});`,
		`select(selected, true);`,
		`.kdoc-group h2{color:#007c89;`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected overview HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	for _, unwanted := range []string{
		`case "/"`,
		`data-kdoc-search`,
		`selectHorizontal`,
	} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("overview HTML must preserve browser search and group-jump semantics, found %q:\n%s", unwanted, rendered)
		}
	}
}

func TestHandlerRendersSelectedSchemaWithBackNavigation(t *testing.T) {
	var loaded struct {
		group    string
		version  string
		resource string
	}
	handler := handler(Config{
		Overview: webTestOverview(),
		Renderer: htmlrender.Renderer{
			ExpandDepth: 1,
		},
		LoadDocument: func(ctx context.Context, group, version, resource string) (*crd.Document, error) {
			loaded.group = group
			loaded.version = version
			loaded.resource = resource
			return webTestDocument(), nil
		},
	}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?group=apps&resource=deployments&version=v1", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected selected schema response 200, got %d:\n%s", recorder.Code, recorder.Body.String())
	}
	if loaded.group != "apps" || loaded.version != "v1" || loaded.resource != "deployments" {
		t.Fatalf("expected loader to receive apps/v1 deployments, got %#v", loaded)
	}
	rendered := recorder.Body.String()
	for _, expected := range []string{
		`data-kdoc-back-url="/"`,
		`case "Escape":`,
		`window.location.href = backURL;`,
		`Deployment`,
		`apiVersion`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected selected schema HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `data-kdoc-search`) {
		t.Fatalf("schema HTML must use browser search instead of custom search controls, got:\n%s", rendered)
	}
	if strings.Contains(rendered, `data-kdoc-quit-url="`) {
		t.Fatalf("overview-selected schema HTML must navigate back instead of quitting, got:\n%s", rendered)
	}
}

func TestHandlerRendersExplicitSchemaWithQuitNavigation(t *testing.T) {
	var quitRequested bool
	handler := handler(Config{
		Docs: []*crd.Document{webTestDocument()},
		Renderer: htmlrender.Renderer{
			ExpandDepth: 1,
		},
	}, func() {
		quitRequested = true
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected explicit schema response 200, got %d:\n%s", recorder.Code, recorder.Body.String())
	}
	rendered := recorder.Body.String()
	for _, expected := range []string{
		`data-kdoc-quit-url="/__kubectl-doc/quit"`,
		`function requestQuit()`,
		`navigator.sendBeacon`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected explicit schema HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `data-kdoc-back-url="`) {
		t.Fatalf("explicit schema HTML must not include overview back navigation, got:\n%s", rendered)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/__kubectl-doc/quit", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected quit endpoint 202, got %d:\n%s", recorder.Code, recorder.Body.String())
	}
	if !quitRequested {
		t.Fatal("expected quit endpoint to notify server")
	}
}

func assertContainsInOrder(t *testing.T, text string, expected []string) {
	t.Helper()
	offset := 0
	for _, item := range expected {
		index := strings.Index(text[offset:], item)
		if index < 0 {
			t.Fatalf("expected %q after offset %d, got:\n%s", item, offset, text)
		}
		offset += index + len(item)
	}
}

func webTestOverview() *kube.Overview {
	return &kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v1"}, ShortNames: []string{"po"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1", "v1beta1"}, ShortNames: []string{"deploy"}},
				},
			},
			{
				Name: "batch",
				Resources: []kube.Resource{
					{Name: "jobs", Versions: []string{"v1"}},
				},
			},
		},
	}
}

func webTestDocument() *crd.Document {
	return &crd.Document{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
		Plural:  "deployments",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "DeploymentSpec is the desired state.",
					},
				},
			},
		},
	}
}
