package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	htmlrender "github.com/sttts/kubectl-doc/internal/render/html"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	"github.com/sttts/kubectl-doc/internal/render/webschema"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	browserweb "github.com/sttts/kubectl-doc/internal/web"
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
	fixturesDir := filepath.Join(filepath.Dir(outDir), "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := writeBrowserOverviewFixture(filepath.Join(fixturesDir, "browser-overview.html"), docs); err != nil {
		return err
	}
	if err := writeBrowserSchemaFixture(filepath.Join(fixturesDir, "browser-schema.html"), docs); err != nil {
		return err
	}
	return writeMkDocsFixture(filepath.Join(fixturesDir, "mkdocs-embedded-schema.html"), docs)
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

func writeBrowserOverviewFixture(path string, docs []*crd.Document) error {
	var out bytes.Buffer
	browserweb.RenderOverview(&out, fixtureOverview(docs))
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeBrowserSchemaFixture(path string, docs []*crd.Document) error {
	var out bytes.Buffer
	if err := fixtureHTMLRenderer().RenderAll(&out, []*crd.Document{docs[0]}); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeMkDocsFixture(path string, docs []*crd.Document) error {
	var out bytes.Buffer
	if err := fixtureHTMLRenderer().RenderAll(&out, []*crd.Document{docs[0]}); err != nil {
		return err
	}
	html := strings.Replace(out.String(), `data-kubectl-doc`, `data-kubectl-doc data-kdoc-details-mode="side-overlay"`, 1)
	html = wrapMkDocsFixture(html)
	return os.WriteFile(path, []byte(html), 0o644)
}

func fixtureHTMLRenderer() htmlrender.Renderer {
	return htmlrender.Renderer{
		ExpandDepth:  3,
		Descriptions: yamlrender.DescriptionTrue,
		Columns:      100,
		BackURL:      "/",
	}
}

func fixtureOverview(docs []*crd.Document) *kube.Overview {
	resourceVersions := make([]string, 0, len(docs))
	for _, doc := range docs {
		resourceVersions = append(resourceVersions, doc.Version)
	}
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
					{Name: "daemonsets", Versions: []string{"v1"}, ShortNames: []string{"ds"}},
				},
			},
			{
				Name: docs[0].Group,
				Resources: []kube.Resource{
					{Name: docs[0].Plural, Versions: resourceVersions, ShortNames: []string{"dgd"}},
				},
			},
		},
	}
}

func wrapMkDocsFixture(html string) string {
	const bodyOpen = "<body>\n"
	const bodyClose = "</body>"
	const headClose = "</head>"
	const shellOpen = `<body class="kdoc-mkdocs-fixture">
<header class="kdoc-mkdocs-header">Platform Docs</header>
<div class="kdoc-mkdocs-shell">
<aside class="kdoc-mkdocs-sidebar">Navigation</aside>
<article class="kdoc-mkdocs-content">
`
	const shellClose = `</article>
<aside class="kdoc-mkdocs-sidebar kdoc-mkdocs-sidebar-right">On this page</aside>
</div>
</body>`
	html = strings.Replace(html, headClose, mkDocsFixtureStyle()+headClose, 1)
	html = strings.Replace(html, bodyOpen, shellOpen, 1)
	return strings.Replace(html, bodyClose, shellClose, 1)
}

func mkDocsFixtureStyle() string {
	return `<style>
body.kdoc-mkdocs-fixture{margin:0;color:#1f2933;background:#fff;font:16px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
.kdoc-mkdocs-header{align-items:center;background:#fff;border-bottom:1px solid #d8dee4;display:flex;height:52px;inset-block-start:0;padding:0 24px;position:sticky;top:0;z-index:100}
.kdoc-mkdocs-shell{display:grid;gap:20px;grid-template-columns:220px minmax(0,1fr) 220px;padding:20px}
.kdoc-mkdocs-sidebar{align-self:start;background:#f6f8fa;border:1px solid #d8dee4;border-radius:8px;color:#57606a;min-height:320px;padding:16px;position:sticky;top:72px}
.kdoc-mkdocs-content{min-width:0}
.kdoc-mkdocs-content>.kubectl-doc{padding:0}
.kdoc-mkdocs-content .kdoc-header{display:none}
.kdoc-mkdocs-content .kdoc-layout{display:block}
.kdoc-mkdocs-content .kdoc-tree{inline-size:100%;max-inline-size:100%;overflow:hidden}
.kdoc-mkdocs-content .kdoc-line{display:grid;grid-template-columns:24px minmax(0,1fr);inline-size:100%;max-inline-size:100%;overflow:hidden;white-space:normal}
.kdoc-mkdocs-content .kdoc-line[hidden]{display:none!important}
.kdoc-mkdocs-content .kdoc-version.kdoc-filtering .kdoc-line.kdoc-filter-visible{display:grid}
.kdoc-mkdocs-content .kdoc-yaml-text{display:block;min-inline-size:0;overflow-wrap:anywhere;white-space:pre-wrap}
.kdoc-mkdocs-content .kdoc-yaml-text *{max-inline-size:100%;min-inline-size:0;overflow-wrap:anywhere}
.kdoc-mkdocs-content.kdoc-wrap-comments .kdoc-yaml-comment-text,.kdoc-mkdocs-content .kdoc-wrap-comments .kdoc-yaml-comment-text{overflow-wrap:normal;white-space:normal}
.kdoc-mkdocs-content .kdoc-wrap-comments .kdoc-comment,.kdoc-mkdocs-content .kdoc-wrap-comments .kdoc-comment-line{overflow-wrap:normal;white-space:pre}
.kdoc-mkdocs-content .kdoc-wrap-comments .kdoc-comment-prefix,.kdoc-mkdocs-content .kdoc-wrap-comments .kdoc-comment-body{overflow-wrap:normal;white-space:pre}
.kdoc-mkdocs-content .kdoc-view-controls{display:none}
@media(max-width:1100px){.kdoc-mkdocs-shell{grid-template-columns:minmax(0,1fr)}.kdoc-mkdocs-sidebar{display:none}}
</style>
`
}
