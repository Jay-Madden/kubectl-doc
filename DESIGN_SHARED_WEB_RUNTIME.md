# Shared Web Runtime Design

## Purpose

`kubectl-doc` has two browser-facing schema renderers:

- `-o html`, a standalone static HTML document with embedded CSS and vanilla
  JavaScript.
- `-o markdown-fern`, an MDX page that mounts a reusable React component.

Both render the same conceptual UI: a foldable YAML schema tree, field focus,
field details, semantic comment wrapping, keyboard navigation, and interactive
filtering. They must share the same semantic model and the same browser
runtime. Any divergence between standalone HTML, browser mode, and Fern is a
bug unless it is explicitly a host-layout concern such as details placement or
z-index.

This document defines a shared browser runtime so both outputs use one
implementation for the expensive parts: loading, indexing, filtering, folding,
focus movement, highlighting, details, comment wrapping, and DOM updates.

The standalone HTML renderer is the blueprint renderer. It already has the
better interaction model and the more carefully optimized DOM behavior. The
shared runtime must preserve the HTML renderer's DOM contract, visual behavior,
keyboard behavior, copy-valid YAML behavior, and performance characteristics.
Fern is a host integration for that runtime, not a competing renderer.

The target architecture is:

```text
                         Go schema model
                               |
                +--------------+--------------+
                |                             |
        tree.Line + fielddetail        web payload v2
                |                             |
                +--------------+--------------+
                               |
                      shared web runtime
                   kubectl-doc-runtime.js
                   kubectl-doc-runtime.css
                               |
                +--------------+--------------+
                |                             |
        standalone -o html           React host adapters
        inline assets                lifecycle wrappers only
```

The React component must be an adapter around the shared runtime, not an
independent renderer. Fern is one host environment for that generic component.
The public React v1 API exposes one lazy full-payload hook,
`loadFullSchema`. Do not add compatibility aliases or Fern-only loading paths;
nothing should need to know whether the host is Fern except for page layout and
static asset URL decisions.

## Factoring Doctrine

The web implementation has one source of truth for each concern.

- Schema semantics live in Go model packages such as `tree`, `fielddetail`, and
  `webschema`.
- Browser interaction semantics live in the shared web runtime asset.
- Styling primitives live in the shared web CSS asset.
- The React adapter owns only React lifecycle, static sidecar loading, and host
  page-layout adaptation.
- Standalone HTML owns only document assembly, asset embedding, and
  self-contained output.

Never fix behavior separately in "the Fern version" and "the HTML version".
Web is web. If a bug affects folding, filtering, focus, details, syntax
highlighting, comments, line grouping, copy-valid YAML, or lazy hydration, fix
the shared model/runtime path and make all web hosts consume it.

Renderer-specific code may decide where a shared component is mounted, what
payload URL it receives, and how its container integrates with host page chrome.
It must not decide what a line means, which lines belong to one logical field,
how filtering matches, how keyboard navigation moves, or how field details are
derived.

Generated copies are allowed only as packaging artifacts. For example, a React
package may need a component-local `kubectl-doc-runtime.js` file so a bundler
can import it. That file must be generated from the shared source, ignored or
otherwise clearly generated, not edited independently, and checked by CI. The
desired end state is a single checked-in runtime source plus
generated/packageable outputs.

Two editable runtime implementations are wrong even if they happen to be
byte-identical today. The authoritative source must be unique; any second file
is either generated mechanically from that source or removed by a packaging
refactor.

## Verification Contract

Web parity is a CI requirement, not a best-effort local check.

The required gates are:

- `make check-generated` verifies generated docs and packaging artifacts,
  including the React-facing runtime copy generated from
  `internal/render/web/assets/kubectl-doc.js`.
- `make test` verifies Go renderers, the localhost browser server, resource
  resolution, and static HTML assembly.
- `make check-fern-dev` generates Fern/React fixtures, builds the local React
  preview, and runs the Playwright embedding parity suite.
- GitHub CI runs `make check-fern-dev` with
  `PLAYWRIGHT_INSTALL_FLAGS="--with-deps chromium"` so Linux browser
  dependencies are installed while local macOS runs keep the default
  `chromium` install.

The Playwright suite must cover both kinds of shared-runtime hosts:

- Payload-mounted hosts, such as React/Fern, where the runtime receives
  `initialSchema` and optional `loadFullSchema`.
- DOM-mounted hosts, such as standalone `-o html` and browser schema pages,
  where the runtime indexes existing server-rendered DOM and must not assume a
  payload object exists.

`hack/ferndev` also generates browser and downstream-docs fixtures under
`fern/dev/public/fixtures/` before the Vite build:

- `browser-overview.html` uses the real localhost overview renderer and covers
  resource/version navigation, group jumps, plain typing filters, shortname
  matches, and preserving browser search for `/`.
- `browser-schema.html` uses the real selected-schema HTML renderer with
  overview back navigation and covers DOM-mounted filtering/folding without
  clearing the static YAML tree, plus schema keyboard navigation and details
  updates.
- `multiversion-schema.html` uses the real static HTML renderer with all served
  versions and covers that DOM-mounted filtering is scoped to the focused
  version section.
- `mkdocs-embedded-schema.html` wraps the same static schema page in a
  MkDocs-style shell and covers layout pressure from sticky headers, sidebars,
  semantic wrapping, overlay details, plain typing filters, and schema keyboard
  navigation.

Any bug in filtering, folding, focus, details, wrapping, syntax highlighting,
selection grouping, lazy full-payload activation, or generated runtime
packaging should get a regression in this shared suite unless it is provably
Go-only.

## Goals

- Use one optimized DOM runtime for `-o html`, `-w` schema pages, and
  `markdown-fern`.
- Keep one behavioral implementation for all web outputs. No web renderer may
  fork filtering, folding, keyboard navigation, line grouping, details, or
  syntax highlighting logic.
- Keep initial page render fast by embedding only the shallow visible schema
  when large resources are exported.
- Load a 2 MB full schema payload into the browser without a noticeable stall.
- Keep filtering responsive for large schemas by avoiding React reconciliation
  and avoiding full DOM rebuilds per keystroke.
- Share keyboard behavior, fold semantics, filter semantics, selection,
  comment wrapping, syntax highlighting, and details rendering.
- Keep selected/copied schema text valid YAML by preserving gutter separation.
- Keep generated data driven by structured schema metadata. Do not parse details
  or important field metadata back out of rendered YAML.
- Keep `-o html` self-contained and usable without Fern or React.
- Keep `markdown-fern` compatible with a generic React component and static
  generated payload sidecars.

## Non-Goals

- Do not make standalone HTML depend on React.
- Do not make Fern load arbitrary OpenAPI from the browser.
- Do not make the runtime an editor or manifest creator.
- Do not require a localhost server for `markdown-fern`.
- Do not optimize by dropping schema fields, descriptions, validations, or
  collapsed descendants.

## Performance Contract

The runtime has two separate performance budgets.

### Initial Interaction Budget

The page must become usable from the shallow payload without waiting for the
full schema payload.

Targets:

- Initial visible schema tree interactive in under 200 ms after the host
  component or static HTML root is mounted. Under 100 ms remains the preferred
  local target.
- Initial DOM nodes proportional to the visible shallow tree, not to the full
  schema.
- Full payload fetch must not block initial focus, fold, details, or copy.

### Full Payload Load Budget

When a full schema payload is 2 MB uncompressed, activating it in the browser
must not take longer than 200 ms of main-thread blocking work. Under 100 ms
remains the preferred local target.

Network transfer time is environment dependent and is not counted in this
budget. The measured browser budget is from "payload bytes available" to
"runtime can answer line/detail/filter queries" on a normal developer laptop.

Targets:

- Main-thread blocking during full payload activation: under 200 ms.
- Main-thread blocking per filter keystroke after full index is available:
  under 16 ms for typical visible trees, under 50 ms worst-case for very large
  schemas.
- Avoid creating DOM nodes for collapsed descendants when the full payload is
  loaded, indexed, or cached.
- Avoid creating DOM nodes for filtered-out descendants.
- Avoid reparsing YAML text during filtering.
- Avoid lowercasing all descriptions on every filter keystroke.
- Avoid React rendering one component per schema line in React host paths.

The 200 ms budgets are acceptance criteria. If a browser benchmark shows that a
single large JSON parse plus index build can exceed the budget, the runtime must
move parse and index construction into a Web Worker and only transfer compact
query results to the main thread.

## Current State

### Shared Runtime Source

The shared web runtime:

- Builds `tree.Line` records from the schema.
- Renders one DOM line per visible line.
- Stores field metadata in data attributes and details HTML.
- Indexes visible DOM lines during startup and after each rendered projection.
- Builds a structured full-payload index without rendering collapsed
  descendants.
- Handles fold/filter/focus by toggling existing DOM state when possible and by
  rendering a new visible projection when the full payload reveals hidden
  descendants.
- Uses class additions/removals and `hidden` instead of rebuilding subtrees.
- Reflows comments only for visible comment nodes.
- Records browser timings in `window.__kubectlDocPerf` and emits
  `kubectl-doc:perf` events for `mount`, `full-schema-activate`,
  `full-schema-load`, and `projection-render`.

This is the performance baseline and the behavioral source of truth.

Limitations:

- The runtime is currently packaged in more than one location for host
  integration. The copies must remain generated and byte-identical; hand edits
  outside the shared source are not allowed.
- Details are duplicated in DOM attributes, increasing HTML size.
- Syntax highlighting is partly text parsing at render time.
- The runtime is coupled to the exact standalone HTML page structure.

### React Host

The React host:

- Generates a structured payload with `lines[]` and `fields[]`.
- Optionally writes full schema sidecars and embeds a shallow payload.
- Mounts a React component.
- Delegates fold/filter/focus/details/rendering to the shared runtime.
- Does not render schema lines through JSX.

Limitations:

- Runtime packaging still has generated React-facing files. Drift must be
  prevented by generation and tests until packaging can consume the shared asset
  directly.
- Host-specific CSS must stay limited to containment, z-index, host layout, and
  theme integration.

## Design Overview

The shared runtime is a small vanilla JavaScript library with a stable host API.
It owns the schema tree UI inside a supplied root element.

```js
const controller = KubectlDoc.mount(root, {
  initialSchema,
  filtering: true,
  loadFullSchema,
  detailsMode: "side-overlay",
  backURL,
  quitURL,
  theme: "default",
});

controller.destroy();
```

Hosts:

- Standalone HTML embeds the runtime assets and calls `KubectlDoc.mount(...)`
  once for the document.
- Web server mode can use the same runtime for selected schema pages and
  overview pages where applicable.
- React creates an empty host div, calls `mount` in an effect, and calls
  `destroy` on unmount. Fern uses that generic React adapter.

React owns only lifecycle and data delivery. The shared runtime owns DOM, focus,
filtering, fold state, details, and keyboard behavior.

## Runtime Modules

The runtime should be internally split into small modules even if it is shipped
as one browser asset.

```text
runtime/
  index.js              public mount API
  schema-store.js       payload decode, line/field access, indexes
  dom-tree.js           row materialization and patching
  fold-state.js         expanded/collapsed state
  filter-state.js       filter matching, ancestor closure, highlighting ranges
  focus-state.js        keyboard movement, parent/child/foldable navigation
  details.js            details rendering from field metadata
  yaml-spans.js         structured YAML segment rendering
  comments.js           semantic wrapping of visible comments
  worker-client.js      optional worker protocol
  worker.js             full payload parse/index worker
```

The Go repository can initially store these as embedded files under
`internal/render/web/assets`. If we later need bundling, keep the public API
stable and generate one browser artifact from these modules.

## Host API

### `KubectlDoc.mount`

```ts
type MountOptions = {
  initialSchema: SchemaPayloadV2;
  filtering?: boolean;
  loadFullSchema?: () => Promise<SchemaPayloadV2 | string | ArrayBuffer>;
  detailsMode?: "inline-side" | "side-overlay" | "none";
  backURL?: string;
  quitURL?: string;
  initialFocusPath?: string;
  initialWrapComments?: boolean;
  keyboard?: boolean;
  classes?: {
    root?: string;
    tree?: string;
    details?: string;
  };
};

type Controller = {
  destroy(): void;
  focusPath(path: string): void;
  setFilter(query: string): void;
  clearFilter(): void;
  expandPath(path: string): void;
  collapsePath(path: string): void;
  loadFull(): Promise<void>;
  snapshot(): RuntimeSnapshot;
};
```

The host must not need access to internal line state for normal use.

### Full Payload Loading

`loadFullSchema` returns either:

- A parsed payload object.
- A JSON string.
- An `ArrayBuffer` with JSON or a future compact binary format.

The runtime decides whether to parse on the main thread or in a worker. The
host should not care.

For React hosts such as Fern, `loadFullSchema` fetches the generated sidecar
URL. For standalone HTML, the full payload can be embedded, omitted, or loaded
from a sibling file depending on output mode.

## Payload v2

The current MDX payload is close to the needed shape, but it should be renamed
and tightened as a shared web payload.

### Document Shape

```json
{
  "schemaVersion": 2,
  "apiVersion": "apps/v1",
  "group": "apps",
  "version": "v1",
  "kind": "Deployment",
  "resource": "deployments",
  "complete": false,
  "fullPayloadURL": "./deployment-schema-full.json",
  "lines": [],
  "fields": []
}
```

### Line Shape

```json
{
  "i": 42,
  "d": 3,
  "id": 17,
  "p": 12,
  "f": "replicas",
  "flags": 11,
  "text": "replicas: 1 # default, minimum: 0",
  "segments": []
}
```

Where:

- `i`: stable full line index.
- `d`: visual depth.
- `id`: detail id or field id.
- `p`: path id, preferably an interned string id.
- `f`: field name.
- `flags`: bitset for `field`, `code`, `metadata`, `required`, `foldable`,
  `collapsed`, `comment`, `blank`.
- `text`: fallback rendered YAML line.
- `segments`: optional structured tokens for syntax rendering.

The initial implementation can keep readable object keys for maintainability.
The performance version should move to compact keys or array records if browser
benchmarks require it.

### Field Shape

```json
{
  "id": 17,
  "path": "spec.replicas",
  "name": "replicas",
  "type": "integer",
  "required": false,
  "description": "Number of desired pods.",
  "metadata": ["default", "minimum: 0"],
  "filter": "replicas\nnumber of desired pods"
}
```

`filter` is pre-normalized by the generator where possible. This avoids
lowercasing all descriptions during every browser load.

### String Interning

For large payloads, repeated strings should be interned:

```json
{
  "strings": ["spec.replicas", "replicas", "integer"],
  "fields": [[0, 1, 2, 0, 3, [4, 5]]],
  "lines": [[42, 3, 17, 0, 1, 11, 6, 7]]
}
```

This is a later optimization. Do not introduce it until benchmark data shows
the readable object format cannot meet the 200 ms hard activation budget.

### Structured YAML Segments

The generator should eventually emit syntax segments instead of requiring each
runtime to parse YAML-looking text.

```json
[
  {"k": "indent", "t": "  "},
  {"k": "key", "t": "replicas"},
  {"k": "punct", "t": ":"},
  {"k": "space", "t": " "},
  {"k": "number", "t": "1"},
  {"k": "space", "t": " "},
  {"k": "comment", "t": "# default, minimum: 0"},
  {"k": "required", "t": "required"}
]
```

Benefits:

- Identical highlighting in HTML and Fern.
- No regex drift.
- Faster row materialization.
- Correct coloring for required/default/validation labels.
- Easier filter highlight placement.

## Loading Strategy

### Shallow Payload

Every generated page gets a shallow payload:

- Includes all root lines and visible lines up to the initial expansion depth.
- Includes field details only for referenced visible fields.
- Includes collapsed placeholders for hidden descendants.
- Includes enough path/depth metadata to expand known visible branches.
- Includes `fullPayloadURL` when complete data is in a sidecar.

The shallow payload should stay small enough that a React/Fern page with
several resources can render quickly. Target: under 100 KB per rendered
resource, preferably under 50 KB.

### Full Payload Sidecar

Full payload sidecars are raw JSON assets.

Reasons to prefer raw JSON:

- No markdown wrapper parsing.
- `response.json()` and worker parsing can use browser-native paths.
- CDN content type and compression are straightforward.
- Sidecar routes can be tested directly.

### Load Triggers

Full payload loading starts:

- On idle when the schema host is visible or near the viewport.
- Immediately when the user expands a collapsed node whose descendants are not
  present.
- Immediately when the user starts filtering and the filter cannot be answered
  from the shallow payload.
- Immediately when the user requests a path that is not in the shallow payload.

Full payload loading does not start:

- For hidden React/Fern tabs that are mounted but not visible.
- For below-fold resources outside the near-viewport margin.
- Merely because the page imports the component.

### Worker Loading

Large full payloads should load through a Web Worker.

Main thread:

```js
worker.postMessage({
  type: "load",
  url: fullPayloadURL,
  schemaVersion: 2
});
```

Worker:

- Fetches the payload.
- Parses JSON.
- Builds indexes.
- Keeps the full schema store in the worker.
- Sends a compact ready message.

```js
worker.postMessage({
  type: "ready",
  lineCount: 18342,
  fieldCount: 6221,
  version: 2
});
```

For queries, the worker returns only the rows or ids needed by the current view.
It should not post the full parsed object back to the main thread.

Fallback:

- If Workers are unavailable, parse on the main thread in a scheduled task.
- Preserve correctness.
- The benchmark budget applies to modern browsers with Workers.

## DOM Strategy

### Runtime Owns the Tree DOM

The runtime creates and owns the tree DOM under its root. Hosts should not map
lines to React children.

The main tree DOM shape remains close to the current HTML renderer:

```html
<div class="kubectl-doc" data-kubectl-doc>
  <section class="kdoc-tree" role="tree">
    <div class="kdoc-line" data-kdoc-line data-index="42">
      <button class="kdoc-fold" data-kdoc-toggle></button>
      <span class="kdoc-yaml-text">...</span>
    </div>
  </section>
  <aside class="kdoc-details"></aside>
</div>
```

### Lazy Materialization

Do not materialize a DOM node just because a logical line exists.

Rows are materialized when:

- The line is visible under current fold state.
- The line is included by current filter state.
- The line is near the viewport if virtualization is enabled.

Rows are removed or recycled when:

- Their ancestor is collapsed.
- They are filtered out.
- They are far outside the viewport in very large expanded trees.

The first step can keep all visible rows in DOM. The second step adds row
recycling/windowing for pathological cases.

### Patch, Do Not Rebuild

For fold and filter updates:

- Compute a new visible line id set.
- Diff against current visible line id set.
- Hide/remove rows that disappeared.
- Insert/materialize rows that appeared.
- Update selection and details if focused row moved.
- Rewrap only comments whose width or visibility changed.

Avoid:

- Rebuilding the whole tree on each key.
- Recreating all line nodes during filtering.
- Reading layout for every row.
- Calling `querySelectorAll` on every update.

### Gutter and Copy Validity

The fold gutter must remain outside selected YAML text.

Rules:

- Fold controls have `user-select: none`.
- YAML text is in a separate selectable span.
- Copying a rectangular selection should not include triangles.
- If terminal/browser selection cannot exclude the gutter reliably, add a copy
  event handler that emits only logical YAML text for selected lines. This does
  not add a visible copy command or button.

## Filtering Strategy

Filtering is plain typing, not browser search. It must be separate from `/`
search in TUI and from browser built-in find in Fern/html.

Filter matches:

- Field name.
- Field description.
- Parent field name.
- Parent field description.
- Path components.
- Future: enum values and validation metadata if useful.

When a descendant matches:

- Ancestors are shown.
- Ancestors are unfolded for the filtered view.
- Direct matches are highlighted in strong orange.
- If the description matches, the field name is highlighted too.

Important behavior:

- `Esc` clears filtering.
- Focus stays on the same logical field when filtering is cleared.
- `Enter` accepts the filter-expanded state and never toggles folds while
  filtering is active.
- `Tab` and `Shift-Tab` jump between direct matches while filtering.
- `n` and `p` have no filtering function.

### Filter Index

Build a field index once:

```js
fieldIndex = [
  {
    id,
    path,
    nameLower,
    descriptionLower,
    parentIds,
    descendantIds,
    lineIds,
  }
]
```

Each keystroke:

1. Normalize query once.
2. Scan indexed field strings, not DOM nodes.
3. Build direct match field id set.
4. Build allowed field id set through ancestor closure.
5. Build visible line id set from allowed fields and structural lines.
6. Patch DOM.
7. Highlight direct matches in currently materialized rows only.

For very large schemas, move steps 2 through 5 into the worker.

### Incremental Filtering

For simple substring filtering, use previous result narrowing:

- If new query extends previous query, scan previous direct match candidates.
- If query shrinks, rescan the full field index.
- If query changes non-monotonically, rescan.

This keeps normal typing cheap.

## Folding Strategy

Folding state is keyed by stable line index or stable path id.

Rules:

- Right on collapsed foldable field expands it.
- Right on expanded field moves to first child.
- Left on expanded foldable field collapses it.
- Left on collapsed field moves to parent.
- Enter toggles the current foldable field.
- Tab and Shift-Tab jump between foldable fields.
- Home and End move to first and last visible field.
- PageUp and PageDown move by approximately half a visible page.
- While a filter is active, the filter interaction rules above take precedence:
  Enter accepts the filtered view instead of toggling the current field.

When filtering temporarily unfolds ancestors:

- Preserve user fold state separately from filter projection.
- Clearing filter restores the previous fold state.
- If the user presses Enter while filtering, keep the currently accepted
  unfolded state where it is semantically meaningful.

## Details Strategy

Details are rendered from `fields[]`, not from line attributes or parsed YAML.

The runtime keeps:

```js
fieldsById: Map<FieldId, FieldDetail>
lineToFieldId: Map<LineId, FieldId>
fieldToLineIds: Map<FieldId, LineId[]>
```

Details modes:

- `inline-side`: standalone HTML style, sticky side panel in the document flow.
- `side-overlay`: React-host style, overlay to the right of the schema frame
  and above host page sidebars such as Fern's right sidebar.
- `none`: useful for screenshots, tests, or embeddings.

Details should include:

- Path.
- Type.
- Required badge.
- Description.
- Validation and metadata.

Details should not include redundant fold/collapse state.

## Comment Wrapping

Semantic comment wrapping is part of the shared runtime.

Rules:

- Wrap is on by default.
- Each wrapped visual line keeps a `#` prefix.
- Continuation lines are aligned to the original comment prefix or the inline
  comment column.
- Wrapping applies only to currently materialized visible comment nodes.
- Width measurement is cached.
- Resize invalidates cached width and schedules one wrap pass.

No pixel-exact offsets should be used to align text. Use semantic CSS
alignment and inherited line height.

## Syntax Highlighting

The shared runtime should render from structured segments when available.

Segment kinds:

- `indent`
- `key`
- `punct`
- `string`
- `number`
- `bool`
- `null`
- `placeholder`
- `type-number`
- `comment`
- `required-label`
- `filter-hit`
- `url`

Fallback text parsing may remain during migration, but it should be removed
once the generator emits segments for all renderers.

## React And Fern Integration

The reusable React component source lives in this repository at
`react/kubectl-doc`. Fern custom React components consume that generic adapter;
Fern is not allowed to carry a separate schema tree renderer. Downstream
documentation repositories consume or vendor the generic component directory.

The component should be minimal:

```tsx
"use client";

import { useEffect, useRef } from "react";

export function KubeSchemaDoc({ data, filtering = true, detailsMode = "side-overlay" }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!ref.current) {
      return;
    }
    let controller;
    loadRuntime().then((runtime) => {
      controller = runtime.mount(ref.current, {
        initialSchema: data,
        filtering,
        detailsMode,
        loadFullSchema: data.fullPayloadURL
          ? () => fetch(data.fullPayloadURL).then((response) => response.json())
          : undefined,
      });
    });
    return () => controller?.destroy();
  }, [data, filtering]);

  return <div ref={ref} className="kubectl-doc kdoc-embedded-host kdoc-react-host" />;
}
```

This component:

- Does not render schema lines as JSX.
- Does not hold expanded line state in React.
- Does not hold filter query in React.
- Does not hold the full parsed schema in React state.
- Delegates to the shared runtime for DOM behavior.

Embedded-host CSS should only adapt containment, z-index, theme variables, and
integration with the host page layout. `kdoc-embedded-host` is the shared
layout contract for React/Fern and static downstream documentation embeddings;
`kdoc-react-host` is only a React adapter identity class.

Static embedding shells that need Fern-style details can opt into the same
runtime mode without React:

```html
<main class="kubectl-doc kdoc-embedded-host" data-kubectl-doc data-kdoc-details-mode="side-overlay">
  ...
</main>
```

The shared runtime reads `data-kdoc-details-mode="side-overlay"`, applies the
same `kdoc-details-side-overlay` and `kdoc-embedded-host` classes used by
React/Fern, and keeps keyboard handling scoped to the focused widget.

## Standalone HTML Integration

Standalone `-o html` embeds:

- Shallow or full payload as JSON.
- `kubectl-doc-styles.ts`, generated from
  `internal/render/web/assets/kubectl-doc.css`.
- `kubectl-doc-runtime.js`, generated by `make gen` from
  `internal/render/web/assets/kubectl-doc.js` and not committed as a maintained
  source file.
- A short boot script:

```html
<script type="application/json" id="kdoc-schema-0">...</script>
<script>
  KubectlDoc.mount(document.querySelector("[data-kubectl-doc]"), {
    initialSchema: JSON.parse(document.getElementById("kdoc-schema-0").textContent),
    filtering: true,
    detailsMode: "inline-side"
  });
</script>
```

For a single-file static HTML output, full payload may be embedded directly.
For documentation-style HTML examples where files can sit next to each other,
sidecars are allowed as a future optimization.

## Generator Changes

Add a shared payload package:

```text
internal/render/webschema
  payload.go
  payload_test.go
  segments.go
  sidecar.go
```

Responsibilities:

- Build payload v2 from `crd.Document`.
- Convert `tree.Line` into payload lines.
- Convert `fielddetail.Field` into payload fields.
- Carry logical line metadata such as root-description grouping, field grouping,
  path, required state, folded state, and filter text from the shared model.
- Create shallow payloads.
- Create full sidecars.
- Generate optional syntax segments.
- Keep stable ids.

Renderers consume this package:

- `internal/render/html` emits HTML host markup plus shared payload.
- `internal/render/markdown` emits Fern MDX plus shared payload and points at
  the reusable React component.
- Future web server mode can reuse the same payload.

## Testing

### Unit Tests

- Payload v2 round-trip.
- Stable ids across shallow and full payloads.
- Shallow payload includes all visible lines and referenced field details.
- Full payload contains collapsed descendants.
- Segment generation for required/default/validation/comment cases.
- Filter index matching rules.
- Fold projection with and without filtering.

### Runtime DOM Tests

Use Playwright or a lightweight browser test harness.

Test cases:

- Initial mount from shallow payload.
- Expand triggers full load.
- Filtering triggers full load when needed.
- Filtering disabled in a React/Fern host ignores typed filter keys while
  preserving fold, focus, and details behavior.
- Filtering applies only to the focused/selected version section when multiple
  resource versions are mounted on one page.
- Filtering highlights direct field and description matches.
- Clearing filter preserves logical focus.
- Enter while filtering accepts unfolded state.
- Left/right/enter/tab keyboard semantics in both payload-mounted and
  DOM-mounted schema hosts.
- Details render from field metadata.
- Comment wrapping keeps semantic `#` prefixes.
- Root-level descriptions and multi-line field descriptions select as one
  logical block.
- Copy selected YAML excludes fold gutters in both DOM-mounted HTML and the
  React/Fern host.
- The React component does not implement its own fold/filter/focus/details logic
  and does not render schema lines as JSX.
- Generated React-facing runtime assets are byte-identical to the shared runtime
  source, or are built from it during packaging with a CI drift check.
- Generated full schema sidecars are served and consumed as static JSON assets,
  not Markdown page routes.
- Filter keystroke work emits the shared `filter-apply` performance event.

### Performance Tests

The Playwright suite has two layers of performance evidence:

- Real generated DynamoGraphDeployment JSON sidecars, currently larger than
  2 MB per served version, prove the hosted-style static JSON route and lazy
  activation path.
- Deterministic synthetic schema tiers prove the budget independent of one
  downstream CRD shape.

Generated or deterministic fixtures:

- 100 KB small CRD.
- 500 KB medium CRD.
- 2 MB large CRD.
- DynamoGraphDeployment real CRD.
- Native Deployment schema fixture.

Benchmark phases:

```text
mount shallow payload
fetch full payload from warm cache
parse/index full payload
first expand after full load
first filter keystroke
incremental filter keystroke
clear filter
expand large subtree
comment wrap visible viewport
```

Budgets:

- Shallow mount: under 200 ms hard, under 100 ms preferred.
- Full payload activation main-thread blocking: under 200 ms hard, under 100 ms
  preferred.
- Filter keystroke p50: under 16 ms.
- Filter keystroke p95: under 50 ms.
- DOM nodes after shallow mount: proportional to visible lines.
- DOM nodes during collapsed full payload: must not approach full line count.

Record:

- `performance.now()` timings.
- `kubectl-doc:perf` events: `mount`, `full-schema-load`,
  `full-schema-activate`, `projection-render`, and `filter-apply`.
- Long task entries where available.
- DOM node count.
- Visible row count.
- Full index construction time.
- Main-thread patch time.

CI can use relaxed thresholds. Local benchmark commands should enforce the
stricter budgets during development.

## Migration Plan

### Phase 1: Extract Assets Without Behavior Change

- Move current HTML CSS and JS into embedded asset files.
- Keep the standalone HTML DOM contract unchanged.
- Add tests that rendered HTML still includes the expected runtime.
- No Fern changes yet.

### Phase 2: Introduce Shared Payload Package

- Move Fern payload generation into `internal/render/webschema`.
- Make HTML capable of rendering from the same payload.
- Keep old HTML line rendering until parity tests are in place.

### Phase 3: Runtime Mount API

- Replace inline standalone boot logic with `KubectlDoc.mount`.
- Keep current DOM-based algorithm.
- Add controller cleanup.
- Add host options for details mode, filtering, back URL, and quit URL.

### Phase 4: React Adapter

- Provide the thin React mount wrapper from `react/kubectl-doc` in this
  repository.
- Replace downstream React line renderers with consumption of that wrapper.
- Treat `react/kubectl-doc/kubectl-doc-runtime.js` and generated style strings
  as packaging outputs from the shared web assets. Do not edit them
  independently.
- Keep the standalone HTML renderer as the blueprint for DOM structure,
  keyboard behavior, folding, filtering, details, wrapping, and copy-valid YAML.
- Load React/Fern full payload sidecars as JSON assets.
- Verify hosted Fern preview sidecar asset routes.
- Add tests to ensure the React component does not map lines to JSX.

### Phase 5: JSON Sidecars

- Add raw JSON sidecar output.
- Use `response.json()` in the React component.
- Keep generated schema payloads out of Fern markdown page routes.

### Phase 6: Worker Parse and Index

- Add worker loading for full payloads over a threshold.
- Keep main-thread fallback.
- Add ready/query/diff worker protocol.
- Measure 2 MB payload load budget.

### Phase 7: Lazy Row Materialization

- Stop rendering every logical line as DOM.
- Materialize only visible rows.
- Add optional viewport row recycling for very large expanded trees.
- Ensure selection and copy still operate on logical visible YAML lines.

### Phase 8: Structured Segments

- Emit syntax segments from Go.
- Update runtime renderer to prefer segments.
- Remove duplicated YAML text parsing from JS/TS.
- Keep plain text fallback for older payloads.

## Risks and Mitigations

### React/Fern Asset Constraints

Fern may restrict custom JavaScript packaging.

Mitigation:

- Keep the generic React component as the official Fern integration.
- Place generated runtime files under `react/kubectl-doc` if bundlers need
  component-local imports, but keep their source in the shared web asset tree
  and enforce drift checks.
- Keep generated schema sidecars under Fern static assets, not page routes.

### Worker Bundling

Workers can be awkward in MDX/custom component bundlers.

Mitigation:

- Support an inline worker blob created from a runtime string.
- Support a same-origin worker URL when available.
- Support main-thread fallback.
- Keep worker protocol isolated from host code.

### Payload Format Churn

Changing payload structure can break existing generated docs.

Mitigation:

- Include `schemaVersion`.
- Runtime supports payload v1 and v2 during migration.
- Tests load current Dynamo generated sidecars.
- Keep compatibility until a release boundary.

### Over-Optimization Too Early

Compact array payloads and binary formats reduce readability.

Mitigation:

- Start with readable v2 objects.
- Add benchmark gates.
- Introduce compact records only if the readable format misses budgets.

### Copy Behavior

Browser selection behavior varies.

Mitigation:

- Keep gutter non-selectable.
- Keep YAML in a separate span.
- Add a copy event handler only if selection still includes gutters in target
  browsers.

## Open Questions

- Can Fern custom React components import a worker asset directly, or must the
  worker be created from a blob?
- What exact machine/browser should define the preferred 2 MB under 100 ms
  benchmark?
- Should the runtime expose a public path-focus API for future deep links?
- Should the payload split field details into separate chunks for extremely
  large native schemas?

## Acceptance Criteria

The shared runtime work is complete when:

- `-o html` and `markdown-fern` both use `KubectlDoc.mount`.
- The React component no longer renders one JSX element per schema line.
- All line semantics used by web renderers come from `tree`, `fielddetail`, and
  `webschema`; no web host infers important metadata by reparsing YAML text.
- Generated React runtime/style artifacts cannot drift from the shared web
  source without a failing test or `make check-generated` failure.
- Runtime behavior for fold, focus, details, filtering, wrapping, and keyboard
  navigation is covered by shared tests.
- Multi-version web pages scope focus and filtering to the currently focused
  version instead of applying filters to every mounted version at once.
- A 2 MB full schema payload can be loaded and activated with less than 200 ms
  of main-thread blocking work.
- Filtering a loaded 2 MB schema renders only the visible projection, not the
  whole DOM.
- Full schema sidecars load through the static JSON route pattern used by
  hosted Fern previews.
- Generated output remains driven by structured schema metadata.
- Copied selected YAML remains valid YAML, excluding fold gutters.
