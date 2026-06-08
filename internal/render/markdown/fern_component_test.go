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
		`preloadFullSchema = true`,
		`preloadFullSchema,`,
		`autoFocus = true`,
		`autoFocus,`,
		`loadFullSchema: loadFullSchema ?? defaultLoadFullSchema(data)`,
		`restoreSnapshot(controller, previousSnapshot);`,
		`return response.json() as Promise<KubeSchemaDocument>;`,
		`const mountedController = activeController(rootRef.current, controller);`,
		`snapshotRef.current = mountedController?.snapshot?.() ?? null;`,
		`mountedController?.destroy();`,
		`export function KubeSchemaDoc`,
		`classNames("kubectl-doc", "kdoc-embedded-host", "kdoc-react-host", className)`,
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
		`onLoadFull`,
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
		`root.getAttribute("data-kdoc-details-mode")`,
		`root.classList.toggle("kdoc-details-side-overlay", scopedKeyboard);`,
		`root.classList.toggle("kdoc-embedded-host", scopedKeyboard || root.classList.contains("kdoc-embedded-host"));`,
		`if(scopedKeyboard && !hostHasFocus()){ return false; }`,
		`var keyTarget = document;`,
		`root.addEventListener("click", handleRootClick, true);`,
		`root.removeEventListener("click", handleRootClick, true);`,
		`keyTarget.addEventListener("keydown", handleCursorKey);`,
		`keyTarget.removeEventListener("keydown", handleCursorKey);`,
		`root.addEventListener("focusin", handleFocusIn);`,
		`root.addEventListener("focusout", handleFocusOut);`,
		`if(line.tokens && line.tokens.length){`,
		`function renderPayloadToken(token)`,
		`function tokenClass(kind)`,
		`function foldSnapshot()`,
		`foldStates.push({path: state.path, expanded: expanded(state.line)});`,
		`function restoreFoldSnapshot(targetController, foldStates)`,
		`function hasLoadedDescendants(line)`,
		`function wantsFullProjectionForExpansion(line)`,
		`function expandWithFullSchema(line)`,
		`function toggleExpandedWithFullSchema(line)`,
		`function scheduleFullSchemaPreload()`,
		`if(mountedOptions.preloadFullSchema === false || fullSchema || loadingFullSchema || !mountedOptions.loadFullSchema){ return; }`,
		`requestFullSchema();`,
		`scheduleFullSchemaPreload();`,
		`function cancelFullSchemaPreload()`,
		`function scheduleInitialFocus()`,
		`optionEnabled(options, "autoFocus", root, "data-kdoc-auto-focus")`,
		`if(!autoFocus || !scopedKeyboard){ return; }`,
		`scheduleInitialFocus();`,
		`function cancelInitialFocus()`,
		`function buildSchemaIndex(schema)`,
		`function renderSchemaProjection(schema, index, projection, focusPathValue, scroll, focusOptions)`,
		`function observeHostTheme()`,
		`function replaceHash(path)`,
		`function handleHashNavigation()`,
		`window.addEventListener("hashchange", handleHashNavigation)`,
		`function fullFilterProjection()`,
		`recordPerf("full-schema-activate", activateStart`,
		`recordPerf("filter-apply", start`,
		`function releaseStaleConsentBackdrop()`,
		`var backdrop = document.querySelector(".onetrust-pc-dark-filter");`,
		`backdrop.style.pointerEvents = "none";`,
		`var filterScopeSection = null;`,
		`function currentVersionSection()`,
		`if(state.version !== scope){ return; }`,
		`setExpanded(line, true);`,
		`folds: foldSnapshot()`,
		`fullSchemaIndex = buildSchemaIndex(schema);`,
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
		`.kdoc-embedded-host{`,
		`.kdoc-embedded-host .kdoc-tree{inline-size:100%;max-inline-size:100%;overflow:hidden}`,
		`.kdoc-embedded-host .kdoc-line{display:grid;grid-template-columns:24px minmax(0,1fr);`,
		`.kdoc-embedded-host .kdoc-version.kdoc-filtering .kdoc-line.kdoc-filter-visible{display:grid}`,
		`.kdoc-embedded-host.kdoc-wrap-comments .kdoc-comment-prefix,.kdoc-embedded-host.kdoc-wrap-comments .kdoc-comment-body{overflow-wrap:normal;white-space:pre}`,
		`.kubectl-doc.kdoc-details-side-overlay:not(.kdoc-has-focus) .kdoc-details{display:none}`,
		`.kubectl-doc.kdoc-details-side-overlay .kdoc-details{box-shadow:`,
		`z-index:2147483647`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected React CSS to contain %q, got:\n%s", expected, css)
		}
	}
}
