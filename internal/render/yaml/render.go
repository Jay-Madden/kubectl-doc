package yamlrender

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/termstyle"
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
	lines := tree.Build(doc, tree.Options{
		ExpandDepth:    r.ExpandDepth,
		Descriptions:   tree.DescriptionMode(r.descriptionMode()),
		Columns:        r.Columns,
		RenderStatus:   r.RenderStatus,
		RenderMetadata: r.RenderMetadata,
	})

	texts := tree.Texts(lines)
	if r.Color {
		for i, line := range lines {
			texts[i] = ColorTreeLine(line)
		}
	}

	_, err := fmt.Fprintln(out, strings.Join(texts, "\n"))
	return err
}

func (r Renderer) descriptionMode() DescriptionMode {
	if r.Descriptions == "" {
		return DescriptionTrue
	}
	return r.Descriptions
}

var (
	keyStyle      = termstyle.KeyStyle
	stringStyle   = termstyle.StringStyle
	scalarStyle   = termstyle.ScalarStyle
	syntaxStyle   = termstyle.SyntaxStyle
	noteStyle     = termstyle.NoteStyle
	requiredStyle = termstyle.RequiredStyle
	urlStyle      = termstyle.URLStyle
)

var (
	requiredPattern = regexp.MustCompile(`\brequired\b`)
	urlPattern      = regexp.MustCompile(`https?://\S+`)
)

// ColorLine applies the terminal YAML syntax palette to one rendered schema line.
func ColorLine(line string) string {
	return colorLine(line, true)
}

// ColorTreeLine applies the terminal YAML syntax palette using tree metadata.
func ColorTreeLine(line tree.Line) string {
	return ColorLineWithMetadata(line.Text, line.Code)
}

// ColorLineWithMetadata applies the terminal YAML syntax palette with explicit field metadata.
func ColorLineWithMetadata(line string, code bool) string {
	return colorLine(line, !code)
}

func colorLine(line string, inferCommentedCode bool) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return line
	}
	if colored, ok := colorCommentedCode(indent, trimmed, inferCommentedCode); ok {
		return colored
	}
	if strings.HasPrefix(trimmed, "#") {
		return indent + colorNoteText(trimmed)
	}

	code := line
	comment := ""
	if index := strings.Index(line, " # "); index >= 0 {
		code = line[:index]
		comment = colorComment(line[index:])
	}
	return colorCode(code) + comment
}

func colorCommentedCode(indent, trimmed string, infer bool) (string, bool) {
	for _, prefix := range []string{"# - # ", "# - ", "# ", "- # "} {
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		code := strings.TrimPrefix(trimmed, prefix)
		if infer && !looksLikeFieldCode(code) {
			return "", false
		}
		body := code
		comment := ""
		if index := strings.Index(code, " # "); index >= 0 {
			body = code[:index]
			comment = colorComment(code[index:])
		}
		return indent + colorCommentPrefix(prefix) + colorCode(body) + comment, true
	}
	return "", false
}

func colorCommentPrefix(prefix string) string {
	var out strings.Builder
	for _, token := range strings.SplitAfter(prefix, " ") {
		switch strings.TrimSpace(token) {
		case "#":
			out.WriteString(noteStyle.Render("#"))
			out.WriteString(strings.TrimPrefix(token, "#"))
		case "-":
			out.WriteString(syntaxStyle.Render("-"))
			out.WriteString(strings.TrimPrefix(token, "-"))
		default:
			out.WriteString(token)
		}
	}
	return out.String()
}

func looksLikeFieldCode(code string) bool {
	if strings.HasPrefix(code, "http://") || strings.HasPrefix(code, "https://") {
		return false
	}
	colon := strings.Index(code, ":")
	if colon <= 0 {
		return false
	}
	key := strings.TrimSpace(code[:colon])
	if key == "" || strings.ContainsAny(key, " \t{}[]") {
		return false
	}
	value := strings.TrimSpace(code[colon+1:])
	if index := strings.Index(value, " # "); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	if value == "" || strings.HasPrefix(value, "#") {
		return true
	}
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "<") || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return true
	}
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	return false
}

func colorComment(comment string) string {
	matches := requiredPattern.FindAllStringIndex(comment, -1)
	if len(matches) == 0 {
		return colorNoteText(comment)
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		out.WriteString(colorNoteText(comment[last:match[0]]))
		out.WriteString(requiredStyle.Render(comment[match[0]:match[1]]))
		last = match[1]
	}
	out.WriteString(colorNoteText(comment[last:]))
	return out.String()
}

func colorNoteText(text string) string {
	var out strings.Builder
	last := 0
	for _, match := range urlPattern.FindAllStringIndex(text, -1) {
		out.WriteString(noteStyle.Render(text[last:match[0]]))
		out.WriteString(urlStyle.Render(text[match[0]:match[1]]))
		last = match[1]
	}
	out.WriteString(noteStyle.Render(text[last:]))
	return out.String()
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
