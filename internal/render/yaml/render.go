package yamlrender

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
)

type Renderer struct {
	ExpandDepth    int
	Color          bool
	Descriptions   DescriptionMode
	Columns        int
	RenderStatus   bool
	RenderMetadata bool
}

type DescriptionMode string

const (
	DescriptionFalse    DescriptionMode = DescriptionMode(tree.DescriptionFalse)
	DescriptionRequired DescriptionMode = DescriptionMode(tree.DescriptionRequired)
	DescriptionTrue     DescriptionMode = DescriptionMode(tree.DescriptionTrue)
)

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	lines := tree.Texts(tree.Build(doc, tree.Options{
		ExpandDepth:    r.ExpandDepth,
		Descriptions:   tree.DescriptionMode(r.descriptionMode()),
		Columns:        r.Columns,
		RenderStatus:   r.RenderStatus,
		RenderMetadata: r.RenderMetadata,
	}))

	if r.Color {
		for i, line := range lines {
			lines[i] = colorLine(line)
		}
	}

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func (r Renderer) descriptionMode() DescriptionMode {
	if r.Descriptions == "" {
		return DescriptionTrue
	}
	return r.Descriptions
}

var (
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	stringStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	scalarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	syntaxStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	noteStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	requiredStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
)

func colorLine(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return indent + noteStyle.Render(trimmed)
	}

	code := line
	comment := ""
	if index := strings.Index(line, " # "); index >= 0 {
		code = line[:index]
		comment = colorComment(line[index:])
	}
	return colorCode(code) + comment
}

func colorComment(comment string) string {
	const requiredLabel = "# required"
	index := strings.Index(comment, requiredLabel)
	if index < 0 {
		return noteStyle.Render(comment)
	}
	return noteStyle.Render(comment[:index+len("# ")]) + requiredStyle.Render("required") + noteStyle.Render(comment[index+len(requiredLabel):])
}

func colorCode(code string) string {
	indent := code[:len(code)-len(strings.TrimLeft(code, " "))]
	trimmed := strings.TrimLeft(code, " ")
	prefix := ""
	if strings.HasPrefix(trimmed, "- ") {
		prefix = "- "
		trimmed = strings.TrimPrefix(trimmed, "- ")
	}

	colon := strings.Index(trimmed, ":")
	if colon < 0 {
		return code
	}

	key := trimmed[:colon]
	rest := trimmed[colon:]
	if strings.TrimSpace(strings.TrimPrefix(rest, ":")) == "" {
		return indent + prefix + keyStyle.Render(key) + colorMappingSeparator(rest)
	}
	return indent + prefix + keyStyle.Render(key) + colorValue(rest)
}

func colorMappingSeparator(rest string) string {
	if rest == "" {
		return rest
	}
	return syntaxStyle.Render(rest[:1]) + rest[1:]
}

func colorValue(rest string) string {
	value := strings.TrimPrefix(rest, ":")
	space := value[:len(value)-len(strings.TrimLeft(value, " "))]
	trimmed := strings.TrimLeft(value, " ")
	if trimmed == "" {
		return colorMappingSeparator(rest)
	}

	style := scalarStyle
	if strings.HasPrefix(trimmed, `"`) {
		style = stringStyle
	}
	if strings.HasPrefix(trimmed, "[") {
		return syntaxStyle.Render(":") + space + colorFlowValue(trimmed)
	}
	return syntaxStyle.Render(":") + space + style.Render(trimmed)
}

func colorFlowValue(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '[', ']', ',':
			out.WriteString(syntaxStyle.Render(value[i : i+1]))
			i++
		case '"':
			start := i
			i++
			for i < len(value) {
				if value[i] == '\\' {
					i += 2
					continue
				}
				if value[i] == '"' {
					i++
					break
				}
				i++
			}
			out.WriteString(stringStyle.Render(value[start:i]))
		case ' ', '\t':
			out.WriteByte(value[i])
			i++
		default:
			start := i
			for i < len(value) && !strings.ContainsRune("[],\" \t", rune(value[i])) {
				i++
			}
			out.WriteString(scalarStyle.Render(value[start:i]))
		}
	}
	return out.String()
}
