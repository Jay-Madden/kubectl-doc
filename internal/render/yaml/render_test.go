package yamlrender

import (
	"strings"
	"testing"
)

func TestColorLineUsesANSIStyles(t *testing.T) {
	colored := colorLine(`spec: "<string>" # comment`)

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "spec") || !strings.Contains(colored, "<string>") || !strings.Contains(colored, "# comment") {
		t.Fatalf("colored line lost content: %q", colored)
	}
}
