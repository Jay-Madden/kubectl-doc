package htmlrender

import (
	"bytes"
	"fmt"
	htmlpkg "html"
	"io"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

const fullExpandDepth = 1000

type Renderer struct {
	ExpandDepth  int
	Descriptions yamlrender.DescriptionMode
	Columns      int
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	return r.RenderAll(out, []*crd.Document{doc})
}

func (r Renderer) RenderAll(out io.Writer, docs []*crd.Document) error {
	docs = compactDocuments(docs)
	if len(docs) == 0 {
		return fmt.Errorf("at least one document is required")
	}

	if _, err := fmt.Fprintf(out, "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n%s\n</head>\n<body>\n", escape(docs[0].Kind), styleElement()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "<main class=\"kubectl-doc\" data-kubectl-doc>\n<header class=\"kdoc-header\">\n<h1>%s</h1>\n", escape(docs[0].Kind)); err != nil {
		return err
	}
	if err := renderMetadata(out, docs); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "<div class=\"kdoc-search\"><input type=\"search\" aria-label=\"Search\" placeholder=\"Search\" data-kdoc-search></div>\n</header>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "<div class=\"kdoc-layout\"><section class=\"kdoc-docs\">"); err != nil {
		return err
	}

	multiple := len(docs) > 1
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if err := r.renderDocument(out, doc, multiple); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "</section><aside class=\"kdoc-details\" data-kdoc-details aria-live=\"polite\"><h2>Details</h2><pre></pre></aside></div>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s\n</main>\n</body>\n</html>\n", scriptElement()); err != nil {
		return err
	}
	return nil
}

func (r Renderer) renderDocument(out io.Writer, doc *crd.Document, multiple bool) error {
	title := "YAML"
	if multiple {
		title = apiVersion(doc.Group, doc.Version)
		if _, err := fmt.Fprintf(out, "<section class=\"kdoc-version\"><h2>%s</h2>\n", escape(title)); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintln(out, "<section class=\"kdoc-version\">"); err != nil {
		return err
	}

	var yaml bytes.Buffer
	if err := (yamlrender.Renderer{
		ExpandDepth:  fullExpandDepth,
		Descriptions: r.Descriptions,
		Columns:      r.Columns,
	}).Render(&yaml, doc); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "<div class=\"kdoc-tree\" role=\"tree\" aria-label=\"%s\">\n", escape(title)); err != nil {
		return err
	}
	for _, line := range buildLines(yaml.String(), r.initialExpandDepth()) {
		if err := renderLine(out, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "</div>\n</section>")
	return err
}

func (r Renderer) initialExpandDepth() int {
	if r.ExpandDepth < 0 {
		return 0
	}
	return r.ExpandDepth
}

func renderLine(out io.Writer, line yamlLine) error {
	classes := "kdoc-line"
	if strings.TrimSpace(line.Text) == "" {
		classes += " kdoc-blank"
	}

	if _, err := fmt.Fprintf(out, "<div class=\"%s\" role=\"treeitem\" data-kdoc-line data-index=\"%d\" data-depth=\"%d\" data-search=\"%s\" data-field=\"%s\" data-path=\"%s\">",
		classes,
		line.Index,
		line.Depth,
		escapeAttr(strings.ToLower(line.Text)),
		escapeAttr(strings.ToLower(line.Field)),
		escapeAttr(line.Path),
	); err != nil {
		return err
	}
	if line.Foldable {
		expanded := "true"
		glyph := "▼"
		if line.Collapsed {
			expanded = "false"
			glyph = "▶"
		}
		if _, err := fmt.Fprintf(out, "<button class=\"kdoc-fold\" type=\"button\" aria-label=\"Toggle\" aria-expanded=\"%s\" data-kdoc-toggle>%s</button>", expanded, glyph); err != nil {
			return err
		}
	} else if _, err := fmt.Fprint(out, "<span class=\"kdoc-gutter\"></span>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "<span class=\"kdoc-yaml-text\">%s</span></div>\n", escape(line.Text)); err != nil {
		return err
	}
	return nil
}

type yamlLine struct {
	Index     int
	Text      string
	Depth     int
	Foldable  bool
	Collapsed bool
	Field     string
	Path      string
}

func buildLines(rendered string, expandDepth int) []yamlLine {
	rawLines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	lines := make([]yamlLine, 0, len(rawLines))
	paths := map[int]string{}
	for i, raw := range rawLines {
		depth := lineDepth(raw)
		field := fieldName(raw)
		path := ""
		if field != "" {
			paths[depth] = field
			for existingDepth := range paths {
				if existingDepth > depth {
					delete(paths, existingDepth)
				}
			}
			path = joinPath(paths, depth)
		}
		lines = append(lines, yamlLine{
			Index: i,
			Text:  raw,
			Depth: depth,
			Field: field,
			Path:  path,
		})
	}

	for i := range lines {
		if strings.TrimSpace(lines[i].Text) == "" {
			continue
		}
		nextDepth, ok := nextContentDepth(lines, i)
		lines[i].Foldable = ok && nextDepth > lines[i].Depth
		lines[i].Collapsed = lines[i].Foldable && lines[i].Depth >= expandDepth
	}
	return lines
}

func lineDepth(line string) int {
	spaces := len(line) - len(strings.TrimLeft(line, " "))
	return spaces / 2
}

func nextContentDepth(lines []yamlLine, index int) (int, bool) {
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].Text) == "" {
			continue
		}
		return lines[i].Depth, true
	}
	return 0, false
}

func fieldName(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	}
	if strings.HasPrefix(trimmed, "# ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
	}
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	}
	commentIndex := strings.Index(trimmed, " # ")
	if commentIndex >= 0 {
		trimmed = trimmed[:commentIndex]
	}
	colon := strings.Index(trimmed, ":")
	if colon <= 0 {
		return ""
	}
	key := strings.TrimSpace(trimmed[:colon])
	if key == "" || strings.ContainsAny(key, " \t{}[]") {
		return ""
	}
	return strings.Trim(key, `"'`)
}

func joinPath(paths map[int]string, depth int) string {
	parts := make([]string, 0, depth+1)
	for i := 0; i <= depth; i++ {
		if part := paths[i]; part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ".")
}

func renderMetadata(out io.Writer, docs []*crd.Document) error {
	doc := docs[0]
	if _, err := fmt.Fprintln(out, "<table class=\"kdoc-metadata\"><tbody>"); err != nil {
		return err
	}
	if len(docs) == 1 {
		if err := metadataRow(out, "API Version", apiVersion(doc.Group, doc.Version)); err != nil {
			return err
		}
	} else if err := metadataRow(out, "Versions", versionList(docs)); err != nil {
		return err
	}
	if err := metadataRow(out, "Kind", doc.Kind); err != nil {
		return err
	}
	if doc.Plural != "" {
		if err := metadataRow(out, "Resource", doc.Plural); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "</tbody></table>")
	return err
}

func metadataRow(out io.Writer, label, value string) error {
	_, err := fmt.Fprintf(out, "<tr><th>%s</th><td><code>%s</code></td></tr>\n", escape(label), escape(value))
	return err
}

func versionList(docs []*crd.Document) string {
	versions := make([]string, 0, len(docs))
	for _, doc := range docs {
		versions = append(versions, apiVersion(doc.Group, doc.Version))
	}
	return strings.Join(versions, ", ")
}

func compactDocuments(docs []*crd.Document) []*crd.Document {
	var out []*crd.Document
	for _, doc := range docs {
		if doc != nil {
			out = append(out, doc)
		}
	}
	return out
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func escape(value string) string {
	return htmlpkg.EscapeString(value)
}

func escapeAttr(value string) string {
	return htmlpkg.EscapeString(value)
}

func styleElement() string {
	return `<style>
.kubectl-doc{box-sizing:border-box;color:#1f2933;background:#fff;font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:100%;padding:24px}
.kubectl-doc *{box-sizing:border-box}
.kdoc-header{border-bottom:1px solid #d8dee4;margin-bottom:16px;padding-bottom:16px}
.kdoc-header h1{font-size:24px;line-height:1.2;margin:0 0 12px}
.kdoc-metadata{border-collapse:collapse;margin:0 0 12px}
.kdoc-metadata th{color:#57606a;font-weight:600;padding:2px 16px 2px 0;text-align:left}
.kdoc-metadata td{padding:2px 0}
.kdoc-search input{border:1px solid #afb8c1;border-radius:6px;font:inherit;max-width:360px;padding:6px 8px;width:100%}
.kdoc-layout{display:grid;gap:16px;grid-template-columns:minmax(0,1fr) minmax(240px,320px)}
.kdoc-docs{min-width:0}
.kdoc-version h2{font-size:18px;margin:16px 0 8px}
.kdoc-tree{background:#f6f8fa;border:1px solid #d8dee4;border-radius:8px;overflow:auto;padding:12px 0}
.kdoc-line{align-items:baseline;display:flex;font:13px/1.55 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;margin:0;min-height:20px;padding:0 12px;white-space:pre}
.kdoc-line[hidden]{display:none}
.kdoc-fold,.kdoc-gutter{background:transparent;border:0;color:#57606a;flex:0 0 24px;font:inherit;height:20px;margin:0;padding:0;text-align:left}
.kdoc-fold{cursor:pointer}
.kdoc-yaml-text{white-space:pre}
.kdoc-match .kdoc-yaml-text{background:#ff8c00;color:#111;padding:1px 0}
.kdoc-current .kdoc-yaml-text{box-shadow:inset 3px 0 0 #111;padding-left:4px}
.kdoc-selected .kdoc-yaml-text{background:#fff7cc}
.kdoc-details{border:1px solid #d8dee4;border-radius:8px;min-width:0;padding:12px;position:sticky;top:12px}
.kdoc-details h2{font-size:16px;margin:0 0 8px}
.kdoc-details pre{font:12px/1.45 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;margin:0;white-space:pre-wrap}
@media(max-width:900px){.kubectl-doc{padding:16px}.kdoc-layout{grid-template-columns:1fr}.kdoc-details{position:static}}
</style>`
}

func scriptElement() string {
	return `<script>
(function(){
  function ready(fn){ if(document.readyState !== "loading"){ fn(); } else { document.addEventListener("DOMContentLoaded", fn); } }
  ready(function(){
    document.querySelectorAll("[data-kubectl-doc]").forEach(function(root){
      var lines = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-line]"));
      var search = root.querySelector("[data-kdoc-search]");
      var details = root.querySelector("[data-kdoc-details] pre");
      var results = [];
      var current = -1;

      function button(line){ return line.querySelector("[data-kdoc-toggle]"); }
      function depth(line){ return Number(line.getAttribute("data-depth") || "0"); }
      function expanded(line){ var b = button(line); return !b || b.getAttribute("aria-expanded") !== "false"; }
      function setExpanded(line, value){
        var b = button(line);
        if(!b){ return; }
        b.setAttribute("aria-expanded", value ? "true" : "false");
        b.textContent = value ? "▼" : "▶";
      }
      function applyFolds(){
        lines.forEach(function(line){ line.hidden = false; });
        lines.forEach(function(line, index){
          if(line.hidden || expanded(line)){ return; }
          var parentDepth = depth(line);
          for(var i = index + 1; i < lines.length; i++){
            if(depth(lines[i]) <= parentDepth && lines[i].textContent.trim() !== ""){ break; }
            lines[i].hidden = true;
          }
        });
      }
      function reveal(line){
        var index = lines.indexOf(line);
        var targetDepth = depth(line);
        for(var i = index - 1; i >= 0; i--){
          var candidateDepth = depth(lines[i]);
          if(candidateDepth < targetDepth){
            setExpanded(lines[i], true);
            targetDepth = candidateDepth;
          }
        }
        applyFolds();
      }
      function select(line){
        lines.forEach(function(item){ item.classList.remove("kdoc-selected"); });
        line.classList.add("kdoc-selected");
        if(details){
          var path = line.getAttribute("data-path");
          var text = line.querySelector(".kdoc-yaml-text").textContent;
          details.textContent = (path ? path + "\n\n" : "") + text;
        }
      }
      function focusResult(next){
        if(results.length === 0){ return; }
        current = (next + results.length) % results.length;
        lines.forEach(function(line){ line.classList.remove("kdoc-current"); });
        var line = results[current];
        reveal(line);
        line.classList.add("kdoc-current");
        select(line);
        line.scrollIntoView({block:"center"});
      }
      function applySearch(){
        var query = (search && search.value || "").toLowerCase();
        var fieldOnly = query.indexOf("/") === 0;
        if(fieldOnly){ query = query.slice(1); }
        results = [];
        current = -1;
        lines.forEach(function(line){
          line.classList.remove("kdoc-match", "kdoc-current");
          if(query === ""){ return; }
          var haystack = fieldOnly ? line.getAttribute("data-field") : line.getAttribute("data-search");
          if((haystack || "").indexOf(query) >= 0){
            line.classList.add("kdoc-match");
            results.push(line);
          }
        });
        if(results.length > 0){ focusResult(0); }
      }

      root.addEventListener("click", function(event){
        var toggle = event.target.closest("[data-kdoc-toggle]");
        if(toggle){
          var line = toggle.closest("[data-kdoc-line]");
          setExpanded(line, !expanded(line));
          applyFolds();
          select(line);
          return;
        }
        var line = event.target.closest("[data-kdoc-line]");
        if(line){ select(line); }
      });
      if(search){
        search.addEventListener("input", applySearch);
        search.addEventListener("keydown", function(event){
          if(event.key === "Escape"){ search.value = ""; applySearch(); search.blur(); event.preventDefault(); }
          if(event.key === "n" || event.key === "ArrowDown"){ focusResult(current + 1); event.preventDefault(); }
          if(event.key === "p" || event.key === "ArrowUp"){ focusResult(current - 1); event.preventDefault(); }
        });
      }
      document.addEventListener("keydown", function(event){
        var tag = event.target && event.target.tagName;
        if(event.key === "/" && tag !== "INPUT" && tag !== "TEXTAREA" && search){
          search.focus();
          event.preventDefault();
        }
      });
      applyFolds();
    });
  });
})();
</script>`
}
