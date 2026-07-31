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

// --- component style round trip ------------------------------------------
//
// These prove the export half of the style loop: what the plugin emits must be
// exactly what the Go receiver expects, and every ambiguity must be refused in
// the tool where the designer can still fix it.

function styleBaseline() {
  return {
    schemaVersion: "pk.design.component-styles.v1",
    stylesheet: "collect.css",
    digest: `sha256:${"a".repeat(64)}`,
    styles: [
      { selector: ".pill", property: "padding-left", value: "10px" },
      { selector: ".pill", property: "gap", value: "4px" },
      { selector: ".collect-icon", property: "border-radius", value: "8px" },
    ],
  };
}

// styleContext installs a baseline and a page of fake styled nodes. The plugin
// reads nodes through sharedPluginValue, which tolerates plain objects, so a
// faithful test needs no Figma runtime.
function styleContext(nodes, baseline = styleBaseline()) {
  const context = pluginContext();
  const store = new Map();
  context.figma.clientStorage = {
    async getAsync(key) {
      return store.get(key);
    },
    async setAsync(key, value) {
      store.set(key, value);
    },
  };
  context.figma.root.setPluginData = () => {};
  context.figma.root.getPluginData = () => "";
  context.figma.root.children = [
    {
      findAll(predicate) {
        return nodes.filter(predicate);
      },
    },
  ];
  context.__baseline = baseline;
  return context;
}

function styledNode(selector, properties) {
  return {
    getSharedPluginData(namespace, key) {
      return key === "pkStyleSelector" ? selector : "";
    },
    ...properties,
  };
}

test("style export emits the coordinate the Go receiver expects", async () => {
  const context = styleContext([styledNode(".pill", { paddingLeft: 12, itemSpacing: 4 })]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);

  const result = await vm.runInContext(
    `exportStyleChanges({ author: "designer", reason: "roomier pills" })`,
    context,
  );
  const payload = JSON.parse(result.payload);

  assert.equal(payload.schemaVersion, "pk.design.component-style-changes.v1");
  assert.equal(payload.stylesheet, "collect.css");
  assert.equal(payload.baseDigest, styleBaseline().digest);
  assert.equal(payload.changes.length, 1, "only the edited declaration is a change");

  const change = payload.changes[0];
  assert.equal(change.before.selector, ".pill");
  assert.equal(change.before.property, "padding-left");
  assert.equal(change.before.value, "10px");
  assert.equal(change.after.value, "12px");
  assert.equal(result.filename, "collect.css-style-changes.json");
});

test("style export ignores declarations the designer left alone", async () => {
  const context = styleContext([styledNode(".pill", { paddingLeft: 10, itemSpacing: 4 })]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);

  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "d", reason: "r" })`, context),
    /No managed component style changes were detected/,
  );
});

test("style export requires author and reason so a change set records provenance", async () => {
  const context = styleContext([styledNode(".pill", { paddingLeft: 12 })]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);

  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "", reason: "r" })`, context),
    /author is required/i,
  );
  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "d", reason: "  " })`, context),
    /reason is required/i,
  );
});

test("style export refuses when one declaration is expressed by two nodes", async () => {
  const context = styleContext([
    styledNode(".pill", { paddingLeft: 12 }),
    styledNode(".pill", { paddingLeft: 14 }),
  ]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);

  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "d", reason: "r" })`, context),
    /cannot be attributed/,
  );
});

test("style export never proposes a declaration the stylesheet does not own", async () => {
  // font-size is not in the baseline: emitting it would ask the receiver to
  // create a CSS rule, which the protocol refuses by design.
  const context = styleContext([styledNode(".pill", { paddingLeft: 12, fontSize: 18 })]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);

  const result = await vm.runInContext(
    `exportStyleChanges({ author: "d", reason: "r" })`,
    context,
  );
  const payload = JSON.parse(result.payload);
  assert.equal(payload.changes.length, 1);
  assert.equal(payload.changes[0].before.property, "padding-left");
});

test("style export refuses without an imported baseline", async () => {
  const context = styleContext([styledNode(".pill", { paddingLeft: 12 })]);
  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "d", reason: "r" })`, context),
    /Import the design bundle/,
  );
});

test("style export refuses when no managed styled nodes exist", async () => {
  const context = styleContext([]);
  await vm.runInContext("storeStyleBaseline(__baseline)", context);
  await assert.rejects(
    () => vm.runInContext(`exportStyleChanges({ author: "d", reason: "r" })`, context),
    /No managed styled nodes/,
  );
});
