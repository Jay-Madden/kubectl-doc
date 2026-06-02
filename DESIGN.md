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
  -> Kubernetes structural schema loading
  -> structural schema normalization
  -> documentation model
  -> renderer
```

The central rule is that schema interpretation happens once, before rendering.
Renderers consume the same documentation model and should not parse OpenAPI,
structural schema, or CRD YAML directly.

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
- `internal/schema`: Kubernetes structural schema normalization and reference
  resolution.
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
-o, --output <format>       yaml|tui|man|browser|markdown|markdown-github|markdown-fern|html
    --web                   shortcut for -o browser
    --nocolor               disable styling in -o yaml
    --version <version>     served CRD version selector
    --expand-depth <n>      initial static expansion depth
```

Implementation notes:

- Define a `cli.Options` struct and bind all Cobra flags into it.
- Normalize `--web` to `OutputBrowser` after parsing.
- Default output is `yaml`.
- Use Cobra validation for positional argument count and flag combinations.
- Use Kubernetes CLI/client-go config loading rules for kubeconfig behavior.
- Add Kubernetes config flags through the standard Kubernetes CLI machinery
  rather than reimplementing kubeconfig parsing.

Mode validation:

- No `-f`: cluster mode. A positional resource selector is optional.
- `-f`: CRD file mode. Cluster discovery is not required.
- `--version` is valid only with `-f`.
- `--web` conflicts with an explicit `-o` value other than `browser`.
- `-o html` and `-o yaml` write to stdout.
- `-o browser` starts a local server and owns the process until Ctrl-C.

## Data Sources

### Cluster Mode

Cluster mode needs discovery before OpenAPI:

1. Build a REST config from kubeconfig and flags.
2. Fetch API groups and resources through discovery.
3. Build the group/resource/version navigation tree.
4. Resolve the optional resource selector using normal `kubectl get`-style
   syntax.
5. Fetch the OpenAPI v3 document for the resolved group-version only.
6. Build docs for either the selected resource or the overview.

OpenAPI v3 is mandatory. Do not call `/openapi/v2`, and do not add a v2 fallback.
OpenAPI v3 is a transport/source format here, not the supported schema language.
Only Kubernetes structural schemas from the fetched document are supported.

OpenAPI v3 fetching:

- Read `/openapi/v3`.
- Find the selected group-version entry.
- Fetch its `serverRelativeURL`.
- Do not cache the fetched schema in the first version.

The overview command does not need to fetch every OpenAPI document. It can render
from discovery alone until the user selects a resource in TUI/browser mode.

### CRD File Mode

CRD mode reads local `apiextensions.k8s.io/v1` CRDs:

1. Decode one or more YAML documents from each `-f` path.
2. Keep only `CustomResourceDefinition` objects.
3. Read names, group, scope, and served versions.
4. Select the requested `--version`, or choose the default served/storage version
   when there is only one reasonable choice.
5. Convert `spec.versions[*].schema.openAPIV3Schema` into Kubernetes'
   structural schema representation, then into the same normalized
   documentation model used for cluster OpenAPI.

A CRD defines one kind. Do not add a `--kind` flag unless requirements change.

## Structural Schema Scope

Use upstream Kubernetes structural schema classes and helpers as the source of
truth for supported schema behavior. The design target is:

```text
k8s.io/apiextensions-apiserver/pkg/apiserver/schema.Structural
k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural
```

The exact imports can be adjusted during implementation if Kubernetes moves
helpers, but the rule stays the same: prefer upstream Kubernetes structural
schema types over custom OpenAPI interpretation.

Supported:

- Structural object, array, map, and scalar schemas.
- Required fields.
- Defaults, enum values, nullable markers, examples, and validations exposed by
  the structural schema.
- Kubernetes structural extensions such as list type, list map keys, map type,
  embedded resource, preserve unknown fields, and int-or-string.
- `x-kubernetes-validations` as documentation.

Unsupported:

- Arbitrary OpenAPI v3 documents outside Kubernetes structural schema rules.
- OpenAPI features that cannot appear in structural Kubernetes schemas.
- Schema behavior that would require evaluating arbitrary `oneOf` or `anyOf`
  branches.

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
```

Resolution should use Kubernetes discovery and REST mapping behavior where
possible. Ambiguity is an error with a list of matches. The resolver must not
silently choose between multiple group/version/kind matches.

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
Kubernetes structural schema, not arbitrary OpenAPI v3.

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
- Composition metadata for `oneOf`, `anyOf`, and `allOf`.
- Recursive-reference marker.

Cluster OpenAPI adapters and CRD adapters both feed the same structural schema
normalizer:

```text
cluster OpenAPI v3 schema -> structural schema -> FieldNode tree
CRD openAPIV3Schema       -> structural schema -> FieldNode tree
```

References should be resolved into nodes until the recursion limit is reached.
After that, render a reference marker instead of expanding forever.

`oneOf` and `anyOf` are documentation-only. Do not evaluate alternatives or try
to choose a valid branch. Render comments and details that explain alternatives
exist while keeping the YAML syntactically valid.

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
- Enums render the default if present; otherwise render one enum value and put
  alternatives in a comment.
- Required fields are uncommented.
- Optional fields are commented and folded by default.
- `status` is collapsed in TUI/browser and rendered as a folded comment in
  non-interactive outputs.
- Lists without defaults render one representative item.
- Maps without defaults render one representative `<key>` entry.
- Nullable fields document nullability in comments/details, not by rendering
  `null` unless `null` is the default.

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
- Apply `--expand-depth`.
- Comment optional fields.
- Represent folded nodes as comments where controls cannot live outside text.
- Add compact metadata comments for defaults, enum alternatives, and simple
  constraints.
- Style output when terminal capabilities support it and `--nocolor` is false.

Color is a presentation layer. The underlying bytes without ANSI sequences must
still parse as YAML.

### TUI Renderer

`-o tui` starts an interactive terminal UI.

Layout:

```text
left/top:  group/resource/version navigation
right/top: foldable YAML tree
bottom:    details pane for focused field
```

Focus model:

- The cursor is always on one JSONPath in the schema.
- Up and Down move by visible field.
- Enter toggles fold state.
- Left moves to the parent field.
- Right moves to the first child, sub-item, or sub-value.
- `q` and F10 exit.

Search:

- `/` searches field names and descriptions.
- `//` searches field names only.
- Esc exits search mode.
- `n`, `p`, Up, and Down move between results.
- Matches are highlighted in strong orange.
- The focused match has an additional non-color marker.

The TUI does not provide a copy command.

### Browser Renderer

`-o browser` starts a localhost server and opens a browser.

Server behavior:

- Bind to localhost with port `0`.
- Print or log the chosen local URL.
- Fetch OpenAPI using the same kubeconfig context as the CLI.
- Keep running until the user sends Ctrl-C.
- Do not define browser quit shortcuts.

The browser UI mirrors the TUI:

- Navigation tree.
- Foldable YAML tree.
- Details pane.
- JSONPath focus.
- Same search semantics.
- Mouse support for fold controls.

The browser UI does not provide copy actions.

### HTML Renderer

`-o html` prints a static HTML document to stdout.

The static document embeds fetched schema data and any JavaScript/CSS needed for
folding, search, focus, keyboard navigation, and details panes. It must not load
external assets or send schema data to external services.

### Man Renderer

`-o man` prints man source to stdout.

The output should be usable as:

```shell
kubectl doc -f crd.yaml -o man | man
```

The man renderer is non-interactive and should favor deterministic expansion and
compact details.

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
- Field details.
- Anchors for fields.
- Diagnostics.

The remaining design work is the exact feature mapping for GitHub vs Fern
Markdown. Until then, keep the renderer interface dialect-aware so the mapping
can evolve without changing schema normalization.

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
- Conversion into upstream Kubernetes structural schema types.
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
3. Build structural schema normalization and YAML renderer with golden tests.
4. Add cluster discovery and OpenAPI v3 fetching.
5. Add resource resolution and overview output.
6. Add Markdown GitHub renderer.
7. Add Markdown Fern renderer.
8. Add static HTML renderer.
9. Add browser server wrapper around the HTML renderer.
10. Add TUI renderer and shared search.

## Open Design Item

- Exact feature mapping for `markdown-github` and `markdown-fern`.
