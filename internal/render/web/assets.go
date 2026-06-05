package web

import (
	_ "embed"
	"strings"
)

//go:embed assets/kubectl-doc.css
var stylesheet string

//go:embed assets/kubectl-doc.js
var runtimeScript string

func StyleElement() string {
	return "<style>\n" + strings.TrimRight(stylesheet, "\n") + "\n</style>"
}

func ScriptElement() string {
	return "<script>\n" + strings.TrimRight(runtimeScript, "\n") + "\n</script>"
}

func Stylesheet() string {
	return stylesheet
}

func RuntimeScript() string {
	return runtimeScript
}
