// Validates: REQ-011.
// Per: ADR-0076.
// Discipline: C-14.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import { webcrypto } from "node:crypto";

const source = await readFile(new URL("./code.js", import.meta.url), "utf8");

test("visual delivery parser accepts stable native fixture coordinates", () => {
  const context = pluginContext();
  const fixture = validFixture();
  const parsed = vm.runInContext(
    `parseVisualDeliveryPayload(${JSON.stringify(JSON.stringify({
      schema: "platformkit.storybook.visual-delivery.v2",
      profile: "pro",
      fixtures: [fixture],
    }))})`,
    context,
  );
  assert.equal(parsed.fixtures[0].id, fixture.id);
});

test("visual delivery parser rejects mismatched Figma identity", () => {
  const context = pluginContext();
  const fixture = validFixture();
  fixture.figma.variant_id = "another-fixture";
  assert.throws(
    () =>
      vm.runInContext(
        `parseVisualDeliveryPayload(${JSON.stringify(JSON.stringify({
          schema: "platformkit.storybook.visual-delivery.v2",
          profile: "pro",
          fixtures: [fixture],
        }))})`,
        context,
      ),
    /Figma identity does not match/,
  );
});

test("profile mapping keeps OSS, Pro, and clients in separate namespaces", () => {
  const context = pluginContext();
  for (const [profile, want] of [
    [{ kind: "oss" }, "oss"],
    [{ kind: "pro" }, "pro"],
    [{ kind: "client", clientId: "collect" }, "clients/collect"],
  ]) {
    context.__profile = profile;
    const got = vm.runInContext(
      `visualDeliveryProfile({
        snapshot: { profile: __profile },
        clientId: __profile.clientId || ""
      })`,
      context,
    );
    assert.equal(got, want);
  }
});

test("artifact envelope enforces OSS to Pro to client ancestry", () => {
  const context = pluginContext();
  const digest = "a".repeat(64);
  for (const [bundle, valid] of [
    [{ profile: "oss" }, true],
    [{ profile: "pro", parentProfile: "oss", parentManifestDigest: digest }, true],
    [{
      profile: "clients/collect",
      clientId: "collect",
      parentProfile: "pro",
      parentManifestDigest: digest,
    }, true],
    [{ profile: "pro", parentProfile: "pro", parentManifestDigest: digest }, false],
    [{
      profile: "clients/collect",
      clientId: "collect",
      parentProfile: "oss",
      parentManifestDigest: digest,
    }, false],
    [{
      profile: "clients/collect",
      clientId: "academy",
      parentProfile: "pro",
      parentManifestDigest: digest,
    }, false],
  ]) {
    context.__bundle = bundle;
    if (valid) {
      assert.doesNotThrow(() =>
        vm.runInContext("validateArtifactBundleProfile(__bundle)", context)
      );
    } else {
      assert.throws(() =>
        vm.runInContext("validateArtifactBundleProfile(__bundle)", context)
      );
    }
  }
});

test("artifact bundle canonical order matches the Go v5 envelope", () => {
  const context = pluginContext();
  const digest = "a".repeat(64);
  context.__bundle = {
    kind: "platformkit.figma.plugin-bundle",
    schema: "platformkit.figma.plugin-bundle.v5",
    version: "5",
    target: "figma",
    product: "PlatformKit",
    profile: "clients/collect",
    clientId: "collect",
    contractVersion: "platformkit.component-contract.v1",
    clientSourceDigest: digest,
    extensionDigest: digest,
    manifestDigest: digest,
    parentProfile: "pro",
    parentManifestDigest: digest,
    tokenBundle: {
      kind: "figma-variables",
      schema: "pk.design.figma-variables.v2",
      product: "PlatformKit",
      clientId: "collect",
      snapshotDigest: `sha256:${digest}`,
      snapshot: {
        schemaVersion: "1",
        profile: {
          id: "clients/collect",
          kind: "client",
          version: "1",
          clientId: "collect",
        },
        provenance: {
          source: "test",
          generatedAt: "2026-07-29T00:00:00Z",
        },
        tokens: [],
      },
      collections: [],
    },
    nativeDelivery: {
      schema: "platformkit.figma.native-delivery.v1",
      version: "1",
      client_id: "collect",
      components: [],
    },
    bundleDigest: digest,
    artifacts: [],
  };

  const keys = vm.runInContext(
    "Object.keys(canonicalArtifactBundle(__bundle))",
    context,
  );
  assert.deepEqual(JSON.parse(JSON.stringify(keys)), [
    "kind",
    "schema",
    "version",
    "target",
    "product",
    "profile",
    "clientId",
    "contractVersion",
    "clientSourceDigest",
    "extensionDigest",
    "manifestDigest",
    "parentProfile",
    "parentManifestDigest",
    "tokenBundle",
    "nativeDelivery",
    "artifacts",
  ]);
});

test("artifact import metadata records exact source and parent provenance", () => {
  const context = pluginContext();
  const values = {};
  context.__node = {
    setSharedPluginData(namespace, key, value) {
      assert.equal(namespace, "platformkit");
      values[key] = value;
    },
  };
  context.__bundle = {
    profile: "clients/collect",
    clientId: "collect",
    contractVersion: "platformkit.component-contract.v1",
    clientSourceDigest: "client-source",
    extensionDigest: "extension",
    manifestDigest: "a".repeat(64),
    parentProfile: "pro",
    parentManifestDigest: "b".repeat(64),
    bundleDigest: "c".repeat(64),
  };

  vm.runInContext("setArtifactBundleMetadata(__node, __bundle)", context);
  assert.deepEqual(values, {
    pkProfile: "clients/collect",
    pkClientId: "collect",
    pkContractVersion: "platformkit.component-contract.v1",
    pkClientSourceDigest: "client-source",
    pkExtensionDigest: "extension",
    pkManifestDigest: "a".repeat(64),
    pkParentProfile: "pro",
    pkParentManifestDigest: "b".repeat(64),
    pkBundleDigest: "c".repeat(64),
  });
});

test("native solution lookup resolves authored identity and exact viewport", async () => {
  const context = pluginContext();
  const fixture = validFixture();
  context.__fixture = fixture;
  const sourceID = `component/${fixture.component_id}`;
  const digest = await sha256Hex(`authored:${fixture.example_id}`);
  const variantID = `${sourceID}/variant/${digest}`;
  const master = {
    getSharedPluginData(_namespace, key) {
      if (key === "pkSourceId") return sourceID;
      if (key === "pkVariantId") return variantID;
      return "";
    },
  };
  const instance = {
    id: "55:81",
    type: "INSTANCE",
    width: fixture.viewport.width,
    height: fixture.viewport.height,
    async getMainComponentAsync() {
      return master;
    },
  };
  context.figma.root.children = [{
    type: "PAGE",
    name: "10 Solution · TestPage",
    getSharedPluginData() {
      return "";
    },
    findAll(predicate) {
      return predicate(instance) ? [instance] : [];
    },
  }];

  const found = await vm.runInContext(
    `findNativeSolutionInstance(__fixture, "")`,
    context,
  );
  assert.equal(found.id, instance.id);

  instance.width -= 1;
  await assert.rejects(
    vm.runInContext(`findNativeSolutionInstance(__fixture, "")`, context),
    /resolved 0 native solution instances/,
  );
});

test("native contract validates editable properties, slots, composition, and solution pages", async () => {
  const context = pluginContext();
  const buttonSource = "component/pk.ui/button";
  const pageSource = "component/pk.ui/data-page";
  const buttonVariant = `${buttonSource}/variant/${await sha256Hex("authored:button-primary")}`;
  const pageVariant = `${pageSource}/variant/${await sha256Hex("authored:data-page-default")}`;

  const buttonMaster = nativeMaster({
    sourceID: buttonSource,
    variantID: buttonVariant,
    name: "Example=Primary",
  });
  const buttonSet = nativeSet({
    sourceID: buttonSource,
    children: [buttonMaster],
    definitions: {
      "Example#axis": { type: "VARIANT", variantOptions: ["Primary"] },
    },
  });
  const actionSlot = {
    id: "slot:actions",
    type: "SLOT",
    slotSettings: { minChildren: 1, maxChildren: null },
    getSharedPluginData(_namespace, key) {
      if (key === "pkSlot") return "actions";
      if (key === "pkSlotPropertyName") return "Slot / actions";
      return "";
    },
  };
  const nestedButton = {
    id: "instance:button",
    type: "INSTANCE",
    async getMainComponentAsync() {
      return buttonMaster;
    },
  };
  const pageMaster = nativeMaster({
    sourceID: pageSource,
    variantID: pageVariant,
    name: "Example=Default, Viewport=Desktop",
    definitions: {
      "Slot / actions#slot": { type: "SLOT" },
      "showTitle#boolean": { type: "BOOLEAN", defaultValue: true },
      "title#text": { type: "TEXT", defaultValue: "Records" },
    },
    descendants: [actionSlot, nestedButton],
  });
  const pageSet = nativeSet({
    sourceID: pageSource,
    children: [pageMaster],
    definitions: {
      "Example#axis": { type: "VARIANT", variantOptions: ["Default"] },
      "Viewport#axis": { type: "VARIANT", variantOptions: ["Desktop"] },
    },
  });
  const solutionInstance = {
    id: "instance:solution",
    type: "INSTANCE",
    width: 1280,
    height: 800,
    async getMainComponentAsync() {
      return pageMaster;
    },
  };
  const solutionRoot = {
    id: "frame:solution-root",
    type: "FRAME",
    name: "Solution / DataPage",
    getSharedPluginData(_namespace, key) {
      return key === "pkCanvasRoot" ? "10 Solution · DataPage" : "";
    },
  };
  context.figma.root.children = [
    nativePage("05 Components", [buttonSet, pageSet]),
    nativePage("10 Solution · DataPage", [solutionRoot, solutionInstance]),
  ];
  context.__nativeContract = {
    schema: "platformkit.figma.native-delivery.v1",
    version: "1",
    components: [
      {
        id: "pk.ui/button",
        source_id: buttonSource,
        name: "Button",
        category: "atom",
        variant_axes: [{ name: "Example", values: ["Primary"] }],
        variants: [{
          id: "button-primary",
          source_id: buttonVariant,
          properties: { Example: "Primary" },
        }],
      },
      {
        id: "pk.ui/data-page",
        source_id: pageSource,
        name: "DataPage",
        category: "page",
        variant_axes: [
          { name: "Example", values: ["Default"] },
          { name: "Viewport", values: ["Desktop"] },
        ],
        slots: [{ name: "actions", required: true, cardinality: "many" }],
        variants: [{
          id: "data-page-default",
          source_id: pageVariant,
          properties: { Example: "Default", Viewport: "Desktop" },
          component_properties: [
            { name: "Slot / actions", type: "SLOT" },
            { name: "showTitle", type: "BOOLEAN", default: true },
            { name: "title", type: "TEXT", default: "Records" },
          ],
          slots: [{
            name: "actions",
            property_name: "Slot / actions",
            required: true,
            cardinality: "many",
            allowed_types: ["Button"],
            min_children: 1,
          }],
          instances: [{
            reference: "Button::Example=Primary",
            component_id: "pk.ui/button",
            component_source_id: buttonSource,
            variant_id: "button-primary",
            variant_source_id: buttonVariant,
          }],
        }],
      },
    ],
    canvases: [{
      page_name: "10 Solution · DataPage",
      root_name: "Solution / DataPage",
    }],
    solutions: [{
      page_name: "10 Solution · DataPage",
      component_id: "pk.ui/data-page",
      component_source_id: pageSource,
      variant_id: "data-page-default",
      variant_source_id: pageVariant,
      width: 1280,
      height: 800,
      color_mode: "light",
    }],
  };

  const result = await vm.runInContext(
    `validateNativeContract(__nativeContract, "")`,
    context,
  );
  assert.deepEqual(
    JSON.parse(JSON.stringify(result)),
    {
      components: 2,
      variants: 2,
      properties: 3,
      slots: 1,
      instances: 1,
      canvases: 1,
      solutions: 1,
    },
  );

  pageMaster.componentPropertyDefinitions["title#text"].defaultValue = "Wrong";
  await assert.rejects(
    vm.runInContext(`validateNativeContract(__nativeContract, "")`, context),
    /property title has wrong default/,
  );
});

test("native delivery schema rejects screenshot fields", async () => {
  const context = pluginContext();
  context.__nativeContract = {
    schema: "platformkit.figma.native-delivery.v1",
    version: "1",
    screenshot: "page.png",
    components: [],
  };
  await assert.rejects(
    vm.runInContext(`validateNativeDeliveryContract(__nativeContract)`, context),
    /unsupported field "screenshot"/,
  );
});

test("native delivery schema rejects tampered nested variant identities", async () => {
  const context = pluginContext();
  const buttonSource = "component/pk.ui/button";
  const pageSource = "component/pk.ui/page";
  const buttonVariant =
    `${buttonSource}/variant/${await sha256Hex("authored:button-primary")}`;
  const pageVariant =
    `${pageSource}/variant/${await sha256Hex("authored:page-default")}`;
  context.__nativeContract = {
    schema: "platformkit.figma.native-delivery.v1",
    version: "1",
    components: [
      {
        id: "pk.ui/button",
        source_id: buttonSource,
        name: "Button",
        category: "atom",
        variants: [{
          id: "button-primary",
          source_id: buttonVariant,
        }],
      },
      {
        id: "pk.ui/page",
        source_id: pageSource,
        name: "Page",
        category: "page",
        variants: [{
          id: "page-default",
          source_id: pageVariant,
          instances: [{
            reference: "Button::Variant=Primary",
            component_id: "pk.ui/button",
            component_source_id: buttonSource,
            variant_id: "button-primary",
            variant_source_id: "tampered",
          }],
        }],
      },
    ],
  };
  await assert.rejects(
    vm.runInContext(`validateNativeDeliveryContract(__nativeContract)`, context),
    /invalid variant source identity/,
  );
});

test("token export round-trips one writable Figma color to its exact owner", async () => {
  const { context } = await governedTokenExportContext({
    writable: true,
    nativeValue: {
      r: 0x1d / 255,
      g: 0x4e / 255,
      b: 0xd8 / 255,
      a: 1,
    },
  });

  const result = await vm.runInContext(
    `exportTokenChanges({
      author: "Designer",
      reason: "Align the Collect brand primary"
    })`,
    context,
  );
  const changeSet = JSON.parse(result.payload);

  assert.equal(result.changes, 1);
  assert.equal(result.profile, "clients/collect");
  assert.equal(result.filename, "clients-collect-token-changes.json");
  assert.equal(changeSet.schemaVersion, "pk.design.token-changes.v1");
  assert.equal(changeSet.baseDigest, context.__bundle.snapshotDigest);
  assert.deepEqual(
    JSON.parse(JSON.stringify(changeSet.profile)),
    JSON.parse(JSON.stringify(context.__bundle.snapshot.profile)),
  );
  assert.equal(changeSet.provenance.source, "figma-plugin");
  assert.equal(changeSet.provenance.author, "Designer");
  assert.equal(changeSet.provenance.reason, "Align the Collect brand primary");
  assert.equal(changeSet.changes[0].operation, "update");
  assert.equal(changeSet.changes[0].before.value, "#2563EB");
  assert.equal(changeSet.changes[0].after.value, "#1D4ED8");
  assert.deepEqual(
    changeSet.changes[0].after.origin,
    changeSet.changes[0].before.origin,
  );
});

test("token export rejects edits to inherited read-only variables", async () => {
  const { context } = await governedTokenExportContext({
    writable: false,
    nativeValue: {
      r: 0x1d / 255,
      g: 0x4e / 255,
      b: 0xd8 / 255,
      a: 1,
    },
  });

  await assert.rejects(
    vm.runInContext(
      `exportTokenChanges({
        author: "Designer",
        reason: "This must remain parent-owned"
      })`,
      context,
    ),
    /read-only variable "\/client\/brand\/500" mode "Default" was edited/,
  );
});

test("plugin validates native nodes without exporting raster snapshots", () => {
  assert.doesNotMatch(source, /\.exportAsync\s*\(/);
  assert.doesNotMatch(source, /figma-native-export/);
  assert.match(source, /figma-native-node-validation/);
});

function nativeMaster({
  sourceID,
  variantID,
  name,
  definitions = {},
  descendants = [],
}) {
  return {
    id: `master:${variantID}`,
    type: "COMPONENT",
    name,
    componentPropertyDefinitions: definitions,
    getSharedPluginData(_namespace, key) {
      if (key === "pkSourceId") return sourceID;
      if (key === "pkVariantId") return variantID;
      if (key === "pkClientId") return "";
      return "";
    },
    findAll(predicate) {
      return descendants.filter(predicate);
    },
  };
}

function nativeSet({ sourceID, children, definitions }) {
  return {
    id: `set:${sourceID}`,
    type: "COMPONENT_SET",
    children,
    componentPropertyDefinitions: definitions,
    getSharedPluginData(_namespace, key) {
      if (key === "pkSourceId") return sourceID;
      if (key === "pkClientId") return "";
      return "";
    },
  };
}

function nativePage(name, descendants) {
  return {
    type: "PAGE",
    name,
    getSharedPluginData() {
      return "";
    },
    findAll(predicate) {
      return descendants.filter(predicate);
    },
  };
}

function pluginContext() {
  const context = vm.createContext({
    __html__: "<html></html>",
    console,
    crypto: webcrypto,
    Date,
    JSON,
    Map,
    Number,
    Object,
    Promise,
    RegExp,
    Set,
    String,
    TextEncoder,
    Uint8Array,
    figma: {
      showUI() {},
      ui: {
        onmessage: null,
        postMessage() {},
      },
      root: {
        children: [],
      },
      async loadAllPagesAsync() {},
    },
  });
  vm.runInContext(source, context);
  return context;
}

async function governedTokenExportContext({ writable, nativeValue }) {
  const context = pluginContext();
  const collection = {
    name: "PlatformKit · collect",
    modes: [{ name: "Default", modeId: "mode:default" }],
    variableIds: ["variable:brand-500"],
  };
  const variable = {
    id: "variable:brand-500",
    name: "brand/500",
    valuesByMode: {
      "mode:default": nativeValue,
    },
  };
  context.__bundle = {
    kind: "figma-variables",
    schema: "pk.design.figma-variables.v2",
    product: "PlatformKit · collect",
    clientId: "collect",
    snapshotDigest: "",
    snapshot: {
      schemaVersion: "pk.design.token-snapshot.v1",
      profile: {
        id: "clients/collect",
        kind: "client",
        version: "1",
        clientId: "collect",
        parentId: "pro",
        parentDigest: `sha256:${"a".repeat(64)}`,
      },
      provenance: {
        source: "client.yaml",
        generatedAt: "2026-07-29T00:00:00Z",
      },
      tokens: [{
        path: "/client/brand/500",
        mode: "default",
        type: "color",
        value: "#2563EB",
        origin: {
          owner: writable ? "collect" : "pro",
          layer: writable ? "client" : "pro",
          sourceUri: writable ? "client.yaml" : "pkds://semantic",
          pointer: writable ? "/design/brand/500" : "/semantic/color/brand",
          writable,
        },
      }],
    },
    collections: [{
      name: collection.name,
      modes: ["Default"],
      variables: [{
        key: "/client/brand/500",
        name: variable.name,
        type: "COLOR",
        values: {
          Default: {
            r: 0x25 / 255,
            g: 0x63 / 255,
            b: 0xeb / 255,
            a: 1,
          },
        },
        binding: {
          tokenPath: "/client/brand/500",
          modeMap: { Default: "default" },
          codecs: writable
            ? { Default: { kind: "color", style: "hex6-upper" } }
            : undefined,
          writable,
        },
      }],
    }],
  };
  context.__bundle.snapshotDigest = await vm.runInContext(
    "digestTokenSnapshot(__bundle.snapshot)",
    context,
  );
  context.figma.clientStorage = {
    async getAsync() {
      return JSON.stringify(context.__bundle);
    },
  };
  context.figma.variables = {
    async getLocalVariableCollectionsAsync() {
      return [collection];
    },
    async getVariableByIdAsync(id) {
      return id === variable.id ? variable : null;
    },
  };
  return { context, collection, variable };
}

function validFixture() {
  return {
    id: "platformkit.component/test-page#test-page-default",
    component_id: "platformkit.component/test-page",
    component_type: "TestPage",
    example_id: "test-page-default",
    example_name: "default",
    browser_baseline_file: "test-page-default-123456789abc.png",
    story: {
      file: "testpage.stories.json",
      title: "Base/Pages/TestPage",
      name: "Default",
    },
    figma: {
      component_id: "platformkit.component/test-page",
      variant_id: "test-page-default",
    },
    viewport: {
      token: "desktop",
      label: "Desktop",
      width: 1280,
      height: 800,
    },
    color_mode: "light",
    density: "medium",
  };
}

async function sha256Hex(value) {
  const digest = await webcrypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(value),
  );
  return Array.from(
    new Uint8Array(digest),
    (byte) => byte.toString(16).padStart(2, "0"),
  ).join("");
}

// --- SHA-256 fallback ------------------------------------------------------
//
// Figma's plugin sandbox provides no Web Crypto, so the bundle integrity check
// — the thing that proves the code about to be executed is the code that was
// compiled — could not run there at all. These pin the fallback against
// published vectors and against Web Crypto itself, because a hash that is
// merely plausible is worse than none: it would verify nothing while appearing
// to.

test("sha256 fallback matches the published FIPS 180-4 vectors", () => {
  const context = pluginContext();
  for (const [input, want] of [
    ["", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"],
    ["abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"],
    [
      "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
      "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
    ],
  ]) {
    context.__input = input;
    const got = vm.runInContext("sha256HexFallback(__input)", context);
    assert.equal(got, want, `vector ${JSON.stringify(input)}`);
  }
});

test("sha256 fallback agrees with Web Crypto, including multi-byte input", async () => {
  const context = pluginContext();
  for (const input of [
    "platformkit",
    "sticker · mule — Collect",   // multi-byte
    "\u{1F600} emoji surrogate pair",
    "x".repeat(1000),             // spans multiple 64-byte blocks
  ]) {
    context.__input = input;
    const fallback = vm.runInContext("sha256HexFallback(__input)", context);
    const viaCrypto = Array.from(
      new Uint8Array(await webcrypto.subtle.digest("SHA-256", new TextEncoder().encode(input))),
      b => b.toString(16).padStart(2, "0"),
    ).join("");
    assert.equal(fallback, viaCrypto, `input ${JSON.stringify(input.slice(0, 20))}`);
  }
});

test("sha256Hex uses the fallback when the host has no Web Crypto", async () => {
  const context = pluginContext();
  // Reproduce the Figma sandbox: no crypto, no TextEncoder.
  vm.runInContext("crypto = undefined; TextEncoder = undefined;", context);
  const got = await vm.runInContext(`sha256Hex("abc")`, context);
  assert.equal(got, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
});

// --- component property definition ownership -------------------------------
//
// Figma throws when componentPropertyDefinitions is read from a variant. Every
// managed component IS a variant of its set, so reading it directly aborted
// conformance on any real library. These pin the resolution both ways.

test("property definitions resolve to the set when the node is a variant", () => {
  const context = pluginContext();
  const set = { type: "COMPONENT_SET", componentPropertyDefinitions: { "size#1": { type: "VARIANT" } } };
  const variant = { type: "COMPONENT", parent: set };
  context.__variant = variant;
  const owner = vm.runInContext("propertyDefinitionOwner(__variant)", context);
  assert.equal(owner.type, "COMPONENT_SET", "a variant must defer to its set");
});

test("property definitions stay on the node when it is not a variant", () => {
  const context = pluginContext();
  const standalone = { type: "COMPONENT", parent: { type: "FRAME" }, componentPropertyDefinitions: {} };
  context.__standalone = standalone;
  const owner = vm.runInContext("propertyDefinitionOwner(__standalone)", context);
  assert.equal(owner.type, "COMPONENT", "a non-variant component owns its own definitions");

  // A missing parent must not throw — nodes are inspected mid-rebuild.
  context.__orphan = { type: "COMPONENT" };
  assert.equal(vm.runInContext("propertyDefinitionOwner(__orphan).type", context), "COMPONENT");
});

// --- instance graph ownership ----------------------------------------------
//
// The contract lists the instances a variant OWNS. Figma's findAll recurses
// into instances too, so a component composing another composed component
// counted its dependency's children as its own and failed conformance.

function fakeInstance(sourceId, variantId, children = []) {
  const main = {
    type: "COMPONENT",
    getSharedPluginData: (_, key) =>
      key === "pkSourceId" ? sourceId : key === "pkVariantId" ? variantId : "",
  };
  return { type: "INSTANCE", mainComponent: main, children };
}

test("instance graph counts owned instances, not those nested inside them", async () => {
  const context = pluginContext();
  // A book instance that itself contains ten instances, plus one upload cell.
  const book = fakeInstance("component/book", "component/book/variant/x", [
    fakeInstance("component/dot", "component/dot/variant/y"),
    fakeInstance("component/slot", "component/slot/variant/z"),
  ]);
  const cell = fakeInstance("component/cell", "component/cell/variant/w");
  const master = { type: "COMPONENT", children: [{ type: "FRAME", children: [book] }, cell] };

  context.__master = master;
  context.__component = { id: "component/preview" };
  context.__variant = {
    id: "preview-default",
    instances: [
      { component_source_id: "component/book", variant_source_id: "component/book/variant/x" },
      { component_source_id: "component/cell", variant_source_id: "component/cell/variant/w" },
    ],
  };

  // Must not throw: the book's two nested instances belong to the book.
  await vm.runInContext(
    "validateVariantInstances(__master, __component, __variant)",
    context,
  );
});

test("instance graph still rejects a genuinely wrong composition", async () => {
  const context = pluginContext();
  const master = {
    type: "COMPONENT",
    children: [fakeInstance("component/unexpected", "component/unexpected/variant/q")],
  };
  context.__master = master;
  context.__component = { id: "component/preview" };
  context.__variant = {
    id: "preview-default",
    instances: [{ component_source_id: "component/book", variant_source_id: "component/book/variant/x" }],
  };
  await assert.rejects(
    () => vm.runInContext("validateVariantInstances(__master, __component, __variant)", context),
    /wrong instance graph/,
  );
});
