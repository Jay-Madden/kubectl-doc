# Kubernetes API Docs Generator Design

## Status

This is the initial implementation design for `kubectl doc`. It translates
[REQUIREMENTS.md](./REQUIREMENTS.md) into a concrete Go architecture. The design
keeps the tool documentation-only: it reads discovery, OpenAPI, and CRD schemas,
then renders schema documentation. It never creates, edits, validates, or applies
Kubernetes resources.

Repository: `git@github.com:sttts/kubectl-doc.git`

Go module path: `github.com/sttts/kubectl-doc`

## Architecture

The tool has one main pipeline:

```text
Cobra CLI
  -> input source
  -> resource/version resolution
  -> source-specific schema adapter
  -> kubectl-doc Structural schema
  -> schema normalization
  -> documentation model
  -> renderer
```

The central rule is that schema interpretation happens once, before rendering.
Renderers consume the same documentation model and should not parse OpenAPI,
kubectl-doc Structural schemas, or CRD YAML directly.

The tool is Kubernetes-schema-specific. It does not try to support arbitrary
OpenAPI v3 documents. The supported schema language is the structural schema
subset used by Kubernetes CRDs and published Kubernetes OpenAPI v3 documents.

## Package Layout

Planned package boundaries:

```text
cmd/kubectl-doc
internal/cli
internal/kube
internal/crd
internal/schema
internal/docmodel
internal/render/yaml
internal/render/kro
internal/render/man
internal/render/markdown
internal/render/html
internal/tui
internal/web
internal/search
internal/testutil
```

Responsibilities:

- `cmd/kubectl-doc`: tiny `main`, calls `cli.NewCommand().Execute()`.
- `internal/cli`: Cobra command, flags, option validation, output dispatch.
- `internal/kube`: kubeconfig loading, discovery, REST mapping, OpenAPI v3 fetch.
- `internal/crd`: CRD file loading and served-version extraction.
- `internal/schema`: kubectl-doc Structural schema types, source adapters,
  normalization, and reference resolution.
- `internal/docmodel`: resource tree, YAML tree, field metadata, diagnostics.
- `internal/render/*`: non-interactive renderers.
- `internal/tui`: interactive terminal UI.
- `internal/web`: localhost browser server and static HTML document assembly.
- `internal/search`: shared search indexing and result navigation.
- `internal/testutil`: fixtures, golden helpers, YAML parse checks.

## CLI

Use Cobra for command and flag handling. Do not build a custom flag parser.

The first version can be a single root command with optional positional resource
selector:

```text
kubectl doc [resource] [flags]
```

The executable is `kubectl-doc`; kubectl invokes it as `kubectl doc`.

Flags:

```text
-f, --filename <path>        CRD manifest path, repeatable
-o, --output <format>       yaml|kro|tui|man|browser|markdown|markdown-github|markdown-fern|html
-i, --interactive           switch default output from yaml to tui
-w, --web                   shortcut for -o browser
    --nocolor               disable styling in -o yaml
    --version <version>     served CRD version selector
    --all-versions          render all served versions where supported
    --expand-depth <n>      initial static expansion depth
    --descriptions <mode>   false|required|true, default true
    --columns <n>           target Markdown paragraph width
    --field-details         include Markdown field detail sections
    --disable-filtering     disable generated filtering in static interactive docs
    --fern-schema-dir <dir> write full markdown-fern schema JSON sidecars
    --fern-schema-url-path <path>
                            relative URL prefix for generated schema JSON sidecars
    --path <json-path>      renderer-specific initial focus or subtree zoom
```

Implementation notes:

- Define a `cli.Options` struct and bind all Cobra flags into it.
- Normalize `-i`/`--interactive` to `OutputTUI` and `-w`/`--web` to
  `OutputBrowser` after parsing.
- Treat `-i`/`--interactive` as switching the renderer default from `yaml` to
  `tui`. It is not a modifier on the YAML renderer.
- Default output is `yaml`.
- Use Cobra validation for positional argument count and flag combinations.
- Validate `--descriptions` as one of `false`, `required`, or `true`.
- Validate `--columns` as non-negative. `0` means auto-detect terminal width
  where possible and otherwise use a deterministic fallback of `80`.
- `--field-details` defaults to false and is opt-in for Markdown renderers.
- Use Kubernetes CLI/client-go config loading rules for kubeconfig behavior.
- Add Kubernetes config flags through the standard Kubernetes CLI machinery
  rather than reimplementing kubeconfig parsing.

Mode validation:

- No `-f`: cluster mode. A positional resource selector is optional. Without a
  selector, the default `yaml` output renders only the resource overview.
- `-f`: CRD file mode. Cluster discovery is not required.
- `--version` is valid only with `-f`.
- `--all-versions` is valid only for documentation-page/schema renderers:
  `html`, `man`, `kro`, `markdown`, `markdown-github`, and `markdown-fern`.
- `--all-versions` conflicts with `--version`.
- `--interactive` conflicts with `--web`.
- `--interactive` conflicts with an explicit `-o` value other than `tui`.
- `--web` conflicts with an explicit `-o` value other than `browser`.
- `-o html` and `-o yaml` write to stdout.
- `-o browser` starts a local server and owns the process until Ctrl-C.
- Browser mode best-effort opens the printed localhost URL with the default
  browser on common desktop environments. Opening failures or missing openers
  are ignored so the command keeps serving.
- Interactive outputs, `tui` and `browser`, do not require a resource selector.
- Non-interactive schema outputs require a selected resource when rendering a
  schema. For `-o yaml`, no resource selector renders the resource overview.
- `--path` is a renderer capability, not a universal output contract. The CLI
  should reject unsupported renderer/path combinations with a clear error before
  fetching schemas.

Selection and version defaulting:

- Cluster selectors use normal `kubectl get`-style resource syntax.
- Qualified cluster selectors use Kubernetes' dot syntax:
  `resource.group`, or `resource.version.group` when the version is explicit.
  The group part may contain dots, for example `widgets.example.com` or
  `widgets.v1.example.com`.
- A two-segment selector such as `pods.v1` is parsed as resource `pods` in group
  `v1`, not as core API version `v1`. Core resources should normally be
  selected by resource name, such as `pods`; non-interactive renderers then use
  the latest served version.
- CRD file mode uses the CRD's single implicit resource.
- Interactive modes show version choices and wait for explicit selection. Do not
  apply version auto-selection in the interactive UI.
- Non-interactive modes choose the latest served version when no explicit version
  is provided.
- For non-interactive auto-selection, stable versions rank above beta versions,
  beta versions rank above alpha versions, and the highest numeric version wins
  within the same stability tier.
- The non-interactive version selection rule applies to cluster resources and CRD
  files.
- Documentation-page renderers, `html`, `man`, `markdown`, `markdown-github`,
  `markdown-fern`, and the `kro` schema renderer default to the latest served
  version and can render all served versions when `--all-versions` is set.
- `yaml` renders exactly one selected version.

Path focus and zoom:

- `--path` addresses the normalized documentation model using the same
  JSON-path-like labels shown by interactive details panes, for example
  `spec.template.spec.containers`.
- Static text renderers that support path zoom render the selected subtree as
  the root of their schema output. This is the natural behavior for `yaml`,
  `markdown`, `markdown-github`, and `markdown-fern`.
- Interactive renderers that support `--path` keep the full document available.
  They expand the target's ancestors, focus the selected field, show its
  details, and scroll it into view. This is the natural behavior for `tui`,
  `browser`, and `html`.
- `browser` should preserve the requested path in its route or query string
  when practical, so refreshes and shared localhost links return to the same
  focused field while the kubectl-doc process is still running.
- Renderers such as `man` and `kro` may reject `--path` until a useful
  renderer-specific behavior is implemented.
- Invalid paths produce a clear error after resource/version selection and
  schema normalization, before rendering starts.

## Data Sources

### Cluster Mode

Cluster mode needs discovery before OpenAPI:

1. Build a REST config from kubeconfig and flags.
2. Fetch API groups and resources through discovery.
3. Build the group/resource/version navigation tree.
4. If a resource selector was provided, resolve it using normal `kubectl
   get`-style syntax.
5. If an interactive renderer has no resource selector, start at the resource
   navigation view and defer OpenAPI fetching until a resource version is
   selected.
6. If a non-interactive renderer has no resource selector, render the discovery
   overview and stop.
7. If a documentation-page renderer has `--all-versions`, fetch and render each
   served version for the selected resource.
8. Otherwise, if a non-interactive renderer has a resource selector without a
   version, auto-select the version.
9. Fetch the OpenAPI v3 document for each resolved group-version.
10. If a built-in resource is missing from the fetched OpenAPI v3 document, try
    the embedded upstream Kubernetes OpenAPI v3 document for the same
    group-version.
11. Build docs for the selected resource and version set.

OpenAPI v3 is mandatory. Do not call `/openapi/v2`, and do not add a v2 fallback.
OpenAPI v3 is a transport/source format here, not the renderer contract. The
cluster adapter reads Kubernetes OpenAPI v3 and converts the schema subset used
by native Kubernetes resources into kubectl-doc's internal Structural model.

OpenAPI v3 fetching:

- Read `/openapi/v3`.
- Find the selected group-version entry.
- Fetch its `serverRelativeURL`.
- Prefer the cluster's fetched schema. The embedded native fallback only covers
  built-in Kubernetes resources when discovery advertises the resource but the
  selected group-version OpenAPI document is incomplete.
- Do not cache the fetched schema in the first version.

The resource overview does not need to fetch every OpenAPI document. It can
render from discovery alone until the user selects a resource in TUI/browser
mode.

YAML overview output renders single-version resources as a scalar, for example
`pods: v1`, and multi-version resources as an inline YAML sequence, for example
`deployments: ["v1","v1beta1"]`. Multi-version lists use the same latest-first
ordering as non-interactive version auto-selection.

### CRD File Mode

CRD mode reads local `apiextensions.k8s.io/v1` CRDs:

1. Decode one or more YAML documents from each `-f` path.
2. Keep only `CustomResourceDefinition` objects.
3. Read names, group, scope, and served versions.
4. In interactive modes, show all served versions for explicit user selection.
   In non-interactive modes, select the requested `--version`, or auto-select
   the latest served version with the same stable/beta/alpha ordering used in
   cluster mode.
5. Convert `spec.versions[*].schema.openAPIV3Schema` into Kubernetes' CRD
   structural schema representation as an adapter step, then copy it into
   kubectl-doc's internal Structural model before normalization.

A CRD defines one kind. Do not add a `--kind` flag unless requirements change.

## Structural Schema Scope

Renderers and the documentation model depend on kubectl-doc's own Structural
schema package. That package starts as a close copy of Kubernetes' CRD
structural schema shape:

```text
k8s.io/apiextensions-apiserver/pkg/apiserver/schema.Structural
```

The copied shape is the lowest common denominator for CRDs and built-in
Kubernetes resources. It is intentionally owned by kubectl-doc so native
OpenAPI support can add variants when Kubernetes built-in resources expose
schema cases that do not fit the CRD structural type exactly.

After the CRD-first epic, the next schema-model validation step is to pass
through the native Kubernetes resources from cluster OpenAPI v3. Each native
resource that cannot be represented cleanly by the copied CRD Structural shape
should drive a small, explicit extension to kubectl-doc's Structural model.
Do not speculate broad OpenAPI support ahead of those concrete cases.

The upstream CRD type and `schema.NewStructural` remain useful inside the CRD
file adapter. They are not the backend contract and must not leak into
renderers, the documentation model, or the cluster OpenAPI adapter.

Initial internal model:

```text
Structural
StructuralOrBool
Generic
Extensions
ValidationExtensions
ValueValidation
JSON
```

Planned extension points:

- Explicit variant markers for native Kubernetes cases that do not fit the CRD
  structural type.
- Reference identity and recursion markers for OpenAPI component references.
- Source diagnostics for unsupported or lossy adapter conversions.
- Kubernetes scalar conveniences such as quantity, time, duration,
  int-or-string, and raw extension rendering hints.

Supported:

- Structural object, array, map, and scalar schemas represented by the internal
  model.
- Required fields.
- Defaults, enum values, nullable markers, examples, and validations exposed by
  the structural schema.
- Kubernetes structural extensions such as list type, list map keys, map type,
  embedded resource, preserve unknown fields, and int-or-string.
- Generated single-ref `allOf` wrappers that Kubernetes OpenAPI v3 uses to add
  field-local metadata to a referenced schema.
- `x-kubernetes-validations` as documentation.

Unsupported:

- Arbitrary OpenAPI v3 documents outside Kubernetes structural schema rules.
- OpenAPI features that cannot appear in structural Kubernetes schemas.
- Schema behavior that would require evaluating arbitrary `oneOf` or `anyOf`
  branches.
- OpenAPI composition quantors (`oneOf`, `anyOf`, `allOf`) as schema structure.
  The exception is Kubernetes int-or-string, detected through
  `x-kubernetes-int-or-string` or generated native `format: int-or-string`.

Unsupported constructs should produce diagnostics where possible instead of
crashing the renderer.

## Resource Resolution

The resolver takes a user selector plus discovery data and returns a
group/version/resource/kind identity.

Supported selector forms:

```text
deployments
deploy
Deployment
deployments.apps
deployments.v1.apps
widgets.example.com
widgets.v1.example.com
```

Resolution should use Kubernetes discovery and REST mapping behavior where
possible, including Kubernetes' `resource.group` and `resource.version.group`
dot grammar. A two-segment selector such as `pods.v1` is not a core-version
selector; it is parsed as resource `pods` in group `v1`. Ambiguity is an error
with a list of matches. The resolver must not silently choose between multiple
group/version/kind matches.

The navigation tree stores:

- API group, with `core` as display name for the empty group.
- Resource plural name.
- Kind.
- Short names.
- Scope.
- Served versions.
- Preferred version marker.
- Verbs and extra discovery metadata for details panes.

## Schema Normalization

The normalized schema model is independent of renderer concerns. Its input is a
kubectl-doc Structural schema, not arbitrary OpenAPI v3 and not an upstream CRD
type.

Core types:

```text
SchemaDoc
ResourceIdentity
FieldNode
FieldMetadata
SchemaRef
ValidationRule
```

`FieldNode` should contain:

- JSONPath.
- Field name.
- Type and format.
- Required/optional state.
- Default, enum, examples.
- Description.
- Constraints.
- Kubernetes OpenAPI extensions.
- Children.
- Map item schema.
- Array item schema.
- Diagnostics for unsupported schema constructs.
- Recursive-reference marker.

Cluster OpenAPI adapters and CRD adapters both feed the same internal
Structural normalizer:

```text
cluster OpenAPI v3 schema -> kubectl-doc Structural -> FieldNode tree
CRD openAPIV3Schema
  -> CRD Structural adapter
  -> kubectl-doc Structural
  -> FieldNode tree
```

References should be resolved into nodes until the recursion limit is reached.
After that, render a reference marker instead of expanding forever.

OpenAPI composition quantors are not renderer structure. Do not evaluate,
merge, or render `oneOf`, `anyOf`, or multi-branch `allOf` alternatives. The
supported Kubernetes-native special cases are int-or-string and generated
single-ref `allOf` wrappers. Int-or-string is represented by the Kubernetes
extension flag
on the field, not by general `anyOf` handling.

## Documentation Model

Renderers receive a `docmodel.Document`:

```text
Document
  Source
  NavigationTree
  ResourceIdentity
  YAMLTree
  FieldIndex
  Diagnostics
```

`YAMLTree` is a structured tree, not a formatted string. It owns:

- YAML key.
- YAML value or placeholder.
- Comment fragments.
- Foldable state hints.
- Required/optional state.
- Status-field marker.
- JSONPath.
- Link to `FieldMetadata`.

The final YAML string is produced only at renderer time. This keeps selection,
folding, syntax highlighting, and Markdown output consistent.

## Field Visualization

The first implementation should use a simple, deterministic visualization policy
and leave room for iteration.

Rules:

- Defaults are rendered as actual YAML values.
- Field-local examples render as actual YAML values when there is no default,
  with compact inline metadata such as `# example string` or
  `# example object primary`.
- Enums render the default if present; otherwise render one enum value and put
  alternatives in a comment.
- Required fields are uncommented.
- If an optional parent contains required descendants, the parent path is also
  rendered as live YAML so those descendants are not hidden behind comments.
  These live optional parents get an inline `# optional` marker.
- Optional fields are commented and folded by default.
- Description comments render immediately before the field they describe, at the
  same indentation as the field key. Static YAML separates sibling field blocks
  with empty lines for readability.
- `--descriptions=true` renders all field descriptions, `required` renders only
  descriptions for required fields, and `false` suppresses description comments.
- `status` is generated as a real schema subtree in interactive outputs and is
  initially collapsed. In non-interactive outputs it is rendered as a folded
  comment.
- Lists without defaults or examples render one representative item.
- Maps without defaults or examples render one representative `<key>` entry.
- Nullable fields document nullability in comments/details, not by rendering
  `null` unless `null` is the default.
- When static YAML collapses an object or object array item because of
  `--expand-depth`, append an inline hint with the minimum depth needed to open
  that node, such as `# show with --expand-depth 4`.

The placeholder table is intentionally small at first:

```text
string       -> "<string>"
integer     -> <integer>
integer/i32 -> <int32>
number      -> <number>
boolean     -> <boolean>
object      -> {}
array       -> []
map         -> <key>: <value>
```

Golden tests should lock down the first policy, then we can improve it by
updating fixtures and expected output.

## Renderers

### YAML Renderer

`-o yaml` is the default and prints to stdout.

Responsibilities:

- Render syntactically valid YAML.
- Render exactly one selected version.
- Apply `--expand-depth`.
- Comment optional fields.
- Represent folded nodes as comments where controls cannot live outside text.
- Add compact metadata comments for defaults, enum alternatives, and simple
  constraints.
- Render schema descriptions according to `--descriptions`.
- Include an inline `--expand-depth` hint on statically collapsed object nodes.
- Style output when terminal capabilities support it and `--nocolor` is false.

Color is a presentation layer. The underlying bytes without ANSI sequences must
still parse as YAML.

### Kro Renderer

`-o kro` prints a Kro SimpleSchema-style YAML schema view to stdout.

The Kro renderer is schema documentation, not manifest-shaped YAML. It maps the
internal Structural model into concise Kro-like type expressions and markers:

```yaml
apiVersion: stable.example.com/v1
kind: CronTab
spec: # required=true description="CronTabSpec describes the desired cron job."
  cronSpec: string | required=true minLength=1 description="Cron expression for running the job."
  image: string | required=true description="Container image used by the job."
  concurrencyPolicy: string | default="Allow" enum="Allow,Forbid,Replace"
  labels: "map[string]string"
  ports:
    - containerPort: integer | required=true format=int32
      name: string | required=true
```

Rendering rules:

- Scalars render as `string`, `integer`, `float`, `boolean`, or `object`.
- Arrays and maps render as quoted Kro type expressions, for example
  `"[]integer"` and `"map[string]string"`.
- Arrays of structured objects render one representative nested list item.
- Maps with structured object values currently render as `map[string]object`
  because Kro's reusable custom type references would require emitting a
  matching `types` section.
- Supported validation/documentation markers include `required`, `default`,
  `description`, `enum`, `minimum`, `maximum`, `pattern`, string length, and
  array item-count markers.
- `--descriptions` controls `description="..."` markers.
- Kubernetes extensions without a Kro marker are preserved as compact comments.
- With `--all-versions`, render a YAML document stream with one schema document
  per served version.

### TUI Renderer

`-o tui` starts an interactive terminal UI.

Use Bubble Tea v2 as the TUI framework. Use Bubbles v2 components where they
fit naturally, for example for resource/version lists, viewport panes, and
search input. Keep schema focus, fold state, search results, and details backed
by kubectl-doc's shared documentation/tree model rather than by reparsing
rendered YAML text.

Layout:

```text
wide:    navigation + foldable YAML tree, details pane on the right
narrow:  navigation + foldable YAML tree, details pane below
```

Focus model:

- The active schema focus is always on one visible JSONPath field in the schema.
  The details view updates automatically when focus changes.
- If `--path` is provided, start with that field focused, its ancestors
  expanded, and its details visible.
- Up and Down move focus to the previous or next visible field.
- Left on an expanded focused field collapses that field and keeps focus there.
  Left on a collapsed field or leaf field moves focus to the parent field when
  one exists.
- Right on a collapsed focused field expands that field and keeps focus there.
  Right on an expanded field moves focus to its first visible child when one
  exists. Right on a leaf field or field without a visible child is a no-op.
- Tab moves focus to the next visible collapsible field.
- Shift-Tab moves focus to the previous visible collapsible field.
- Enter toggles the focused field when it is collapsible.
- Home moves focus to the first visible field.
- End moves focus to the last visible field.
- `q` and F10 exit.

Rendering behavior:

- Comment wrapping is automatic and semantic, using the available pane width.
  Wrapped comment continuations must keep YAML comment indentation and a fresh
  `#` per visual line. Unlike HTML, TUI wrapping does not need a user toggle.

Search:

- `/` searches field names and descriptions.
- `//` searches field names only.
- Esc exits search mode.
- `n` and `p` move to the next and previous search result.
- Matches are highlighted in strong orange.
- The focused match has an additional non-color marker.

The TUI does not provide a copy command.

### Browser Renderer

`-o browser` starts a localhost server, prints the browser URL, and best-effort
opens that URL in the default browser on common desktop environments.

Server behavior:

- Bind to localhost with port `0`.
- Print or log the chosen local URL.
- Ignore URL-opening failures and continue serving.
- Fetch OpenAPI using the same kubeconfig context as the CLI.
- When no resource selector is passed in cluster mode, serve discovery-backed
  group/resource/version navigation first and fetch OpenAPI lazily when a
  resource version route is requested. Do not render every cluster schema into a
  single static page.
- Keep running until the user sends Ctrl-C.
- Do not define browser quit shortcuts.

The browser UI mirrors the TUI:

- Navigation tree.
- Foldable YAML tree.
- Details pane.
- Full metadata tree, initially collapsed. Use the resource OpenAPI metadata
  schema when available; otherwise synthesize Kubernetes `ObjectMeta` for CRDs.
- JSONPath focus.
- Same search semantics.
- Mouse support for fold controls.
- If `--path` is provided, keep the full selected schema available, expand the
  target's ancestors, focus the target, show its details, and scroll it into
  view. Do not reduce browser mode to only the selected subtree.

The browser UI does not provide copy actions.

### HTML Renderer

`-o html` prints a static HTML document to stdout.

The static document embeds fetched schema data for the selected CRD or resource
and any JavaScript/CSS needed for folding, search, focus, keyboard navigation,
and details panes. It must not load external assets or send schema data to
external services.

`-o html` is optimized for one selected CRD/resource document, plus
`--all-versions` for that selected resource. It is not the unfiltered cluster
browser. In cluster mode with no selected resource, use `-o browser`/`-w` so the
localhost server can load schemas lazily from discovery navigation.

The generated HTML should be self-contained and embeddable. Scope CSS and
JavaScript under a kubectl-doc root element so the same output can be used as a
standalone static page, iframe target, or documentation fragment.

By default, HTML renders the latest served version. With `--all-versions`, it
renders every served version for the selected resource.

If `--path` is provided, static HTML uses the same semantics as browser mode:
render the full selected schema, expand and focus the requested field, show its
details, and scroll it into view on load.

### Man Renderer

`-o man` prints man source to stdout.

The output should be usable as:

```shell
kubectl doc -f crd.yaml -o man | man
```

The man renderer is non-interactive and should favor deterministic expansion and
compact details.

By default, man output renders the latest served version. With `--all-versions`,
it renders every served version for the selected resource.

### Markdown Renderers

Markdown output is one page/file per invocation and prints to stdout.

Supported dialects for the first version:

- `markdown`
- `markdown-github`
- `markdown-fern`

`markdown` aliases `markdown-github`.

Both dialects should render:

- Resource identity.
- Group/resource/version navigation summary when relevant.
- Fenced YAML block.
- Field details when requested with `--field-details`.
- Anchors for fields.
- Diagnostics.

By default, Markdown renders the latest served version. With `--all-versions`,
it renders every served version for the selected resource in the same page/file.
The page keeps a single resource title and metadata table, then emits one
version section per API version in latest-first order.

`markdown-github` should stay portable: fenced `yaml` code blocks, standard
tables, anchors, and optionally coarse `<details>/<summary>` sections. It should
not depend on JavaScript and should not pretend to support per-field fold icons
inside a syntax-highlighted YAML fence.

The first GitHub mapping is:

- H1 resource title.
- Metadata table with API version or all rendered versions.
- `## YAML` for a single rendered version.
- `## <apiVersion>` sections for `--all-versions`.
- `<details open><summary>YAML</summary>` or version-specific summaries around
  fenced YAML examples.
- Field detail headings with explicit `<a id="field-..."></a>` anchors and
  JSON-path-like field labels when `--field-details` is set.

Markdown renderers should wrap and reindent generated prose paragraphs to the
configured column width. That includes schema descriptions rendered as YAML
comments inside fenced examples. Preserve paragraph breaks and the YAML comment
indentation prefix while reflowing text.

`markdown-fern` emits Fern-compatible MDX. It should feel close to the current
selected-resource HTML output, but as static Fern documentation: no localhost
server and no dynamic discovery overview. The design path is to generate MDX
that uses Fern page components plus a Fern-compatible interactive schema
component fed by a deterministic schema payload. This is needed to support the
HTML-mode interaction contract: per-field fold/unfold, focus details, keyboard
navigation where practical, comment wrapping, and filtering.

The first Fern mapping is:

- Frontmatter with the resource kind as the title.
- H1 resource title and metadata table.
- `<Tabs>` and `<Tab>` around version sections when `--all-versions` renders
  multiple versions.
- `<AccordionGroup>` and `<Accordion>` sections only around secondary/static
  detail regions where they improve page shape. Do not wrap the primary
  interactive YAML schema tree in an accordion/disclosure.
- A generated schema component such as `KubeSchemaDoc` with an embedded
  structured payload.
- For large schemas, `--fern-schema-dir` embeds a shallow initial payload in the
  MDX page and writes complete static schema JSON sidecars. The shallow
  payload points at those files through `fullPayloadURL`, using
  `--fern-schema-url-path` when set.
- The Fern component hydrates the full payload on expand/filter immediately and
  through idle loading only when the schema component is visible or near the
  viewport. This keeps hidden version tabs and below-fold resources from
  eagerly fetching full payloads just because the MDX runtime mounted them.
- While filtering or expanding a shallow payload, the component shows a compact
  full-schema loading state until the generated static payload arrives or the
  load is known to be a no-op.
- Filtering enabled by default and disabled with `--disable-filtering`.
- Optional static `<ParamField>` field details with stable field anchors when
  `--field-details` is set.

A selected-resource Fern export is the first implementation target. A static API
group export is the second target: generate a resource index and one section per
resource in the selected API group, fetching schemas during generation rather
than in the browser. Prefer an explicit future flag such as `--api-group
<group>` over overloading resource selector syntax.

The reusable Fern component/runtime should be packaged separately from the
generated MDX page output. `markdown-fern` should emit the page and payload, not
a full Fern project.

The long-term implementation direction is a shared optimized web runtime used
by both standalone `-o html` and Fern MDX. The runtime owns the expensive DOM
work for folding, filtering, focus, details, wrapping, and loading. The Fern
React component should become a lifecycle adapter around that runtime rather
than independently rendering every schema line. See
`DESIGN_SHARED_WEB_RUNTIME.md`.

The reusable Fern component source belongs to this repository under
`fern/components/kubectl-doc`. Documentation projects such as Dynamo consume or
vendor that component instead of carrying their own kubectl-doc schema
renderer.

## Search Index

Build a search index from the document model, not from rendered text.

Indexed fields:

- JSONPath.
- Field name.
- Description.
- Type and format.
- Enum values.
- Constraint summaries.

Search results should return JSONPath references. Renderers can then focus or
highlight the corresponding visible node.

## Error Handling

Errors should be typed where useful:

```text
ErrClusterUnavailable
ErrOpenAPIV3Unavailable
ErrResourceAmbiguous
ErrResourceNotFound
ErrInvalidCRD
ErrMissingCRDSchema
ErrNonStructuralSchema
ErrUnsupportedSchema
```

CLI errors should include:

- What input failed.
- The source mode, cluster or CRD file.
- A suggested next action when obvious.

Ambiguous resource errors should list matching group/version/kind entries.

## Testing

Unit tests:

- Cobra option validation.
- Resource selector parsing.
- Discovery-to-navigation tree conversion.
- CRD decoding and version selection.
- OpenAPI v3 URL selection.
- Conversion into kubectl-doc Structural schema types.
- CRD adapter coverage from upstream Kubernetes CRD Structural into kubectl-doc
  Structural.
- Structural schema normalization.
- Recursion handling.
- Search indexing.

Golden tests:

- Overview YAML.
- Resource YAML.
- Markdown GitHub.
- Markdown Fern.
- Man source.
- Static HTML shape.

Integration-style tests with fixtures:

- Built-in resource OpenAPI v3 documents.
- CRDs with multiple versions.
- Non-structural or unsupported schema constructs.
- Defaults.
- Enums.
- Maps and arrays.
- Nullable fields.
- CEL validations.
- Kubernetes OpenAPI extensions.
- Ambiguous resources.

Renderer acceptance tests:

- Rendered/exported YAML parses.
- Required and optional fields are distinguishable.
- `status` follows the renderer-specific default.
- Browser and TUI search can find field names and descriptions.
- No renderer exposes a copy command or copy button.

## Issue Tracking

Use `bd` as the primary issue tracker for implementation work.

Expected workflow:

- Create implementation tasks with `bd create`.
- Use `bd list`, `bd ready`, `bd status`, and `bd show` to choose work.
- Use `bd dep` or `bd link` to model dependencies between design, renderer,
  schema, and CLI tasks.
- Use `bd note` or `bd comment` for implementation notes that should stay with
  the issue.
- Close completed work with `bd close`.

Project tasks should be represented in `bd`, not as ad hoc TODO files. GitHub
Issues can be added later as an integration or mirror if needed, but the local
source of truth is `bd`.

## Implementation Plan

1. Initialize the Go module, Cobra command, and option validation.
2. Implement CRD file mode first because it needs no cluster.
3. Add the internal Structural schema package copied from Kubernetes' CRD
   Structural shape.
4. Build the CRD adapter into the internal Structural model.
5. Build schema normalization and YAML renderer with golden tests.
6. Add cluster discovery and resource overview output.
7. Add resource resolution and OpenAPI v3 fetching.
8. Add Markdown GitHub renderer.
9. Add Markdown Fern renderer.
10. Add static HTML renderer for selected CRD/resource documents.
11. Add browser server wrapper with discovery navigation and lazy schema routes.
12. Add interactive Fern MDX renderer.
13. Add TUI renderer and shared search.

## Open Design Item

- Choose the output name and packaging shape for the interactive Fern MDX
  renderer.
