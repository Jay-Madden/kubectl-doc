# Resource Resolution Requirements

## Purpose

Resource resolution turns user selectors, cluster discovery data, and CRD file
metadata into a concrete resource/version selection. It must behave like users
expect from `kubectl`, while preserving explicit version choices where the docs
UI needs them.

This document is the regression contract for:

- Resource overview construction.
- Kubectl-style selector parsing.
- Version ordering and defaulting.
- Cluster and CRD source differences.
- Ambiguity handling.

## Input Sources

Resource resolution has two sources.

Cluster mode:

- Uses Kubernetes discovery to list API groups, resources, versions, kinds,
  singular names, shortnames, namespaced scope, and verbs.
- Uses OpenAPI v3 only after a concrete resource/version has been selected.
- Does not fetch every OpenAPI schema just to show the overview.

CRD file mode:

- Reads one or more `apiextensions.k8s.io/v1` CRD manifests.
- Uses the CRD's `spec.group`, `spec.names`, `spec.scope`, and served
  `spec.versions`.
- A CRD has one resource/kind identity. The only required disambiguation is the
  served version when multiple served versions exist.

## Overview

The overview groups resources by API group.

Requirements:

- The legacy core API group is displayed as `core`.
- `core` is sorted before all other API groups.
- Non-core API groups are sorted lexicographically.
- Resource names are rendered as plural resource names.
- Versions are shown on the same line as the resource.
- A single served version renders as a string, for example `v1`.
- Multiple served versions render as a YAML flow sequence, for example
  `["v2","v1beta1"]`.
- Multi-version sequences are sorted latest-first.
- Shortnames, kind names, singular names, namespaced scope, and verbs are
  preserved as metadata for resolution, details, filtering, and future UI
  affordances.
- Resource names in graphical or browser overview tiles are not bold by
  default; API group labels should remain visually distinct.

The overview must be cheap enough to serve for large clusters. Browser mode must
serve an overview first and lazily fetch OpenAPI for the selected resource
version.

## Selectors

Cluster selectors must support the normal Kubernetes resource lookup forms:

- Plural resource names, for example `deployments`.
- Singular resource names, when discovery provides them.
- Kind names, for example `Deployment`.
- Shortnames, for example `po`.
- Group-qualified resources, for example `deployments.apps`.
- Version-qualified resources, for example `deployments.v1.apps`.

Dot syntax rules:

- `resource.group` selects by resource and API group.
- `resource.version.group` selects by resource, API version, and API group.
- API groups may contain dots.
- `widgets.example.com` means resource `widgets` in group `example.com`.
- `widgets.v1.example.com` means resource `widgets`, version `v1`, group
  `example.com`.
- `pods.v1` is not a core `v1` selector. It means resource `pods` in API group
  `v1`.
- Core resources should normally be selected by resource, kind, or shortname and
  then use version defaulting where appropriate.

The resolver must not implement ad hoc alternatives that conflict with
Kubernetes selector expectations.

## Ambiguity

When a selector matches multiple resources, the tool must not guess silently.

Requirements:

- Report all matching group/version/kind/resource candidates clearly.
- Ask the user to use a more explicit selector.
- Prefer a deterministic output order in diagnostics.
- Do not use latest-version defaulting to hide an ambiguous resource identity.

Example diagnostic shape:

```text
deployments matches:
  apps/v1 Deployment
  custom.example.com/v1 Deployment
Use a fully qualified selector such as deployments.v1.apps.
```

## Version Ordering

Version ordering is latest-first.

Rules:

- Stable versions sort before beta versions.
- Beta versions sort before alpha versions.
- Within the same stability tier, the highest numeric major version wins.
- A version with a higher beta/alpha number wins within the same major version
  and stability tier.
- Unknown version suffixes sort after recognized stable, beta, and alpha forms
  but remain deterministic.

Examples:

```text
v2, v1
v1, v1beta2, v1beta1, v1alpha1
v1beta2, v1beta1, v1alpha2, v1alpha1
```

## Defaulting

Interactive modes:

- `-o tui`, `-i`, `-o browser`, and `-w` do not require a selected resource.
- Without a resource selector, they start in overview mode.
- They show explicit version choices.
- They do not apply the latest-version heuristic before the user selects a
  version.

Non-interactive schema modes:

- When a resource is selected without an explicit version, the latest served
  version is selected using the version ordering rules.
- This applies to cluster resources and CRD file mode.
- `html`, `man`, `kro`, `markdown`, `markdown-github`, and `markdown-fern`
  default to the latest served version and support `--all-versions`.
- `yaml` renders exactly one selected version.

No-resource behavior:

- Bare `kubectl doc` shows the resource overview when it is not switched into an
  interactive view.
- If stdout and stdin are interactive terminals and the user did not explicitly
  select an output renderer, bare `kubectl doc` defaults to TUI overview mode.
- Explicit schema-oriented output must return a clear error when it cannot
  render without a selected resource.

## CRD File Mode

CRD mode requirements:

- `-f <crd>` does not require a cluster.
- The CRD's resource is implicit.
- `--version <version>` selects a served CRD version.
- When no version is provided for a non-interactive schema renderer, select the
  latest served CRD version.
- Interactive modes show served versions and wait for explicit selection.
- Non-served versions are ignored for rendering unless future diagnostics choose
  to surface them separately.
- Missing structural schemas in served versions produce clear diagnostics.

## Tests

The test suite must cover:

- Plural, singular, kind, and shortname selectors.
- `resource.group` and `resource.version.group`.
- Groups that contain dots.
- The `pods.v1` parsing rule.
- Ambiguous selectors and deterministic diagnostics.
- Stable/beta/alpha version ordering.
- Single-version and multi-version overview rendering.
- Shortnames preserved in overview metadata.
- CRD latest-version selection.
- Interactive no-resource overview behavior.
- Non-interactive latest-version defaulting.
