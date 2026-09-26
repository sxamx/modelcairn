# Interactive route editor v1

[Español](editor-rutas-interactivo-v1.es.md)

This design records the behavior of the real web console, not a separate mockup.
The canvas is the primary control; a permanent resource form is not part of the
normal route workflow.

## Interaction and backend meaning

- The entry node contains the model alias used by applications. Creating a route
  edits it in a dialog. Existing aliases are displayed but changes remain in
  advanced configuration because renaming affects clients.
- A model can be chosen from the palette to append.
  The destination dialog selects a provider-compatible API key. A sole compatible
  key may be preselected, but is still visible and changeable.
- Dragging a destination changes only its visual position. Tapping opens its
  details; Up/Down buttons change actual fallback priority and reconnect the edges.
- The arrows map exactly to the strategy's sequential destination list, regardless
  of where cards are placed.
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
- Free positions are stored in that browser only and are not synced across
  devices or written to the backend. Priority is stored in the strategy after
  saving and publishing.
- The editor never presents arbitrary branches: the backend implements ordered
  fallback, not a general workflow engine. Condition nodes, traffic percentages,
  and fictitious estimates from the reference mockup are not shown. Advanced
  strategy fields remain in advanced configuration.

## Responsive and accessibility

The canvas scrolls horizontally and supports zoom on narrow screens, with a Fit
button for orientation. Touch pointer dragging moves cards; tapping a palette
item and using the dialog's Up/Down controls are non-drag alternatives for
functional changes. Buttons have accessible names.

Acceptance: arrows and priority labels match saved fallback order; moving cards
does not alter that order or write to the server; creating and saving use
plan/apply; publication is explicit for an existing route.
