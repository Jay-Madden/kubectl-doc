package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	htmlrender "github.com/sttts/kubectl-doc/internal/render/html"
	krorender "github.com/sttts/kubectl-doc/internal/render/kro"
	markdownrender "github.com/sttts/kubectl-doc/internal/render/markdown"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	"github.com/sttts/kubectl-doc/internal/web"
)

type Options struct {
	Filenames    []string
	Output       string
	NoColor      bool
	Version      string
	AllVersions  bool
	ExpandDepth  int
	Descriptions string
	Columns      int
	Interactive  bool
	Web          bool
	FieldDetails bool
	OpenBrowser  func(string) error
}

const (
	OutputYAML           = "yaml"
	OutputTUI            = "tui"
	OutputBrowser        = "browser"
	OutputHTML           = "html"
	OutputMarkdown       = "markdown"
	OutputMarkdownGitHub = "markdown-github"
	OutputMarkdownFern   = "markdown-fern"
	OutputKro            = "kro"
)

type Dependencies struct {
	LoadOverview         func() (*kube.Overview, error)
	LoadResourceResolver func() (*kube.ResourceResolver, error)
	LoadOpenAPIClient    func() (*kube.OpenAPIClient, error)
	OpenBrowser          func(string) error
}

func NewCommand(out, errOut io.Writer) *cobra.Command {
	return NewCommandWithDeps(out, errOut, Dependencies{
		LoadOverview:         kube.LoadOverview,
		LoadResourceResolver: kube.LoadResourceResolver,
		LoadOpenAPIClient:    kube.LoadOpenAPIClient,
		OpenBrowser:          openBrowser,
	})
}

func NewCommandWithDeps(out, errOut io.Writer, deps Dependencies) *cobra.Command {
	if deps.LoadOverview == nil {
		deps.LoadOverview = kube.LoadOverview
	}
	if deps.LoadResourceResolver == nil {
		deps.LoadResourceResolver = kube.LoadResourceResolver
	}
	if deps.LoadOpenAPIClient == nil {
		deps.LoadOpenAPIClient = kube.LoadOpenAPIClient
	}

	opts := Options{
		Output:       OutputYAML,
		ExpandDepth:  2,
		Descriptions: string(yamlrender.DescriptionTrue),
		OpenBrowser:  deps.OpenBrowser,
	}

	cmd := &cobra.Command{
		Use:          "kubectl-doc [resource]",
		Short:        "Render Kubernetes API documentation",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.normalizeShortcuts(cmd); err != nil {
				return err
			}
			if err := opts.validate(args); err != nil {
				return err
			}

			if len(opts.Filenames) == 0 && len(args) == 0 {
				if opts.Output == OutputBrowser {
					return opts.serveBrowserOverview(contextFromCommand(cmd), out, deps)
				}
				if opts.Output != OutputYAML {
					return fmt.Errorf("resource selector required for -o %s", opts.Output)
				}
				overview, err := deps.LoadOverview()
				if err != nil {
					return err
				}
				renderer := yamlrender.OverviewRenderer{
					Color: supportsColor(out, opts.NoColor),
				}
				return renderer.Render(out, overview)
			}
			if len(opts.Filenames) == 0 {
				resolver, err := deps.LoadResourceResolver()
				if err != nil {
					return err
				}
				if opts.AllVersions {
					resolvedVersions, err := resolver.ResolveAllVersions(args[0])
					if err != nil {
						return err
					}
					openAPIClient, err := deps.LoadOpenAPIClient()
					if err != nil {
						return err
					}
					var docs []*crd.Document
					for _, resolved := range resolvedVersions {
						openAPIDocument, err := openAPIClient.GroupVersionDocument(contextFromCommand(cmd), resolved.Group, resolved.Version)
						if err != nil {
							return err
						}
						doc, err := kube.BuildDocumentFromOpenAPIV3WithNativeFallback(openAPIDocument, resolved)
						if err != nil {
							return err
						}
						docs = append(docs, doc)
					}
					return opts.renderDocuments(contextFromCommand(cmd), out, docs)
				}

				resolved, err := resolver.Resolve(args[0])
				if err != nil {
					return err
				}
				openAPIClient, err := deps.LoadOpenAPIClient()
				if err != nil {
					return err
				}
				openAPIDocument, err := openAPIClient.GroupVersionDocument(contextFromCommand(cmd), resolved.Group, resolved.Version)
				if err != nil {
					return err
				}
				doc, err := kube.BuildDocumentFromOpenAPIV3WithNativeFallback(openAPIDocument, resolved)
				if err != nil {
					return err
				}

				return opts.renderDocument(contextFromCommand(cmd), out, doc)
			}

			if opts.AllVersions {
				docs, err := crd.LoadAllVersions(opts.Filenames)
				if err != nil {
					return err
				}
				return opts.renderDocuments(contextFromCommand(cmd), out, docs)
			}

			doc, err := crd.Load(opts.Filenames, opts.Version)
			if err != nil {
				return err
			}

			return opts.renderDocument(contextFromCommand(cmd), out, doc)
		},
	}

	cmd.SetOut(out)
	cmd.SetErr(errOut)

	cmd.Flags().StringArrayVarP(&opts.Filenames, "filename", "f", nil, "CRD manifest path")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", OutputYAML, "output format")
	cmd.Flags().BoolVar(&opts.NoColor, "nocolor", false, "disable color in yaml output")
	cmd.Flags().StringVar(&opts.Version, "version", "", "served CRD version")
	cmd.Flags().BoolVar(&opts.AllVersions, "all-versions", false, "render all served versions where supported")
	cmd.Flags().IntVar(&opts.ExpandDepth, "expand-depth", 2, "initial expansion depth")
	cmd.Flags().StringVar(&opts.Descriptions, "descriptions", string(yamlrender.DescriptionTrue), "render descriptions: false, required, or true")
	cmd.Flags().IntVar(&opts.Columns, "columns", 0, "target columns for Markdown paragraph wrapping")
	cmd.Flags().BoolVar(&opts.FieldDetails, "field-details", false, "render Markdown field detail sections")
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "shortcut for -o tui")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "shortcut for -o browser")

	return cmd
}

func contextFromCommand(cmd *cobra.Command) context.Context {
	if cmd.Context() == nil {
		return context.Background()
	}
	return cmd.Context()
}

func (o *Options) normalizeShortcuts(cmd *cobra.Command) error {
	outputChanged := cmd.Flags().Changed("output")
	if o.Interactive && o.Web {
		return fmt.Errorf("--interactive conflicts with --web")
	}
	if o.Interactive {
		if outputChanged && o.Output != OutputTUI {
			return fmt.Errorf("--interactive conflicts with -o %s", o.Output)
		}
		o.Output = OutputTUI
	}
	if o.Web {
		if outputChanged && o.Output != OutputBrowser {
			return fmt.Errorf("--web conflicts with -o %s", o.Output)
		}
		o.Output = OutputBrowser
	}
	return nil
}

func (o Options) validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("expected at most one resource selector")
	}
	switch o.Output {
	case OutputYAML, OutputHTML, OutputBrowser, OutputMarkdown, OutputMarkdownGitHub, OutputMarkdownFern, OutputKro:
	case OutputTUI:
		return fmt.Errorf("-o %s is not implemented yet", o.Output)
	default:
		return fmt.Errorf("unsupported output format %q", o.Output)
	}
	if o.AllVersions {
		if o.Output == OutputYAML {
			return fmt.Errorf("--all-versions is not supported with -o yaml")
		}
		switch o.Output {
		case OutputHTML, OutputBrowser, OutputMarkdown, OutputMarkdownGitHub, OutputMarkdownFern, OutputKro:
		default:
			return fmt.Errorf("--all-versions is not implemented yet for -o %s", o.Output)
		}
	}
	if o.ExpandDepth < 0 {
		return fmt.Errorf("--expand-depth must be non-negative")
	}
	if o.Columns < 0 {
		return fmt.Errorf("--columns must be non-negative")
	}
	switch yamlrender.DescriptionMode(o.Descriptions) {
	case yamlrender.DescriptionFalse, yamlrender.DescriptionRequired, yamlrender.DescriptionTrue:
	default:
		return fmt.Errorf("--descriptions must be one of false, required, true")
	}
	if len(o.Filenames) > 0 {
		if len(args) > 0 {
			return fmt.Errorf("resource selectors are not supported with -f; the CRD resource is implicit")
		}
		if o.AllVersions && o.Version != "" {
			return fmt.Errorf("--all-versions conflicts with --version")
		}
		return nil
	}
	if o.Version != "" {
		return fmt.Errorf("--version requires -f; use resource.version.group selector syntax in cluster mode")
	}
	return nil
}

func (o Options) renderDocument(ctx context.Context, out io.Writer, doc *crd.Document) error {
	return o.renderDocuments(ctx, out, []*crd.Document{doc})
}

func (o Options) renderDocuments(ctx context.Context, out io.Writer, docs []*crd.Document) error {
	switch o.Output {
	case OutputYAML:
		if len(docs) != 1 {
			return fmt.Errorf("-o yaml requires exactly one document")
		}
		renderer := yamlrender.Renderer{
			ExpandDepth:  o.ExpandDepth,
			Color:        supportsColor(out, o.NoColor),
			Descriptions: yamlrender.DescriptionMode(o.Descriptions),
		}
		return renderer.Render(out, docs[0])
	case OutputHTML:
		renderer := o.htmlRenderer()
		return renderer.RenderAll(out, docs)
	case OutputBrowser:
		return web.Serve(ctx, out, web.Config{
			Docs:     docs,
			Renderer: o.htmlRenderer(),
			OpenURL:  o.OpenBrowser,
		})
	case OutputMarkdown, OutputMarkdownGitHub:
		renderer := markdownrender.Renderer{
			Dialect:          markdownrender.DialectGitHub,
			ExpandDepth:      o.ExpandDepth,
			Descriptions:     yamlrender.DescriptionMode(o.Descriptions),
			Columns:          markdownColumns(out, o.Columns),
			HideFieldDetails: !o.FieldDetails,
		}
		return renderer.RenderAll(out, docs)
	case OutputMarkdownFern:
		renderer := markdownrender.Renderer{
			Dialect:          markdownrender.DialectFern,
			ExpandDepth:      o.ExpandDepth,
			Descriptions:     yamlrender.DescriptionMode(o.Descriptions),
			Columns:          markdownColumns(out, o.Columns),
			HideFieldDetails: !o.FieldDetails,
		}
		return renderer.RenderAll(out, docs)
	case OutputKro:
		renderer := krorender.Renderer{
			Descriptions: yamlrender.DescriptionMode(o.Descriptions),
		}
		return renderer.RenderAll(out, docs)
	default:
		return fmt.Errorf("unsupported output format %q", o.Output)
	}
}

func (o Options) serveBrowserOverview(ctx context.Context, out io.Writer, deps Dependencies) error {
	overview, err := deps.LoadOverview()
	if err != nil {
		return err
	}

	var resolverOnce sync.Once
	var resolver *kube.ResourceResolver
	var resolverErr error
	var openAPIOnce sync.Once
	var openAPIClient *kube.OpenAPIClient
	var openAPIErr error

	loadResolver := func() (*kube.ResourceResolver, error) {
		resolverOnce.Do(func() {
			resolver, resolverErr = deps.LoadResourceResolver()
		})
		return resolver, resolverErr
	}
	loadOpenAPIClient := func() (*kube.OpenAPIClient, error) {
		openAPIOnce.Do(func() {
			openAPIClient, openAPIErr = deps.LoadOpenAPIClient()
		})
		return openAPIClient, openAPIErr
	}

	return web.Serve(ctx, out, web.Config{
		Overview: overview,
		Renderer: o.htmlRenderer(),
		OpenURL:  o.OpenBrowser,
		LoadDocument: func(ctx context.Context, group, version, resource string) (*crd.Document, error) {
			resolver, err := loadResolver()
			if err != nil {
				return nil, err
			}
			resolved, err := resolver.ResolveGroupVersionResource(group, version, resource)
			if err != nil {
				return nil, err
			}
			openAPIClient, err := loadOpenAPIClient()
			if err != nil {
				return nil, err
			}
			return buildClusterDocument(ctx, openAPIClient, resolved)
		},
	})
}

func openBrowser(rawURL string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return exec.Command("open", rawURL).Start()
}

func (o Options) htmlRenderer() htmlrender.Renderer {
	return htmlrender.Renderer{
		ExpandDepth:  o.ExpandDepth,
		Descriptions: yamlrender.DescriptionMode(o.Descriptions),
		Columns:      o.Columns,
	}
}

func buildClusterDocument(ctx context.Context, openAPIClient *kube.OpenAPIClient, resolved kube.ResourceIdentity) (*crd.Document, error) {
	openAPIDocument, err := openAPIClient.GroupVersionDocument(ctx, resolved.Group, resolved.Version)
	if err != nil {
		return nil, err
	}
	return kube.BuildDocumentFromOpenAPIV3WithNativeFallback(openAPIDocument, resolved)
}

func markdownColumns(out io.Writer, columns int) int {
	if columns > 0 {
		return columns
	}
	if width := terminalWidth(out); width > 0 {
		return width
	}
	return 80
}

func terminalWidth(out io.Writer) int {
	file, ok := out.(*os.File)
	if !ok {
		return 0
	}
	if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
		return width
	}
	return 0
}

func supportsColor(out io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
