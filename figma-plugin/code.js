// PlatformKit OSS Design Delivery plugin.
// Shared plugin data is the canonical cross-tool metadata channel. The
// kebab-case values below are read-only compatibility for files written by
// the original private-data bridge.
const SHARED_PLUGIN_NAMESPACE = "platformkit";
const KEY_PRODUCT = "pkProduct";
const KEY_PROFILE = "pkProfile";
const KEY_CLIENT_ID = "pkClientId";
const KEY_CONTRACT_VERSION = "pkContractVersion";
const KEY_CLIENT_SOURCE_DIGEST = "pkClientSourceDigest";
const KEY_EXTENSION_DIGEST = "pkExtensionDigest";
const KEY_MANIFEST_DIGEST = "pkManifestDigest";
const KEY_PARENT_PROFILE = "pkParentProfile";
const KEY_PARENT_MANIFEST_DIGEST = "pkParentManifestDigest";
const KEY_BUNDLE_DIGEST = "pkBundleDigest";

const ARTIFACT_BUNDLE_KIND = "platformkit.figma.plugin-bundle";
const ARTIFACT_BUNDLE_SCHEMA = "platformkit.figma.plugin-bundle.v5";
const GOVERNED_TOKEN_SCHEMA = "pk.design.figma-variables.v2";
const TOKEN_SNAPSHOT_SCHEMA = "pk.design.token-snapshot.v1";
const TOKEN_CHANGESET_SCHEMA = "pk.design.token-changes.v1";
const VISUAL_DELIVERY_SCHEMA = "platformkit.storybook.visual-delivery.v2";
const NATIVE_DELIVERY_SCHEMA = "platformkit.figma.native-delivery.v1";
const NATIVE_CONFORMANCE_SCHEMA = "platformkit.figma.native-conformance.v1";
const SOLUTION_PAGE_PREFIX = "10 Solution · ";
const TOKEN_STORAGE_KEY = "platformkit.governed-token-bundle.v2";
const TOKEN_ROOT_CHUNK_PREFIX = "governedTokenBundle";
const TOKEN_ROOT_CHUNK_SIZE = 48000;

// Component style round trip. Styles are carried as CSS declaration values, so
// the payload is independent of how a component's renderer branches or loops —
// which is what lets it serve every component rather than the subset a
// structural model can express.
const STYLE_SNAPSHOT_SCHEMA = "pk.design.component-styles.v1";
const STYLE_CHANGESET_SCHEMA = "pk.design.component-style-changes.v1";
const STYLE_STORAGE_KEY = "platformkit.component-styles.v1";
const STYLE_ROOT_CHUNK_PREFIX = "componentStyleBaseline";

// A managed node declares which CSS rule it stands for. Without this stamp a
// node's geometry cannot be attributed to a selector, so unstamped nodes are
// skipped rather than guessed at.
const KEY_STYLE_SELECTOR = "pkStyleSelector";

// Figma node geometry mapped onto the CSS properties it represents. The map is
// deliberately small: every entry round-trips losslessly through a plain
// numeric pixel value, which is the only kind of edit this protocol claims to
// carry.
const STYLE_NODE_PROPERTIES = [
  { node: "paddingLeft", css: "padding-left", unit: "px" },
  { node: "paddingRight", css: "padding-right", unit: "px" },
  { node: "paddingTop", css: "padding-top", unit: "px" },
  { node: "paddingBottom", css: "padding-bottom", unit: "px" },
  { node: "itemSpacing", css: "gap", unit: "px" },
  { node: "cornerRadius", css: "border-radius", unit: "px" },
  { node: "strokeWeight", css: "border-width", unit: "px" },
  { node: "fontSize", css: "font-size", unit: "px" },
];
const NATIVE_DELIVERY_STORAGE_KEY = "platformkit.native-delivery-contract.v1";
const NATIVE_DELIVERY_ROOT_CHUNK_PREFIX = "nativeDeliveryContract";

figma.showUI(__html__, {
  width: 520,
  height: 720,
  themeColors: true
});

figma.ui.onmessage = async (message) => {
  if (!message || typeof message.type !== "string") {
    return;
  }

  if (message.type === "close") {
    figma.closePlugin();
    return;
  }

  if (message.type === "import-artifacts") {
    try {
      const parsed = await parseArtifactBundlePayload(message.payload);
      const result = await importArtifactBundle(parsed);
      figma.ui.postMessage({
        type: "artifacts-success",
        result
      });
      figma.notify(
        `Imported ${result.tokenDelivery.variables} native variables and ${result.artifacts} native Figma artifacts`
      );
    } catch (error) {
      const messageText = error instanceof Error ? error.message : String(error);
      figma.ui.postMessage({
        type: "artifacts-error",
        message: messageText
      });
      figma.notify(`PlatformKit artifact import failed: ${messageText}`, { error: true });
    }
    return;
  }

  if (message.type === "import-tokens") {
    try {
      const parsed = await parseTokenPayload(message.payload);
      const result = await importTokens(parsed);
      figma.ui.postMessage({
        type: "tokens-success",
        result
      });
      figma.notify(`Imported ${result.variables} variables across ${result.collections} collections`);
    } catch (error) {
      const messageText = error instanceof Error ? error.message : String(error);
      figma.ui.postMessage({
        type: "tokens-error",
        message: messageText
      });
      figma.notify(`PlatformKit token import failed: ${messageText}`, { error: true });
    }
    return;
  }

  if (message.type === "export-style-changes") {
    try {
      const result = await exportStyleChanges({
        author: stringValue(message.author),
        reason: stringValue(message.reason),
      });
      figma.ui.postMessage({ type: "style-changes-success", result });
      figma.notify(`Exported ${result.changes} style change(s).`);
    } catch (error) {
      const text = error && error.message ? error.message : String(error);
      figma.ui.postMessage({ type: "style-changes-error", message: text });
      figma.notify(text, { error: true });
    }
    return;
  }

  if (message.type === "export-token-changes") {
    try {
      const result = await exportTokenChanges({
        author: stringValue(message.author),
        reason: stringValue(message.reason)
      });
      figma.ui.postMessage({
        type: "changes-success",
        result
      });
      figma.notify(`Exported ${result.changes} governed token change${result.changes === 1 ? "" : "s"}`);
    } catch (error) {
      const messageText = error instanceof Error ? error.message : String(error);
      figma.ui.postMessage({
        type: "changes-error",
        message: messageText
      });
      figma.notify(`PlatformKit token export failed: ${messageText}`, { error: true });
    }
    return;
  }

  if (message.type === "validate-native-delivery") {
    try {
      const result = await validateNativeDelivery(message.payload);
      figma.ui.postMessage({
        type: "native-conformance-success",
        result
      });
      figma.notify(
        `Validated ${result.fixtures} native solution fixture${result.fixtures === 1 ? "" : "s"}`
      );
    } catch (error) {
      const messageText = error instanceof Error ? error.message : String(error);
      figma.ui.postMessage({
        type: "native-conformance-error",
        message: messageText
      });
      figma.notify(`PlatformKit native delivery validation failed: ${messageText}`, { error: true });
    }
    return;
  }
};

async function parseArtifactBundlePayload(payload) {
  if (typeof payload !== "string") {
    throw new Error("artifact bundle payload must be a JSON string");
  }

  let parsed;
  try {
    parsed = JSON.parse(payload);
  } catch (error) {
    throw new Error("artifact bundle is not valid JSON");
  }

  await validateArtifactBundle(parsed);
  return parsed;
}

async function validateArtifactBundle(bundle) {
  if (!bundle || typeof bundle !== "object" || Array.isArray(bundle)) {
    throw new Error("artifact bundle must be a JSON object");
  }
  if (bundle.kind !== ARTIFACT_BUNDLE_KIND) {
    throw new Error(`artifact bundle kind must be "${ARTIFACT_BUNDLE_KIND}", got "${bundle.kind}"`);
  }
  if (bundle.schema !== ARTIFACT_BUNDLE_SCHEMA) {
    throw new Error(`artifact bundle schema must be "${ARTIFACT_BUNDLE_SCHEMA}", got "${bundle.schema}"`);
  }
  if (typeof bundle.version !== "string" || bundle.version.trim() === "") {
    throw new Error("artifact bundle version is required");
  }
  if (typeof bundle.target !== "string" || bundle.target.trim() === "") {
    throw new Error("artifact bundle target is required");
  }
  for (const key of [
    "product",
    "profile",
    "clientId",
    "contractVersion",
    "clientSourceDigest",
    "extensionDigest",
    "manifestDigest",
    "parentProfile",
    "parentManifestDigest",
  ]) {
    if (bundle[key] !== undefined && typeof bundle[key] !== "string") {
      throw new Error(`artifact bundle ${key} must be a string when provided`);
    }
  }
  validateArtifactBundleProfile(bundle);
  if (stringValue(bundle.contractVersion) === "") {
    throw new Error("artifact bundle contractVersion is required");
  }
  if (!isSHA256Hex(bundle.manifestDigest)) {
    throw new Error("artifact bundle requires a valid manifestDigest");
  }
  if (!Array.isArray(bundle.artifacts) || bundle.artifacts.length === 0) {
    throw new Error("artifact bundle must contain at least one artifact");
  }
  if (!bundle.tokenBundle || typeof bundle.tokenBundle !== "object" || Array.isArray(bundle.tokenBundle)) {
    throw new Error("artifact bundle requires one governed tokenBundle");
  }
  await validateTokenBundle(bundle.tokenBundle);
  if (bundle.tokenBundle.schema !== GOVERNED_TOKEN_SCHEMA) {
    throw new Error(`artifact tokenBundle schema must be "${GOVERNED_TOKEN_SCHEMA}"`);
  }
  if (stringValue(bundle.product) !== stringValue(bundle.tokenBundle.product)) {
    throw new Error("artifact bundle product does not match its governed tokenBundle");
  }
  if (stringValue(bundle.clientId) !== stringValue(bundle.tokenBundle.clientId)) {
    throw new Error("artifact bundle clientId does not match its governed tokenBundle");
  }
  if (stringValue(bundle.profile) !== visualDeliveryProfile(bundle.tokenBundle)) {
    throw new Error("artifact bundle profile does not match its governed tokenBundle");
  }
  await validateNativeDeliveryContract(bundle.nativeDelivery);
  if (stringValue(bundle.clientId) !== stringValue(bundle.nativeDelivery.client_id)) {
    throw new Error("artifact bundle clientId does not match its nativeDelivery");
  }
  if (!isSHA256Hex(bundle.bundleDigest)) {
    throw new Error("artifact bundle requires a valid bundleDigest");
  }

  for (const artifact of bundle.artifacts) {
    if (!artifact || typeof artifact !== "object" || Array.isArray(artifact)) {
      throw new Error("each artifact must be an object");
    }
    if (typeof artifact.kind !== "string" || artifact.kind.trim() === "") {
      throw new Error("each artifact requires kind");
    }
    if (artifact.kind.trim().toLowerCase() === "bootstrap") {
      throw new Error("executable token bootstrap artifacts are retired; use tokenBundle");
    }
    if (typeof artifact.path !== "string" || artifact.path.trim() === "") {
      throw new Error("each artifact requires path");
    }
    const normalizedPath = normalizeArtifactPath(artifact.path);
    if (normalizedPath === "") {
      throw new Error(`artifact ${artifact.path} must stay inside the bundle root`);
    }
    if (artifact.path.replaceAll("\\", "/") !== normalizedPath || !normalizedPath.toLowerCase().endsWith(".js")) {
      throw new Error(`artifact ${artifact.path} must be a relative JavaScript file`);
    }
    if (typeof artifact.name !== "undefined" && typeof artifact.name !== "string") {
      throw new Error(`artifact ${artifact.path} name must be a string when provided`);
    }
    if (typeof artifact.language !== "string" || artifact.language.toLowerCase() !== "javascript") {
      throw new Error(`artifact ${artifact.path} has unsupported language ${artifact.language}`);
    }
    if (typeof artifact.source !== "string" || artifact.source.trim() === "") {
      throw new Error(`artifact ${artifact.path} requires source`);
    }
    if (artifact.source.length > 10 * 1024 * 1024) {
      throw new Error(`artifact ${artifact.path} exceeds the 10 MiB source limit`);
    }
    if (!isSHA256Hex(artifact.sha256)) {
      throw new Error(`artifact ${artifact.path} requires a valid sha256 digest`);
    }
  }

  const paths = bundle.artifacts.map((artifact) => normalizeArtifactPath(artifact.path));
  if (new Set(paths).size !== paths.length) {
    throw new Error("artifact bundle contains duplicate paths");
  }

  await verifyArtifactBundleIntegrity(bundle);
}

function normalizeArtifactPath(path) {
  const raw = String(path || "").replaceAll("\\", "/");
  if (raw === "" || raw.startsWith("/") || /^[A-Za-z]:\//.test(raw)) {
    return "";
  }
  const parts = [];
  for (const part of raw.split("/")) {
    if (part === "" || part === ".") {
      continue;
    }
    if (part === "..") {
      return "";
    }
    parts.push(part);
  }
  return parts.join("/");
}

function isSHA256Hex(value) {
  return typeof value === "string" && /^[a-f0-9]{64}$/i.test(value.trim());
}

function validateArtifactBundleProfile(bundle) {
  const profile = stringValue(bundle.profile);
  const clientId = stringValue(bundle.clientId);
  const parentProfile = stringValue(bundle.parentProfile);
  const parentManifestDigest = stringValue(bundle.parentManifestDigest);

  if (profile === "oss") {
    if (clientId !== "") {
      throw new Error("OSS artifact bundle cannot declare a client");
    }
    if (parentProfile !== "" || parentManifestDigest !== "") {
      throw new Error("OSS artifact bundle cannot declare parent provenance");
    }
    return;
  }
  if (profile === "pro") {
    if (clientId !== "") {
      throw new Error("Pro artifact bundle cannot declare a client");
    }
    if (parentProfile !== "oss" || !isSHA256Hex(parentManifestDigest)) {
      throw new Error("Pro artifact bundle requires exact OSS parent provenance");
    }
    return;
  }
  if (profile.startsWith("clients/")) {
    const profileClientId = profile.slice("clients/".length);
    if (profileClientId === "" || profileClientId !== clientId) {
      throw new Error(
        `client artifact bundle profile "${profile}" does not match client "${clientId}"`
      );
    }
    if (parentProfile !== "pro" || !isSHA256Hex(parentManifestDigest)) {
      throw new Error("client artifact bundle requires exact Pro parent provenance");
    }
    return;
  }
  throw new Error(`unsupported artifact bundle profile "${profile}"`);
}

async function sha256Hex(value) {
  if (typeof crypto === "undefined" || !crypto.subtle || typeof TextEncoder !== "function") {
    throw new Error("Figma runtime does not provide Web Crypto for bundle integrity verification");
  }
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
}

function canonicalArtifactBundle(bundle) {
  const canonical = {};
  for (const key of [
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
  ]) {
    if (key === "tokenBundle") {
      canonical[key] = canonicalTokenBundle(bundle.tokenBundle);
    } else if (key === "nativeDelivery") {
      canonical[key] = canonicalNativeDelivery(bundle.nativeDelivery);
    } else if (key === "artifacts") {
      canonical[key] = bundle.artifacts.map(canonicalArtifact);
    } else if (isNonEmptyStringProperty(bundle, key)) {
      canonical[key] = bundle[key];
    }
  }
  return canonical;
}

function canonicalNativeDelivery(contract) {
  const canonical = {
    schema: contract.schema,
    version: contract.version,
  };
  copyOptionalString(canonical, contract, "client_id");
  canonical.components = contract.components.map(canonicalNativeComponent);
  if (Array.isArray(contract.canvases) && contract.canvases.length > 0) {
    canonical.canvases = contract.canvases.map(canvas => {
      const item = { page_name: canvas.page_name };
      copyOptionalString(item, canvas, "client_id");
      item.root_name = canvas.root_name;
      return item;
    });
  }
  if (Array.isArray(contract.solutions) && contract.solutions.length > 0) {
    canonical.solutions = contract.solutions.map(canonicalNativeSolution);
  }
  return canonical;
}

function canonicalNativeComponent(component) {
  const canonical = {
    id: component.id,
    source_id: component.source_id,
    name: component.name,
    category: component.category,
  };
  if (Array.isArray(component.props) && component.props.length > 0) {
    canonical.props = component.props.map(canonicalNativeProp);
  }
  if (Array.isArray(component.variant_axes) && component.variant_axes.length > 0) {
    canonical.variant_axes = component.variant_axes.map(axis => ({
      name: axis.name,
      values: [...axis.values],
    }));
  }
  if (Array.isArray(component.slots) && component.slots.length > 0) {
    canonical.slots = component.slots.map(canonicalAuthoredSlot);
  }
  canonical.variants = component.variants.map(canonicalNativeVariant);
  return canonical;
}

function canonicalNativeProp(prop) {
  const canonical = { name: prop.name };
  copyOptionalString(canonical, prop, "type");
  copyOptionalString(canonical, prop, "role");
  if (prop.required === true) canonical.required = true;
  if (Object.prototype.hasOwnProperty.call(prop, "default")) {
    canonical.default = canonicalJSONValue(prop.default);
  }
  copyOptionalString(canonical, prop, "description");
  if (Array.isArray(prop.enum) && prop.enum.length > 0) canonical.enum = [...prop.enum];
  if (Array.isArray(prop.enum_labels) && prop.enum_labels.length > 0) {
    canonical.enum_labels = [...prop.enum_labels];
  }
  if (Array.isArray(prop.examples) && prop.examples.length > 0) {
    canonical.examples = prop.examples.map(canonicalJSONValue);
  }
  if (typeof prop.min === "number") canonical.min = prop.min;
  if (typeof prop.max === "number") canonical.max = prop.max;
  copyOptionalString(canonical, prop, "pattern");
  return canonical;
}

function canonicalAuthoredSlot(slot) {
  const canonical = { name: slot.name };
  copyOptionalString(canonical, slot, "description");
  if (slot.required === true) canonical.required = true;
  if (Array.isArray(slot.allowed_types) && slot.allowed_types.length > 0) {
    canonical.allowed_types = [...slot.allowed_types];
  }
  canonical.cardinality = slot.cardinality;
  if (Array.isArray(slot.attrs) && slot.attrs.length > 0) {
    canonical.attrs = slot.attrs.map(canonicalNativeProp);
  }
  if (slot.scope !== undefined) {
    canonical.scope = {};
    if (Array.isArray(slot.scope.fields) && slot.scope.fields.length > 0) {
      canonical.scope.fields = slot.scope.fields.map(canonicalNativeProp);
    }
  }
  return canonical;
}

function canonicalNativeVariant(variant) {
  const canonical = {
    id: variant.id,
    source_id: variant.source_id,
  };
  if (hasObjectEntries(variant.properties)) {
    canonical.properties = canonicalJSONValue(variant.properties);
  }
  if (Array.isArray(variant.component_properties) && variant.component_properties.length > 0) {
    canonical.component_properties = variant.component_properties.map(property => {
      const item = { name: property.name, type: property.type };
      if (Object.prototype.hasOwnProperty.call(property, "default")) {
        item.default = canonicalJSONValue(property.default);
      }
      copyOptionalString(item, property, "target_id");
      copyOptionalString(item, property, "target_source_id");
      return item;
    });
  }
  if (Array.isArray(variant.slots) && variant.slots.length > 0) {
    canonical.slots = variant.slots.map(slot => {
      const item = {
        name: slot.name,
        property_name: slot.property_name,
      };
      if (slot.required === true) item.required = true;
      item.cardinality = slot.cardinality;
      if (Array.isArray(slot.allowed_types) && slot.allowed_types.length > 0) {
        item.allowed_types = [...slot.allowed_types];
      }
      item.min_children = slot.min_children;
      if (typeof slot.max_children === "number") item.max_children = slot.max_children;
      return item;
    });
  }
  if (Array.isArray(variant.instances) && variant.instances.length > 0) {
    canonical.instances = variant.instances.map(instance => {
      const item = {
        reference: instance.reference,
        component_id: instance.component_id,
        component_source_id: instance.component_source_id,
      };
      copyOptionalString(item, instance, "variant_id");
      copyOptionalString(item, instance, "variant_source_id");
      return item;
    });
  }
  return canonical;
}

function canonicalNativeSolution(solution) {
  const canonical = { page_name: solution.page_name };
  copyOptionalString(canonical, solution, "client_id");
  canonical.component_id = solution.component_id;
  canonical.component_source_id = solution.component_source_id;
  canonical.variant_id = solution.variant_id;
  canonical.variant_source_id = solution.variant_source_id;
  canonical.width = solution.width;
  canonical.height = solution.height;
  copyOptionalString(canonical, solution, "color_mode");
  return canonical;
}

function canonicalTokenBundle(bundle) {
  const canonical = {
    kind: bundle.kind,
    schema: bundle.schema,
    product: bundle.product,
  };
  copyOptionalString(canonical, bundle, "clientId");
  canonical.snapshotDigest = bundle.snapshotDigest;
  canonical.snapshot = canonicalTokenSnapshot(bundle.snapshot);
  canonical.collections = bundle.collections.map(canonicalTokenCollection);
  if (Array.isArray(bundle.diagnostics) && bundle.diagnostics.length > 0) {
    canonical.diagnostics = bundle.diagnostics.map((diagnostic) => {
      const item = { code: diagnostic.code };
      copyOptionalString(item, diagnostic, "path");
      item.message = diagnostic.message;
      return item;
    });
  }
  if (hasObjectEntries(bundle.metadata)) {
    canonical.metadata = canonicalJSONValue(bundle.metadata);
  }
  return canonical;
}

function canonicalTokenSnapshot(snapshot) {
  const canonical = {
    schemaVersion: snapshot.schemaVersion,
    profile: canonicalTokenProfile(snapshot.profile),
    provenance: canonicalTokenProvenance(snapshot.provenance),
    tokens: snapshot.tokens.map(canonicalSnapshotToken),
  };
  if (hasObjectEntries(snapshot.metadata)) {
    canonical.metadata = canonicalJSONValue(snapshot.metadata);
  }
  return canonical;
}

function canonicalTokenProfile(profile) {
  const canonical = {
    id: profile.id,
    kind: profile.kind,
    version: profile.version,
  };
  for (const key of ["clientId", "parentId", "parentDigest"]) {
    copyOptionalString(canonical, profile, key);
  }
  return canonical;
}

function canonicalTokenProvenance(provenance) {
  const canonical = {
    source: provenance.source,
    generatedAt: provenance.generatedAt,
  };
  copyOptionalString(canonical, provenance, "author");
  copyOptionalString(canonical, provenance, "reason");
  return canonical;
}

function canonicalSnapshotToken(token) {
  const canonical = {
    path: token.path,
    mode: token.mode,
  };
  copyOptionalString(canonical, token, "type");
  canonical.value = canonicalJSONValue(token.value);
  copyOptionalString(canonical, token, "description");
  canonical.origin = {
    owner: token.origin.owner,
    layer: token.origin.layer,
    sourceUri: token.origin.sourceUri,
  };
  copyOptionalString(canonical.origin, token.origin, "pointer");
  canonical.origin.writable = token.origin.writable;
  return canonical;
}

function canonicalTokenCollection(collection) {
  return {
    name: collection.name,
    modes: collection.modes.slice(),
    variables: collection.variables.map(canonicalTokenVariable),
  };
}

function canonicalTokenVariable(variable) {
  const canonical = {
    key: variable.key,
    name: variable.name,
    type: variable.type,
  };
  copyOptionalString(canonical, variable, "description");
  if (hasObjectEntries(variable.values)) {
    canonical.values = canonicalJSONValue(variable.values);
  }
  if (hasObjectEntries(variable.aliases)) {
    canonical.aliases = canonicalJSONValue(variable.aliases);
  }
  canonical.binding = {
    tokenPath: variable.binding.tokenPath,
    modeMap: canonicalJSONValue(variable.binding.modeMap),
  };
  if (hasObjectEntries(variable.binding.codecs)) {
    canonical.binding.codecs = canonicalStringMap(
      variable.binding.codecs,
      canonicalTokenCodec,
    );
  }
  canonical.binding.writable = variable.binding.writable;
  return canonical;
}

function canonicalTokenCodec(codec) {
  const canonical = { kind: codec.kind };
  copyOptionalString(canonical, codec, "style");
  copyOptionalString(canonical, codec, "unit");
  if (typeof codec.scale === "number" && codec.scale !== 0) {
    canonical.scale = codec.scale;
  }
  if (typeof codec.precision === "number" && codec.precision !== 0) {
    canonical.precision = codec.precision;
  }
  return canonical;
}

function canonicalJSONValue(value) {
  if (Array.isArray(value)) {
    return value.map(canonicalJSONValue);
  }
  if (!value || typeof value !== "object") {
    return value;
  }
  return canonicalStringMap(value, canonicalJSONValue);
}

function canonicalStringMap(value, transform) {
  const canonical = {};
  for (const key of Object.keys(value).sort()) {
    canonical[key] = transform(value[key]);
  }
  return canonical;
}

function copyOptionalString(target, source, key) {
  if (isNonEmptyStringProperty(source, key)) {
    target[key] = source[key];
  }
}

function hasObjectEntries(value) {
  return value && typeof value === "object" && !Array.isArray(value) && Object.keys(value).length > 0;
}

function canonicalArtifact(artifact) {
  const canonical = {};
  for (const key of ["kind", "name", "path", "language", "source", "sha256"]) {
    if (key !== "name" || isNonEmptyStringProperty(artifact, key)) {
      canonical[key] = artifact[key];
    }
  }
  return canonical;
}

function isNonEmptyStringProperty(value, key) {
  return Object.prototype.hasOwnProperty.call(value, key) && typeof value[key] === "string" && value[key].length > 0;
}

async function verifyArtifactBundleIntegrity(bundle) {
  for (const artifact of bundle.artifacts) {
    const actual = await sha256Hex(artifact.source);
    if (actual.toLowerCase() !== artifact.sha256.trim().toLowerCase()) {
      throw new Error(`artifact ${artifact.path} sha256 mismatch`);
    }
  }
  const actualBundleDigest = await sha256Hex(JSON.stringify(canonicalArtifactBundle(bundle)));
  if (actualBundleDigest.toLowerCase() !== bundle.bundleDigest.trim().toLowerCase()) {
    throw new Error("artifact bundle digest mismatch");
  }
}

async function importArtifactBundle(bundle) {
  await ensureDocumentLoaded();
  // Figma has no transaction boundary for arbitrary plugin mutations. Parse
  // every artifact before executing the first one so syntax failures cannot
  // leave a partially regenerated file.
  preflightArtifactSources(bundle);
  const tokenDelivery = await importTokens(bundle.tokenBundle);
  const counts = {};
  const results = [];
  const total = bundle.artifacts.length;

  for (let index = 0; index < total; index++) {
    const artifact = bundle.artifacts[index];
    const label = artifactLabel(artifact);
    figma.ui.postMessage({
      type: "artifacts-progress",
      current: index + 1,
      total,
      artifact: label
    });

    const result = await executeArtifact(artifact, {
      product: bundle.product || "",
      version: bundle.version,
      profile: bundle.profile,
      clientId: bundle.clientId || "",
      contractVersion: bundle.contractVersion || "",
      clientSourceDigest: bundle.clientSourceDigest || "",
      extensionDigest: bundle.extensionDigest || "",
      manifestDigest: bundle.manifestDigest,
      parentProfile: bundle.parentProfile || "",
      parentManifestDigest: bundle.parentManifestDigest || "",
      bundleDigest: bundle.bundleDigest || "",
      index,
      total
    });
    counts[artifact.kind] = (counts[artifact.kind] || 0) + 1;
    results.push({
      kind: artifact.kind,
      name: artifact.name || "",
      path: artifact.path,
      result
    });
  }

  if (stringValue(bundle.product) !== "") {
    setMetadata(figma.root, KEY_PRODUCT, bundle.product);
  }
  setArtifactBundleMetadata(figma.root, bundle);
  const nativeConformance = await validateNativeContract(
    bundle.nativeDelivery,
    stringValue(bundle.clientId)
  );
  await storeNativeDeliveryContract(bundle.nativeDelivery);

  return {
    product: bundle.product || "",
    profile: bundle.profile,
    parentProfile: bundle.parentProfile || "",
    artifacts: total,
    tokenDelivery,
    nativeConformance,
    counts,
    results
  };
}

function preflightArtifactSources(bundle) {
  for (const artifact of bundle.artifacts) {
    try {
      // Constructing the function parses the source without running it.
      new Function(
        "figma",
        "platformkit",
        `"use strict"; return (async () => {\n${artifact.source}\n})();`
      );
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`${artifactLabel(artifact)} failed syntax preflight: ${message}`);
    }
  }
}

async function executeArtifact(artifact, context) {
  const label = artifactLabel(artifact);
  try {
    const run = new Function(
      "figma",
      "platformkit",
      `"use strict"; return (async () => {\n${artifact.source}\n})();`
    );
    const result = await run(figma, context);
    if (result === undefined || result === null) {
      return "";
    }
    if (typeof result === "string") {
      return result;
    }
    return JSON.stringify(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`${label} failed: ${message}`);
  }
}

function artifactLabel(artifact) {
  const name = stringValue(artifact.name);
  if (name !== "") {
    return `${artifact.kind}:${name}`;
  }
  return `${artifact.kind}:${artifact.path}`;
}

function setMetadata(node, key, value) {
  if (!node) {
    return;
  }
  try {
    node.setSharedPluginData(SHARED_PLUGIN_NAMESPACE, key, String(value || ""));
  } catch (error) {
    throw new Error(`cannot write PlatformKit shared metadata ${key}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

function setArtifactBundleMetadata(node, bundle) {
  // The current bundle is authoritative. Clear fields omitted by a core
  // bundle so a previous client import cannot leave stale provenance on the
  // document root.
  setMetadata(node, KEY_PROFILE, bundle.profile);
  setMetadata(node, KEY_CLIENT_ID, typeof bundle.clientId === "string" ? bundle.clientId : "");
  setMetadata(node, KEY_CONTRACT_VERSION, typeof bundle.contractVersion === "string" ? bundle.contractVersion : "");
  setMetadata(node, KEY_CLIENT_SOURCE_DIGEST, typeof bundle.clientSourceDigest === "string" ? bundle.clientSourceDigest : "");
  setMetadata(node, KEY_EXTENSION_DIGEST, typeof bundle.extensionDigest === "string" ? bundle.extensionDigest : "");
  setMetadata(node, KEY_MANIFEST_DIGEST, bundle.manifestDigest);
  setMetadata(node, KEY_PARENT_PROFILE, typeof bundle.parentProfile === "string" ? bundle.parentProfile : "");
  setMetadata(node, KEY_PARENT_MANIFEST_DIGEST, typeof bundle.parentManifestDigest === "string" ? bundle.parentManifestDigest : "");
  setMetadata(node, KEY_BUNDLE_DIGEST, typeof bundle.bundleDigest === "string" ? bundle.bundleDigest : "");
}

function setTokenMetadata(node, bundle) {
  setMetadata(node, KEY_PROFILE, visualDeliveryProfile(bundle));
  setMetadata(node, KEY_CLIENT_ID, typeof bundle.clientId === "string" ? bundle.clientId : "");
}

function stringValue(value) {
  if (typeof value !== "string") {
    return "";
  }
  return value.trim();
}

async function ensureDocumentLoaded() {
  if (typeof figma.loadAllPagesAsync === "function") {
    await figma.loadAllPagesAsync();
  }
}

// ---------------------------------------------------------------------------
// Native delivery conformance: inspect editable nodes, never export snapshots
// ---------------------------------------------------------------------------

async function validateNativeDeliveryContract(contract) {
  if (!contract || typeof contract !== "object" || Array.isArray(contract)) {
    throw new Error("artifact bundle requires one nativeDelivery object");
  }
  assertOnlyKeys(
    contract,
    ["schema", "version", "client_id", "components", "canvases", "solutions"],
    "nativeDelivery"
  );
  if (contract.schema !== NATIVE_DELIVERY_SCHEMA) {
    throw new Error(
      `native delivery schema must be "${NATIVE_DELIVERY_SCHEMA}", got "${contract.schema || ""}"`
    );
  }
  if (contract.version !== "1") {
    throw new Error(`unsupported native delivery version "${contract.version || ""}"`);
  }
  if (contract.client_id !== undefined && typeof contract.client_id !== "string") {
    throw new Error("native delivery client_id must be a string");
  }
  if (!Array.isArray(contract.components)) {
    throw new Error("native delivery components must be an array");
  }
  if (contract.canvases !== undefined && !Array.isArray(contract.canvases)) {
    throw new Error("native delivery canvases must be an array");
  }
  if (contract.components.length === 0 && (contract.canvases || []).length === 0) {
    throw new Error("native delivery requires at least one component or canvas");
  }
  if (contract.solutions !== undefined && !Array.isArray(contract.solutions)) {
    throw new Error("native delivery solutions must be an array");
  }

  const componentSources = new Map();
  const variantSources = new Map();
  for (const component of contract.components) {
    assertOnlyKeys(
      component,
      ["id", "source_id", "name", "category", "props", "variant_axes", "slots", "variants"],
      "native component"
    );
    for (const key of ["id", "source_id", "name", "category"]) {
      if (stringValue(component[key]) === "") {
        throw new Error(`native component ${key} is required`);
      }
    }
    const expectedSource = componentSourceID(component.id);
    if (component.source_id !== expectedSource) {
      throw new Error(`native component ${component.id} has invalid source_id`);
    }
    if (componentSources.has(component.source_id)) {
      throw new Error(`duplicate native component source_id ${component.source_id}`);
    }
    componentSources.set(component.source_id, component);
    validateAuthoredProps(component.props || [], `native component ${component.id}`);
    validateAuthoredSlots(component.slots || [], `native component ${component.id}`);
    validateVariantAxes(component.variant_axes || [], component.id);
    if (!Array.isArray(component.variants) || component.variants.length === 0) {
      throw new Error(`native component ${component.id} has no variants`);
    }

    const componentVariants = new Map();
    for (const variant of component.variants) {
      assertOnlyKeys(
        variant,
        ["id", "source_id", "properties", "component_properties", "slots", "instances"],
        `native component ${component.id} variant`
      );
      if (stringValue(variant.id) === "" || stringValue(variant.source_id) === "") {
        throw new Error(`native component ${component.id} has an incomplete variant identity`);
      }
      const expectedVariantSource =
        `${component.source_id}/variant/` + await sha256Hex(`authored:${variant.id}`);
      if (variant.source_id !== expectedVariantSource) {
        throw new Error(`native component ${component.id} variant ${variant.id} has invalid source_id`);
      }
      if (componentVariants.has(variant.source_id)) {
        throw new Error(`native component ${component.id} has duplicate variant ${variant.id}`);
      }
      componentVariants.set(variant.source_id, variant);
      if (
        variant.properties !== undefined &&
        (!variant.properties || typeof variant.properties !== "object" || Array.isArray(variant.properties))
      ) {
        throw new Error(`native component ${component.id} variant ${variant.id} properties must be an object`);
      }
      validateNativeComponentProperties(
        variant.component_properties || [],
        `${component.id}/${variant.id}`
      );
      validateNativeSlots(variant.slots || [], `${component.id}/${variant.id}`);
      await validateNativeInstances(variant.instances || [], `${component.id}/${variant.id}`);
    }
    variantSources.set(component.source_id, componentVariants);
  }

  const canvasPages = new Set();
  for (const canvas of contract.canvases || []) {
    assertOnlyKeys(canvas, ["page_name", "client_id", "root_name"], "native canvas");
    if (stringValue(canvas.page_name) === "" || stringValue(canvas.root_name) === "") {
      throw new Error("native canvas has incomplete identity");
    }
    if (stringValue(canvas.client_id) !== stringValue(contract.client_id)) {
      throw new Error(`native canvas ${canvas.page_name} has mismatched client identity`);
    }
    if (canvasPages.has(canvas.page_name)) {
      throw new Error(`duplicate native canvas page ${canvas.page_name}`);
    }
    canvasPages.add(canvas.page_name);
  }

  const solutionKeys = new Set();
  for (const solution of contract.solutions || []) {
    assertOnlyKeys(
      solution,
      [
        "page_name",
        "client_id",
        "component_id",
        "component_source_id",
        "variant_id",
        "variant_source_id",
        "width",
        "height",
        "color_mode",
      ],
      "native solution"
    );
    for (const key of [
      "page_name",
      "component_id",
      "component_source_id",
      "variant_id",
      "variant_source_id",
    ]) {
      if (stringValue(solution[key]) === "") {
        throw new Error(`native solution ${key} is required`);
      }
    }
    if (stringValue(solution.client_id) !== stringValue(contract.client_id)) {
      throw new Error(`native solution ${solution.page_name} has mismatched client identity`);
    }
    if (solution.component_source_id !== componentSourceID(solution.component_id)) {
      throw new Error(`native solution ${solution.page_name} has invalid component source identity`);
    }
    const expectedVariantSource =
      `${solution.component_source_id}/variant/` +
      await sha256Hex(`authored:${solution.variant_id}`);
    if (solution.variant_source_id !== expectedVariantSource) {
      throw new Error(`native solution ${solution.page_name} has invalid variant source identity`);
    }
    if (
      !Number.isFinite(solution.width) ||
      solution.width <= 0 ||
      !Number.isFinite(solution.height) ||
      solution.height <= 0
    ) {
      throw new Error(`native solution ${solution.page_name} has invalid viewport dimensions`);
    }
    if (!canvasPages.has(solution.page_name)) {
      throw new Error(`native solution ${solution.page_name} has no native canvas contract`);
    }
    const variants = variantSources.get(solution.component_source_id);
    if (!componentSources.has(solution.component_source_id) ||
        !variants ||
        !variants.has(solution.variant_source_id)) {
      throw new Error(`native solution ${solution.page_name} references an absent authored variant`);
    }
    const key = nativeSolutionKey(solution);
    if (solutionKeys.has(key)) {
      throw new Error(`duplicate native solution fixture ${key}`);
    }
    solutionKeys.add(key);
  }
  return contract;
}

function assertOnlyKeys(value, allowed, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  const allowedKeys = new Set(allowed);
  for (const key of Object.keys(value)) {
    if (!allowedKeys.has(key)) {
      throw new Error(`${label} contains unsupported field "${key}"`);
    }
  }
}

function componentSourceID(authoredID) {
  const id = stringValue(authoredID);
  return id.startsWith("component/") ? id : `component/${id}`;
}

function validateAuthoredProps(props, label) {
  if (!Array.isArray(props)) throw new Error(`${label} props must be an array`);
  for (const prop of props) {
    assertOnlyKeys(
      prop,
      [
        "name", "type", "role", "required", "default", "description", "enum",
        "enum_labels", "examples", "min", "max", "pattern",
      ],
      `${label} prop`
    );
    if (stringValue(prop.name) === "") throw new Error(`${label} prop name is required`);
  }
}

function validateAuthoredSlots(slots, label) {
  if (!Array.isArray(slots)) throw new Error(`${label} slots must be an array`);
  for (const slot of slots) {
    assertOnlyKeys(
      slot,
      ["name", "description", "required", "allowed_types", "cardinality", "attrs", "scope"],
      `${label} slot`
    );
    if (stringValue(slot.name) === "" || !["one", "many"].includes(slot.cardinality)) {
      throw new Error(`${label} has an invalid authored slot`);
    }
    validateAuthoredProps(slot.attrs || [], `${label} slot ${slot.name}`);
    if (slot.scope !== undefined) {
      assertOnlyKeys(slot.scope, ["fields"], `${label} slot ${slot.name} scope`);
      validateAuthoredProps(slot.scope.fields || [], `${label} slot ${slot.name} scope`);
    }
  }
}

function validateVariantAxes(axes, componentID) {
  if (!Array.isArray(axes)) throw new Error(`native component ${componentID} variant_axes must be an array`);
  const names = new Set();
  for (const axis of axes) {
    assertOnlyKeys(axis, ["name", "values"], `native component ${componentID} variant axis`);
    if (stringValue(axis.name) === "" || !Array.isArray(axis.values) || axis.values.length === 0) {
      throw new Error(`native component ${componentID} has an invalid variant axis`);
    }
    if (names.has(axis.name)) throw new Error(`native component ${componentID} has duplicate axis ${axis.name}`);
    names.add(axis.name);
  }
}

function validateNativeComponentProperties(properties, label) {
  if (!Array.isArray(properties)) throw new Error(`${label} component_properties must be an array`);
  const names = new Set();
  for (const property of properties) {
    assertOnlyKeys(
      property,
      ["name", "type", "default", "target_id", "target_source_id"],
      `${label} component property`
    );
    const name = stringValue(property.name);
    if (name === "" || !["TEXT", "BOOLEAN", "INSTANCE_SWAP", "SLOT"].includes(property.type)) {
      throw new Error(`${label} has an invalid native component property`);
    }
    if (names.has(name.toLowerCase())) throw new Error(`${label} has duplicate native property ${name}`);
    names.add(name.toLowerCase());
    if (
      property.type === "INSTANCE_SWAP" &&
      (stringValue(property.target_id) === "" ||
       property.target_source_id !== componentSourceID(property.target_id))
    ) {
      throw new Error(`${label} instance-swap property ${name} has invalid target identity`);
    }
    if (
      property.type !== "INSTANCE_SWAP" &&
      (stringValue(property.target_id) !== "" ||
       stringValue(property.target_source_id) !== "")
    ) {
      throw new Error(`${label} property ${name} has an unexpected instance target`);
    }
  }
}

function validateNativeSlots(slots, label) {
  if (!Array.isArray(slots)) throw new Error(`${label} slots must be an array`);
  const names = new Set();
  for (const slot of slots) {
    assertOnlyKeys(
      slot,
      [
        "name", "property_name", "required", "cardinality",
        "allowed_types", "min_children", "max_children",
      ],
      `${label} native slot`
    );
    const name = stringValue(slot.name);
    const propertyName = stringValue(slot.property_name);
    if (
      name === "" ||
      propertyName === "" ||
      !["one", "many"].includes(slot.cardinality) ||
      !Number.isInteger(slot.min_children) ||
      slot.min_children < 0 ||
      (slot.max_children !== undefined &&
       (!Number.isInteger(slot.max_children) || slot.max_children < slot.min_children))
    ) {
      throw new Error(`${label} has an invalid native slot`);
    }
    if (names.has(propertyName.toLowerCase())) {
      throw new Error(`${label} has duplicate native slot property ${propertyName}`);
    }
    names.add(propertyName.toLowerCase());
  }
}

async function validateNativeInstances(instances, label) {
  if (!Array.isArray(instances)) throw new Error(`${label} instances must be an array`);
  for (const instance of instances) {
    assertOnlyKeys(
      instance,
      ["reference", "component_id", "component_source_id", "variant_id", "variant_source_id"],
      `${label} native instance`
    );
    if (
      stringValue(instance.reference) === "" ||
      stringValue(instance.component_id) === "" ||
      instance.component_source_id !== componentSourceID(instance.component_id) ||
      (stringValue(instance.variant_id) === "") !== (stringValue(instance.variant_source_id) === "")
    ) {
      throw new Error(`${label} has an invalid native instance edge`);
    }
    if (stringValue(instance.variant_id) !== "") {
      const expectedVariantSource =
        `${instance.component_source_id}/variant/` +
        await sha256Hex(`authored:${instance.variant_id}`);
      if (instance.variant_source_id !== expectedVariantSource) {
        throw new Error(`${label} native instance ${instance.reference} has invalid variant source identity`);
      }
    }
  }
}

async function validateNativeContract(contract, expectedClientId) {
  await validateNativeDeliveryContract(contract);
  if (stringValue(contract.client_id) !== stringValue(expectedClientId)) {
    throw new Error("native delivery contract does not match the active client scope");
  }
  await ensureDocumentLoaded();

  let variantsValidated = 0;
  let propertiesValidated = 0;
  let slotsValidated = 0;
  let instancesValidated = 0;
  let canvasesValidated = 0;
  for (const canvas of contract.canvases || []) {
    validateNativeCanvas(canvas, expectedClientId);
    canvasesValidated++;
  }
  for (const component of contract.components) {
    const sets = [];
    for (const page of figma.root.children) {
      if (page.type !== "PAGE" || typeof page.findAll !== "function") continue;
      for (const candidate of page.findAll(node =>
        node.type === "COMPONENT_SET" &&
        sharedPluginValue(node, "pkSourceId") === component.source_id &&
        sharedPluginValue(node, KEY_CLIENT_ID) === stringValue(expectedClientId)
      )) {
        sets.push(candidate);
      }
    }
    if (sets.length !== 1) {
      throw new Error(
        `native component ${component.id} resolved ${sets.length} ComponentSets; expected exactly one`
      );
    }
    const componentSet = sets[0];
    validateComponentVariantAxes(componentSet, component);
    const masters = (componentSet.children || []).filter(node => node.type === "COMPONENT");
    if (masters.length !== component.variants.length) {
      throw new Error(
        `native component ${component.id} has ${masters.length} variants; expected ${component.variants.length}`
      );
    }
    for (const variant of component.variants) {
      const matches = masters.filter(master =>
        sharedPluginValue(master, "pkVariantId") === variant.source_id
      );
      if (matches.length !== 1) {
        throw new Error(
          `native component ${component.id} variant ${variant.id} resolved ${matches.length} masters`
        );
      }
      const master = matches[0];
      validateVariantPropertyValues(master, component, variant);
      propertiesValidated += validateVariantComponentProperties(master, component, variant);
      slotsValidated += validateVariantSlots(master, component, variant);
      instancesValidated += await validateVariantInstances(master, component, variant);
      variantsValidated++;
    }
  }

  let solutionsValidated = 0;
  for (const solution of contract.solutions || []) {
    await findNativeSolutionInstanceByContract(solution, expectedClientId);
    solutionsValidated++;
  }
  return {
    components: contract.components.length,
    variants: variantsValidated,
    properties: propertiesValidated,
    slots: slotsValidated,
    instances: instancesValidated,
    canvases: canvasesValidated,
    solutions: solutionsValidated,
  };
}

function validateNativeCanvas(canvas, expectedClientId) {
  const pages = figma.root.children.filter(page =>
    page.type === "PAGE" &&
    page.name === canvas.page_name &&
    sharedPluginValue(page, KEY_CLIENT_ID) === stringValue(expectedClientId)
  );
  if (pages.length !== 1) {
    throw new Error(
      `native canvas ${canvas.page_name} resolved ${pages.length} pages; expected exactly one`
    );
  }
  const roots = typeof pages[0].findAll === "function"
    ? pages[0].findAll(node =>
      node.type === "FRAME" &&
      sharedPluginValue(node, "pkCanvasRoot") === canvas.page_name
    )
    : [];
  if (roots.length !== 1 || roots[0].name !== canvas.root_name) {
    throw new Error(
      `native canvas ${canvas.page_name} is missing its editable root ${canvas.root_name}`
    );
  }
}

function sharedPluginValue(node, key) {
  if (!node || typeof node.getSharedPluginData !== "function") return "";
  return stringValue(node.getSharedPluginData(SHARED_PLUGIN_NAMESPACE, key));
}

function nativePropertyName(key) {
  const raw = String(key || "");
  const separator = raw.lastIndexOf("#");
  return separator > 0 ? raw.slice(0, separator) : raw;
}

function componentPropertyDefinitions(node, type) {
  const definitions = [];
  for (const [key, definition] of Object.entries(node.componentPropertyDefinitions || {})) {
    if (definition && definition.type === type) {
      definitions.push({ key, name: nativePropertyName(key), definition });
    }
  }
  return definitions;
}

function validateComponentVariantAxes(componentSet, component) {
  const actual = componentPropertyDefinitions(componentSet, "VARIANT");
  if (actual.length !== component.variant_axes.length) {
    throw new Error(
      `native component ${component.id} exposes ${actual.length} variant axes; expected ${component.variant_axes.length}`
    );
  }
  for (const axis of component.variant_axes) {
    const matches = actual.filter(candidate => candidate.name === axis.name);
    if (matches.length !== 1) {
      throw new Error(`native component ${component.id} is missing variant axis ${axis.name}`);
    }
    const options = matches[0].definition.variantOptions;
    if (Array.isArray(options)) {
      const got = [...options].sort();
      const want = [...axis.values].sort();
      if (JSON.stringify(got) !== JSON.stringify(want)) {
        throw new Error(`native component ${component.id} variant axis ${axis.name} has wrong values`);
      }
    }
  }
}

function validateVariantPropertyValues(master, component, variant) {
  const actual = parseNativeVariantProperties(master.name);
  const expected = variant.properties || {};
  const expectedKeys = Object.keys(expected).sort();
  const actualKeys = Object.keys(actual).sort();
  if (JSON.stringify(actualKeys) !== JSON.stringify(expectedKeys)) {
    throw new Error(`native component ${component.id} variant ${variant.id} has wrong property axes`);
  }
  for (const key of expectedKeys) {
    if (actual[key] !== expected[key]) {
      throw new Error(`native component ${component.id} variant ${variant.id} has wrong ${key} value`);
    }
  }
}

function parseNativeVariantProperties(raw) {
  const properties = {};
  for (const part of String(raw || "").split(",")) {
    const segments = part.split("=");
    if (segments.length !== 2) continue;
    const key = stringValue(segments[0]);
    const value = stringValue(segments[1]);
    if (key !== "" && value !== "") properties[key] = value;
  }
  return properties;
}

function validateVariantComponentProperties(master, component, variant) {
  const expectedProperties = variant.component_properties || [];
  const allowedTypes = new Set(["TEXT", "BOOLEAN", "INSTANCE_SWAP", "SLOT"]);
  const actual = [];
  for (const [key, definition] of Object.entries(master.componentPropertyDefinitions || {})) {
    if (definition && allowedTypes.has(definition.type)) {
      actual.push({ key, name: nativePropertyName(key), definition });
    }
  }
  if (actual.length !== expectedProperties.length) {
    throw new Error(
      `native component ${component.id} variant ${variant.id} exposes ${actual.length} editable properties; ` +
      `expected ${expectedProperties.length}`
    );
  }
  for (const property of expectedProperties) {
    const matches = actual.filter(candidate =>
      candidate.name === property.name && candidate.definition.type === property.type
    );
    if (matches.length !== 1) {
      throw new Error(
        `native component ${component.id} variant ${variant.id} is missing ${property.type} property ${property.name}`
      );
    }
    if (
      Object.prototype.hasOwnProperty.call(property, "default") &&
      JSON.stringify(matches[0].definition.defaultValue) !== JSON.stringify(property.default)
    ) {
      throw new Error(
        `native component ${component.id} variant ${variant.id} property ${property.name} has wrong default`
      );
    }
  }
  return actual.length;
}

function validateVariantSlots(master, component, variant) {
  const expectedSlots = variant.slots || [];
  const actual = typeof master.findAll === "function"
    ? master.findAll(node => node.type === "SLOT" && sharedPluginValue(node, "pkSlot") !== "")
    : [];
  if (actual.length !== expectedSlots.length) {
    throw new Error(
      `native component ${component.id} variant ${variant.id} has ${actual.length} slots; expected ${expectedSlots.length}`
    );
  }
  for (const slot of expectedSlots) {
    const matches = actual.filter(candidate =>
      sharedPluginValue(candidate, "pkSlot") === slot.name &&
      sharedPluginValue(candidate, "pkSlotPropertyName") === slot.property_name
    );
    if (matches.length !== 1) {
      throw new Error(
        `native component ${component.id} variant ${variant.id} is missing slot ` +
        `${slot.name} (${slot.property_name})`
      );
    }
    const settings = matches[0].slotSettings || {};
    if (
      settings.minChildren !== undefined &&
      settings.minChildren !== slot.min_children
    ) {
      throw new Error(`native component ${component.id} variant ${variant.id} slot ${slot.name} has wrong minimum`);
    }
    const expectedMax = slot.max_children === undefined ? null : slot.max_children;
    if (settings.maxChildren !== undefined && settings.maxChildren !== expectedMax) {
      throw new Error(`native component ${component.id} variant ${variant.id} slot ${slot.name} has wrong maximum`);
    }
  }
  return actual.length;
}

async function validateVariantInstances(master, component, variant) {
  const actualNodes = typeof master.findAll === "function"
    ? master.findAll(node => node.type === "INSTANCE")
    : [];
  const actual = [];
  for (const instance of actualNodes) {
    const target = await instanceMainComponent(instance);
    if (!target) {
      throw new Error(`native component ${component.id} variant ${variant.id} has a detached instance`);
    }
    actual.push(
      `${sharedPluginValue(target, "pkSourceId")}\u0000${sharedPluginValue(target, "pkVariantId")}`
    );
  }
  const expected = (variant.instances || []).map(instance =>
    `${instance.component_source_id}\u0000${instance.variant_source_id || ""}`
  );
  actual.sort();
  expected.sort();
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`native component ${component.id} variant ${variant.id} has the wrong instance graph`);
  }
  return actual.length;
}

async function validateNativeDelivery(payload) {
  const delivery = parseVisualDeliveryPayload(payload);
  const tokenBundle = await loadGovernedTokenBundle();
  const nativeDelivery = await loadNativeDeliveryContract();
  const profile = visualDeliveryProfile(tokenBundle);
  if (delivery.profile !== profile) {
    throw new Error(
      `visual delivery profile "${delivery.profile || "missing"}" does not match attached token profile "${profile}"`
    );
  }
  const expectedClientId =
    tokenBundle.snapshot.profile.kind === "client"
      ? stringValue(tokenBundle.snapshot.profile.clientId || tokenBundle.clientId)
      : "";
  if (stringValue(nativeDelivery.client_id) !== expectedClientId) {
    throw new Error("attached native delivery contract does not match the governed token profile");
  }

  await ensureDocumentLoaded();
  const nativeConformance = await validateNativeContract(nativeDelivery, expectedClientId);
  if (delivery.fixtures.length !== (nativeDelivery.solutions || []).length) {
    throw new Error(
      `visual delivery has ${delivery.fixtures.length} Page fixtures but native delivery has ` +
      `${(nativeDelivery.solutions || []).length}`
    );
  }
  const fixtures = [];
  for (const fixture of delivery.fixtures) {
    const solutions = (nativeDelivery.solutions || []).filter(solution =>
      solution.component_id === fixture.component_id &&
      solution.variant_id === fixture.example_id &&
      solution.width === fixture.viewport.width &&
      solution.height === fixture.viewport.height
    );
    if (solutions.length !== 1) {
      throw new Error(`${fixture.id} resolved ${solutions.length} native solution contracts; expected exactly one`);
    }
    const instance = await findNativeSolutionInstanceByContract(solutions[0], expectedClientId);
    fixtures.push({
      id: fixture.id,
      component_id: fixture.component_id,
      variant_id: fixture.example_id,
      width: fixture.viewport.width,
      height: fixture.viewport.height,
      node_id: instance.id
    });
  }

  const report = {
    schema: NATIVE_CONFORMANCE_SCHEMA,
    source: "figma-native-node-validation",
    profile,
    validated_at: new Date().toISOString(),
    document_id: figma.root.id,
    token_snapshot_digest: tokenBundle.snapshotDigest,
    design_manifest_digest: stringValue(figma.root.getSharedPluginData(
      SHARED_PLUGIN_NAMESPACE,
      KEY_MANIFEST_DIGEST
    )),
    parent_profile: stringValue(figma.root.getSharedPluginData(
      SHARED_PLUGIN_NAMESPACE,
      KEY_PARENT_PROFILE
    )),
    parent_manifest_digest: stringValue(figma.root.getSharedPluginData(
      SHARED_PLUGIN_NAMESPACE,
      KEY_PARENT_MANIFEST_DIGEST
    )),
    bundle_digest: stringValue(figma.root.getSharedPluginData(
      SHARED_PLUGIN_NAMESPACE,
      KEY_BUNDLE_DIGEST
    )),
    native_contract: nativeConformance,
    fixtures
  };
  if (typeof figma.fileKey === "string" && figma.fileKey !== "") {
    report.file_key = figma.fileKey;
  }

  return {
    profile,
    fixtures: fixtures.length,
    filename: `platformkit-figma-native-conformance-${profile.replaceAll("/", "-")}.json`,
    payload: JSON.stringify(report, null, 2) + "\n"
  };
}

function parseVisualDeliveryPayload(payload) {
  if (typeof payload !== "string" || payload.trim() === "") {
    throw new Error("visual delivery manifest payload is required");
  }
  let manifest;
  try {
    manifest = JSON.parse(payload);
  } catch (_) {
    throw new Error("visual delivery manifest is not valid JSON");
  }
  if (!manifest || typeof manifest !== "object" || Array.isArray(manifest)) {
    throw new Error("visual delivery manifest must be a JSON object");
  }
  if (manifest.schema !== VISUAL_DELIVERY_SCHEMA) {
    throw new Error(
      `visual delivery schema must be "${VISUAL_DELIVERY_SCHEMA}", got "${manifest.schema || ""}"`
    );
  }
  if (typeof manifest.profile !== "string" || manifest.profile.trim() === "") {
    throw new Error("visual delivery manifest profile is required");
  }
  if (!Array.isArray(manifest.fixtures) || manifest.fixtures.length === 0) {
    throw new Error("visual delivery manifest contains no native Page fixtures");
  }

  const ids = new Set();
  const files = new Set();
  for (const fixture of manifest.fixtures) {
    if (!fixture || typeof fixture !== "object" || Array.isArray(fixture)) {
      throw new Error("visual delivery fixture must be an object");
    }
    for (const key of ["id", "component_id", "example_id", "browser_baseline_file"]) {
      if (typeof fixture[key] !== "string" || fixture[key].trim() === "") {
        throw new Error(`visual delivery fixture ${key} is required`);
      }
    }
    if (
      !fixture.figma ||
      fixture.figma.component_id !== fixture.component_id ||
      fixture.figma.variant_id !== fixture.example_id
    ) {
      throw new Error(`${fixture.id} Figma identity does not match its authored identity`);
    }
    if (
      !fixture.viewport ||
      !Number.isInteger(fixture.viewport.width) ||
      fixture.viewport.width <= 0 ||
      !Number.isInteger(fixture.viewport.height) ||
      fixture.viewport.height <= 0
    ) {
      throw new Error(`${fixture.id} has invalid viewport dimensions`);
    }
    if (
      fixture.browser_baseline_file.includes("/") ||
      fixture.browser_baseline_file.includes("\\") ||
      !fixture.browser_baseline_file.endsWith(".png")
    ) {
      throw new Error(`${fixture.id} has unsafe browser baseline file ${fixture.browser_baseline_file}`);
    }
    if (ids.has(fixture.id)) {
      throw new Error(`duplicate visual delivery fixture ${fixture.id}`);
    }
    if (files.has(fixture.browser_baseline_file)) {
      throw new Error(`duplicate browser baseline file ${fixture.browser_baseline_file}`);
    }
    ids.add(fixture.id);
    files.add(fixture.browser_baseline_file);
  }
  return manifest;
}

function visualDeliveryProfile(tokenBundle) {
  const profile = tokenBundle.snapshot && tokenBundle.snapshot.profile;
  if (!profile || typeof profile.kind !== "string") {
    throw new Error("governed token delivery has no profile kind");
  }
  switch (profile.kind) {
    case "oss":
      return "oss";
    case "pro":
      return "pro";
    case "client": {
      const clientId = stringValue(profile.clientId || tokenBundle.clientId);
      if (clientId === "" || !/^[A-Za-z0-9._-]+$/.test(clientId)) {
        throw new Error("client visual delivery requires a safe client id");
      }
      return `clients/${clientId}`;
    }
    default:
      throw new Error(`unsupported governed design profile kind "${profile.kind}"`);
  }
}

async function findNativeSolutionInstance(fixture, expectedClientId) {
  const componentSourceId = componentSourceID(fixture.component_id);
  const variantSourceId =
    `${componentSourceId}/variant/` +
    await sha256Hex(`authored:${fixture.example_id}`);
  return findNativeSolutionInstanceByContract({
    page_name: "",
    component_source_id: componentSourceId,
    variant_source_id: variantSourceId,
    width: fixture.viewport.width,
    height: fixture.viewport.height,
  }, expectedClientId, fixture.id);
}

async function findNativeSolutionInstanceByContract(solution, expectedClientId, label = "") {
  const matches = [];

  for (const page of figma.root.children) {
    if (
      page.type !== "PAGE" ||
      !page.name.startsWith(SOLUTION_PAGE_PREFIX) ||
      (stringValue(solution.page_name) !== "" && page.name !== solution.page_name) ||
      stringValue(page.getSharedPluginData(
        SHARED_PLUGIN_NAMESPACE,
        KEY_CLIENT_ID
      )) !== expectedClientId
    ) {
      continue;
    }
    const candidates = page.findAll(node => node.type === "INSTANCE");
    for (const candidate of candidates) {
      const mainComponent = await instanceMainComponent(candidate);
      if (!mainComponent) {
        continue;
      }
      if (
        mainComponent.getSharedPluginData(
          SHARED_PLUGIN_NAMESPACE,
          "pkSourceId"
        ) !== solution.component_source_id ||
        mainComponent.getSharedPluginData(
          SHARED_PLUGIN_NAMESPACE,
          "pkVariantId"
        ) !== solution.variant_source_id
      ) {
        continue;
      }
      if (
        Math.abs(candidate.width - solution.width) > 0.01 ||
        Math.abs(candidate.height - solution.height) > 0.01
      ) {
        continue;
      }
      matches.push(candidate);
    }
  }

  if (matches.length !== 1) {
    const fixtureLabel = label || nativeSolutionKey(solution);
    throw new Error(
      `${fixtureLabel} resolved ${matches.length} native solution instances at ` +
      `${solution.width}x${solution.height}; expected exactly one`
    );
  }
  return matches[0];
}

function nativeSolutionKey(solution) {
  return [
    solution.page_name,
    solution.component_source_id,
    solution.variant_source_id,
    solution.width,
    solution.height,
  ].join("\u0000");
}

async function instanceMainComponent(instance) {
  if (typeof instance.getMainComponentAsync === "function") {
    return await instance.getMainComponentAsync();
  }
  try {
    return instance.mainComponent || null;
  } catch (_) {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Token import: creates native Figma Variables from PlatformKit design tokens
// ---------------------------------------------------------------------------

async function parseTokenPayload(payload) {
  if (typeof payload !== "string") {
    throw new Error("token payload must be a JSON string");
  }

  let parsed;
  try {
    parsed = JSON.parse(payload);
  } catch (error) {
    throw new Error("token payload is not valid JSON");
  }

  await validateTokenBundle(parsed);
  return parsed;
}

async function validateTokenBundle(bundle) {
  if (!bundle || typeof bundle !== "object" || Array.isArray(bundle)) {
    throw new Error("token bundle must be a JSON object");
  }
  if (bundle.kind !== "figma-variables") {
    throw new Error(`token bundle kind must be "figma-variables", got "${bundle.kind}"`);
  }
  if (!Array.isArray(bundle.collections) || bundle.collections.length === 0) {
    throw new Error("token bundle must have at least one collection");
  }
  if (bundle.prune !== undefined && typeof bundle.prune !== "boolean") {
    throw new Error("token bundle prune must be boolean when provided");
  }
  const governed = bundle.schema === GOVERNED_TOKEN_SCHEMA;
  if (bundle.schema && !governed && bundle.schema !== "platformkit.figma-variables.v1") {
    throw new Error(`unsupported token bundle schema "${bundle.schema}"`);
  }
  if (governed) {
    validateGovernedTokenEnvelope(bundle);
    const actualDigest = await digestTokenSnapshot(bundle.snapshot);
    if (actualDigest !== bundle.snapshotDigest) {
      throw new Error(`token snapshot digest mismatch: bundle ${bundle.snapshotDigest}, actual ${actualDigest}`);
    }
  }
  const collectionNames = new Set();
  const variableKeys = new Set();
  for (const coll of bundle.collections) {
    if (typeof coll.name !== "string" || coll.name.trim() === "") {
      throw new Error("each collection requires a name");
    }
    if (!Array.isArray(coll.modes) || coll.modes.length === 0) {
      throw new Error(`collection "${coll.name}" requires at least one mode`);
    }
    if (!Array.isArray(coll.variables)) {
      throw new Error(`collection "${coll.name}" requires a variables array`);
    }
    if (collectionNames.has(coll.name)) {
      throw new Error(`duplicate token collection "${coll.name}"`);
    }
    collectionNames.add(coll.name);
    const modeNames = new Set();
    for (const mode of coll.modes) {
      if (typeof mode !== "string" || mode.trim() === "" || modeNames.has(mode)) {
        throw new Error(`collection "${coll.name}" has invalid or duplicate mode names`);
      }
      modeNames.add(mode);
    }
    const variableNames = new Set();
    for (const variable of coll.variables) {
      if (!variable || typeof variable !== "object" || typeof variable.name !== "string" || variable.name.trim() === "") {
        throw new Error(`collection "${coll.name}" has an invalid variable`);
      }
      if (!["COLOR", "FLOAT", "STRING", "BOOLEAN"].includes(variable.type)) {
        throw new Error(`variable "${coll.name}/${variable.name}" has unsupported type "${variable.type}"`);
      }
      if (
        variable.values !== undefined &&
        (variable.values === null || typeof variable.values !== "object" || Array.isArray(variable.values))
      ) {
        throw new Error(`variable "${coll.name}/${variable.name}" values must be an object`);
      }
      if (
        variable.aliases !== undefined &&
        (variable.aliases === null || typeof variable.aliases !== "object" || Array.isArray(variable.aliases))
      ) {
        throw new Error(`variable "${coll.name}/${variable.name}" aliases must be an object`);
      }
      if (!variable.values && !variable.aliases) {
        throw new Error(`variable "${coll.name}/${variable.name}" requires values or aliases`);
      }
      if (variableNames.has(variable.name)) {
        throw new Error(`duplicate token variable "${coll.name}/${variable.name}"`);
      }
      variableNames.add(variable.name);
      if (governed) {
        validateVariableBinding(coll, variable, modeNames);
        if (variableKeys.has(variable.key)) {
          throw new Error(`duplicate governed variable key "${variable.key}"`);
        }
        variableKeys.add(variable.key);
      }
    }
  }
  if (governed) {
    for (const coll of bundle.collections) {
      for (const variable of coll.variables) {
        for (const [modeName, targetKey] of Object.entries(variable.aliases || {})) {
          if (!variableKeys.has(targetKey)) {
            throw new Error(`variable "${variable.key}" mode "${modeName}" aliases missing key "${targetKey}"`);
          }
        }
      }
    }
  }
}

async function importTokens(bundle) {
  await ensureDocumentLoaded();
  let variableCount = 0;
  let collectionCount = 0;

  const existingCollections = await figma.variables.getLocalVariableCollectionsAsync();
  const imported = new Map();

  for (const collSpec of bundle.collections) {
    const collection = upsertCollection(existingCollections, collSpec.name);
    syncModes(collection, collSpec.modes, bundle.prune === true);
    collectionCount += 1;

    const modeMap = buildModeMap(collection);
    const existingVars = await getCollectionVariables(collection);

    for (const varSpec of collSpec.variables) {
      const variable = upsertVariable(existingVars, collection, varSpec);
      if (typeof varSpec.description === "string") {
        variable.description = varSpec.description;
      }
      setVariableValues(variable, varSpec, modeMap);
      imported.set(variableKey(collSpec, varSpec), {
        collection,
        collectionSpec: collSpec,
        modeMap,
        variable,
        variableSpec: varSpec
      });
      variableCount += 1;
    }
  }

  // Alias creation is deliberately a second pass: every target variable must
  // exist before any semantic variable is bound to it.
  for (const info of imported.values()) {
    setVariableAliases(info, imported);
  }

  if (stringValue(bundle.product) !== "") {
    setMetadata(figma.root, KEY_PRODUCT, bundle.product);
  }
  setTokenMetadata(figma.root, bundle);
  if (bundle.schema === GOVERNED_TOKEN_SCHEMA) {
    await storeGovernedTokenBundle(bundle);
  }

  return {
    product: bundle.product || "",
    collections: collectionCount,
    variables: variableCount,
    governed: bundle.schema === GOVERNED_TOKEN_SCHEMA,
    profile: bundle.snapshot && bundle.snapshot.profile ? bundle.snapshot.profile.id : ""
  };
}

function upsertCollection(existing, name) {
  for (const coll of existing) {
    if (coll.name === name) {
      return coll;
    }
  }
  const coll = figma.variables.createVariableCollection(name);
  return coll;
}

function syncModes(collection, modeNames, allowPrune) {
  const existingModes = collection.modes;

  // Rename only a newly-created/default collection mode unless explicit prune
  // was requested. This avoids silently renaming a user's unrelated mode in a
  // same-named collection.
  if (existingModes.length > 0 && modeNames.length > 0) {
    const first = existingModes[0];
    const looksGenerated = /^Mode\s+\d+$/i.test(first.name);
    if (first.name !== modeNames[0] && (allowPrune || looksGenerated)) {
      collection.renameMode(first.modeId, modeNames[0]);
    }
  }

  // Add missing modes.
  const existingNames = new Set(collection.modes.map((m) => m.name));
  for (let i = 0; i < modeNames.length; i++) {
    if (!existingNames.has(modeNames[i])) {
      if (i === 0 && existingModes.length > 0) {
        continue; // already renamed above
      }
      collection.addMode(modeNames[i]);
    }
  }

  // Mode deletion is destructive and therefore opt-in. Normal sync is
  // additive so an accidental omission cannot erase a client's mode.
  if (allowPrune) {
    const wantedNames = new Set(modeNames);
    for (const mode of collection.modes) {
      if (!wantedNames.has(mode.name) && collection.modes.length > 1) {
        collection.removeMode(mode.modeId);
      }
    }
  }
}

function buildModeMap(collection) {
  const map = {};
  for (const mode of collection.modes) {
    map[mode.name] = mode.modeId;
  }
  return map;
}

async function getCollectionVariables(collection) {
  const vars = {};
  for (const varId of collection.variableIds) {
    const variable = await figma.variables.getVariableByIdAsync(varId);
    if (variable) {
      vars[variable.name] = variable;
    }
  }
  return vars;
}

function resolveVariableType(typeStr) {
  switch (typeStr) {
    case "COLOR":
      return "COLOR";
    case "FLOAT":
      return "FLOAT";
    case "STRING":
      return "STRING";
    case "BOOLEAN":
      return "BOOLEAN";
    default:
      return "FLOAT";
  }
}

function upsertVariable(existing, collection, varSpec) {
  const name = varSpec.name;
  const resolvedType = resolveVariableType(varSpec.type);
  if (existing[name]) {
    if (existing[name].resolvedType && existing[name].resolvedType !== resolvedType) {
      throw new Error(`variable "${collection.name}/${name}" type mismatch: existing ${existing[name].resolvedType}, bundle ${resolvedType}`);
    }
    return existing[name];
  }
  const variable = figma.variables.createVariable(name, collection, resolvedType);
  existing[name] = variable;
  return variable;
}

function setVariableValues(variable, varSpec, modeMap) {
  const values = varSpec.values && typeof varSpec.values === "object" ? varSpec.values : {};
  for (const [modeName, rawValue] of Object.entries(values)) {
    const modeId = modeMap[modeName];
    if (!modeId) {
      continue;
    }

    let value;
    if (varSpec.type === "COLOR") {
      if (!rawValue || typeof rawValue !== "object" || Array.isArray(rawValue)) {
        throw new Error(`invalid color value for ${variable.name}/${modeName}`);
      }
      value = {
        r: checkedColorChannel(rawValue.r, "r", variable.name, modeName),
        g: checkedColorChannel(rawValue.g, "g", variable.name, modeName),
        b: checkedColorChannel(rawValue.b, "b", variable.name, modeName),
        a: rawValue.a === undefined ? 1 : checkedColorChannel(rawValue.a, "a", variable.name, modeName)
      };
    } else if (varSpec.type === "FLOAT") {
      if (typeof rawValue !== "number" || !Number.isFinite(rawValue)) {
        throw new Error(`invalid float value for ${variable.name}/${modeName}`);
      }
      value = rawValue;
    } else if (varSpec.type === "STRING") {
      value = String(rawValue);
    } else if (varSpec.type === "BOOLEAN") {
      if (typeof rawValue !== "boolean") {
        throw new Error(`invalid boolean value for ${variable.name}/${modeName}`);
      }
      value = rawValue;
    } else {
      value = rawValue;
    }

    variable.setValueForMode(modeId, value);
  }
}

function setVariableAliases(info, imported) {
  const aliases = info.variableSpec.aliases && typeof info.variableSpec.aliases === "object"
    ? info.variableSpec.aliases
    : {};
  for (const [modeName, targetKey] of Object.entries(aliases)) {
    const modeId = info.modeMap[modeName];
    if (!modeId) {
      throw new Error(`variable "${info.variableSpec.name}" has no Figma mode "${modeName}"`);
    }
    const target = imported.get(targetKey);
    if (!target) {
      throw new Error(`variable "${info.variableSpec.name}" aliases missing variable key "${targetKey}"`);
    }
    info.variable.setValueForMode(modeId, figma.variables.createVariableAlias(target.variable));
  }
}

function checkedColorChannel(value, channel, variableName, modeName) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0 || value > 1) {
    throw new Error(`invalid color channel ${channel} for ${variableName}/${modeName}`);
  }
  return value;
}

function validateGovernedTokenEnvelope(bundle) {
  if (!bundle.snapshot || typeof bundle.snapshot !== "object" || Array.isArray(bundle.snapshot)) {
    throw new Error("governed token bundle requires snapshot");
  }
  if (bundle.snapshot.schemaVersion !== TOKEN_SNAPSHOT_SCHEMA) {
    throw new Error(`unsupported token snapshot schema "${bundle.snapshot.schemaVersion}"`);
  }
  if (!bundle.snapshot.profile || typeof bundle.snapshot.profile.id !== "string") {
    throw new Error("governed token snapshot requires profile identity");
  }
  if (!Array.isArray(bundle.snapshot.tokens) || bundle.snapshot.tokens.length === 0) {
    throw new Error("governed token snapshot requires tokens");
  }
  if (typeof bundle.snapshotDigest !== "string" || !/^sha256:[a-f0-9]{64}$/i.test(bundle.snapshotDigest)) {
    throw new Error("governed token bundle requires a sha256 snapshotDigest");
  }
}

function validateVariableBinding(collection, variable, modeNames) {
  if (typeof variable.key !== "string" || variable.key.trim() === "") {
    throw new Error(`variable "${collection.name}/${variable.name}" requires stable key`);
  }
  const binding = variable.binding;
  if (!binding || typeof binding !== "object" || Array.isArray(binding)) {
    throw new Error(`variable "${variable.key}" requires binding`);
  }
  if (typeof binding.tokenPath !== "string" || binding.tokenPath !== variable.key) {
    throw new Error(`variable "${variable.key}" binding.tokenPath must equal its key`);
  }
  if (!binding.modeMap || typeof binding.modeMap !== "object" || Array.isArray(binding.modeMap)) {
    throw new Error(`variable "${variable.key}" requires binding.modeMap`);
  }
  if (typeof binding.writable !== "boolean") {
    throw new Error(`variable "${variable.key}" binding.writable must be boolean`);
  }
  for (const [modeName, snapshotMode] of Object.entries(binding.modeMap)) {
    if (!modeNames.has(modeName) || typeof snapshotMode !== "string" || snapshotMode.trim() === "") {
      throw new Error(`variable "${variable.key}" has invalid mode binding "${modeName}"`);
    }
  }
}

function variableKey(collectionSpec, variableSpec) {
  if (typeof variableSpec.key === "string" && variableSpec.key !== "") {
    return variableSpec.key;
  }
  return `${collectionSpec.name}/${variableSpec.name}`;
}

async function storeGovernedTokenBundle(bundle) {
  const raw = JSON.stringify(bundle);
  try {
    await figma.clientStorage.setAsync(TOKEN_STORAGE_KEY, raw);
  } catch (_) {
    // The document copy below is the portable fallback.
  }
  const oldCount = Number(figma.root.getPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}Count`) || "0");
  const chunks = [];
  for (let offset = 0; offset < raw.length; offset += TOKEN_ROOT_CHUNK_SIZE) {
    chunks.push(raw.slice(offset, offset + TOKEN_ROOT_CHUNK_SIZE));
  }
  for (let index = 0; index < chunks.length; index++) {
    figma.root.setPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}${index}`, chunks[index]);
  }
  for (let index = chunks.length; index < oldCount; index++) {
    figma.root.setPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}${index}`, "");
  }
  figma.root.setPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}Count`, String(chunks.length));
  figma.root.setPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}Digest`, bundle.snapshotDigest);
}

async function loadGovernedTokenBundle() {
  await ensureDocumentLoaded();
  let raw = "";
  try {
    raw = await figma.clientStorage.getAsync(TOKEN_STORAGE_KEY) || "";
  } catch (_) {}
  if (raw === "") {
    const count = Number(figma.root.getPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}Count`) || "0");
    if (!Number.isInteger(count) || count < 1 || count > 100) {
      throw new Error("No governed token delivery is attached to this Figma file. Import a v2 token bundle first.");
    }
    for (let index = 0; index < count; index++) {
      const chunk = figma.root.getPluginData(`${TOKEN_ROOT_CHUNK_PREFIX}${index}`);
      if (chunk === "") {
        throw new Error(`Stored governed token delivery is missing chunk ${index + 1}/${count}`);
      }
      raw += chunk;
    }
  }
  let bundle;
  try {
    bundle = JSON.parse(raw);
  } catch (_) {
    throw new Error("Stored governed token delivery is corrupt; re-import a fresh bundle.");
  }
  await validateTokenBundle(bundle);
  return bundle;
}

async function storeNativeDeliveryContract(contract) {
  const raw = JSON.stringify(contract);
  try {
    await figma.clientStorage.setAsync(NATIVE_DELIVERY_STORAGE_KEY, raw);
  } catch (_) {
    // The document copy below is the portable fallback.
  }
  const oldCount = Number(
    figma.root.getPluginData(`${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}Count`) || "0"
  );
  const chunks = [];
  for (let offset = 0; offset < raw.length; offset += TOKEN_ROOT_CHUNK_SIZE) {
    chunks.push(raw.slice(offset, offset + TOKEN_ROOT_CHUNK_SIZE));
  }
  for (let index = 0; index < chunks.length; index++) {
    figma.root.setPluginData(
      `${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}${index}`,
      chunks[index]
    );
  }
  for (let index = chunks.length; index < oldCount; index++) {
    figma.root.setPluginData(`${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}${index}`, "");
  }
  figma.root.setPluginData(
    `${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}Count`,
    String(chunks.length)
  );
}

async function loadNativeDeliveryContract() {
  await ensureDocumentLoaded();
  let raw = "";
  try {
    raw = await figma.clientStorage.getAsync(NATIVE_DELIVERY_STORAGE_KEY) || "";
  } catch (_) {}
  if (raw === "") {
    const count = Number(
      figma.root.getPluginData(`${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}Count`) || "0"
    );
    if (!Number.isInteger(count) || count < 1 || count > 100) {
      throw new Error(
        "No native delivery contract is attached to this Figma file. Import a v3 artifact bundle first."
      );
    }
    for (let index = 0; index < count; index++) {
      const chunk = figma.root.getPluginData(
        `${NATIVE_DELIVERY_ROOT_CHUNK_PREFIX}${index}`
      );
      if (chunk === "") {
        throw new Error(
          `Stored native delivery contract is missing chunk ${index + 1}/${count}`
        );
      }
      raw += chunk;
    }
  }
  let contract;
  try {
    contract = JSON.parse(raw);
  } catch (_) {
    throw new Error("Stored native delivery contract is corrupt; re-import a fresh bundle.");
  }
  await validateNativeDeliveryContract(contract);
  return contract;
}

async function exportTokenChanges(options) {
  if (options.author === "") {
    throw new Error("Author is required for an auditable token export.");
  }
  if (options.reason === "") {
    throw new Error("Reason is required for an auditable token export.");
  }
  const bundle = await loadGovernedTokenBundle();
  const current = await collectManagedVariables(bundle);
  const baseTokens = new Map();
  for (const token of bundle.snapshot.tokens) {
    baseTokens.set(`${token.mode}:${token.path}`, token);
  }

  const changes = [];
  const violations = [];
  for (const collectionSpec of bundle.collections) {
    for (const variableSpec of collectionSpec.variables) {
      const key = variableKey(collectionSpec, variableSpec);
      const info = current.get(key);
      if (!info) {
        violations.push(`managed variable "${key}" is missing`);
        continue;
      }
      const aliases = variableSpec.aliases || {};
      const values = variableSpec.values || {};
      for (const [figmaMode, snapshotMode] of Object.entries(variableSpec.binding.modeMap)) {
        const modeId = info.modeMap[figmaMode];
        if (!modeId) {
          violations.push(`managed variable "${key}" is missing mode "${figmaMode}"`);
          continue;
        }
        const native = info.variable.valuesByMode[modeId];
        const aliasTargetKey = aliases[figmaMode];
        if (aliasTargetKey !== undefined) {
          const target = current.get(aliasTargetKey);
          if (!isVariableAlias(native) || !target || native.id !== target.variable.id) {
            violations.push(`read-only alias "${key}" mode "${figmaMode}" was detached or retargeted`);
          }
          continue;
        }
        const expectedNative = values[figmaMode];
        if (nativeValuesEqual(variableSpec.type, native, expectedNative)) {
          continue;
        }
        if (!variableSpec.binding.writable) {
          violations.push(`read-only variable "${key}" mode "${figmaMode}" was edited`);
          continue;
        }
        const codec = variableSpec.binding.codecs && variableSpec.binding.codecs[figmaMode];
        if (!codec) {
          violations.push(`writable variable "${key}" mode "${figmaMode}" has no round-trip codec`);
          continue;
        }
        const coordinate = `${snapshotMode}:${variableSpec.binding.tokenPath}`;
        const before = baseTokens.get(coordinate);
        if (!before) {
          violations.push(`variable "${key}" targets missing snapshot coordinate "${coordinate}"`);
          continue;
        }
        const after = deepClone(before);
        try {
          after.value = decodeNativeValue(variableSpec.type, native, codec);
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          violations.push(`variable "${key}" mode "${figmaMode}": ${message}`);
          continue;
        }
        changes.push({ operation: "update", before: deepClone(before), after });
      }
    }
  }
  if (violations.length > 0) {
    throw new Error(`Governed export refused:\n- ${violations.join("\n- ")}\nRe-import the delivery to restore read-only variables.`);
  }
  if (changes.length === 0) {
    throw new Error("No managed source-token changes were detected.");
  }
  changes.sort((left, right) => {
    const leftKey = `${left.after.mode}:${left.after.path}`;
    const rightKey = `${right.after.mode}:${right.after.path}`;
    return leftKey.localeCompare(rightKey);
  });
  const changeSet = {
    schemaVersion: TOKEN_CHANGESET_SCHEMA,
    profile: deepClone(bundle.snapshot.profile),
    baseDigest: bundle.snapshotDigest,
    provenance: {
      source: "figma-plugin",
      generatedAt: new Date().toISOString(),
      author: options.author,
      reason: options.reason
    },
    changes
  };
  const profileID = String(bundle.snapshot.profile.id || "platformkit").replace(/[^a-z0-9.-]+/gi, "-");
  return {
    changes: changes.length,
    profile: bundle.snapshot.profile.id,
    baseDigest: bundle.snapshotDigest,
    filename: `${profileID}-token-changes.json`,
    payload: JSON.stringify(changeSet, null, 2) + "\n"
  };
}

async function collectManagedVariables(bundle) {
  const collections = await figma.variables.getLocalVariableCollectionsAsync();
  const byName = new Map(collections.map(collection => [collection.name, collection]));
  const managed = new Map();
  for (const collectionSpec of bundle.collections) {
    const collection = byName.get(collectionSpec.name);
    if (!collection) {
      throw new Error(`managed collection "${collectionSpec.name}" is missing`);
    }
    const variables = await getCollectionVariables(collection);
    const modeMap = buildModeMap(collection);
    for (const variableSpec of collectionSpec.variables) {
      const variable = variables[variableSpec.name];
      if (!variable) {
        continue;
      }
      managed.set(variableKey(collectionSpec, variableSpec), {
        collection,
        collectionSpec,
        modeMap,
        variable,
        variableSpec
      });
    }
  }
  return managed;
}

function nativeValuesEqual(type, left, right) {
  if (type === "COLOR") {
    if (!left || !right || isVariableAlias(left) || typeof left !== "object" || typeof right !== "object") {
      return false;
    }
    return ["r", "g", "b", "a"].every(channel => {
      const leftValue = channel === "a" && left[channel] === undefined ? 1 : left[channel];
      const rightValue = channel === "a" && right[channel] === undefined ? 1 : right[channel];
      return typeof leftValue === "number" && typeof rightValue === "number" &&
        Math.abs(leftValue - rightValue) <= 1e-7;
    });
  }
  if (type === "FLOAT") {
    return typeof left === "number" && typeof right === "number" && Math.abs(left - right) <= 1e-7;
  }
  return left === right;
}

function isVariableAlias(value) {
  return Boolean(value && typeof value === "object" && value.type === "VARIABLE_ALIAS" && typeof value.id === "string");
}

function decodeNativeValue(type, native, codec) {
  if (type === "COLOR" && codec.kind === "color") {
    return decodeColor(native, codec.style || "rgb-triplet");
  }
  if (type === "FLOAT" && (codec.kind === "dimension" || codec.kind === "duration")) {
    if (typeof native !== "number" || !Number.isFinite(native)) {
      throw new Error("native value is not a finite number");
    }
    const scale = typeof codec.scale === "number" && codec.scale !== 0 ? codec.scale : 1;
    const value = formatNumber(native / scale, Math.max(Number(codec.precision) || 0, 6));
    if (codec.style === "unitless-string") {
      return value;
    }
    return value + (codec.unit || "");
  }
  if (type === "FLOAT" && codec.kind === "number") {
    if (typeof native !== "number" || !Number.isFinite(native)) {
      throw new Error("native value is not a finite number");
    }
    const rendered = formatNumber(native, Math.max(Number(codec.precision) || 0, 6));
    return codec.style === "string" ? rendered : Number(rendered);
  }
  if (type === "STRING" && codec.kind === "string") {
    return String(native);
  }
  if (type === "BOOLEAN" && codec.kind === "boolean") {
    if (typeof native !== "boolean") {
      throw new Error("native value is not boolean");
    }
    return native;
  }
  throw new Error(`unsupported writable codec "${codec.kind}" for Figma type "${type}"`);
}

function decodeColor(native, style) {
  if (!native || typeof native !== "object" || isVariableAlias(native)) {
    throw new Error("native color is invalid");
  }
  const channel = name => {
    const value = native[name];
    if (typeof value !== "number" || !Number.isFinite(value) || value < 0 || value > 1) {
      throw new Error(`native color channel "${name}" is outside 0..1`);
    }
    return Math.round(value * 255);
  };
  const r = channel("r");
  const g = channel("g");
  const b = channel("b");
  const alpha = native.a === undefined ? 1 : native.a;
  if (typeof alpha !== "number" || !Number.isFinite(alpha) || alpha < 0 || alpha > 1) {
    throw new Error("native color alpha is outside 0..1");
  }
  if (style.startsWith("hex")) {
    const withAlpha = style.startsWith("hex4") || style.startsWith("hex8");
    if (!withAlpha && Math.abs(alpha - 1) > 1e-7) {
      throw new Error("the source color format does not support alpha");
    }
    const upper = style.endsWith("-upper");
    let pairs = [r, g, b].map(hexByte);
    if (withAlpha) {
      pairs.push(hexByte(Math.round(alpha * 255)));
    }
    if (style.startsWith("hex3") || style.startsWith("hex4")) {
      const short = pairs.every(pair => pair[0] === pair[1]);
      if (short) {
        pairs = pairs.map(pair => pair[0]);
      }
    }
    let rendered = "#" + pairs.join("");
    rendered = upper ? rendered.toUpperCase() : rendered.toLowerCase();
    return rendered;
  }
  if (style === "rgb-function") {
    if (Math.abs(alpha - 1) > 1e-7) {
      throw new Error("the source rgb() format does not support alpha");
    }
    return `rgb(${r}, ${g}, ${b})`;
  }
  if (style === "rgba-function") {
    return `rgba(${r}, ${g}, ${b}, ${formatNumber(alpha, 4)})`;
  }
  if (Math.abs(alpha - 1) > 1e-7) {
    throw new Error("the source RGB triplet format does not support alpha");
  }
  return `${r} ${g} ${b}`;
}

function hexByte(value) {
  return Math.max(0, Math.min(255, value)).toString(16).padStart(2, "0");
}

function formatNumber(value, precision) {
  if (!Number.isFinite(value)) {
    throw new Error("number is not finite");
  }
  const bounded = Math.max(0, Math.min(12, precision));
  const rendered = value.toFixed(bounded).replace(/\.?0+$/, "");
  return rendered === "-0" || rendered === "" ? "0" : rendered;
}

async function digestTokenSnapshot(snapshot) {
  const content = canonicalSnapshotContent(snapshot);
  return "sha256:" + await sha256Hex(JSON.stringify(content));
}

function canonicalSnapshotContent(snapshot) {
  const profile = {
    id: snapshot.profile.id,
    kind: snapshot.profile.kind,
    version: snapshot.profile.version
  };
  for (const key of ["clientId", "parentId", "parentDigest"]) {
    if (typeof snapshot.profile[key] === "string" && snapshot.profile[key] !== "") {
      profile[key] = snapshot.profile[key];
    }
  }
  const tokens = snapshot.tokens.map(token => {
    const canonical = {
      path: token.path,
      mode: token.mode
    };
    if (typeof token.type === "string" && token.type !== "") {
      canonical.type = token.type;
    }
    canonical.value = stableJSONValue(token.value);
    if (typeof token.description === "string" && token.description !== "") {
      canonical.description = token.description;
    }
    canonical.origin = {
      owner: token.origin.owner,
      layer: token.origin.layer,
      sourceUri: token.origin.sourceUri
    };
    if (typeof token.origin.pointer === "string" && token.origin.pointer !== "") {
      canonical.origin.pointer = token.origin.pointer;
    }
    canonical.origin.writable = token.origin.writable === true;
    return canonical;
  });
  const content = {
    schemaVersion: snapshot.schemaVersion,
    profile,
    tokens
  };
  if (snapshot.metadata && Object.keys(snapshot.metadata).length > 0) {
    content.metadata = stableJSONValue(snapshot.metadata);
  }
  return content;
}

function stableJSONValue(value) {
  if (Array.isArray(value)) {
    return value.map(stableJSONValue);
  }
  if (value && typeof value === "object") {
    const output = {};
    for (const key of Object.keys(value).sort()) {
      output[key] = stableJSONValue(value[key]);
    }
    return output;
  }
  return value;
}

function deepClone(value) {
  return JSON.parse(JSON.stringify(value));
}

// ---------------------------------------------------------------------------
// Component style round trip
// ---------------------------------------------------------------------------

// storeStyleBaseline persists the snapshot an export diffs against, using the
// same chunked-blob scheme the token baseline uses: clientStorage first, with
// document plugin data as the portable fallback so a file opened on another
// machine can still be exported from.
async function storeStyleBaseline(snapshot) {
  const payload = JSON.stringify(snapshot);
  await figma.clientStorage.setAsync(STYLE_STORAGE_KEY, snapshot);
  const chunks = [];
  for (let index = 0; index < payload.length; index += TOKEN_ROOT_CHUNK_SIZE) {
    chunks.push(payload.slice(index, index + TOKEN_ROOT_CHUNK_SIZE));
  }
  chunks.forEach((chunk, index) => {
    figma.root.setPluginData(`${STYLE_ROOT_CHUNK_PREFIX}${index}`, chunk);
  });
  figma.root.setPluginData(`${STYLE_ROOT_CHUNK_PREFIX}Count`, String(chunks.length));
  return snapshot;
}

async function loadStyleBaseline() {
  const stored = await figma.clientStorage.getAsync(STYLE_STORAGE_KEY);
  if (stored && typeof stored === "object") {
    return validateStyleSnapshot(stored);
  }
  const count = Number(figma.root.getPluginData(`${STYLE_ROOT_CHUNK_PREFIX}Count`) || "0");
  if (!Number.isInteger(count) || count < 1 || count > 100) {
    throw new Error(
      "No component style baseline is stored in this file. Import the design bundle before exporting style changes.",
    );
  }
  let payload = "";
  for (let index = 0; index < count; index += 1) {
    const chunk = figma.root.getPluginData(`${STYLE_ROOT_CHUNK_PREFIX}${index}`);
    if (!chunk) {
      throw new Error("The stored component style baseline is incomplete; re-import the design bundle.");
    }
    payload += chunk;
  }
  return validateStyleSnapshot(JSON.parse(payload));
}

function validateStyleSnapshot(snapshot) {
  if (!snapshot || typeof snapshot !== "object") {
    throw new Error("component style baseline is not an object");
  }
  if (snapshot.schemaVersion !== STYLE_SNAPSHOT_SCHEMA) {
    throw new Error(`unsupported component style schema ${JSON.stringify(snapshot.schemaVersion)}`);
  }
  if (!isNonEmptyStringProperty(snapshot, "stylesheet")) {
    throw new Error("component style baseline does not name its stylesheet");
  }
  if (!Array.isArray(snapshot.styles) || snapshot.styles.length === 0) {
    throw new Error("component style baseline carries no declarations");
  }
  return snapshot;
}

// styleCoordinate mirrors the Go coordinate exactly; the two sides must agree
// or a change set cannot be applied.
function styleCoordinate(value) {
  const atRule = stringValue(value.atRule);
  if (atRule === "") {
    return `${value.selector}|${value.property}`;
  }
  return `${atRule}|${value.selector}|${value.property}`;
}

// formatPixels renders a Figma number the way CSS declares it, so an untouched
// value compares equal to the baseline rather than looking edited.
function formatPixels(value) {
  if (typeof value !== "number" || !isFinite(value)) {
    return null;
  }
  const rounded = Math.round(value * 1000) / 1000;
  return `${rounded}px`;
}

// collectStyledNodes walks managed component sets and yields every node that
// declares which CSS rule it stands for.
async function collectStyledNodes() {
  await ensureDocumentLoaded();
  const found = [];
  for (const page of figma.root.children) {
    const nodes = page.findAll
      ? page.findAll((node) => sharedPluginValue(node, KEY_STYLE_SELECTOR) !== "")
      : [];
    for (const node of nodes) {
      found.push({ selector: sharedPluginValue(node, KEY_STYLE_SELECTOR), node });
    }
  }
  return found;
}

// readNodeDeclarations reads the CSS declarations one node currently expresses.
function readNodeDeclarations(selector, node) {
  const out = [];
  for (const mapping of STYLE_NODE_PROPERTIES) {
    if (!Object.prototype.hasOwnProperty.call(node, mapping.node)) {
      continue;
    }
    const formatted = formatPixels(node[mapping.node]);
    if (formatted === null) {
      continue;
    }
    out.push({ selector, property: mapping.css, value: formatted });
  }
  return out;
}

// exportStyleChanges diffs the document against the stored baseline and emits a
// pk.design.component-style-changes.v1 payload.
//
// Every refusal is collected rather than thrown on first sight: a designer who
// touched six things wants all six problems at once, not a six-round
// conversation. This mirrors exportTokenChanges.
async function exportStyleChanges(options) {
  const author = stringValue(options && options.author);
  const reason = stringValue(options && options.reason);
  if (author === "") {
    throw new Error("An author is required so the change set records who proposed it.");
  }
  if (reason === "") {
    throw new Error("A reason is required so the change set records why.");
  }

  const baseline = await loadStyleBaseline();
  const baseIndex = new Map();
  for (const value of baseline.styles) {
    baseIndex.set(styleCoordinate(value), value);
  }

  const styled = await collectStyledNodes();
  if (styled.length === 0) {
    throw new Error(
      "No managed styled nodes were found. Import the design bundle before exporting style changes.",
    );
  }

  const violations = [];
  const changes = [];
  const seen = new Set();

  for (const entry of styled) {
    for (const declaration of readNodeDeclarations(entry.selector, entry.node)) {
      const coordinate = styleCoordinate(declaration);
      const before = baseIndex.get(coordinate);
      if (!before) {
        // A node expressing a declaration the stylesheet does not own would
        // become a new CSS rule, which this protocol deliberately cannot make.
        continue;
      }
      if (seen.has(coordinate)) {
        violations.push(
          `${coordinate} is expressed by more than one node; the edit cannot be attributed`,
        );
        continue;
      }
      seen.add(coordinate);
      if (before.value === declaration.value) {
        continue;
      }
      changes.push({ before, after: { ...before, value: declaration.value } });
    }
  }

  if (violations.length > 0) {
    throw new Error(`Style export refused:\n- ${violations.join("\n- ")}`);
  }
  if (changes.length === 0) {
    throw new Error("No managed component style changes were detected.");
  }

  changes.sort((left, right) =>
    styleCoordinate(left.before).localeCompare(styleCoordinate(right.before)),
  );

  const changeSet = {
    schemaVersion: STYLE_CHANGESET_SCHEMA,
    stylesheet: baseline.stylesheet,
    baseDigest: baseline.digest,
    provenance: {
      source: "figma-plugin",
      generatedAt: new Date().toISOString(),
      author,
      reason,
    },
    changes,
  };
  const filename = `${baseline.stylesheet.replace(/[^a-z0-9.-]+/gi, "-")}-style-changes.json`;
  return {
    changes: changes.length,
    stylesheet: baseline.stylesheet,
    baseDigest: baseline.digest,
    filename,
    payload: `${JSON.stringify(changeSet, null, 2)}\n`,
  };
}
