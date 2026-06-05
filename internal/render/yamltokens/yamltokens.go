package yamltokens

import (
	"strconv"
	"strings"
)

type Token struct {
	Kind string
	Text string
}

type Comment struct {
	Prefix     string
	WrapPrefix string
	Text       string
}

type Line struct {
	Tokens  []Token
	Comment *Comment
}

const (
	KindBool        = "bool"
	KindComment     = "comment"
	KindKey         = "key"
	KindNull        = "null"
	KindNumber      = "number"
	KindPlaceholder = "placeholder"
	KindPunct       = "punct"
	KindRequired    = "required"
	KindScalar      = "scalar"
	KindString      = "string"
	KindTypeNumber  = "type-number"
)

func Render(text string, fieldLine bool) Line {
	indentLength := len(text) - len(strings.TrimLeft(text, " "))
	indent := text[:indentLength]
	rest := text[indentLength:]
	if rest == "" {
		return Line{Tokens: []Token{{Text: indent}}}
	}
	if prefix, wrapPrefix, ok := standaloneCommentPrefixes(rest, fieldLine); ok {
		return Line{Comment: &Comment{
			Prefix:     indent + prefix,
			WrapPrefix: indent + wrapPrefix,
			Text:       strings.TrimPrefix(rest, prefix),
		}}
	}
	if strings.HasPrefix(rest, "# ") {
		content := strings.TrimPrefix(rest, "# ")
		if fieldLine {
			return Line{Tokens: append([]Token{{Text: indent}, {Kind: KindComment, Text: "# "}}, renderYAMLCode(content)...)}
		}
		return Line{Tokens: []Token{{Text: indent}, {Kind: KindComment, Text: rest}}}
	}
	return Line{Tokens: append([]Token{{Text: indent}}, renderYAMLCode(rest)...)}
}

func Class(kind string) string {
	switch kind {
	case KindBool:
		return "kdoc-yaml-bool"
	case KindComment:
		return "kdoc-yaml-comment"
	case KindKey:
		return "kdoc-yaml-key"
	case KindNull:
		return "kdoc-yaml-null"
	case KindNumber:
		return "kdoc-yaml-number"
	case KindPlaceholder:
		return "kdoc-yaml-placeholder"
	case KindPunct:
		return "kdoc-yaml-punct"
	case KindRequired:
		return "kdoc-required-label"
	case KindScalar:
		return "kdoc-yaml-scalar"
	case KindString:
		return "kdoc-yaml-string"
	case KindTypeNumber:
		return "kdoc-yaml-type-number"
	default:
		return ""
	}
}

func standaloneCommentPrefixes(rest string, fieldLine bool) (string, string, bool) {
	if fieldLine {
		return "", "", false
	}
	switch {
	case strings.HasPrefix(rest, "# - # "):
		return "# - # ", "#   # ", true
	case strings.HasPrefix(rest, "- # "):
		return "- # ", "  # ", true
	case strings.HasPrefix(rest, "# "):
		return "# ", "# ", true
	default:
		return "", "", false
	}
}

func renderYAMLCode(code string) []Token {
	inlineComment := ""
	if index := strings.Index(code, " # "); index >= 0 {
		inlineComment = code[index:]
		code = code[:index]
	}

	var out []Token
	if strings.HasPrefix(code, "- ") {
		out = append(out, Token{Kind: KindPunct, Text: "-"}, Token{Text: " "})
		code = strings.TrimPrefix(code, "- ")
	} else if code == "-" {
		out = append(out, Token{Kind: KindPunct, Text: "-"})
		code = ""
	}

	if colon := strings.Index(code, ":"); colon > 0 {
		key := code[:colon]
		value := code[colon+1:]
		out = append(out, Token{Kind: KindKey, Text: key}, Token{Kind: KindPunct, Text: ":"})
		out = append(out, renderYAMLValue(value)...)
	} else {
		out = append(out, renderYAMLValue(code)...)
	}
	if inlineComment != "" {
		out = append(out, renderYAMLComment(inlineComment)...)
	}
	return out
}

func renderYAMLComment(comment string) []Token {
	index, end := requiredCommentToken(comment)
	if index < 0 {
		return []Token{{Kind: KindComment, Text: comment}}
	}
	var out []Token
	if prefix := comment[:index]; prefix != "" {
		out = append(out, Token{Kind: KindComment, Text: prefix})
	}
	out = append(out, Token{Kind: KindRequired, Text: "required"})
	if suffix := comment[end:]; suffix != "" {
		out = append(out, Token{Kind: KindComment, Text: suffix})
	}
	return out
}

func requiredCommentToken(comment string) (int, int) {
	const token = "required"
	lower := strings.ToLower(comment)
	for start := 0; start < len(lower); {
		index := strings.Index(lower[start:], token)
		if index < 0 {
			return -1, -1
		}
		index += start
		end := index + len(token)
		if commentTokenBoundary(comment, index-1) && commentTokenBoundary(comment, end) {
			return index, end
		}
		start = end
	}
	return -1, -1
}

func commentTokenBoundary(comment string, index int) bool {
	if index < 0 || index >= len(comment) {
		return true
	}
	switch comment[index] {
	case ' ', '\t', ',', ';', '#':
		return true
	default:
		return false
	}
}

func renderYAMLValue(value string) []Token {
	leadingLength := len(value) - len(strings.TrimLeft(value, " "))
	if leadingLength == len(value) {
		return []Token{{Text: value}}
	}
	out := []Token{{Text: value[:leadingLength]}}
	return append(out, renderYAMLScalar(value[leadingLength:])...)
}

func renderYAMLScalar(value string) []Token {
	var out []Token
	for i := 0; i < len(value); {
		switch value[i] {
		case '[', ']', '{', '}', ',', ':':
			out = append(out, Token{Kind: KindPunct, Text: value[i : i+1]})
			i++
		case ' ', '\t':
			out = append(out, Token{Text: value[i : i+1]})
			i++
		case '"', '\'':
			end := quotedEnd(value, i)
			out = append(out, Token{Kind: KindString, Text: value[i:end]})
			i = end
		default:
			end := tokenEnd(value, i)
			out = append(out, renderScalarToken(value[i:end]))
			i = end
		}
	}
	return out
}

func quotedEnd(value string, start int) int {
	quote := value[start]
	for i := start + 1; i < len(value); i++ {
		if value[i] == '\\' && quote == '"' {
			i++
			continue
		}
		if value[i] == quote {
			return i + 1
		}
	}
	return len(value)
}

func tokenEnd(value string, start int) int {
	for i := start; i < len(value); i++ {
		switch value[i] {
		case '[', ']', '{', '}', ',', ':', ' ', '\t':
			return i
		}
	}
	return len(value)
}

func renderScalarToken(token string) Token {
	switch {
	case strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">"):
		return renderPlaceholderToken(token)
	case token == "true" || token == "false":
		return Token{Kind: KindBool, Text: token}
	case token == "null":
		return Token{Kind: KindNull, Text: token}
	case isNumber(token):
		return Token{Kind: KindNumber, Text: token}
	default:
		return Token{Kind: KindScalar, Text: token}
	}
}

func renderPlaceholderToken(token string) Token {
	switch inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">"); {
	case inner == "string" || inner == "name":
		return Token{Kind: KindString, Text: token}
	case inner == "boolean":
		return Token{Kind: KindBool, Text: token}
	case isNumberPlaceholder(inner):
		return Token{Kind: KindTypeNumber, Text: token}
	default:
		return Token{Kind: KindPlaceholder, Text: token}
	}
}

func isNumberPlaceholder(inner string) bool {
	switch inner {
	case "integer", "number", "int", "int32", "int64", "float", "float32", "float64", "double":
		return true
	default:
		return false
	}
}

func isNumber(token string) bool {
	if token == "" {
		return false
	}
	_, err := strconv.ParseFloat(token, 64)
	return err == nil
}
