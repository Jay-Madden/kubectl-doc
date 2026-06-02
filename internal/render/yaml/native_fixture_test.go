package yamlrender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/kube"
	"sigs.k8s.io/yaml"
)

type nativeOpenAPIDocument struct {
	Components struct {
		Schemas map[string]nativeOpenAPISchema `json:"schemas"`
	} `json:"components"`
}

type nativeOpenAPISchema struct {
	GVKs nativeOpenAPIGVKList `json:"x-kubernetes-group-version-kind"`
}

type nativeOpenAPIGVK struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type nativeOpenAPIGVKList []nativeOpenAPIGVK

func (l *nativeOpenAPIGVKList) UnmarshalJSON(data []byte) error {
	var list []nativeOpenAPIGVK
	if err := json.Unmarshal(data, &list); err == nil {
		*l = list
		return nil
	}

	var single nativeOpenAPIGVK
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*l = []nativeOpenAPIGVK{single}
	return nil
}

func TestRenderEmbeddedNativeOpenAPIV3Fixtures(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		minGVKs int
	}{
		{name: "core", path: "v1.json", minGVKs: 20},
		{name: "apiextensions", path: "apiextensions.k8s.io/v1.json", minGVKs: 3},
		{name: "apps", path: "apps/v1.json", minGVKs: 16},
		{name: "batch", path: "batch/v1.json", minGVKs: 8},
		{name: "batch beta", path: "batch/v1beta1.json", minGVKs: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := readNativeOpenAPIFixture(t, test.path)
			gvks := nativeFixtureGVKs(t, data)
			if len(gvks) < test.minGVKs {
				t.Fatalf("expected at least %d GVK schemas, got %d", test.minGVKs, len(gvks))
			}

			for _, gvk := range gvks {
				t.Run(nativeFixtureTestName(gvk), func(t *testing.T) {
					identity := kube.ResourceIdentity{
						Group:    gvk.Group,
						Version:  gvk.Version,
						Resource: nativeFixtureResourceName(gvk.Kind),
						Kind:     gvk.Kind,
					}
					doc, err := kube.BuildDocumentFromOpenAPIV3(data, identity)
					if err != nil {
						t.Fatal(err)
					}

					var out bytes.Buffer
					renderer := Renderer{ExpandDepth: 2, Descriptions: DescriptionFalse}
					if err := renderer.Render(&out, doc); err != nil {
						t.Fatal(err)
					}

					rendered := out.String()
					for _, expected := range []string{
						fmt.Sprintf("apiVersion: %s\n", nativeFixtureAPIVersion(gvk)),
						fmt.Sprintf("kind: %s\n", gvk.Kind),
					} {
						if !strings.Contains(rendered, expected) {
							t.Fatalf("expected rendered YAML to contain %q, got:\n%s", expected, rendered)
						}
					}

					var yamlDocument map[string]interface{}
					if err := yaml.Unmarshal([]byte(rendered), &yamlDocument); err != nil {
						t.Fatalf("rendered invalid YAML: %v\n%s", err, rendered)
					}

					if gvk.Group == "apps" && gvk.Version == "v1" && gvk.Kind == "Deployment" {
						for _, expected := range []string{"\nspec:", "\n  selector:", "\n  template:"} {
							if !strings.Contains(rendered, expected) {
								t.Fatalf("expected rendered Deployment to contain %q, got:\n%s", expected, rendered)
							}
						}
						if strings.Contains(rendered, "\nspec: {}") {
							t.Fatalf("expected rendered Deployment spec to be expanded, got:\n%s", rendered)
						}
					}
				})
			}
		})
	}
}

func nativeFixtureGVKs(t *testing.T, data []byte) []nativeOpenAPIGVK {
	t.Helper()

	var document nativeOpenAPIDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	var out []nativeOpenAPIGVK
	for _, schema := range document.Components.Schemas {
		out = append(out, schema.GVKs...)
	}
	return out
}

func readNativeOpenAPIFixture(t *testing.T, relativePath string) []byte {
	t.Helper()

	path := filepath.Join("..", "..", "kube", "testdata", "openapi_v3_0_0", filepath.FromSlash(relativePath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read native OpenAPI fixture %s: %v", path, err)
	}
	return data
}

func nativeFixtureTestName(gvk nativeOpenAPIGVK) string {
	if gvk.Group == "" {
		return gvk.Version + "_" + gvk.Kind
	}
	return strings.ReplaceAll(gvk.Group+"_"+gvk.Version+"_"+gvk.Kind, ".", "_")
}

func nativeFixtureAPIVersion(gvk nativeOpenAPIGVK) string {
	if gvk.Group == "" {
		return gvk.Version
	}
	return gvk.Group + "/" + gvk.Version
}

func nativeFixtureResourceName(kind string) string {
	lower := strings.ToLower(kind)
	switch {
	case strings.HasSuffix(lower, "s"):
		return lower + "es"
	case strings.HasSuffix(lower, "y"):
		return strings.TrimSuffix(lower, "y") + "ies"
	default:
		return lower + "s"
	}
}
