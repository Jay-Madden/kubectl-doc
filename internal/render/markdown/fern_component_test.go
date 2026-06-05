package markdownrender

import (
	"os"
	"strings"
	"testing"
)

func TestFernComponentIsSharedRuntimeAdapter(t *testing.T) {
	data, err := os.ReadFile("../../../fern/components/kubectl-doc/KubeSchemaDoc.tsx")
	if err != nil {
		t.Fatalf("read Fern component: %v", err)
	}
	component := string(data)

	for _, expected := range []string{
		`import { kubectlDocStyles } from "./kubectl-doc-styles";`,
		`const styleElementID = "kubectl-doc-fern-styles";`,
		`function ensureKubectlDocStyles()`,
		`style.textContent = kubectlDocStyles;`,
		`import("./kubectl-doc-runtime.js")`,
		`runtime.mount(rootRef.current, {`,
		`initialSchema: data`,
		`filtering`,
		`detailsMode: "side-overlay"`,
		`loadFullSchema: loadFullSchema ?? onLoadFull ?? defaultLoadFullSchema(data)`,
		`controller?.destroy();`,
		`export function KubeSchemaDoc`,
	} {
		if !strings.Contains(component, expected) {
			t.Fatalf("expected Fern component to contain %q, got:\n%s", expected, component)
		}
	}

	for _, unwanted := range []string{
		`useState`,
		`visibleLines.map`,
		`data.lines.map`,
		`<SchemaLine`,
		`function SchemaLine`,
		`setExpanded`,
		`setFocusedId`,
	} {
		if strings.Contains(component, unwanted) {
			t.Fatalf("Fern component must not render or own schema state via %q, got:\n%s", unwanted, component)
		}
	}
}

func TestFernRuntimePreservesHTMLBlueprintBehavior(t *testing.T) {
	runtimeData, err := os.ReadFile("../../../fern/components/kubectl-doc/kubectl-doc-runtime.js")
	if err != nil {
		t.Fatalf("read Fern runtime: %v", err)
	}
	runtime := string(runtimeData)
	for _, expected := range []string{
		`function mount(root, options)`,
		`renderSchema(root, options.initialSchema, options);`,
		`root.classList.toggle("kdoc-details-side-overlay", scopedKeyboard);`,
		`var keyTarget = scopedKeyboard ? root : document;`,
		`keyTarget.addEventListener("keydown", handleCursorKey);`,
		`keyTarget.removeEventListener("keydown", handleCursorKey);`,
		`root.addEventListener("focusin", handleFocusIn);`,
		`root.addEventListener("focusout", handleFocusOut);`,
		`if(line.tokens && line.tokens.length){`,
		`function renderPayloadToken(token)`,
		`function tokenClass(kind)`,
		`var currentFilter = filterQuery;`,
		`foldStates.push({path: state.path, expanded: expanded(state.line)});`,
		`if(currentFilter && nextController && nextController.setFilter){ nextController.setFilter(currentFilter); }`,
		`if(currentPath && nextController && nextController.focusPath){ nextController.focusPath(currentPath, {scroll:false}); }`,
	} {
		if !strings.Contains(runtime, expected) {
			t.Fatalf("expected Fern runtime to contain %q, got:\n%s", expected, runtime)
		}
	}
	for _, unwanted := range []string{
		`function renderInlineYAML`,
		`function renderYAMLCode`,
		`function renderScalarToken`,
		`function requiredCommentToken`,
	} {
		if strings.Contains(runtime, unwanted) {
			t.Fatalf("Fern runtime must not duplicate YAML tokenization via %q, got:\n%s", unwanted, runtime)
		}
	}

	cssData, err := os.ReadFile("../../../fern/components/kubectl-doc/kubectl-doc-styles.ts")
	if err != nil {
		t.Fatalf("read generated Fern runtime CSS: %v", err)
	}
	css := string(cssData)
	for _, expected := range []string{
		`.kdoc-fern-host{`,
		`.kdoc-fern-host .kdoc-tree{inline-size:100%;max-inline-size:100%;overflow:hidden}`,
		`.kdoc-fern-host .kdoc-line{display:grid;grid-template-columns:24px minmax(0,1fr);`,
		`.kdoc-fern-host.kdoc-details-side-overlay:not(.kdoc-has-focus) .kdoc-details{display:none}`,
		`.kdoc-fern-host.kdoc-details-side-overlay .kdoc-details{box-shadow:`,
		`z-index:2147483647`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected Fern CSS to contain %q, got:\n%s", expected, css)
		}
	}
}
