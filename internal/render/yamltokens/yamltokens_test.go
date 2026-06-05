package yamltokens

import (
	"strings"
	"testing"
)

func TestRenderHTMLDoesNotRetokenizeGeneratedMarkup(t *testing.T) {
	rendered := RenderHTML(`apiVersion: nvidia.com/v1beta1`, false)
	for _, expected := range []string{
		`<span class="kdoc-yaml-key">apiVersion</span>`,
		`<span class="kdoc-yaml-punct">:</span>`,
		`<span class="kdoc-yaml-scalar">nvidia.com/v1beta1</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected %q in %s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `<span class=<span`) {
		t.Fatalf("token markup was corrupted: %s", rendered)
	}
}

func TestRenderRequiredCommentToken(t *testing.T) {
	rendered := RenderHTML(`name: "<string>" # required, minLength: 1`, true)
	for _, expected := range []string{
		`<span class="kdoc-yaml-comment"> # </span>`,
		`<span class="kdoc-required-label">required</span>`,
		`<span class="kdoc-yaml-comment">, minLength: 1</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected %q in %s", expected, rendered)
		}
	}
}

func TestRenderStandaloneComment(t *testing.T) {
	rendered := RenderHTML(`  # description`, false)
	for _, expected := range []string{
		`data-kdoc-comment-prefix="  # "`,
		`data-kdoc-comment-wrap-prefix="  # "`,
		`data-kdoc-comment-text="description"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected %q in %s", expected, rendered)
		}
	}

	rendered = RenderHTML(`#`, false)
	for _, expected := range []string{
		`data-kdoc-comment-prefix="#"`,
		`data-kdoc-comment-wrap-prefix="# "`,
		`data-kdoc-comment-text=""`,
		`<span class="kdoc-yaml-comment kdoc-comment-prefix">#</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected %q in %s", expected, rendered)
		}
	}
}
