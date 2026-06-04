# Interactive Navigation Requirements

## Purpose

Interactive navigation defines the shared behavior for TUI and web/browser
views. Users should be able to move through the resource overview, open a
version, inspect the schema tree, fold and unfold fields, and return to the
overview without losing context.

The TUI and web/browser renderers should feel like the same product even though
their rendering surfaces differ.

## Modes

Interactive renderers have two primary modes:

- Overview mode: API group, resource, and version selection.
- Schema mode: foldable YAML tree plus details for the focused field.

Interactive renderers may also have transient modes:

- TUI search mode.
- Filtering mode.
- Browser-native find.

Escape resolves transient modes before it navigates away:

1. Leave search mode or clear filtering.
2. From schema mode, return to overview.
3. From TUI overview mode, exit the app.
4. From web/browser overview mode, remain in overview.

Browser mode has no quit key. The browser tab is closed manually and the
`kubectl doc` process is stopped with Ctrl-C.

## Overview Mode

Overview mode shows:

- API groups.
- Resources.
- Served versions on the same line as resources.
- The current cursor/focus.

Requirements:

- Interactive mode does not auto-select the latest version.
- Selecting a version opens schema mode for that exact group/resource/version.
- Returning from schema mode restores the previous overview cursor position and
  scroll position where practical.
- API group labels are visually distinct.
- Resources are not bold by default.

Keys:

- Up/down move between selectable resource/version rows.
- Left/right jump to the previous/next API group.
- Tab/shift-tab behave like right/left and jump between API groups.
- Home moves to the first selectable resource/version.
- End moves to the last selectable resource/version.
- Enter opens the focused resource/version.
- Slash is search, not filtering, according to the renderer's search rules.

Scrolling:

- Moving to the first selectable resource in the first API group scrolls to the
  top so the group header is visible.
- Moving to the last selectable resource in the last API group scrolls to the
  bottom so the end context is visible.
- Resizing the terminal or browser must not leave the cursor outside the visible
  screen.

## Schema Mode

Schema mode shows:

- A YAML-shaped schema tree.
- One focused logical field.
- Fold controls for foldable fields.
- Details for the focused field.

Focus is always logical:

- The cursor is attached to one field or sub-value path.
- Moving, filtering, folding, resizing, or returning from overview must preserve
  logical focus where possible.
- The focused path may be shown in interactive details.

Keys:

- Up/down move to the previous/next visible field.
- Left on an expanded field collapses it.
- Left on a collapsed field moves to its parent.
- Right on a collapsed field expands it.
- Right on an expanded field moves to its first child or item.
- Right on a scalar or leaf field is a no-op.
- Enter toggles fold state.
- Tab jumps to the next foldable field.
- Shift-tab jumps to the previous foldable field.
- Home moves to the first field.
- End moves to the last field.
- PageUp and PageDown move roughly half a page.
- Escape returns to overview when no transient mode is active.

TUI quit keys:

- `q` exits the TUI.
- F10 exits the TUI.
- Ctrl-C exits the TUI and must be handled reliably.
- Escape exits the TUI only from overview mode when no transient mode is active.
- Escape in schema mode returns to overview before it can quit.

## Search

TUI search:

- `/` enters search mode.
- Search matches field names and descriptions.
- `n` moves to the next result.
- `p` moves to the previous result.
- Up/down may move across results while search mode is active.
- Escape exits search mode.
- Results are highlighted in strong orange.
- The focused result remains distinguishable without relying only on color.

Web/browser search:

- Browser find is the search implementation.
- `/` must be left available to browser find.
- There is no custom web search input for schema pages.
- Browser find does not alter fold state or filtering state.

Filtering is specified separately in
[REQUIREMENTS_FILTERING.md](./REQUIREMENTS_FILTERING.md).

## Details Layout

Details must show structured metadata for the focused field.

TUI:

- On wide screens, schema occupies about 75 percent of the width and details
  about 25 percent.
- A vertical line separates schema and details.
- On narrower screens, details may move below the schema.
- The separator must span the full visible screen height.
- A low-contrast key hint line stays visible at the bottom.

Web/browser:

- Details are aligned beside the schema on wide screens.
- Details become sticky when scrolling down and should stay at the top of the
  viewport without visual jumps.
- Details move below or otherwise adapt on narrow screens.
- Details text wraps without overflowing its pane.

Details content:

- Uses readable structured rows or sections.
- Does not use proportional alignment tricks that break with font changes.
- Required `yes` is red.
- Required `no` is green.
- Foldable/collapsed booleans are not shown when the fold state is already
  visible in the schema tree.

## Visual Stability

Interactive renderers must avoid layout jumps.

Requirements:

- Focusing a row must not shift the row horizontally.
- The cursor highlight spans the whole visible line where appropriate.
- The cursor style is visible but not so strong that it hides syntax colors.
- Fold gutter controls must not receive default browser focus outlines.
- Resizing must recompute viewport and scroll state.
- The vertical separator and key hints must remain visible even when the schema
  is shorter than the screen.

## Copy Behavior

Users should be able to copy valid YAML from the schema area.

Requirements:

- Fold gutter controls should be excluded from copied text where the platform
  allows it.
- The TUI should hide or separate gutters during selection if that is the most
  practical way to keep copied YAML valid.
- The web renderer should place triangles in non-selectable controls.
- Highlighting and cursor styling must not inject extra text into copied YAML.

## Comment Wrapping

Web/browser schema views include comment wrapping controls:

- Wrapping is enabled by default.
- The wrap toggle remains sticky near the bottom right with enough spacing from
  the viewport edge.
- The toggle is a compact iOS-style control with the label `wrap`.
- Wrapped comments are semantic YAML comments, with a new `#` on each wrapped
  line.
- Toggling wrapping must not visually move the first line of the schema.

TUI comment wrapping is automatic and based on the available pane width.

## Tests

The test suite must cover:

- Overview keyboard navigation.
- Schema keyboard navigation.
- Left/right fold semantics.
- Tab/shift-tab foldable jumps.
- Escape returning from schema to overview.
- Overview cursor restoration after returning from schema.
- Terminal resize clamping.
- Details layout rendering contracts where practical.
- Gutter/copy validity behavior where practical.
- Browser sticky details and wrap-control script contracts.
