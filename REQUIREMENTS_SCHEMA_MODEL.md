# Schema Model Requirements

## Purpose

The schema model is the internal Kubernetes-specific representation consumed by
all renderers. It exists so renderers can be consistent and can rely on
structured metadata instead of parsing OpenAPI or rendered YAML.

The model is structural-schema-like, copied from the Kubernetes CRD structural
schema shape where useful, but owned by `kubectl-doc` and extendable for native
Kubernetes resource cases.

## Supported Input Language

Supported:

- Kubernetes OpenAPI v3 documents from clusters.
- `apiextensions.k8s.io/v1` CRD schemas.
- Kubernetes structural schema features.
- Native Kubernetes OpenAPI extensions and generated-schema patterns needed to
  document built-in resources.

Unsupported:

- Kubernetes OpenAPI v2.
- Arbitrary OpenAPI v3 documents.
- General OpenAPI authoring features outside Kubernetes structural schema
  requirements.
- Using backend types that make the core model depend on CRDs.

OpenAPI v3 is an input transport format, not the renderer contract.

## Ownership

The model must be owned by this project.

Requirements:

- Start from the upstream Kubernetes CRD Structural shape where it is a good
  match.
- Do not make the backend depend directly on CRD-only structural types.
- Extend the project-owned model when native resource schemas expose cases that
  CRD structural schemas cannot represent.
- Keep extensions minimal and driven by observed Kubernetes native resource
  needs.
- After the initial implementation, pass through native Kubernetes resources and
  add model variants only where real schemas require them.

## Core Schema Metadata

Each field should preserve:

- Name.
- Path.
- Description.
- Type.
- Format.
- Required status.
- Default.
- Examples.
- Enum values.
- Nullable status.
- Deprecated status.
- Object properties.
- Array item schema.
- Map/additional-properties schema.
- References and resolved reference identity.

Validation metadata:

- Numeric minimum and maximum.
- Exclusive minimum and maximum.
- Multiple-of.
- String minLength and maxLength.
- String pattern.
- Array minItems and maxItems.
- Array uniqueItems.
- Object minProperties and maxProperties.
- CEL validation rules.

Kubernetes extension metadata:

- `x-kubernetes-int-or-string`
- `x-kubernetes-preserve-unknown-fields`
- `x-kubernetes-embedded-resource`
- `x-kubernetes-list-type`
- `x-kubernetes-list-map-keys`
- `x-kubernetes-map-type`
- `x-kubernetes-validations`

## Composition

OpenAPI composition is intentionally limited.

Requirements:

- Do not evaluate general `oneOf`.
- Do not evaluate general `anyOf`.
- Do not merge arbitrary multi-branch `allOf`.
- Detect Kubernetes int-or-string schemas, including the `anyOf` shape used by
  Kubernetes where applicable.
- Support generated single-reference `allOf` wrappers when they are used only to
  attach field metadata to a referenced schema.
- Preserve unsupported composition as diagnostics or compact metadata rather
  than pretending it was fully understood.

## References

Reference handling requirements:

- Resolve references needed to render selected schemas.
- Preserve enough reference identity to avoid infinite recursion.
- Stop recursive expansion at a deterministic depth and render a reference hint.
- Native Kubernetes embedded object schemas must be handled without depending on
  CRD-only code paths.
- Missing references produce clear diagnostics.

## Kubernetes Resource Normalization

The normalized document model for a Kubernetes resource includes:

- `apiVersion`
- `kind`
- `metadata`
- `spec` when present
- `status` when present

Requirements:

- `apiVersion`, `kind`, and `metadata` are generated or normalized for all
  resources.
- CRDs with incomplete metadata schema still get useful Kubernetes metadata.
- Namespaced resources get `metadata.namespace`.
- Cluster-scoped resources do not get `metadata.namespace`.
- Status is present when the schema has it, but authoring renderers collapse or
  comment it by default.
- Resource-level group, version, kind, resource name, shortnames, and scope are
  carried alongside the schema.

## Line And Field Metadata

Renderers need line-level metadata.

Requirements:

- Generated YAML lines carry structured metadata for the field path, line role,
  indentation, foldability, comment status, inline metadata, and syntax spans.
- TUI and HTML syntax highlighting use line metadata.
- Filtering and search use field metadata.
- Details panes use field metadata.
- Renderers must not parse rendered YAML text for non-trivial behavior.

Line roles should cover at least:

- Field line.
- Description comment.
- Inline metadata comment.
- Continuation comment.
- Empty separator.
- Fold gutter/control.

## Diagnostics

The model should preserve diagnostics that renderers can surface:

- Unsupported schema constructs.
- Missing schemas.
- Missing references.
- Non-structural CRD schema problems.
- Unsupported composition.
- Recursive reference truncation.

Diagnostics must not make rendered YAML invalid.

## Tests

The test suite must cover:

- CRD structural schema conversion.
- Native OpenAPI v3 conversion.
- Int-or-string detection.
- Single-reference `allOf` metadata wrappers.
- Unsupported composition diagnostics.
- Kubernetes extensions.
- Defaults, examples, enums, and validations.
- ObjectMeta generation and namespace behavior.
- Recursive references.
- Line metadata coverage for renderers.
