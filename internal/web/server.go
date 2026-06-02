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

	if _, err := fmt.Fprintf(out, "http://%s/\n", listener.Addr().String()); err != nil {
		_ = server.Close()
		return err
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
	renderDocs(w, config.Renderer, []*crd.Document{doc})
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
.kdoc-group h2{font-size:16px;margin:0 0 8px}
.kdoc-resource{margin:8px 0}
.kdoc-resource strong{display:block}
.kdoc-version{display:inline-block;margin:4px 8px 0 0}
.kdoc-version a{color:#0969da;text-decoration:none}
.kdoc-version a:focus,.kdoc-version a:hover{text-decoration:underline}
</style>`)
	_, _ = fmt.Fprint(out, "\n</head>\n<body>\n<main class=\"kubectl-doc\">\n<h1>Kubernetes resources</h1>\n<ul class=\"kdoc-overview\">\n")
	if overview != nil {
		for _, group := range overview.Groups {
			_, _ = fmt.Fprintf(out, "<li class=\"kdoc-group\"><h2>%s</h2>\n", escape(group.Name))
			for _, resource := range group.Resources {
				_, _ = fmt.Fprintf(out, "<div class=\"kdoc-resource\"><strong>%s</strong>", escape(resource.Name))
				for _, version := range resource.Versions {
					_, _ = fmt.Fprintf(out, "<span class=\"kdoc-version\"><a href=\"%s\">%s</a></span>", escape(linkFor(group.Name, resource.Name, version)), escape(version))
				}
				_, _ = fmt.Fprint(out, "</div>\n")
			}
			_, _ = fmt.Fprint(out, "</li>\n")
		}
	}
	_, _ = fmt.Fprint(out, "</ul>\n</main>\n</body>\n</html>\n")
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
