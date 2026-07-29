# PlatformKit Design Delivery for Figma

This Desktop plugin imports the governed OSS, PlatformKit Pro, and per-client
design deliveries into a real Figma library.

## Delivery model

The visible design is never a JSON or screenshot snapshot. A compiled v5
delivery creates and updates:

- native Figma Variable collections, modes, and aliases;
- native ComponentSets and authored variants;
- native text, boolean, instance-swap, and slot component properties;
- curated library and solution pages assembled from component instances.

The bundle also contains a token snapshot, but that snapshot is hidden
optimistic-concurrency state. The plugin stores it in plugin data so a later
Figma edit can be exported as a precise `pk.design.token-changes.v1` change
set. Designers edit native Figma Variables and instances, not the snapshot.

Storybook remains the executable browser truth. Figma conformance inspects
native nodes, identities, variants, and viewports without exporting raster
references. Storybook visual regression compares browser renders only with
reviewed browser baselines; neither tool substitutes images for editable Figma
components or solution pages.

## Compiled bundle

`figma-plugin-bundle.json` uses
`platformkit.figma.plugin-bundle.v5` and contains exactly:

1. one validated `pk.design.figma-variables.v2` token bundle;
2. one `platformkit.figma.native-delivery.v1` structural contract;
3. component-master artifacts in dependency order;
4. curated native page artifacts in compiler order.

The plugin validates the token snapshot digest, bundle digest, artifact
digests, ownership profile, client identity, aliases, native component and
variant identities, editable properties, slots, nested instance edges, and
paths. It imports native Variables before executing component or page
artifacts, then validates the resulting editable graph before accepting the
delivery. Executable token bootstrap scripts are rejected.

Only import compiled bundles produced by a trusted PlatformKit workspace:
component and page artifacts are generated Figma Plugin API JavaScript.

## Generate a delivery

```bash
# PlatformKit Pro library + showcase.
platformkit design compile \
  --target figma \
  --profile pro \
  --parent-manifest frontend/platformkit-frontend-kit/storybook/.generated/design/oss/design-manifest.json \
  --mode split \
  --out .tmp/design/figma/pro

# Per-client library + assembled showcase. The component extension is the
# explicit client-owned addition/replacement contract; client.yaml is the
# canonical brand source.
platformkit design compile \
  --target figma \
  --profile clients/collect \
  --mode split \
  --manifest overlays/septagon-demos/collect/dist/design/design-manifest.json \
  --parent-manifest frontend/platformkit-frontend-kit/storybook/.generated/design/pro/design-manifest.json \
  --preview-manifest overlays/septagon-demos/collect/dist/design/storybook-preview-manifest.json \
  --client-source overlays/septagon-demos/collect/client.yaml \
  --component-extension overlays/septagon-demos/collect/design/component-extension.json \
  --out .tmp/design/figma/clients/collect
```

Verify every produced directory before import:

```bash
platformkit design verify \
  --target figma \
  --profile clients/collect \
  --input .tmp/design/figma/clients/collect/library
```

Import the generated `figma-plugin-bundle.json` through the **Compiled
Delivery** tab. Direct CLI/MCP publication is intentionally fail-closed until
that transport can apply the same governed token bundle atomically; publishing
components without Variables would create a partial file.

## Install and test in Figma Desktop

1. Open the intended Figma library file.
2. Choose **Plugins → Development → Import plugin from manifest…**
3. Select
   `overlays/septagon-oss-workspace/pk-tools/figma-plugin/manifest.json`.
4. Run **PlatformKit Design Delivery**.
5. In **Compiled Delivery**, import `figma-plugin-bundle.json`.
6. Confirm the `PlatformKit Sources` and `PlatformKit Colors` collections,
   including Light/Dark modes and semantic aliases.
7. Change a writable source variable.
8. Use **Export token changes**. Apply the emitted change set through the
   matching OSS, Pro, or client receiver, rebuild Storybook, and run its visual
   delivery gate.
9. In **Conformance**, load the matching
   `platformkit.storybook.visual-delivery.json` and validate the native
   delivery. The plugin resolves every authored component/variant identity to
   exactly one native master, verifies its properties, slots, and composition
   graph, resolves each full-viewport solution instance, and downloads a
   structural conformance report. It does not export or import screenshots.

Inherited variables are read-only in a client profile. Only variables owned by
that client overlay are exported as writable changes.

## Figma and Storybook

The two tools share contract identity, variant selectors, token paths, fixtures,
and viewport dimensions:

- Figma is the editable specification and composition surface.
- Storybook is the live Go/browser implementation.
- The plugin validates native solution instances at the contract's exact
  physical viewport and emits structural provenance.
- Storybook/Chromium resolves the same authored fixture identity, rejects
  missing or runtime-generated fixtures, and emits browser actual/diff
  evidence against a reviewed browser baseline.
- Browser PNGs exist only as test evidence; they never become Figma layers,
  component masters, token sources, or cross-engine parity references.

For Dev Mode handoff, connect the published Figma components to their code
implementations with Figma Code Connect template files. Templates are the
framework-neutral route for PlatformKit’s Go-rendered components.

## Recovery tab

The standalone **Tokens** tab accepts a governed v2 Variables bundle for
diagnostics and recovery. There is no legacy structural or screenshot import:
the governed compiled delivery is the only path that can create component
masters and solution pages.

## Files

| File | Purpose |
|---|---|
| `manifest.json` | Local Figma Desktop plugin manifest |
| `code.js` | Validated bundle import, native Variables, and token change export |
| `ui.html` | Delivery import and round-trip UI |
| `package.json` | Local syntax and contract checks |

The design compiler architecture is documented in
`frontend/platformkit-design-system/docs/FIGMA_COMPILER_ARCHITECTURE.md`.
