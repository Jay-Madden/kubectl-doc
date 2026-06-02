package yamlrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/kube"
)

func TestColorLineUsesANSIStyles(t *testing.T) {
	colored := colorLine(`spec: "<string>" # comment`)

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "spec") || !strings.Contains(colored, "<string>") || !strings.Contains(colored, "# comment") {
		t.Fatalf("colored line lost content: %q", colored)
	}
}

func TestColorLineStylesYAMLPunctuation(t *testing.T) {
	colored := colorLine(`deployments: ["v2","v1"]`)

	for _, token := range []string{":", "[", ",", "]"} {
		expected := syntaxStyle.Render(token)
		if !strings.Contains(colored, expected) {
			t.Fatalf("expected syntax-styled %q in %q", token, colored)
		}
	}
	for _, token := range []string{`"v2"`, `"v1"`} {
		expected := stringStyle.Render(token)
		if !strings.Contains(colored, expected) {
			t.Fatalf("expected string-styled %q in %q", token, colored)
		}
	}
}

func TestRenderOverview(t *testing.T) {
	var out bytes.Buffer
	overview := &kube.Overview{
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

	renderer := OverviewRenderer{}
	if err := renderer.Render(&out, overview); err != nil {
		t.Fatal(err)
	}

	expected := `core:
  pods: v1
apps:
  deployments: ["v1","v1beta1"]
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
}

func TestRenderOverviewColor(t *testing.T) {
	var out bytes.Buffer
	overview := &kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v2", "v1"}},
				},
			},
		},
	}

	renderer := OverviewRenderer{Color: true}
	if err := renderer.Render(&out, overview); err != nil {
		t.Fatal(err)
	}

	colored := out.String()
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "core") || !strings.Contains(colored, "pods") || !strings.Contains(colored, "v2") {
		t.Fatalf("colored overview lost content: %q", colored)
	}
	for _, token := range []string{":", "[", ",", "]"} {
		if !strings.Contains(colored, syntaxStyle.Render(token)) {
			t.Fatalf("expected syntax-styled %q in %q", token, colored)
		}
	}
}
