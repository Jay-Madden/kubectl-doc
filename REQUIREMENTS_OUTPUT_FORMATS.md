# Output Format Requirements

## Purpose

`kubectl doc` supports multiple renderers that consume the same structured
documentation model. Each renderer has a different output surface, but field
meaning, required/default/validation display, version selection, and schema
shape must stay consistent.

## Common Rules

All renderers:

- Use OpenAPI v3 or CRD schema input converted into the shared documentation
  model.
- Do not reinterpret raw OpenAPI or parse rendered YAML to recover metadata.
- Use the same resource resolution and version defaulting rules.
- Preserve descriptions, defaults, examples, enums, validations, and Kubernetes
  extensions where the format can represent them.
- Keep output deterministic for tests and generated examples.

Renderer support:

```text
yaml
kro
tui
browser
html
man
markdown
markdown-github
markdown-fern
```

Shortcuts:

- `-i` and `--interactive` select `tui`.
- `-w` and `--web` select `browser`.
- When stdin and stdout are interactive terminals and no output was explicitly
  selected, the default renderer is `tui`.
- Explicit `-o yaml` keeps YAML output even on an interactive terminal.

## YAML

`-o yaml` prints a YAML-shaped documentation view to stdout.

Requirements:

- Output is syntactically valid YAML.
- Output is colored when the terminal supports color.
- `--nocolor` disables color and styling.
- A selected resource is rendered as one selected version.
- No-resource behavior follows the CLI/resource resolution contract.
- Description comments are controlled by `--descriptions`.
- Initial static expansion is controlled by `--expand-depth`.
- Collapsed nodes include useful `--expand-depth` hints when possible.

The YAML renderer is optimized for authoring-oriented copy/edit workflows, not
for generating complete manifests.

## TUI

`-o tui` starts the interactive terminal renderer.

Requirements:

- Works with and without a selected resource.
- Without a selected resource, starts in overview mode.
- With a selected resource, focuses the selected resource/version while keeping
  overview context available.
- Uses the shared interactive navigation rules.
- Shows schema and details panes.
- Exits on `q`, F10, and Ctrl-C.
- Does not exit on Escape.

The implementation should use Bubble Tea v2.

## Browser

`-o browser` starts a localhost server and prints the served URL.

Requirements:

- `-w` and `--web` are shortcuts.
- Bind to localhost on a random available port by using port `0`.
- Best-effort open the URL in the default browser on common desktop
  environments.
- If opening the browser fails, continue serving and wait as usual.
- The process lifecycle is manual. Users stop it with Ctrl-C.
- Browser mode has no quit key.
- Without a selected resource, serve a scalable overview first.
- Fetch OpenAPI lazily when a resource/version is selected.
- Do not pre-render every schema in an unfiltered cluster.
- Use the shared web navigation and filtering behavior.

Browser mode is optimized for live cluster exploration.

## HTML

`-o html` prints a static interactive HTML document to stdout.

Requirements:

- Optimized for one selected CRD or resource.
- Suitable for embedding into documentation sites.
- Includes CSS-based syntax highlighting.
- CSS should be customizable by documentation sites.
- Includes JavaScript for folding, focus, details, keyboard navigation,
  filtering, and wrapping.
- Does not include copy buttons or copy commands.
- Does not include a custom search box.
- Browser find remains the search mechanism.
- Details and wrapping behavior follow the interactive navigation requirements.

Static HTML is not the scalable unfiltered-cluster browser mode.

## Markdown

`-o markdown` defaults to GitHub Markdown and is equivalent to
`-o markdown-github`.

Required dialects:

- `markdown-github`
- `markdown-fern`

Requirements:

- Output is one page/file per invocation.
- GitHub Markdown output is useful without JavaScript. Fern MDX may use its
  generated schema component runtime for interactivity, with schema data embedded
  at generation time.
- Output defaults to the latest selected version and supports `--all-versions`.
- `--field-details` controls whether separate field detail sections are emitted.
- Field details default to disabled.
- YAML examples include description comments and compact inline metadata.
- Generated prose paragraphs and YAML description comments are wrapped to
  `--columns`.
- `--columns` defaults to detected terminal width when available, otherwise
  `80`.
- Markdown dialect support should use only features that are sensible for that
  target.

GitHub Markdown:

- Uses fenced YAML blocks.
- May use GitHub-supported disclosure constructs where they remain portable.
- Should avoid JavaScript-dependent behavior.

Fern Markdown:

- Uses Fern-compatible Markdown features.
- Should be suitable for inclusion in Fern documentation.
- Emits MDX using built-in Fern components where they fit, especially
  frontmatter, accordions, tabs, and page/resource structure.
- Uses a Fern-compatible interactive schema component for the schema tree. That
  component must wrap the shared kubectl-doc web runtime so fold/unfold, focus
  details, keyboard navigation, filtering, line grouping, syntax highlighting,
  and comment wrapping are identical to `-o html`.
- Must use the same runtime and payload model as the selected-resource HTML
  renderer. Differences are limited to Fern page integration concerns such as
  import paths, static sidecar URLs, containment, and overlay placement.
- Should support selected-resource export first and later a static API group
  export that renders multiple resources in the group without a dynamic
  realtime overview.
- Filtering is enabled by default and disabled with `--disable-filtering`.
- The first Fern page renderer remains `markdown-fern`; the reusable React
  component/runtime is packaged separately from the generated MDX page output.
- See [REQUIREMENTS_FERN_MDX.md](./REQUIREMENTS_FERN_MDX.md).

## Kro

`-o kro` prints a Kro SimpleSchema-style documentation view.

Requirements:

- Output is documentation, not a resource manifest.
- Use Kro-like marker syntax for type and constraints where possible.
- Render descriptions as `description="..."` markers when enabled.
- Render defaults, enums, required markers, numeric constraints, string
  constraints, array constraints, and object constraints where possible.
- Arrays of structured objects render nested representative list items instead
  of generated type names.
- Kubernetes-specific metadata without Kro equivalents may be compact comments.
- Supports latest-version defaulting and `--all-versions`.

## Man

`-o man` prints man source to stdout.

Requirements:

- Output is pipeable into `man`, for example
  `kubectl doc -f crd.yaml -o man | man`.
- Output is useful without interactivity.
- Supports latest-version defaulting and `--all-versions`.
- Preserves descriptions and field metadata in a readable terminal-manual form.

## Generated Examples

Generated examples are part of the renderer contract.

Requirements:

- `make gen` updates examples.
- CI checks that generated examples are current.
- README-linked examples should use non-trivial CRDs where possible.
- Static HTML examples should be published with a `text/html` content type, for
  example through GitHub Pages.

## Tests

The test suite must cover:

- Renderer selection and shortcuts.
- Interactive-terminal defaulting.
- Stdout versus localhost-server lifecycle.
- YAML validity.
- Markdown paragraph wrapping.
- Static HTML script/style contracts.
- Browser overview server behavior.
- Kro rendering of nested arrays and maps.
- Man output smoke coverage.
- Generated example freshness.
