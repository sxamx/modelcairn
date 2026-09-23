# Interactive route editor v1

[Español](editor-rutas-interactivo-v1.es.md)

This design records the behavior of the real web console, not a separate mockup.
The canvas is the primary control; a permanent resource form is not part of the
normal route workflow.

## Interaction and backend meaning

- The entry node contains the model alias used by applications. Creating a route
  edits it in a dialog. Existing aliases are displayed but changes remain in
  advanced configuration because renaming affects clients.
- A model can be dragged from the palette onto the canvas or tapped to append.
  The destination dialog selects a provider-compatible API key. A sole compatible
  key may be preselected, but is still visible and changeable.
- Dragging a destination changes its position; tapping opens its details. Up/down
  buttons provide the same ordering operation without drag.
- The vertical order maps exactly to the strategy's sequential destination list.
  A fallback edge means the router may try the next destination after an eligible
  failure; it does not promise retries for every error.
- Node metrics are observed for the model across the installation, not measured
  per route. Missing measurements remain explicitly missing.

## Persistence boundaries

- A new route stays local until review calls configuration plan and confirmation
  applies the destinations, strategy, and route atomically.
- Editing an existing route creates local proposals. Save plans and applies new
  destinations plus the strategy draft atomically. Only an explicit publication
  changes traffic. Discarding local edits writes nothing.
- The editor never presents arbitrary branches or free-positioned nodes, because
  the current backend implements ordered fallback, not a general workflow engine.
  Advanced strategy fields remain in advanced configuration.

## Responsive and accessibility

The board stacks vertically on narrow screens. Touch pointer dragging is
supported; tapping a palette item and using the dialog's up/down controls are
non-drag alternatives. Node and palette actions have accessible button names.

Acceptance: visual order equals the saved fallback order; opening or moving
nodes never writes immediately; creating and saving use plan/apply; publication
is explicit for an existing route.
