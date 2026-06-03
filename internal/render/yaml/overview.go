package yamlrender

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/kube"
)

type OverviewRenderer struct {
	Color bool
}

func (r OverviewRenderer) Render(out io.Writer, overview *kube.Overview) error {
	lines := make([]string, 0, len(overview.Groups))
	for _, group := range overview.Groups {
		lines = append(lines, fmt.Sprintf("%s:", group.Name))
		for _, resource := range group.Resources {
			lines = append(lines, fmt.Sprintf("  %s: %s", resource.Name, renderOverviewVersions(resource.Versions)))
		}
	}
	if r.Color {
		for i, line := range lines {
			lines[i] = ColorLine(line)
		}
	}

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func renderOverviewVersions(versions []string) string {
	if len(versions) == 0 {
		return "[]"
	}
	if len(versions) == 1 {
		return versions[0]
	}

	quoted := make([]string, 0, len(versions))
	for _, version := range versions {
		quoted = append(quoted, strconv.Quote(version))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
