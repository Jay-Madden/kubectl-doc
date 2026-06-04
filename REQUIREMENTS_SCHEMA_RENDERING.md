# Schema Rendering Requirements

## Purpose

Schema rendering turns the normalized Kubernetes documentation model into a
YAML-shaped view. The output is documentation, but the visible YAML must remain
syntactically valid so users can copy it, edit placeholders, and understand the
shape of the resource.

Rendering must be driven by structured field and line metadata. Renderers must
not parse their own YAML output with regular expressions to rediscover schema
meaning.

## YAML Validity

The rendered schema view must be valid YAML when copied from the YAML area.

Requirements:

- Fold controls such as triangles must not be part of copied YAML text whenever
  the renderer can avoid it.
- Browser and TUI gutters must be visually separate from selectable YAML.
- Placeholders must be valid YAML values, for example `"<string>"`, `0`,
  `false`, `[]`, `{}`, or a valid schema default.
- Field metadata belongs in YAML comments, hover UI, focus details, or details
  sections.
- Metadata must not make the YAML syntactically invalid.
- The YAML does not need to be a semantically valid Kubernetes manifest.

## Root Resource Fields

For Kubernetes resources, the rendered root must include:

- `apiVersion`
- `kind`
- `metadata`

Root ordering:

```yaml
apiVersion: group/version
kind: Kind
metadata:
spec:
status:
```

Requirements:

- `apiVersion`, `kind`, and `metadata` are always placed at the top.
- `apiVersion`, `kind`, and `metadata` are selectable fields in interactive
  renderers.
- Their descriptions and details are available in details panes.
- Their descriptions are stripped from the YAML view to keep the top of the
  manifest compact.
- Metadata field descriptions, such as `metadata.labels` and
  `metadata.annotations`, must still be available when those fields are focused.
- Namespaced resources include `metadata.namespace: "<namespace>"`.
- Cluster-scoped resources omit `metadata.namespace`.
- Interactive renderers generate full metadata and keep it collapsed by default.

## Required And Optional Fields

Required fields:

- Must not be hidden behind YAML comments.
- Are rendered live as normal YAML.
- Are marked with compact inline metadata containing `required`.
- Use red styling for the word `required` in color-capable renderers.

Optional fields:

- Are rendered as commented YAML when they can be omitted without hiding required
  descendants.
- Are folded by default unless the initial expansion depth or interactive state
  expands them.
- If an optional parent contains required descendants, the parent path may be
  rendered live so those descendants remain valid YAML. Such parents should be
  marked with `optional`.

Inline metadata is lower-case and comma-separated:

```yaml
replicas: 1 # default, minimum: 0
selector: # required
template: # optional
```

Ordering requirements:

- Example/default markers come before `required` or `optional`.
- Validation constraints follow required/optional markers.
- The `#` marker itself is not styled as required; only the word `required` is
  red or boxed.

Examples:

```yaml
image: "nginx" # default, required
names: # example array, required
  - "<string>"
```

## Descriptions

Description comments render immediately before their field at the same
indentation as the field.

Required indentation shape:

```yaml
# description
foo:
  # some description
  bar:
    # description
    abc: 42

    # new field
    def: "hallo"

  # description
  ghi: 42
```

Requirements:

- `--descriptions=true` renders all descriptions.
- `--descriptions=required` renders descriptions for required fields.
- `--descriptions=false` suppresses description comments.
- Long descriptions are wrapped semantically, with a new `#` on every wrapped
  line.
- Follow-on comments after a field should align to the field's inline comment
  column where practical.
- One-line fields without descriptions do not get trailing blank lines unless a
  following non-one-line field needs separation.
- Sibling field blocks with descriptions or nested content are separated by a
  blank line.

## Scalars

Scalar rendering rules:

- Defaults render as the field value when present.
- Field-local examples render as the field value when no default is present and
  the example is compact and valid YAML.
- Enum fields render a default when present, otherwise the first enum value.
- Generic placeholders are used only when no default, example, or enum value is
  available.
- Type placeholders should not use colors that imply validation failure.

Examples:

```yaml
name: "<string>" # minLength: 1
enabled: true # default
port: <int32> # minimum: 1, maximum: 65535
mode: Prod # enum: Prod | Dev | Stage
sharedMemorySize: <int-or-string> # intOrString
```

## Objects, Arrays, And Maps

Objects:

- Render required children first.
- Render optional children after required children.
- Fold or comment optional children by default.
- Preserve `x-kubernetes-preserve-unknown-fields` as compact metadata.

Arrays:

- Render one representative item when no default or example is present.
- Arrays of objects render a nested object item, not a generated type name.
- Array item comments must not accidentally double-comment fields.
- Array fold controls refer to the array or item as a single field focus unit
  where possible.

Maps:

- Render one representative `<key>` entry when no default or example is present.
- Kubernetes list-map metadata such as `listType` and `listMapKeys` is shown as
  compact inline metadata and in details.

Examples:

```yaml
containers:
  - name: "<string>" # required
    image: "<string>" # required

labels:
  <key>: "<string>"
```

## Status

Status is authoring-oriented by default.

Requirements:

- Interactive renderers generate `status` but keep it collapsed by default.
- Non-interactive renderers show `status` as a folded/commented node by default.
- Status can still be expanded or included through renderer-specific fold state
  and future flags.

## Details

Focused details must include all available field metadata:

- Path.
- Type and format.
- Required or optional status.
- Description.
- Default.
- Examples.
- Enum values.
- Numeric constraints.
- String constraints.
- Array constraints.
- Object constraints.
- CEL validations.
- Kubernetes OpenAPI extensions.

Details must be rendered from structured schema metadata, not from rendered YAML
comments.

## Colors

Color-capable renderers should style:

- Keys.
- Scalar placeholders and scalar values.
- Comments.
- URLs in comments.
- Required markers.
- Defaults, examples, enums, and validations.
- Punctuation such as `:`, `[`, `]`, and `,`.

Requirements:

- `--nocolor` disables terminal color.
- Required uses red only for the required marker, not the entire comment.
- URL highlighting is subtle and must not dominate comments.
- Type colors should be distinct but should not look like error diagnostics.

## Tests

The test suite must cover:

- Valid YAML for copied/selected schema output.
- Required fields not commented.
- Optional fields commented unless live parents are needed.
- Inline metadata ordering.
- Description indentation and blank-line rules.
- Arrays and maps with representative items.
- Metadata namespace behavior.
- Metadata descriptions available in details.
- Status collapsed/commented defaults.
- Full validation metadata in details.
- Syntax highlighting driven by structured line metadata.
