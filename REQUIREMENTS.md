# Kubernetes API Docs Generator Requirements

## Purpose

Build a `kubectl doc` plugin that turns Kubernetes OpenAPI schemas into a
foldable YAML tree. The tool should help users understand which resources are
available in a cluster and inspect valid YAML-shaped schema examples without
leaving the terminal, while also supporting browser and Markdown output for richer
documentation surfaces.

## Product Goals

- Show an overview of cluster resources grouped by API group and served versions.
- Render a selected resource schema as a YAML-shaped documentation view.
- Keep required fields visible and make optional or nested fields easy to expand.
- Surface descriptions, defaults, enum values, validation constraints, and
  Kubernetes-specific OpenAPI extensions close to the field they describe.
- Support live clusters and local CRD files.
- Use one documentation model that can render to terminal, interactive TUI,
  browser, and Markdown.

## Implementation Language

The tool must be written in Go.

Reasons:

- `kubectl doc` should behave like a native Kubernetes CLI extension.
- Go gives direct access to `client-go`, Kubernetes discovery, REST config
  loading, OpenAPI helpers, and CRD API types.
- A single static binary is practical for Krew and direct installation.
- The same codebase can serve terminal, TUI, Markdown, and static HTML renderers
  without requiring a runtime on the user's machine.

## Non-Goals

- The tool does not mutate cluster state.
- The tool is documentation-only. It must never become a resource creator,
  scaffolder, editor, or manifest application workflow.
- The tool does not validate manifests. It only documents constraints exposed by
  Kubernetes OpenAPI and CRD schemas.
- The tool does not support Kubernetes OpenAPI v2. OpenAPI v3 is mandatory.
- The tool does not support arbitrary OpenAPI v3 schemas. It only supports the
  Kubernetes structural schema subset.

## Command Line Interface

The executable name should be `kubectl-doc`, which Kubernetes invokes as:

```shell
kubectl doc
kubectl doc -o tui
kubectl doc deployments
kubectl doc deployments -o tui
kubectl doc deployments -i
kubectl doc deployments -o browser
kubectl doc deployments -w
kubectl doc -f ./crd.yaml --version v1
kubectl doc -f ./crd.yaml -o man | man
```

Required commands and flags:

- `kubectl doc`: show the resource overview for the current cluster.
- `kubectl doc -o tui`: show an interactive resource browser for the current
  cluster.
- `kubectl doc <resource>`: show the auto-selected version of a resource.
- Cluster resource selectors must follow normal `kubectl get` resource syntax,
  including short names and qualified forms such as `deployments.apps` and
  `deployments.v1.apps`. The explicit version form is
  `resource.version.group`; DNS-style groups work as `widgets.example.com` or
  `widgets.v1.example.com`. A selector such as `pods.v1` is not a core `v1`
  version selector; it means resource `pods` in group `v1`.
- `kubectl doc -f <path>`: read one or more local CRD YAML files and render docs
  for their served versions without requiring a cluster. A CRD defines one kind;
  version selection is the only required disambiguation for CRD files.
- `-o, --output <format>`: select an output renderer. Supported values are
  `yaml`, `kro`, `tui`, `man`, `browser`, `markdown`, `markdown-github`,
  `markdown-fern`, and `html`. The default is `yaml`.
- `-i, --interactive`: shortcut for `-o tui`.
- `-w, --web`: shortcut for `-o browser`. On macOS the tool should
  best-effort open the printed localhost URL in the default browser; if opening
  fails, browser mode continues to run and waits as usual.
- `--nocolor`: disable color in `-o yaml` output.
- `--version <version>`: select a specific served CRD version when reading a CRD
  manifest. Cluster mode uses the resource selector syntax instead.
- `--all-versions`: include all served versions for renderers that support
  documentation pages, namely `html`, `man`, `markdown`, `markdown-github`, and
  `markdown-fern`.
- `-p, --path <json-path>`: zoom into a specific schema field or sub-value and
  render that node as the root of the documentation view.
- `--expand-depth <n>`: initial fold expansion depth.
- `--descriptions=false|required|true`: control YAML description comments.
  The default is `true`. `required` renders descriptions only for required
  fields. `false` suppresses description comments.
- `--columns <n>`: target line width for Markdown paragraph reindent/wrapping.
  The default is the current terminal width when available, otherwise `80`.
- `--field-details`: include Markdown field detail sections with anchors.
  The default is `false`; generated YAML examples and description comments
  should be useful on their own.

The plugin must honor normal kubeconfig and context behavior. In Go, this points
toward using the Kubernetes CLI/client-go loading rules instead of inventing a
separate kubeconfig parser.

Renderer selection must be explicit. The tool should not auto-switch into TUI
mode based on terminal detection.

Resource selection behavior:

- Interactive modes, `-o tui` and `-o browser`, do not require a selected
  resource. Without a resource they start at the group/resource/version
  selection view.
- Non-interactive schema renderers require a selected resource when they are
  rendering a schema. With no selected resource, the default `-o yaml` output
  shows the resource overview instead of dumping every schema.
- Overview-capable non-interactive output should show only the resource overview
  when no resource is selected.
- YAML authoring output must include `metadata.namespace: "<namespace>"` for
  namespaced resources and omit it for cluster-scoped resources.
- In interactive modes, resource and version selection are both explicit UI
  choices. The tool should show all served versions and should not auto-apply the
  version heuristic.
- In non-interactive modes, when a selected resource has no explicit version,
  auto-select the latest served version. Stable versions win over beta versions,
  beta versions win over alpha versions, and the highest numeric version wins
  within the same stability tier. This applies to cluster resources and CRD
  files.
- `html`, `man`, `kro`, `markdown`, `markdown-github`, and `markdown-fern`
  default to the auto-selected latest version and support `--all-versions`.
- `yaml` renders one selected version only.
- `--path` applies after resource and version selection. It accepts a JSON Path
  into the selected schema and zooms all renderers to that field as their
  initial/root view. Interactive renderers should focus the selected path; static
  renderers should print only the selected subtree. If the path does not exist,
  the command must return a clear error.

## Input Sources

### Cluster Discovery

For cluster mode, the tool should use Kubernetes discovery APIs to build the
overview and map user resource names to group-version-kind tuples.

The resource overview should look like:

```yaml
core:
  pods: v1
apps:
  deployments: ["v1","v1beta1"]
  daemonsets: v1
```

The overview should:

- Group by API group, using `core` for the legacy core API group.
- Show plural resource names by default.
- Show all served versions. Single-version resources render the version as a
  scalar string. Multi-version resources render versions as a YAML flow sequence,
  ordered by the same latest-version rule used for selection: stable before
  beta before alpha, then higher numeric versions.
- Include kind, short names, namespaced/cluster scope, and verbs in details or
  hover panels.
- Sort deterministically, with `core` first and the remaining groups
  lexicographically.

### OpenAPI

The only supported cluster schema source is OpenAPI v3:

- Discover group-version OpenAPI documents from `/openapi/v3`.
- Always fetch the OpenAPI v3 schema for the resolved group-version from the
  returned `serverRelativeURL`.
- If a cluster advertises a built-in Kubernetes resource through discovery but
  omits its schema from the fetched OpenAPI v3 group-version document, fall back
  to an embedded upstream Kubernetes OpenAPI v3 document for the same
  group-version. This fallback is only for built-in resources; CRDs must come
  from their CRD schema or the cluster OpenAPI response.
- Do not cache cluster OpenAPI data in the first version. The per-group-version
  v3 documents are small enough for direct fetching.

Clusters without OpenAPI v3 support are unsupported.
OpenAPI v3 is a transport/source format for Kubernetes structural schemas, not a
general OpenAPI v3 input format.

### CRD Files

`kubectl doc -f <crd>` must support `apiextensions.k8s.io/v1`
`CustomResourceDefinition` files by reading:

- `spec.group`
- `spec.names.kind`
- `spec.names.plural`
- `spec.names.shortNames`
- `spec.scope`
- `spec.versions[*].name`
- `spec.versions[*].served`
- `spec.versions[*].schema.openAPIV3Schema`

If a CRD version is served but has no structural schema, the tool should report
that clearly and still show metadata from the CRD.

## Resource Resolution

Resource lookup must support:

- Plural resource names, for example `deployments`.
- Singular names, if discovery provides them.
- Kinds, for example `Deployment`.
- Short names, for example `deploy`.
- Fully qualified references for ambiguous matches.
- Kubernetes' dot syntax: `resource.group` for group-qualified lookup and
  `resource.version.group` for explicit version lookup. The group may contain
  dots. `pods.v1` is not core/v1; it is parsed as resource `pods` in group `v1`.

When a lookup is ambiguous, the tool must not guess silently. It should show the
matching resources and ask the user to choose a more explicit selector, for
example:

```text
deployments matches:
  apps/v1 Deployment
  custom.example.com/v1 Deployment
Use a fully qualified selector such as deployments.v1.apps.
```

## YAML Tree Rendering

The rendered document is a YAML-shaped documentation view. It should include
`apiVersion` and `kind` at the root when the selected schema represents a
Kubernetes resource.

Example terminal shape:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "<name>"
spec:
  # DeploymentSpec is the specification of the desired behavior.
  replicas: 1 # default, integer, minimum: 0
  selector:
    matchLabels: {}
  template:
    spec:
      containers:
        - name: "<string>"
          image: "<string>"
```

The rendered or selected text must be syntactically valid YAML. It does not need
to be a semantically valid Kubernetes manifest. This requirement means:

- Fold controls such as `▼` and `▶` are rendered in a non-selectable gutter,
  browser control, or TUI overlay whenever possible.
- If a renderer cannot keep controls outside the selected text, controls must be
  expressed as YAML comments, for example `# ▶ optionalField`.
- Placeholder values must be valid YAML values, for example `"<string>"`, `0`,
  `false`, `[]`, `{}`, or a valid default value from the schema.
- Field metadata belongs in comments, hover panels, side panes, or details
  sections; it must not make the YAML invalid.

Default visibility:

- Required fields are visible, unfolded to the initial expansion depth, and
  rendered as uncommented YAML.
- Optional fields are rendered below required fields as commented YAML and are
  folded by default.
- `status` is collapsed by default in interactive renderers.
- `status` is represented as a folded YAML comment in non-interactive renderers.
- Unknown additional properties are shown as map placeholders.
- Recursive references stop at a configurable depth and render a reference link
  instead of expanding forever.

## Field Metadata

Each schema node should preserve and render:

- Description.
- Required or optional status.
- Type and format, for example `string`, `integer/int32`, `object`, or
  `array`.
- Default value.
- Enum values.
- Numeric constraints such as minimum, maximum, exclusive bounds, and multiple.
- String constraints such as min length, max length, and pattern.
- Array constraints such as min items, max items, unique items, and item type.
- Object constraints such as additional properties and property names.
- Nullability.
- Deprecation.
- Examples, where available, including OpenAPI schema `example` and `examples`
  blobs.
- Validation rules, including Kubernetes CEL rules in
  `x-kubernetes-validations`.
- Kubernetes extensions such as `x-kubernetes-int-or-string`,
  `x-kubernetes-preserve-unknown-fields`, `x-kubernetes-embedded-resource`,
  `x-kubernetes-list-type`, `x-kubernetes-list-map-keys`, and
  `x-kubernetes-map-type`.

For enum fields, the visible value should prefer a valid default when present.
Otherwise, use the first enum value and show the complete enum set in metadata:

```yaml
strategy: RollingUpdate # enum: Recreate | RollingUpdate
```

In rich renderers, field comments should stay compact and full metadata should be
available on hover, focus, or a side panel.

## Field Visualization

Every field type needs a sensible visualization for every renderer. The
visualization must preserve syntactically valid YAML, even when placeholder
values are not semantically valid Kubernetes values.

General rules:

- If the schema provides a default value, render that value directly.
- If the schema provides no default but has a field-local OpenAPI example,
  render the example value directly when it can be represented as valid YAML.
  Annotate the field with compact metadata such as `# example string`,
  `# example object`, or `# example array`; named examples may include the
  example name after the type.
- If the schema provides enum values and no default or example is selected,
  render one enum value and list the other allowed values in a YAML comment.
- Simple validation constraints, such as `minLength`, `maxLength`, `minimum`,
  `maximum`, and `pattern`, may be shown directly in a compact YAML comment.
- Rich renderers should expose the same constraints in the focused details pane
  or hover/focus UI.
- Field-specific examples should be preferred over generic placeholders when
  they are available and compact.
- Scalars should use compact placeholders when no default or example is present.
- Objects should render their required children first, followed by folded or
  commented optional children.
- Description comments should render directly before the field they describe,
  at the same indentation as that field. In YAML output, sibling field blocks
  should be separated by an empty line when descriptions or nested blocks are
  present.
- Required fields must not be hidden behind YAML comments. If an optional parent
  contains required descendants, the parent path must remain live YAML so the
  required descendants are valid YAML too. Such live optional parents should be
  marked inline with `# optional`.
- Arrays should render one representative item when no default or example is
  present.
- Maps should render one representative `<key>` entry when no default or example
  is present.
- Nullable fields should document nullability in comments and details. They do
  not need to render `null` unless `null` is the default.
- OpenAPI composition quantors such as `oneOf`, `anyOf`, and multi-branch
  `allOf` should not be evaluated, merged, or rendered as schema structure.
  Supported Kubernetes-native special cases are int-or-string detection via
  `x-kubernetes-int-or-string` or generated native `format: int-or-string`, and
  generated single-ref `allOf` wrappers used to attach field metadata to a
  referenced schema.
- Static YAML output should annotate collapsed object or array-item nodes with
  the minimum `--expand-depth` value needed to open that node, for example
  `podTemplate: {} # show with --expand-depth 4`.

Placeholder examples:

```yaml
someString: "<string>"
someInt32: <int32>
someBoolean: <boolean>
someExample: "prod" # example string
someObjectExample: {"mode":"active"} # example object primary
someEnum: BoldDefault # enum: Foo | Bar
someConstrainedString: "<string>" # minLength: 3, maxLength: 63
someList:
  - name: "<string>"
    value: "<string>"
someMap:
  <key>: "<value>"
```

Lists without defaults should sketch one representative item below the list key.
Maps without defaults should sketch one representative key/value entry.

## Folding Behavior

Foldable nodes include:

- Objects.
- Arrays of objects.
- Maps.
- Optional scalar fields with long descriptions or many constraints.

Browser and HTML mode:

- `-o browser` starts a localhost server and prints the interactive browser URL.
  `-w`/`--web` is a shortcut for this mode. On macOS the server URL is opened
  in the default browser on a best-effort basis; failures are ignored.
- The localhost server binds to localhost on a random available port by using
  port `0`.
- The localhost server fetches OpenAPI from the cluster using the user's
  kubeconfig context and serves the browser UI.
- Browser mode must work without a selected resource. In that mode it serves a
  lightweight discovery/resource/version navigation page first and fetches the
  selected group-version OpenAPI schema lazily when the user chooses a resource
  version. It must not pre-render every cluster schema into one giant page.
- Browser mode has no quit key. The user closes the browser tab manually and uses
  Ctrl-C to stop the `kubectl doc` process.
- `-o html` prints a static interactive document to stdout for the selected CRD
  or resource. It is optimized for embedding one selected schema document, not
  for unfiltered cluster browsing.
- The browser/HTML interaction model should mirror `-o tui` as closely as
  practical.
- A navigation pane shows the group, resource, and version tree.
- The main pane shows the foldable YAML tree for the selected resource version.
- The metadata tree is generated fully in interactive backends and starts
  collapsed, including standard Kubernetes `ObjectMeta` fields for CRDs whose
  OpenAPI schema does not explicitly include them.
- A details pane shows all details for the currently focused field, including
  descriptions, defaults, enum values, constraints, and Kubernetes OpenAPI
  extensions.
- `▶` expands and `▼` collapses nodes by click.
- Keyboard navigation follows TUI semantics.
- Hover and focus show descriptions and constraints.

Interactive terminal mode:

- `-o tui` opens a split-screen interface.
- `kubectl doc -o tui` starts in resource browser mode.
- A navigation menu shows the cluster's group, resource, and version tree.
- Selecting a version loads its OpenAPI schema and shows the foldable YAML tree.
- When a resource is supplied, the navigation menu is still visible and the
  selected resource version is focused.
- The upper area shows the navigation menu and foldable YAML tree.
- The lower horizontal pane shows all details for the currently focused field,
  including descriptions, defaults, enum values, constraints, and Kubernetes
  OpenAPI extensions.
- The cursor is always focused on one JSON Path in the schema. Interactive modes
  may show the focused JSON Path.
- Up and Down move the focus by visible field.
- Enter toggles fold state.
- Left moves focus to the parent field.
- Right moves focus to the first child field, sub-item, or sub-value.
- `q` and F10 exit.

Interactive search:

- `/` enters search mode and searches field names and descriptions.
- `//` enters field-only search mode.
- Esc leaves search mode.
- `n`, `p`, Up, and Down move across search results while in search mode.
- Search matches must be highlighted clearly in a strong orange color. The
  focused match must also be distinguishable without relying on color alone.

YAML output:

- `-o yaml` is the default renderer.
- Output is static, valid YAML printed to stdout.
- When the terminal supports it, YAML output should use color and text styling for
  documentation affordances such as comments, required fields, defaults, and enum
  values.
- `--nocolor` disables color and text styling.
- `--descriptions` controls schema description comments in YAML output:
  `true` shows all field descriptions, `required` shows only required field
  descriptions, and `false` hides them.
- Provide deterministic static expansion with `--expand-depth`.
- Collapsed nodes should include an inline hint for the next `--expand-depth`
  value that would expand them.

Kro output:

- `-o kro` prints a Kro SimpleSchema-style YAML schema view to stdout.
- The output is documentation, not a Kubernetes manifest. It is intended to show
  types and constraints concisely with Kro-like marker syntax such as
  `required=true`, `default=...`, `enum=...`, `minimum=...`, `minLength=...`,
  and `description=...`.
- `-o kro` defaults to the auto-selected latest version and supports
  `--all-versions`.
- Descriptions are controlled by `--descriptions`, rendered as SimpleSchema
  `description="..."` markers where possible.
- Arrays and maps should use Kro type expressions such as `"[]integer"` and
  `"map[string]string"`. Arrays of structured objects should render a nested
  representative list item instead of a generated custom type reference.
- Kubernetes-specific extensions that do not have Kro SimpleSchema markers may
  be preserved as compact comments.

Man output:

- `-o man` outputs man source.
- The output is intended to be pipeable into `man`, for example
  `kubectl doc -f crd.yaml -o man | man`.
- The man renderer must remain useful without interactivity.
- The man renderer defaults to the auto-selected latest version and supports
  `--all-versions`.

Markdown output:

- `-o markdown` defaults to GitHub Markdown and is equivalent to
  `-o markdown-github`.
- `markdown-github` and `markdown-fern` are the only Markdown dialects required
  for the first version. Additional dialects are out of scope for now.
- Markdown output is intended for reuse in documentation systems.
- Markdown output is one page/file per invocation. It is filtered by the same
  flags and resource, group, and version selectors used by the command.
- Markdown renderers default to the auto-selected latest version and support
  `--all-versions`.
- With `--all-versions`, Markdown output renders one resource page with a
  metadata table listing all rendered versions in latest-first order and one
  section per API version.
- Each Markdown dialect should use the most sensible features supported by that
  target: GitHub-flavored Markdown for `markdown-github` and Fern-compatible
  Markdown for `markdown-fern`.
- Markdown renderers must not require JavaScript to be useful.
- Dialects may use headings, comments, fenced YAML blocks, reference tables,
  anchors, and dialect-supported disclosure/details constructs.
- Markdown output can include a field details section with stable anchors for
  every schema field when `--field-details` is set. Field details should include
  at least type, required status, description, and compact metadata for
  defaults, examples, enums, validations, and Kubernetes extensions.
- Markdown renderers must reindent and wrap generated prose paragraphs,
  including YAML description comments inside fenced schema examples, to the
  configured `--columns` width.
- Pure Markdown dialects are not expected to provide per-field fold controls
  inside syntax-highlighted YAML. They should prefer portable highlighted YAML
  and coarse disclosure where the target supports it.
- `markdown-github` should use standard tables, fenced `yaml` blocks, stable
  anchors, and coarse `<details>/<summary>` wrappers for schema sections.
- `markdown-fern` should emit Fern-compatible MDX, including frontmatter and
  coarse accordions or code-block attributes where those improve documentation
  reuse.
- `markdown-fern` may emit MDX that uses Fern-supported components such as
  accordions, tooltips, tabs, code-block attributes, and custom components when
  those features improve documentation reuse without making the page depend on
  kubectl-doc JavaScript.
- An interactive Fern variant is in scope as a Fern-specific renderer target.
  It should use Fern MDX/custom component integration rather than the generic
  static HTML runtime when that gives better docs-site integration.

HTML constraints:

- Should be generated from the same documentation model as Markdown.
- `-o html` must print a static HTML document with the fetched schema data
  embedded to stdout.
- `-o html` requires a selected resource in cluster mode. It should be used for a
  selected resource or CRD file, while unfiltered cluster exploration belongs to
  `-o browser`/`-w`.
- HTML defaults to the auto-selected latest version and supports
  `--all-versions`.
- May include embedded JavaScript and CSS for folding, search, focus, keyboard
  navigation, and details panes.
- HTML should be suitable for documentation embedding: self-contained, scoped to
  a root element, and usable as either a standalone page or embedded fragment.
- Must not load external assets or send schema data to external services.

## Markdown-Based Document Model

All renderers should consume the same intermediate document model:

- Overview sections.
- Group/resource/version navigation tree.
- Resource identity.
- YAML tree.
- Field metadata table.
- Cross-reference index.
- Diagnostics and source information.

This keeps browser, terminal, man, and Markdown outputs consistent and makes
golden testing practical.

The implementation may render Markdown first and then transform it for terminal
or HTML, but the YAML view itself should remain a structured tree until the last
rendering step. That avoids string-based schema manipulation.

## Error Handling

The tool should produce actionable errors for:

- Cluster unreachable.
- Authentication or authorization failure.
- Discovery unavailable.
- OpenAPI unavailable.
- Resource lookup ambiguity.
- Missing CRD schema.
- Invalid YAML or invalid CRD input.
- Unsupported or recursive schema constructs.

Errors should include the failing input source and a suggested next action when
there is one.

## Security And Privacy

- Never mutate cluster resources.
- Do not execute code from schemas or descriptions.
- Escape schema descriptions in browser output.
- Do not phone home or load external browser assets by default.
- Avoid putting credentials or bearer tokens in logs, generated HTML, or crash
  reports.

## Accessibility

Browser output must support:

- Keyboard navigation.
- Focus-visible fold controls.
- Screen-reader labels for fold state and field metadata.
- Sufficient color contrast.
- Usable output without hover-only information.

Terminal output must not rely on color alone to communicate required/optional
status or validation severity.

## Testing Requirements

Required test coverage:

- Golden tests for overview output.
- Golden tests for resource YAML trees.
- YAML parse tests for rendered/exported YAML output.
- OpenAPI v3 fixtures for built-in resources.
- CRD fixtures with multiple versions, enum/defaults, maps, lists, nullable
  fields, CEL validations, and Kubernetes extensions.
- Ambiguous resource lookup tests.
- Recursive reference handling tests.
- Browser rendering tests for fold state, search, navigation, and details panes.
- TUI navigation tests for group, resource, and version selection.
- Terminal renderer snapshot tests, including no-color mode.

Acceptance checks for every renderer:

- The output identifies the source cluster or CRD file.
- The selected resource identity is visible.
- Required fields are distinguishable from optional fields.
- Metadata is available without breaking YAML validity.
- Exported YAML parses successfully.

## References

- Kubernetes kubectl plugin documentation:
  https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/
- Kubernetes API overview:
  https://kubernetes.io/docs/reference/using-api/
- Kubernetes discovery and OpenAPI behavior:
  https://kubernetes.io/docs/concepts/overview/kubernetes-api/
- Fern documentation platform:
  https://buildwithfern.com/

## Open Design Questions

- What output name and packaging shape should the interactive Fern MDX renderer
  use?
