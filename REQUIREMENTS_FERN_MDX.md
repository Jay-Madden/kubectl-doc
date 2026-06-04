# Fern MDX Renderer Requirements

## Purpose

Fern output should produce documentation that can be dropped into a Fern docs
site as MDX. It should feel close to kubectl-doc's current HTML schema view:
foldable YAML, focusable fields, keyboard navigation, details for the focused
field, comment wrapping, and field filtering. The difference is that Fern output
is static documentation, not a localhost server and not a realtime discovery UI.

The concrete product target is to replace brittle generated Kubernetes API
reference pages such as NVIDIA Dynamo's Kubernetes Deployment API Reference. If
that page 404s or renders only a poor YAML-style API reference, kubectl-doc's
Fern output should be the replacement users can publish as the real product API
reference.

The goal is to generate Fern-native MDX plus a Fern-compatible schema component
integration. The component should use the same structured schema/tree payload as
the HTML renderer, but it must not fetch OpenAPI after page load.

## Renderer Names

The first Fern renderer is:

- `-o markdown-fern`

`markdown-fern` emits Fern-compatible MDX to stdout. It is the Fern dialect of
the Markdown renderer, but unlike GitHub Markdown it is expected to use a
Fern-compatible interactive schema component.

Component packaging:

- `markdown-fern` remains the output name for generated MDX pages.
- The reusable Fern component/runtime should be packaged separately from the
  generated page output.
- The generated page should import or reference a stable component such as
  `KubeSchemaDoc` or `KubeSchemaTree`.
- The exact import path is an implementation detail to settle before coding,
  but it must be configurable or documented well enough for real Fern projects.

## Non-Goals

- Do not generate copy buttons or copy commands.
- Do not generate a Fern project, full `docs.yml`, navigation tree, or asset
  folder in the first Fern path.
- Do not embed kubectl-doc's generic static HTML runtime verbatim; use a
  Fern-compatible component/runtime.
- Do not depend on a running kubectl-doc process, cluster credentials, or
  localhost server after the page has been generated.
- Do not provide a dynamic cluster overview or realtime schema fetching in Fern
  output. Live exploration remains `-w`/`-o browser`.
- Do not implement filtering by parsing rendered YAML strings. Filtering must
  use the embedded structured schema payload.

## Source And Shape

`markdown-fern` follows the general Markdown renderer contract for selected
resources:

- One page/file per invocation.
- Prints to stdout.
- Defaults to the latest selected version.
- Supports `--all-versions`.
- Honors `--descriptions`, `--expand-depth`, `--columns`, and
  `--field-details`.
- Honors `--disable-filtering`.
- Uses the shared documentation model and YAML tree.
- Does not parse rendered YAML to recover schema metadata.

The emitted file should be valid MDX suitable for a Fern page under the user's
`fern/pages` tree.

## Export Scopes

Fern output supports two static scopes.

Selected resource export:

- This is the default `markdown-fern` behavior.
- It renders one Kubernetes resource kind and one or more served versions.
- It should be visually close to `-o html` for a selected resource, using Fern
  components instead of kubectl-doc JavaScript.
- It is suitable for a Fern page such as
  `fern/pages/reference/apps/deployment.mdx`.

API group export:

- This is a future extension of `markdown-fern`.
- It renders all selected resources in one API group into a sensible static MDX
  page or generated page set.
- It must not behave like `-w`; it does not fetch schemas after page load and it
  does not show a dynamic realtime cluster overview.
- It may fetch all resource schemas for the selected group during generation.
- It should present a compact API group index at the top, then one section per
  resource.
- Each resource section should use the same selected-resource layout: identity,
  version tabs, YAML accordion, and optional field details.
- Large API groups may need a later split-page mode, but the first design should
  start with a single static group page because `markdown-fern` writes stdout.

Open CLI design for API group export:

- Prefer an explicit flag over overloading ambiguous resource selectors, for
  example `--api-group <group>`.
- `core` should select the legacy core API group.
- Group export is cluster-only; CRD file input already has an implicit resource.
- `--all-versions` controls whether each resource renders all served versions or
  only the latest served version.

## Interactivity

Fern output must preserve the HTML renderer's useful interactivity for the
selected static schema data.

Required interactions:

- Per-field fold and unfold.
- Focus one logical field/path at a time.
- Details for the focused field.
- Keyboard navigation matching the HTML view where practical.
- Semantic comment wrapping.
- Filtering by plain typing, enabled by default.
- Filtering over field names and descriptions.
- Filtering over the full logical schema tree, including collapsed descendants.
- Highlighting matched text in the main schema view.
- Native browser find remains separate from filtering.

Filtering is opt-out:

- Add `--disable-filtering`.
- For `markdown-fern`, filtering is enabled unless `--disable-filtering` is set.
- When filtering is disabled, generated MDX should not show filter UI and the
  component payload may omit filter indexes if that keeps output smaller.
- Folding, focus, details, and version/resource navigation remain available when
  filtering is disabled.

The generated page must embed all schema data needed for these interactions. It
must not fetch OpenAPI or resource schemas after the Fern page loads.

## Fern Components

Use Fern built-ins for page-level structure and a custom Fern-compatible schema
component for the interactive tree.

Required first mapping:

- Frontmatter with `title`.
- H1 resource title.
- Metadata table for API version, kind, resource, and rendered versions.
- For API group export, a top-level group title and resource index.
- `<Tabs>` and `<Tab>` for `--all-versions` when multiple versions are rendered.
- `<AccordionGroup>` and `<Accordion>` for coarse sections such as YAML and
  resource sections when they improve the page layout.
- A generated schema component invocation, for example:
  `<KubeSchemaDoc data={...} filtering={true} />`.
- Fenced YAML code blocks may still be emitted as fallback/static examples, but
  the primary interactive tree should be component-driven.
- `<ParamField>` may be used for non-interactive field detail fallback, but it
  is not enough for the primary interactive tree.

Acceptable supporting components:

- `<Badge>` for required/optional/default/deprecated labels if it improves
  readability.
- `<Callout>` or Fern's note component for diagnostics.
- `<Tooltip>` only for compact metadata that remains understandable without
  hover.

## YAML Section

The YAML-shaped tree remains the primary documentation artifact.

Requirements:

- Render an interactive YAML-shaped schema tree through the Fern schema
  component.
- Preserve copy-valid YAML semantics as far as Fern/React rendering allows.
- Keep YAML comments and inline metadata as in other Markdown dialects.
- Enable `wordWrap` so long comments are readable in documentation pages.
- Disable line numbers for fallback code blocks unless a future flag explicitly
  asks for them.
- Wrap the YAML section in an open accordion by default when a surrounding
  accordion improves the page layout.
- For `--all-versions`, put each version's YAML in its own version tab or
  version-labeled accordion.

Example shape:

````mdx
import { KubeSchemaDoc } from "@/components/kubectl-doc/KubeSchemaDoc";

<KubeSchemaDoc data={deploymentSchema} filtering={true} />
````

## Field Details

In `markdown-fern`, field details are part of the interactive schema component,
matching the HTML details pane. `--field-details` controls whether an additional
static field detail section is emitted below the interactive tree.

Requirements:

- Default remains no additional static field details.
- The interactive component always has focused-field details.
- Use `<ParamField>` for each documented field when additional static field
  details are enabled.
- `path` is the field's JSON-path-like path.
- `type` is the normalized field type, including format when useful.
- `required={true}` is emitted for required fields.
- `default="..."` is emitted when the default can be represented as a string.
- `deprecated={true}` is emitted for deprecated fields.
- Enum values may be rendered as a union-style `type`, for example
  `"\"Prod\" | \"Dev\" | \"Stage\""`, when that remains readable.
- Additional validations and Kubernetes extensions are rendered inside the
  component body as compact Markdown bullets or prose.
- Field descriptions are rendered as the component body and wrapped to
  `--columns`.

Example shape:

```mdx
<ParamField path="spec.replicas" type="integer/int32" default="1">
  Desired number of pod replicas.

  - minimum: 0
</ParamField>
```

Large schemas should group field details by top-level path, such as `metadata`,
`spec`, and `status`, inside coarse accordions. Do not emit one accordion per
field by default.

## Versioned Output

For `--all-versions`, prefer tabs:

```mdx
<Tabs>
  <Tab title="apps/v1">
    ...
  </Tab>
  <Tab title="apps/v1beta1">
    ...
  </Tab>
</Tabs>
```

Requirements:

- Versions are ordered latest-first using the shared version ordering rule.
- Each tab contains that version's YAML and optional field details.
- The page-level metadata table lists all rendered versions.
- Anchors remain stable and include version disambiguation when necessary.

## Escaping

Fern output is MDX, so values must be escaped for both Markdown and JSX contexts.

Requirements:

- Escape JSX attribute values.
- Escape MDX text where raw `{`, `<`, and `>` would be interpreted as JSX.
- Preserve YAML fenced block content exactly except for Markdown fence safety.
- Avoid generated raw HTML unless it is intentionally portable and tested.

## Component Payload

The schema component payload should be generated from structured metadata.

Payload requirements:

- Resource identity.
- Version identity.
- YAML/tree lines with path, depth, foldability, initial collapsed state, and
  syntax spans or enough metadata to compute syntax spans.
- Field details keyed by path.
- Required/default/example/enum/validation metadata.
- Filtering index, unless `--disable-filtering` is set.
- Stable anchors for fields and resource sections.

Payload shape should be deterministic and golden-testable. Prefer JSON embedded
in the MDX page or imported from a generated adjacent data file only if that
packaging mode is explicitly designed later.

## Tests

Tests must cover:

- Frontmatter title.
- Metadata table.
- Single-version YAML accordion.
- `--all-versions` tabs.
- Interactive schema component invocation.
- `--disable-filtering` disabling generated filtering support.
- Field details emitted as `<ParamField>` when enabled.
- No field details by default.
- JSX/MDX escaping for descriptions, enum values, defaults, and paths.
- Stable anchors and version disambiguation.
- Static API group export shape, once implemented.
- No dynamic OpenAPI fetching or realtime cluster overview.

## References

- Fern Accordion component:
  https://buildwithfern.com/learn/docs/writing-content/components/accordions
- Fern Tabs component:
  https://buildwithfern.com/learn/docs/writing-content/components/tabs
- Fern Code block component:
  https://buildwithfern.com/learn/docs/writing-content/components/code-blocks
- Fern ParamField component:
  https://buildwithfern.com/learn/docs/writing-content/components/parameter-fields
- Fern custom React components:
  https://buildwithfern.com/learn/docs/content/custom-react-components
- NVIDIA Dynamo Kubernetes Deployment API Reference target:
  https://docs.nvidia.com/dynamo/dev/kubernetes-deployment/api-reference
