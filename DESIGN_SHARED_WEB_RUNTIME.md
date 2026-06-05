# Shared Web Runtime Design

## Purpose

`kubectl-doc` has two browser-facing schema renderers:

- `-o html`, a standalone static HTML document with embedded CSS and vanilla
  JavaScript.
- `-o markdown-fern`, an MDX page that mounts a Fern custom React component.

Both render the same conceptual UI: a foldable YAML schema tree, field focus,
field details, semantic comment wrapping, keyboard navigation, and interactive
filtering. Today they share the generated schema model, but they do not share
the browser runtime. The HTML renderer has accumulated careful DOM-oriented
performance optimizations, while the Fern component reimplements behavior in
React and maps visible lines to JSX.

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
        standalone -o html           Fern React host
        inline assets                lifecycle wrapper only
```

The Fern component must become an adapter around the shared runtime, not an
independent renderer.

## Goals

- Use one optimized DOM runtime for `-o html`, `-w` schema pages, and
  `markdown-fern`.
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
- Keep `markdown-fern` compatible with Fern custom React components and static
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

- Initial visible schema tree interactive in under 100 ms after the host
  component or static HTML root is mounted.
- Initial DOM nodes proportional to the visible shallow tree, not to the full
  schema.
- Full payload fetch must not block initial focus, fold, details, or copy.

### Full Payload Load Budget

When a full schema payload is 2 MB uncompressed, loading it in the browser must
not take longer than 100 ms of main-thread blocking work.

Network transfer time is environment dependent and is not counted in this
budget. The measured browser budget is from "payload bytes available" to
"runtime can answer line/detail/filter queries" on a normal developer laptop.

Targets:

- Main-thread blocking during full payload load: under 100 ms.
- Main-thread blocking per filter keystroke after full index is available:
  under 16 ms for typical visible trees, under 50 ms worst-case for very large
  schemas.
- Avoid creating DOM nodes for collapsed descendants.
- Avoid creating DOM nodes for filtered-out descendants.
- Avoid reparsing YAML text during filtering.
- Avoid lowercasing all descriptions on every filter keystroke.
- Avoid React rendering one component per schema line in the Fern path.

These budgets are acceptance criteria. If a browser benchmark shows that a
single 2 MB JSON parse plus index build can exceed the budget, the runtime must
move parse and index construction into a Web Worker and only transfer compact
query results to the main thread.

## Current State

### HTML Renderer

The standalone HTML renderer:

- Builds `tree.Line` records from the schema.
- Renders one DOM line per generated line.
- Stores field metadata in data attributes and details HTML.
- Indexes DOM lines once during startup.
- Handles fold/filter/focus by toggling existing DOM state.
- Uses class additions/removals and `hidden` instead of rebuilding subtrees.
- Reflows comments only for visible comment nodes.

This is the performance baseline and the behavioral source of truth.

Limitations:

- The runtime is emitted as a Go string, which makes sharing and testing hard.
- Details are duplicated in DOM attributes, increasing HTML size.
- Syntax highlighting is partly text parsing at render time.
- The runtime is coupled to the exact standalone HTML page structure.

### Fern Renderer

The Fern renderer:

- Generates a structured payload with `lines[]` and `fields[]`.
- Optionally writes full schema sidecars and embeds a shallow payload.
- Mounts a custom React component.
- Reimplements fold/filter/focus/details in React state.
- Renders visible lines through JSX.

Limitations:

- Filtering can cause React reconciliation over many lines.
- Behavior can drift from standalone HTML.
- Syntax highlighting and keyboard behavior are duplicated.
- The full schema data is held in React state.

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
- Fern React creates an empty host div, calls `mount` in an effect, and calls
  `destroy` on unmount.

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

For Fern, `loadFullSchema` fetches the generated sidecar URL. For standalone
HTML, the full payload can be embedded, omitted, or loaded from a sibling file
depending on output mode.

## Payload v2

The current Fern payload is close to the needed shape, but it should be renamed
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
the readable object format cannot meet the 100 ms load budget.

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

The shallow payload should stay small enough that a Fern page with several
resources can render quickly. Target: under 100 KB per rendered resource,
preferably under 50 KB.

### Full Payload Sidecar

Full payload sidecars should be raw JSON when the host can serve them as assets.
Markdown fences with base64 are acceptable only as a compatibility fallback.

Reasons to prefer raw JSON:

- No base64 expansion.
- No markdown wrapper parsing.
- `response.json()` and worker parsing can use browser-native paths.
- CDN content type and compression are straightforward.
- Sidecar routes can be tested directly.

For Fern, this requires verifying the supported asset/page route. If Fern can
only serve `.md` pages for sidecars, the runtime should still avoid base64 for
large payloads when possible, for example with a fenced plain JSON block. The
long-term target remains raw JSON assets.

### Load Triggers

Full payload loading starts:

- On idle when the schema host is visible or near the viewport.
- Immediately when the user expands a collapsed node whose descendants are not
  present.
- Immediately when the user starts filtering and the filter cannot be answered
  from the shallow payload.
- Immediately when the user requests a path that is not in the shallow payload.

Full payload loading does not start:

- For hidden Fern tabs that are mounted but not visible.
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
- `Enter` accepts the filter-expanded state where required.
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
- `side-overlay`: Fern style, overlay to the right of the schema frame and
  above Fern's own right sidebar.
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

## Fern Integration

Fern custom React components remain the integration point. The reusable
component source lives in this repository at `fern/components/kubectl-doc`.
Downstream documentation repositories consume or vendor that directory; they do
not own separate schema tree renderers.

The component should be minimal:

```tsx
"use client";

import { useEffect, useRef } from "react";
import { ensureKubectlDocRuntime } from "./runtime";

export function KubeSchemaDoc({ data, filtering = true }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!ref.current) {
      return;
    }
    const runtime = ensureKubectlDocRuntime();
    const controller = runtime.mount(ref.current, {
      initialSchema: data,
      filtering,
      detailsMode: "side-overlay",
      loadFullSchema: data.fullPayloadURL
        ? () => fetch(data.fullPayloadURL).then((response) => response.arrayBuffer())
        : undefined,
    });
    return () => controller.destroy();
  }, [data, filtering]);

  return <div ref={ref} className="kubectl-doc kdoc-fern-host" />;
}
```

This component:

- Does not render schema lines as JSX.
- Does not hold expanded line state in React.
- Does not hold filter query in React.
- Does not hold the full parsed schema in React state.
- Delegates to the shared runtime for DOM behavior.

Fern-specific CSS should only adapt containment, z-index, theme variables, and
integration with Fern's page layout.

## Standalone HTML Integration

Standalone `-o html` embeds:

- Shallow or full payload as JSON.
- `kubectl-doc-runtime.css`.
- `kubectl-doc-runtime.js`.
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
- Create shallow payloads.
- Create full sidecars.
- Generate optional syntax segments.
- Keep stable ids.

Renderers consume this package:

- `internal/render/html` emits HTML host markup plus shared payload.
- `internal/render/markdown` emits Fern MDX plus shared payload.
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
- Filtering highlights direct field and description matches.
- Clearing filter preserves logical focus.
- Enter while filtering accepts unfolded state.
- Left/right/enter/tab keyboard semantics.
- Details render from field metadata.
- Comment wrapping keeps semantic `#` prefixes.
- Copy selected YAML excludes fold gutters.

### Performance Tests

Add generated fixtures:

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

- Shallow mount: under 100 ms.
- Full payload parse/index main-thread blocking: under 100 ms.
- Filter keystroke p50: under 16 ms.
- Filter keystroke p95: under 50 ms.
- DOM nodes after shallow mount: proportional to visible lines.
- DOM nodes during collapsed full payload: must not approach full line count.

Record:

- `performance.now()` timings.
- Long task entries where available.
- DOM node count.
- Visible row count.
- Worker parse/index time.
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

### Phase 4: Fern Adapter

- Provide the thin Fern mount wrapper from `fern/components/kubectl-doc` in
  this repository.
- Replace downstream React line renderers with consumption of that wrapper.
- Keep the standalone HTML renderer as the blueprint for DOM structure,
  keyboard behavior, folding, filtering, details, wrapping, and copy-valid YAML.
- Keep the current sidecar loading behavior initially.
- Verify hosted Fern preview sidecar routes.
- Add tests to ensure Fern component does not map lines to JSX.

### Phase 5: Raw JSON Sidecars

- Add raw JSON sidecar output where supported.
- Keep markdown-fenced sidecars as fallback.
- Prefer `ArrayBuffer` or `response.json()` in the worker.
- Remove base64 for large full payloads when possible.

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

### Fern Asset Constraints

Fern may restrict arbitrary static JSON routes or custom JavaScript packaging.

Mitigation:

- Keep React custom component as the official Fern integration.
- Place runtime files under `fern/components/kubectl-doc` if Fern only bundles
  component-local imports.
- Keep markdown sidecar compatibility until raw JSON asset serving is verified.

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

- Can Fern serve raw JSON sidecars with `application/json`, or do sidecars need
  to remain Markdown pages?
- Can Fern custom React components import a worker asset directly, or must the
  worker be created from a blob?
- What exact machine/browser should define the 2 MB under 100 ms benchmark?
- Should the runtime expose a public path-focus API for future deep links?
- Should the payload split field details into separate chunks for extremely
  large native schemas?

## Acceptance Criteria

The shared runtime work is complete when:

- `-o html` and `markdown-fern` both use `KubectlDoc.mount`.
- The Fern React component no longer renders one JSX element per schema line.
- Runtime behavior for fold, focus, details, filtering, wrapping, and keyboard
  navigation is covered by shared tests.
- A 2 MB full schema payload can be loaded and indexed with less than 100 ms of
  main-thread blocking work.
- Filtering a loaded 2 MB schema does not rebuild the whole DOM.
- Full schema sidecars load successfully in hosted Fern previews.
- Generated output remains driven by structured schema metadata.
- Copied selected YAML remains valid YAML, excluding fold gutters.
