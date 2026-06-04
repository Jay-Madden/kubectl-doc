package web

import (
	"context"
	"errors"
	"fmt"
	htmlpkg "html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	htmlrender "github.com/sttts/kubectl-doc/internal/render/html"
)

type DocumentLoader func(context.Context, string, string, string) (*crd.Document, error)

type Config struct {
	Docs         []*crd.Document
	Overview     *kube.Overview
	LoadDocument DocumentLoader
	Renderer     htmlrender.Renderer
	OpenURL      func(string) error
}

func Serve(ctx context.Context, out io.Writer, config Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if out == nil {
		out = io.Discard
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on localhost: %w", err)
	}

	server := &http.Server{
		Handler: handler(config),
	}
	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErr <- err
	}()

	browserURL := fmt.Sprintf("http://%s/", listener.Addr().String())
	if _, err := fmt.Fprintln(out, browserURL); err != nil {
		_ = server.Close()
		return err
	}
	if config.OpenURL != nil {
		_ = config.OpenURL(browserURL)
	}

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return err
		}
		return <-serverErr
	case err := <-serverErr:
		return err
	}
}

func handler(config Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if len(compactDocuments(config.Docs)) > 0 {
			renderDocs(w, config.Renderer, config.Docs)
			return
		}

		query := r.URL.Query()
		if query.Get("resource") != "" || query.Get("version") != "" {
			renderSelectedResource(r.Context(), w, config, query)
			return
		}
		renderOverview(w, config.Overview)
	})
	return mux
}

func renderSelectedResource(ctx context.Context, w http.ResponseWriter, config Config, query url.Values) {
	if config.LoadDocument == nil {
		http.Error(w, "schema loading is not configured", http.StatusInternalServerError)
		return
	}
	resource := strings.TrimSpace(query.Get("resource"))
	version := strings.TrimSpace(query.Get("version"))
	group := strings.TrimSpace(query.Get("group"))
	if resource == "" || version == "" {
		http.Error(w, "resource and version query parameters are required", http.StatusBadRequest)
		return
	}

	doc, err := config.LoadDocument(ctx, group, version, resource)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderer := config.Renderer
	renderer.BackURL = "/"
	renderDocs(w, renderer, []*crd.Document{doc})
}

func renderDocs(w http.ResponseWriter, renderer htmlrender.Renderer, docs []*crd.Document) {
	if err := renderer.RenderAll(w, docs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderOverview(out io.Writer, overview *kube.Overview) {
	_, _ = fmt.Fprint(out, "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>Kubernetes resources</title>\n")
	_, _ = fmt.Fprint(out, `<style>
.kubectl-doc{box-sizing:border-box;color:#1f2933;background:#fff;font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:100%;padding:24px}
.kubectl-doc *{box-sizing:border-box}
.kubectl-doc h1{font-size:24px;line-height:1.2;margin:0 0 16px}
.kdoc-overview{display:grid;gap:18px;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));margin:0;padding:0}
.kdoc-group{border:1px solid #d8dee4;border-radius:8px;list-style:none;margin:0;padding:12px}
.kdoc-group h2{color:#007c89;font-size:16px;margin:0 0 8px}
.kdoc-resource{align-items:baseline;display:flex;flex-wrap:wrap;gap:6px;margin:6px 0}
.kdoc-resource.kdoc-overview-row-selected{background:#fff7cc}
.kdoc-resource-name{color:#1f2933}
.kdoc-version{display:inline-block}
.kdoc-version a{border-radius:4px;color:#0969da;padding:0 3px;text-decoration:none}
.kdoc-version a:focus,.kdoc-version a:hover{text-decoration:underline}
.kdoc-version a.kdoc-overview-selected{background:#fff7cc;color:#0550ae;font-weight:600;outline:1px solid #f0d35b}
</style>`)
	_, _ = fmt.Fprint(out, "\n</head>\n<body>\n<main class=\"kubectl-doc\" data-kdoc-overview-root>\n<h1>Kubernetes resources</h1>\n<ul class=\"kdoc-overview\" data-kdoc-overview>\n")
	index := 0
	if overview != nil {
		for _, group := range overview.Groups {
			_, _ = fmt.Fprintf(out, "<li class=\"kdoc-group\"><h2>%s</h2>\n", escape(group.Name))
			for _, resource := range group.Resources {
				_, _ = fmt.Fprintf(out, "<div class=\"kdoc-resource\"><span class=\"kdoc-resource-name\">%s</span>", escape(resource.Name))
				for _, version := range resource.Versions {
					_, _ = fmt.Fprintf(out, "<span class=\"kdoc-version\"><a href=\"%s\" data-kdoc-overview-item data-index=\"%d\">%s</a></span>", escape(linkFor(group.Name, resource.Name, version)), index, escape(version))
					index++
				}
				_, _ = fmt.Fprint(out, "</div>\n")
			}
			_, _ = fmt.Fprint(out, "</li>\n")
		}
	}
	_, _ = fmt.Fprint(out, "</ul>\n</main>\n")
	_, _ = fmt.Fprint(out, overviewScript())
	_, _ = fmt.Fprint(out, "\n</body>\n</html>\n")
}

func overviewScript() string {
	return `<script>
(function(){
  var storageKey = "kubectl-doc-overview-focus";
  function ready(fn){ if(document.readyState !== "loading"){ fn(); } else { document.addEventListener("DOMContentLoaded", fn); } }
  function storageGet(){
    try { return window.sessionStorage.getItem(storageKey); } catch(_err) { return null; }
  }
  function storageSet(value){
    try { window.sessionStorage.setItem(storageKey, String(value)); } catch(_err) {}
  }
  ready(function(){
    var items = Array.prototype.slice.call(document.querySelectorAll("[data-kdoc-overview-item]"));
    if(!items.length){ return; }
    var selected = Number(storageGet());
    if(!Number.isFinite(selected) || selected < 0 || selected >= items.length){ selected = 0; }
    function rowFor(item){ return item.closest(".kdoc-resource"); }
    function pageDistance(){
      var item = items[selected] || items[0];
      var height = item ? Math.max(item.getBoundingClientRect().height, 18) : 18;
      return Math.max(1, Math.floor(window.innerHeight / height / 2));
    }
    function verticalOverlap(a, b){
      return Math.max(0, Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top));
    }
    function selectHorizontal(direction){
      var current = items[selected];
      if(!current){ return false; }
      var currentRect = current.getBoundingClientRect();
      var currentCenterX = currentRect.left + currentRect.width / 2;
      var currentCenterY = currentRect.top + currentRect.height / 2;
      var best = -1;
      var bestScore = Infinity;
      items.forEach(function(item, index){
        if(index === selected){ return; }
        var rect = item.getBoundingClientRect();
        var centerX = rect.left + rect.width / 2;
        if(direction < 0 && centerX >= currentCenterX){ return; }
        if(direction > 0 && centerX <= currentCenterX){ return; }
        var centerY = rect.top + rect.height / 2;
        var overlapPenalty = verticalOverlap(currentRect, rect) > 0 ? 0 : Math.abs(centerY - currentCenterY) * 4;
        var score = Math.abs(centerX - currentCenterX) + overlapPenalty;
        if(score < bestScore){
          best = index;
          bestScore = score;
        }
      });
      if(best < 0){ return false; }
      select(best, true);
      return true;
    }
    function select(index, scroll){
      selected = Math.max(0, Math.min(items.length - 1, index));
      items.forEach(function(item){
        item.classList.remove("kdoc-overview-selected");
        item.removeAttribute("aria-selected");
        var row = rowFor(item);
        if(row){ row.classList.remove("kdoc-overview-row-selected"); }
      });
      var item = items[selected];
      item.classList.add("kdoc-overview-selected");
      item.setAttribute("aria-selected", "true");
      var row = rowFor(item);
      if(row){ row.classList.add("kdoc-overview-row-selected"); }
      storageSet(selected);
      if(scroll && item.scrollIntoView){ item.scrollIntoView({block:"nearest", inline:"nearest"}); }
    }
    items.forEach(function(item, index){
      item.addEventListener("click", function(){ select(index, false); });
      item.addEventListener("focus", function(){ select(index, false); });
    });
    document.addEventListener("keydown", function(event){
      if(event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey){ return; }
      var handled = true;
      switch(event.key){
      case "ArrowUp":
        select(selected - 1, true);
        break;
      case "ArrowDown":
        select(selected + 1, true);
        break;
      case "ArrowLeft":
        handled = selectHorizontal(-1);
        break;
      case "ArrowRight":
        handled = selectHorizontal(1);
        break;
      case "Home":
        select(0, true);
        break;
      case "End":
        select(items.length - 1, true);
        break;
      case "PageUp":
        select(selected - pageDistance(), true);
        break;
      case "PageDown":
        select(selected + pageDistance(), true);
        break;
      case "Enter":
        storageSet(selected);
        window.location.href = items[selected].href;
        break;
      default:
        handled = false;
      }
      if(handled){
        event.preventDefault();
        event.stopPropagation();
      }
    });
    select(selected, true);
  });
})();
</script>`
}

func linkFor(group, resource, version string) string {
	values := url.Values{}
	if group != "" && group != kube.CoreGroup {
		values.Set("group", group)
	}
	values.Set("resource", resource)
	values.Set("version", version)
	return "/?" + values.Encode()
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

func escape(value string) string {
	return htmlpkg.EscapeString(value)
}
