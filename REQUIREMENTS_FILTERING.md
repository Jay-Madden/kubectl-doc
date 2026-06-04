# Interactive Filtering Requirements

## Purpose

Interactive filtering narrows the visible Kubernetes API documentation while the
user types. It is available in the TUI and web/browser interactive views for
both the resource overview and the selected resource schema view.

Filtering is distinct from search:

- Filtering changes which rows are visible.
- Search navigates matches without changing the visible tree.
- The two features must not share state or key behavior.

This document is the regression contract for filtering. Future changes should
keep these behaviors covered by tests.

## Scope

Filtering applies to:

- TUI resource overview.
- TUI schema/resource view.
- Web/browser resource overview.
- Web/browser schema/resource view.

Filtering does not apply to static renderers such as `yaml`, `kro`, `html`,
`markdown`, `man`, or `markdown-*` after they have been emitted as files or
stdout.

## Activation

Filtering starts immediately when the user types printable text in an
interactive view.

Requirements:

- Plain typing appends to the active filter string.
- Backspace removes one character.
- Escape clears the filter.
- The current filter string is shown as a lightweight overlay near the top of
  the view.
- Matching text is highlighted in strong orange in the main view.
- Filtering must not require focusing an input field.
- Filtering must not parse rendered YAML text to discover fields or metadata.
  It must use the structured overview and document model.

The `/` key is reserved for search:

- In the TUI, `/` enters modal search.
- In web/browser views, `/` must be left to the browser so browser find opens.
- Typing `/` must not start or modify the filter.

## Overview Filtering

The resource overview filter must match:

- API group names.
- Resource names.
- Served versions.
- Resource shortname aliases.

Matching is case-insensitive substring matching.

Behavior:

- Matching a group keeps that group visible.
- Matching a resource keeps that resource and its group visible.
- Matching a version keeps that resource, version, and group visible.
- Matching a shortname alias keeps the owning resource and group visible even if
  the shortname is not rendered as primary row text.
- Group rows remain navigable context, but version/resource rows are the primary
  selectable targets.
- Versions are shown behind resources in the overview.
- API group labels remain visually distinct from resources.

Navigation while an overview filter is active:

- Up/down move through the visible selectable resources/versions.
- Left/right jump to the previous/next visible API group.
- Tab/shift-tab behave like right/left and jump between visible API groups.
- Home moves to the first visible selectable resource/version.
- End moves to the last visible selectable resource/version.
- Enter opens the focused resource/version.
- Escape clears the filter and keeps focus on the same logical
  resource/version when it still exists in the unfiltered overview.

Edge scrolling:

- Moving to the first selectable resource in the first visible group scrolls to
  the top so the group header is visible.
- Moving to the last selectable resource in the last visible group scrolls to
  the bottom so the end context is visible.

## Schema Filtering

The schema filter is evaluated against the full logical schema tree, including
fields that are currently collapsed.

A field is an immediate/direct match if:

- Its field name contains the filter substring, or
- Its field description contains the filter substring.

A field is visible during filtering if:

- It is an immediate/direct match.
- One of its descendants is an immediate/direct match.
- One of its parents is an immediate/direct match.
- One of its parent descriptions is an immediate/direct match.

This makes matching subtrees usable while still preserving enough ancestry to
understand where a field lives.

Rendering requirements:

- Ancestors of matching fields are revealed while the filter is active, even if
  they were collapsed before filtering.
- Matching text is highlighted only in the main schema view.
- If a field matches through its description and the filter string is not in the
  field name, the field name itself is highlighted as the visible match anchor.
- Matching text must not be highlighted in details panes.
- The YAML/schema view must still be rendered from structured line metadata, not
  from regex parsing of rendered YAML text.
- Comment, key, value, validation, default, required, and URL styling must
  remain intact while match highlighting is applied.

Navigation while a schema filter is active:

- Up/down move normally through visible fields.
- Left/right keep their normal schema navigation behavior.
- Home moves to the first visible field.
- End moves to the last visible field.
- Enter accepts the current filtered view and keeps branches unfolded that were
  revealed by the filter.
- Escape clears the filter and restores the previous folded state, except that
  ancestors of the currently focused field stay expanded so the focus remains
  visible.
- Tab jumps to the next immediate/direct match.
- Shift-tab jumps to the previous immediate/direct match.
- Tab and shift-tab must not jump to rows that are visible only because a parent
  matched.
- `n` and `p` have no filtering behavior. They remain reserved for search result
  navigation in TUI search mode.

## Search Interaction

Search and filtering are separate features.

TUI search:

- `/` enters search mode.
- `n` moves to the next search result.
- `p` moves to the previous search result.
- Escape exits search mode first.
- If no search mode is active, Escape clears filtering first.

Web/browser search:

- Browser find is the search UI.
- The web renderer must not install a custom `/` search box.
- Filtering must not prevent browser find from working.

## Focus Preservation

Filtering must preserve logical focus, not just screen row numbers.

Requirements:

- If a focused resource/version remains available after clearing an overview
  filter, focus stays on that same logical resource/version.
- If a focused field remains available after clearing a schema filter, focus
  stays on that same logical field.
- If the exact focused item is no longer available, focus should move to the
  nearest sensible visible item without leaving the viewport in an invalid
  state.
- Clearing a filter must never leave the cursor outside the visible screen.

## Fold State

Filtering temporarily reveals matching paths without permanently rewriting the
user's folded state unless the user accepts it.

Requirements:

- Starting a filter records the current folded state.
- While filtering, matching descendants of collapsed nodes become visible.
- Escape clears the filter and restores the recorded folded state, except for
  ancestors needed to keep the current focus visible.
- Enter clears the filter and commits the revealed branches, keeping them
  unfolded.
- Existing user-expanded branches stay expanded unless explicitly collapsed by
  normal navigation.

## Interactive Default

When stdout and stdin are interactive terminals and the user did not explicitly
select an output renderer, `kubectl doc` defaults to interactive TUI mode.

Requirements:

- Explicit `-o yaml` keeps YAML output even on an interactive terminal.
- `-i` and `--interactive` continue to force TUI mode.
- Non-interactive stdout, such as pipes and redirects, keeps the normal YAML
  default.

## Test Requirements

The test suite must cover:

- TUI overview filtering by group, resource, version, and shortname alias.
- Web overview filtering by group, resource, version, and shortname alias.
- TUI schema filtering of collapsed descendants.
- Web schema filtering of collapsed descendants.
- Parent and parent-description matches showing descendants.
- Highlighting only direct matched substrings in strong orange.
- No filtering behavior for `n` and `p`.
- Tab and shift-tab jumping only between immediate/direct matches.
- Escape clearing filters while preserving logical focus.
- Escape restoring fold state except for the current focus path.
- Enter committing filter-revealed branches as unfolded.
- Browser `/` search remaining native.
- TUI `/` search remaining modal and separate from filtering.
- Interactive-terminal defaulting to TUI only when no output was explicitly
  selected.
