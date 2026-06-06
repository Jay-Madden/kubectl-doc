package markdownrender

import (
	"os"
	"strings"
	"testing"
)

func TestReactComponentIsSharedRuntimeAdapter(t *testing.T) {
	data, err := os.ReadFile("../../../react/kubectl-doc/KubeSchemaDoc.tsx")
	if err != nil {
		t.Fatalf("read React component: %v", err)
	}
	component := string(data)

	for _, expected := range []string{
		`import { kubectlDocStyles } from "./kubectl-doc-styles";`,
		`const defaultStyleElementID = "kubectl-doc-react-styles";`,
		`function ensureKubectlDocStyles(styleElementID: string)`,
		`style.textContent = kubectlDocStyles;`,
		`import("./kubectl-doc-runtime.js")`,
		`runtimeLoader?: () => Promise<KubectlDocRuntime> | KubectlDocRuntime;`,
		`runtime.mount(rootRef.current, {`,
		`rootRef.current.innerHTML = "";`,
		`initialSchema: data`,
		`filtering`,
		`detailsMode,`,
		`wrapControl,`,
		`wrapComments,`,
		`loadFullSchema: loadFullSchema ?? onLoadFull ?? defaultLoadFullSchema(data)`,
		`restoreSnapshot(controller, previousSnapshot);`,
		`return response.json() as Promise<KubeSchemaDocument>;`,
		`const mountedController = activeController(rootRef.current, controller);`,
		`snapshotRef.current = mountedController?.snapshot?.() ?? null;`,
		`mountedController?.destroy();`,
		`export function KubeSchemaDoc`,
		`classNames("kubectl-doc", "kdoc-react-host", className)`,
	} {
		if !strings.Contains(component, expected) {
			t.Fatalf("expected React component to contain %q, got:\n%s", expected, component)
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
		`atob(`,
		`TextDecoder`,
		`match(/`,
	} {
		if strings.Contains(component, unwanted) {
			t.Fatalf("React component must not render or own schema state via %q, got:\n%s", unwanted, component)
		}
	}
}

func TestSharedRuntimePreservesHTMLBlueprintBehavior(t *testing.T) {
	runtimeData, err := os.ReadFile("../../../internal/render/web/assets/kubectl-doc.js")
	if err != nil {
		t.Fatalf("read shared runtime: %v", err)
	}
	runtime := string(runtimeData)
	for _, expected := range []string{
		`function mount(root, options)`,
		`renderSchema(root, options.initialSchema, options);`,
		`root.classList.toggle("kdoc-details-side-overlay", scopedKeyboard);`,
		`var keyTarget = scopedKeyboard ? root : document;`,
		`root.addEventListener("click", handleRootClick, true);`,
		`root.removeEventListener("click", handleRootClick, true);`,
		`keyTarget.addEventListener("keydown", handleCursorKey);`,
		`keyTarget.removeEventListener("keydown", handleCursorKey);`,
		`root.addEventListener("focusin", handleFocusIn);`,
		`root.addEventListener("focusout", handleFocusOut);`,
		`if(line.tokens && line.tokens.length){`,
		`function renderPayloadToken(token)`,
		`function tokenClass(kind)`,
		`var currentFilter = filterQuery;`,
		`function foldSnapshot()`,
		`foldStates.push({path: state.path, expanded: expanded(state.line)});`,
		`function restoreFoldSnapshot(targetController, foldStates)`,
		`function hasLoadedDescendants(line)`,
		`function wantsFullSchemaForExpansion(line)`,
		`function expandWithFullSchema(line)`,
		`function toggleExpandedWithFullSchema(line)`,
		`function releaseStaleConsentBackdrop()`,
		`var backdrop = document.querySelector(".onetrust-pc-dark-filter");`,
		`backdrop.style.pointerEvents = "none";`,
		`var filterScopeSection = null;`,
		`function currentVersionSection()`,
		`if(state.version !== scope){ return; }`,
		`setExpanded(line, true);`,
		`folds: foldSnapshot()`,
		`if(currentFilter && nextController && nextController.setFilter){ nextController.setFilter(currentFilter); }`,
		`if(currentPath && nextController && nextController.focusPath){ nextController.focusPath(currentPath, {scroll:false}); }`,
	} {
		if !strings.Contains(runtime, expected) {
			t.Fatalf("expected shared runtime to contain %q, got:\n%s", expected, runtime)
		}
	}
	for _, unwanted := range []string{
		`function renderInlineYAML`,
		`function renderYAMLCode`,
		`function renderScalarToken`,
		`function requiredCommentToken`,
	} {
		if strings.Contains(runtime, unwanted) {
			t.Fatalf("shared runtime must not duplicate YAML tokenization via %q, got:\n%s", unwanted, runtime)
		}
	}

	cssData, err := os.ReadFile("../../../react/kubectl-doc/kubectl-doc-styles.ts")
	if err != nil {
		t.Fatalf("read generated React runtime CSS: %v", err)
	}
	css := string(cssData)
	for _, expected := range []string{
		`.kdoc-react-host{`,
		`.kdoc-react-host .kdoc-tree{inline-size:100%;max-inline-size:100%;overflow:hidden}`,
		`.kdoc-react-host .kdoc-line{display:grid;grid-template-columns:24px minmax(0,1fr);`,
		`.kdoc-react-host .kdoc-version.kdoc-filtering .kdoc-line.kdoc-filter-visible{display:grid}`,
		`.kdoc-react-host.kdoc-wrap-comments .kdoc-comment-prefix,.kdoc-react-host.kdoc-wrap-comments .kdoc-comment-body{overflow-wrap:normal;white-space:pre}`,
		`.kdoc-react-host.kdoc-details-side-overlay:not(.kdoc-has-focus) .kdoc-details{display:none}`,
		`.kdoc-react-host.kdoc-details-side-overlay .kdoc-details{box-shadow:`,
		`z-index:2147483647`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected React CSS to contain %q, got:\n%s", expected, css)
		}
	}
}
