package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

type Options struct {
	Filenames   []string
	Output      string
	NoColor     bool
	Version     string
	AllVersions bool
	ExpandDepth int
	Web         bool
}

const (
	OutputYAML = "yaml"
)

func NewCommand(out, errOut io.Writer) *cobra.Command {
	opts := Options{
		Output:      OutputYAML,
		ExpandDepth: 2,
	}

	cmd := &cobra.Command{
		Use:          "kubectl-doc [resource]",
		Short:        "Render Kubernetes API documentation",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(args); err != nil {
				return err
			}

			doc, err := crd.Load(opts.Filenames, opts.Version)
			if err != nil {
				return err
			}

			renderer := yamlrender.Renderer{ExpandDepth: opts.ExpandDepth}
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
	cmd.Flags().BoolVar(&opts.Web, "web", false, "shortcut for -o browser")

	return cmd
}

func (o Options) validate(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("cluster resource selectors are not implemented in the CRD-only MVP")
	}
	if o.Web {
		return fmt.Errorf("--web is not implemented in the CRD-only MVP")
	}
	if o.Output != OutputYAML {
		return fmt.Errorf("-o %s is not implemented in the CRD-only MVP", o.Output)
	}
	if o.AllVersions {
		return fmt.Errorf("--all-versions is not supported with -o yaml")
	}
	if len(o.Filenames) == 0 {
		return fmt.Errorf("-o yaml requires a selected resource; pass a CRD with -f")
	}
	if o.ExpandDepth < 0 {
		return fmt.Errorf("--expand-depth must be non-negative")
	}
	return nil
}
