package yamltokens

import (
	"html"
	"strings"
)

func RenderHTML(text string, fieldLine bool) string {
	return HTML(Render(text, fieldLine))
}

func HTML(line Line) string {
	if line.Comment != nil {
		return commentHTML(*line.Comment)
	}

	var out strings.Builder
	for _, token := range line.Tokens {
		out.WriteString(tokenHTML(token))
	}
	return out.String()
}

func tokenHTML(token Token) string {
	className := Class(token.Kind)
	if className == "" {
		return escape(token.Text)
	}
	return `<span class="` + className + `">` + escape(token.Text) + `</span>`
}

func commentHTML(comment Comment) string {
	var out strings.Builder
	out.WriteString(`<span class="kdoc-comment" data-kdoc-comment data-kdoc-comment-prefix="`)
	out.WriteString(escapeAttr(comment.Prefix))
	out.WriteString(`" data-kdoc-comment-wrap-prefix="`)
	out.WriteString(escapeAttr(comment.WrapPrefix))
	out.WriteString(`" data-kdoc-comment-text="`)
	out.WriteString(escapeAttr(comment.Text))
	out.WriteString(`"><span class="kdoc-yaml-comment kdoc-comment-prefix">`)
	out.WriteString(escape(comment.Prefix))
	out.WriteString(`</span><span class="kdoc-yaml-comment kdoc-comment-body">`)
	out.WriteString(escape(comment.Text))
	out.WriteString(`</span></span>`)
	return out.String()
}

func escape(value string) string {
	return html.EscapeString(value)
}

func escapeAttr(value string) string {
	return html.EscapeString(value)
}
