package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

type Options struct {
	Filenames    []string
	Output       string
	NoColor      bool
	Version      string
	AllVersions  bool
	ExpandDepth  int
	Descriptions string
	Interactive  bool
	Web          bool
}

const (
	OutputYAML    = "yaml"
	OutputTUI     = "tui"
	OutputBrowser = "browser"
)

type Dependencies struct {
	LoadOverview func() (*kube.Overview, error)
}

func NewCommand(out, errOut io.Writer) *cobra.Command {
	return NewCommandWithDeps(out, errOut, Dependencies{
		LoadOverview: kube.LoadOverview,
	})
}

func NewCommandWithDeps(out, errOut io.Writer, deps Dependencies) *cobra.Command {
	if deps.LoadOverview == nil {
		deps.LoadOverview = kube.LoadOverview
	}

	opts := Options{
		Output:       OutputYAML,
		ExpandDepth:  2,
		Descriptions: string(yamlrender.DescriptionTrue),
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

			if len(opts.Filenames) == 0 {
				overview, err := deps.LoadOverview()
				if err != nil {
					return err
				}
				renderer := yamlrender.OverviewRenderer{
					Color: supportsColor(out, opts.NoColor),
				}
				return renderer.Render(out, overview)
			}

			doc, err := crd.Load(opts.Filenames, opts.Version)
			if err != nil {
				return err
			}

			renderer := yamlrender.Renderer{
				ExpandDepth:  opts.ExpandDepth,
				Color:        supportsColor(out, opts.NoColor),
				Descriptions: yamlrender.DescriptionMode(opts.Descriptions),
			}
			return renderer.Render(out, doc)
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
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "shortcut for -o tui")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "shortcut for -o browser")

	return cmd
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
	if o.Output != OutputYAML {
		return fmt.Errorf("-o %s is not implemented yet", o.Output)
	}
	if o.AllVersions {
		return fmt.Errorf("--all-versions is not supported with -o yaml")
	}
	if o.ExpandDepth < 0 {
		return fmt.Errorf("--expand-depth must be non-negative")
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
		return nil
	}
	if o.Version != "" {
		return fmt.Errorf("--version requires -f until cluster schema rendering is implemented")
	}
	if len(args) > 0 {
		return fmt.Errorf("cluster resource schema rendering is not implemented yet; omit the resource to show the discovery overview")
	}
	return nil
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
