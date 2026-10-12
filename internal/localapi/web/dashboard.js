"use strict";

// state keeps session secrets, edits and command retries in memory only; no URL/cookie/storage bootstrap.
const state = { token: "", session: "", draft: null, selected: null, saved: false, adapters: [], client: null, busy: false, invalid: new Set(), run: null, frozen: null, pending: null, timer: null, epoch: 0 };
// element resolves fixed shell IDs; untrusted content is always assigned through text/value properties.
function element(id) { return document.getElementById(id); }
// clone snapshots public JSON without converting exact decimal strings to numbers.
function clone(value) { return JSON.parse(JSON.stringify(value)); }
// notice presents a fixed operation result or structured public error as plain text.
function notice(message, error = false) { element("notice").textContent = message; element("notice").dataset.error = String(error); }
// safeID checks application identities before constructing any API path.
function safeID(value) { if (!/^[a-zA-Z0-9_-]{1,64}$/.test(value)) throw new Error("Use an ID with 1–64 letters, digits, underscores or hyphens."); return value; }
// requestID creates fresh command correlation without deriving IDs from user input or secrets.
function requestID() { const bytes = crypto.getRandomValues(new Uint8Array(16)); return "ui-" + Array.from(bytes, function hex(b) { /* Encode random bytes into a safe fixed-size identity. */ return b.toString(16).padStart(2, "0"); }).join(""); }
// api sends exact-origin protected requests with finite timeouts and validates the response identity.
// A transport/protocol error remains ambiguous for mutations; only an authenticated envelope is definitive.
async function api(path, method = "GET", fields = null, id = requestID(), rawProfile = null) {
  if (!state.token) throw new Error("Connect this local session first.");
  const epoch = state.epoch;
  let body = fields === null ? undefined : JSON.stringify({ schema_version: 1, request_id: id, ...fields });
  if (rawProfile !== null) body = body.slice(0, -1) + ',"profile":' + rawProfile + "}";
  if (body && new TextEncoder().encode(body).length > 1048576) throw new Error("Document exceeds the 1 MiB API body limit.");
  const response = await fetch(path, { method, headers: { Authorization: "Bearer " + state.token, "X-Request-ID": id, "Content-Type": "application/json" }, body, credentials: "omit", cache: "no-store", redirect: "error", signal: AbortSignal.timeout(5000) });
  const envelope = await response.json();
  if (epoch !== state.epoch) throw new Error("Session changed; reconnect and read status.");
  if (envelope.schema_version !== 1 || envelope.request_id !== id) throw new Error("Unexpected API response identity.");
  if (!response.ok || envelope.error) { const error = new Error(envelope.error ? envelope.error.field + ": " + envelope.error.reason : "API request failed."); error.definite = true; throw error; }
  return envelope.result;
}
// dirty compares the editor to the last loaded/saved selection, preserving pending invalid input as unsaved.
function dirty() { return state.invalid.size > 0 || !state.selected || JSON.stringify(state.draft) !== JSON.stringify(state.selected.profile); }
// controls reflects local editing and command state; readiness/phase timing never drive the engine here.
function controls() {
  for (const node of document.querySelectorAll("#workspace button, #workspace input, #workspace select, #workspace textarea")) node.disabled = state.busy || !state.token;
  element("connect").disabled = state.busy || !!state.token;
  element("disconnect").disabled = state.busy || !state.token;
  element("token").disabled = state.busy || !!state.token;
  element("create-key").disabled = state.busy || !state.token || !!state.client;
  element("start").disabled = state.busy || !state.token || !state.saved || dirty() || !!state.pending || !!state.run && ["executing", "cancelling"].includes(state.run.status);
  element("retry").hidden = !state.pending;
  element("pending").textContent = state.pending ? "Acknowledgment unknown. Retry uses the same command ID and frozen target. Do not start another run." : "";
  element("edit-state").textContent = state.draft ? (dirty() ? "Unsaved edits — validate and explicitly save before starting." : state.saved ? "Saved revision " + state.selected.profile_revision : "Launch profile is unsaved — explicitly save before starting.") : "";
  if (typeof diagnosticControls === "function") diagnosticControls();
  if (typeof comparisonControls === "function") comparisonControls();
}
// perform serializes explicit UI actions; status polling remains independent and never overlaps itself.
async function perform(action) {
  if (state.busy) return;
  state.busy = true; controls();
  try { await action(); } catch (error) { notice(error.message, true); }
  finally { state.busy = false; controls(); }
}
// labelText makes schema field names legible while retaining exact units for canonical inputs.
function labelText(key) { return key.replaceAll("_", " ").replace(/ ns$/, " (ns)"); }
// inputField builds one editable typed field; integer fields are bounded by the shared API validator.
function inputField(parent, object, key, choices = null) {
  const label = document.createElement("label"); label.textContent = labelText(key);
  const value = object[key]; const field = document.createElement(choices ? "select" : "input");
  if (choices) { for (const choice of new Set([String(value), ...choices])) { const option = document.createElement("option"); option.value = choice; option.textContent = choice; field.append(option); } }
  else { field.type = "text"; field.spellcheck = false; if (typeof value === "number") field.inputMode = "numeric"; }
  field.value = String(value);
  field.addEventListener("input", function editField() { /* Keep decimal schema fields as text; record invalid integer edits rather than coercing them. */
    const next = typeof value === "number" ? Number(field.value) : field.value;
    if (typeof value === "number" && (!/^-?\d+$/.test(field.value) || !Number.isSafeInteger(next))) state.invalid.add(field);
    else { state.invalid.delete(field); object[key] = next; }
    controls();
  });
  label.append(field); parent.append(label);
}
// jsonField edits open metadata/arrays without losing exact decimal text; malformed edits block save/start.
function jsonField(parent, object, key) {
  const label = document.createElement("label"); label.textContent = labelText(key) + " (JSON)";
  const field = document.createElement("textarea"); field.rows = 3; field.value = JSON.stringify(object[key], null, 2); field.spellcheck = false;
  field.addEventListener("input", function editJSON() { /* Retain malformed pending input visibly until it parses, without replacing the prior model. */
    try { object[key] = JSON.parse(field.value); state.invalid.delete(field); } catch { state.invalid.add(field); }
    controls();
  });
  label.append(field); parent.append(label);
}
// objectFields derives common controls from the shared normalized profile instead of per-protocol forms.
function objectFields(parent, object) {
  const grid = document.createElement("div"); grid.className = "grid";
  for (const [key, value] of Object.entries(object)) {
    if (value !== null && typeof value === "object") jsonField(grid, object, key);
    else inputField(grid, object, key, key === "mode" ? ["steady", "burst", "quiet"] : null);
  }
  parent.append(grid);
}
// descriptor resolves the selected registered adapter without silently substituting unsupported carriers.
function descriptor() { return state.adapters.find(function match(item) { /* Match stable adapter identity only. */ return item.adapter_id === state.draft.adapter_id; }); }
// adapterOptions renders descriptor-declared option types, retaining unknown/unsupported settings for validation.
function adapterOptions(parent) {
  const d = descriptor(); const grid = document.createElement("div"); grid.className = "grid";
  const options = state.draft.adapter_options;
  if (!options || typeof options !== "object" || Array.isArray(options)) { jsonField(grid, state.draft, "adapter_options"); }
  else for (const key of new Set([...Object.keys(d ? d.options : {}), ...Object.keys(options)])) {
    const description = d && d.options[key];
    const label = document.createElement("label"); label.textContent = key + " — " + (description || "unsupported option");
    const field = document.createElement("textarea"); field.rows = 1; field.spellcheck = false; field.value = key in options ? JSON.stringify(options[key]) : "";
    field.addEventListener("input", function editOption() { /* Require explicit JSON values, including booleans; never choose hidden option defaults. */
      try { options[key] = JSON.parse(field.value); state.invalid.delete(field); } catch { state.invalid.add(field); }
      controls();
    });
    label.append(field); grid.append(label);
  }
  parent.append(grid);
  element("capabilities").textContent = d ? "Registered " + d.adapter_id + " contract " + d.adapter_contract_version + "; maximum authenticated frame " + d.max_frame_bytes + " bytes; families " + d.address_families.join(", ") + "; platforms " + d.platforms.join(", ") + ". Payload excludes measurement/authentication framing. Fragmentation and adjustable headers are unavailable in this slice. Requested DSCP is not evidence of applied or captured marking." : "Unsupported adapter: no fallback. Choose a registered adapter and validate its explicit options.";
}
// renderPhases provides ordered common phase controls and independent directional fields.
function renderPhases() {
  const root = element("phases"); root.replaceChildren();
  state.draft.phases.forEach(function phaseForm(phase, index) { /* Preserve phase order and identities until the operator explicitly changes them. */
    const card = document.createElement("div"); card.className = "phase";
    const title = document.createElement("h3"); title.textContent = "Phase " + (index + 1); card.append(title);
    const head = document.createElement("div"); head.className = "grid"; inputField(head, phase, "phase_id"); inputField(head, phase, "duration_ns");
    const override = { deadline_ns: phase.deadline_ns === undefined ? "" : phase.deadline_ns };
    inputField(head, override, "deadline_ns");
    head.lastChild.firstElementChild.addEventListener("input", function phaseDeadline(event) { /* Empty override inherits the run deadline; nonempty decimal text is validated by the API. */ if (event.target.value === "") delete phase.deadline_ns; else phase.deadline_ns = event.target.value; controls(); });
    card.append(head);
    for (const direction of ["client", "server"]) { const h = document.createElement("h3"); h.textContent = direction === "client" ? "Upstream data" : "Downstream data"; card.append(h); objectFields(card, phase[direction]); }
    const row = document.createElement("div"); row.className = "row";
    for (const [text, delta] of [["Move up", -1], ["Move down", 1], ["Remove", 0]]) {
      const button = document.createElement("button"); button.textContent = text;
      button.addEventListener("click", function reorderPhase() { /* Phase editing never mutates the frozen selection or active run. */
        if (state.invalid.size) { notice("Resolve invalid inputs before reordering phases.", true); return; }
        const phases = state.draft.phases;
        if (delta === 0) { if (phases.length === 1) { notice("At least one phase is required.", true); return; } phases.splice(index, 1); }
        else if (index + delta >= 0 && index + delta < phases.length) [phases[index], phases[index + delta]] = [phases[index + delta], phases[index]];
        renderEditor(); controls();
      });
      row.append(button);
    }
    card.append(row); root.append(card);
  });
}
// renderEditor builds fields from the normalized schema, keeping adapter choices registry-driven.
function renderEditor() {
  state.invalid.clear(); element("profile-fields").replaceChildren(); element("advanced-fields").replaceChildren();
  const common = document.createElement("div"); common.className = "grid";
  const advanced = element("advanced-fields");
  const keys = ["profile_id", "pairing_ref", "adapter_id", "adapter_contract_version", "address_family", "server_address", "destination_port", "source_port", "warmup_ns", "grace_ns", "receipt_deadline_ns"];
  for (const key of keys) {
    inputField(common, state.draft, key, key === "adapter_id" ? state.adapters.map(function adapterID(d) { /* Enumerate only registered adapter IDs. */ return d.adapter_id; }) : key === "address_family" ? (descriptor() ? descriptor().address_families : []) : null);
    if (key === "adapter_id") common.lastChild.firstElementChild.addEventListener("change", function chooseAdapter() { /* Keep options explicit; changing protocol does not rewrite a workload silently. */
      if (state.invalid.size) { notice("Resolve invalid inputs before changing adapters.", true); return; }
      const d = descriptor(); if (d) state.draft.adapter_contract_version = d.adapter_contract_version;
      renderEditor(); controls();
    });
  }
  element("profile-fields").append(common); adapterOptions(element("profile-fields"));
  for (const [key, value] of Object.entries(state.draft)) {
    if (keys.includes(key) || ["schema_version", "phases", "adapter_options"].includes(key)) continue;
    const h = document.createElement("h3"); h.textContent = labelText(key); advanced.append(h);
    if (["limits", "calibration"].includes(key)) objectFields(advanced, value);
    else if (value !== null && typeof value === "object") jsonField(advanced, state.draft, key);
    else inputField(advanced, state.draft, key);
  }
  renderPhases(); element("profile-json").value = JSON.stringify(state.draft, null, 2);
}
// selectProfile replaces the draft only on explicit load/save/connect; revisions remain separate from edits.
function selectProfile(selected, saved) { if (selected.profile.phases.length > 64) throw new Error("This dashboard edits at most 64 phases; use the CLI for larger profiles."); state.selected = clone(selected); state.draft = clone(selected.profile); state.saved = saved; renderEditor(); controls(); }
// catalog loads bounded pages sequentially and rejects excessive catalogs rather than silently truncating them.
async function catalog(namespace) {
  const all = []; let cursor = "0";
  for (let page = 0; page < 16; page++) {
    const result = await api("/api/v1/" + namespace + "?cursor=" + cursor + "&limit=16"); all.push(...result.items);
    if (!result.next_cursor) return all;
    if (!/^\d+$/.test(result.next_cursor) || Number(result.next_cursor) <= Number(cursor)) throw new Error("Invalid catalog cursor.");
    cursor = result.next_cursor;
  }
  throw new Error("Catalog exceeds dashboard capacity.");
}
// fillSelect renders saved identities through text properties, preserving the explicit selection where possible.
function fillSelect(id, items) {
  const field = element(id); const previous = field.value; field.replaceChildren();
  for (const item of items) { const option = document.createElement("option"); option.value = item; option.textContent = item; field.append(option); }
  if (items.includes(previous)) field.value = previous;
}
// refreshCatalogs reads profiles/pairings serially to respect the server's single saved-operation admission.
async function refreshCatalogs() {
  fillSelect("profiles", await catalog("profiles"));
  state.pairings = await catalog("pairings"); fillSelect("pairings", state.pairings.map(function pairingID(p) { /* Extract only public catalog identities. */ return p.pairing_id; })); previewPairing();
}
// previewPairing displays server identity and endpoint before selection/import, without connecting remotely.
function previewPairing() {
  const pairing = (state.pairings || []).find(function matchPairing(p) { /* Resolve the selected safe catalog identity. */ return p.pairing_id === element("pairings").value; });
  element("pairing-preview").textContent = pairing ? JSON.stringify(pairing.bundle, null, 2) : "No pairing selected.";
}
// readyDraft blocks malformed local input; the authoritative shared validator remains the API.
function readyDraft() { if (state.invalid.size) throw new Error("Resolve invalid integer/JSON fields before validating or saving."); return clone(state.draft); }
// showRun presents actual API states and nulls; final completeness is never inferred from HTTP success.
function showRun(run) {
  state.run = run; element("run-id").value = run.control_run_id; element("run-state").textContent = "Run " + run.control_run_id + " · " + run.status;
  element("run-detail").textContent = JSON.stringify(run, null, 2); controls();
}
// schedulePoll coalesces one status read per second only while an observed run is executing/cancelling.
function schedulePoll() {
  clearTimeout(state.timer);
  if (!state.token || !state.run || !["executing", "cancelling"].includes(state.run.status)) return;
  const target = state.run.control_run_id;
  state.timer = setTimeout(async function pollStatus() { /* Poll completion never alters scheduling or retries a mutation. Stop visibly on errors. */
    try { const run = await api("/api/v1/runs/" + safeID(target)); if (state.run && state.run.control_run_id === target) { showRun(run); schedulePoll(); } }
    catch { notice("Status refresh stopped. Read status explicitly to recover; the run may still be active.", true); }
  }, 1000);
}
// transmitPending retries only the exact retained command, preserving ambiguous acknowledgment identity.
async function transmitPending() {
  const command = state.pending;
  if (!command || command.session !== state.session) throw new Error("Retained command belongs to another API process. Reconnect the original session to retry, or reload this page for the new process.");
  try {
    const run = await api(command.path, "POST", command.fields, command.id);
    if (command.frozen) { state.frozen = clone(command.frozen); element("frozen").textContent = JSON.stringify(state.frozen, null, 2); }
    state.pending = null; showRun(run); schedulePoll(); notice("Run command acknowledged: " + run.status + ".");
  } catch (error) { if (error.definite) state.pending = null; else error.message = "Command acknowledgment unknown. Retry the retained command or read status. " + error.message; throw error; }
}
// disconnect forgets the token and stops reads; it does not send a run cancellation or alter saved files.
function disconnect() {
  if (typeof clearDiagnostics === "function") clearDiagnostics();
  if (typeof clearComparison === "function") clearComparison();
  state.epoch++; state.token = ""; state.session = ""; clearTimeout(state.timer); element("token").value = ""; element("workspace").hidden = true; notice("Disconnected. Any admitted measurement continues until completion or explicit cancellation."); controls();
}
// bind wraps explicit actions with serialization and visible errors, avoiding unhandled promises.
function bind(id, action) { element(id).addEventListener("click", function clicked() { /* One explicit UI action at a time. */ void perform(action); }); }

bind("connect", async function connectSession() { /* Bootstrap over the exact protected local origin, retaining no token in the form. */
  const token = element("token").value.trim(); if (!/^[a-f0-9]{64}$/.test(token)) throw new Error("Paste the 64-character session token from this client-api process.");
  state.token = token; state.epoch++; element("token").value = "";
  state.session = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(token))), function fingerprintByte(b) { /* Retain only a nonsecret process fingerprint with ambiguous commands after disconnect. */ return b.toString(16).padStart(2, "0"); }).join("");
  try { const setup = await api("/api/v1/setup"); state.adapters = await catalog("adapters"); state.client = setup.client_enrollment; selectProfile(setup.selected_profile, setup.saved); await refreshCatalogs(); element("enrollment").textContent = state.client ? JSON.stringify(state.client, null, 2) : "No managed client identity. Generate one explicitly, then enroll its public key on the server."; if (state.client) element("client-id").value = state.client.client_id; element("workspace").hidden = false; notice("Local session connected. Review pairing and explicitly save your workload."); }
  catch (error) { disconnect(); throw error; }
});
bind("disconnect", disconnect);
bind("refresh-catalogs", async function refreshSaved() { /* Refresh catalogs without replacing unsaved editor fields. */ await refreshCatalogs(); notice("Saved lists refreshed; editor unchanged."); });
element("pairings").addEventListener("change", previewPairing);
bind("use-pairing", function usePairing() { /* Assign only the pairing reference; preserve all pending inputs during explicit selection. */ if (state.invalid.size) throw new Error("Resolve invalid inputs before selecting a pairing."); state.draft.pairing_ref = safeID(element("pairings").value); renderEditor(); notice("Pairing reference selected. Review the data address separately and save edits."); });
bind("import-pairing", async function importPairing() { /* Replacement requires the explicit checkbox; bundle identity is previewed as plain JSON. */
  const bundle = JSON.parse(element("bundle").value); const id = safeID(element("pairing-id").value);
  await api("/api/v1/pairings/import", "POST", { pairing_id: id, replace: element("replace").checked, bundle }); element("replace").checked = false; await refreshCatalogs(); element("pairings").value = id; previewPairing(); notice("Public pairing saved. Review and select its reference explicitly.");
});
bind("create-key", async function createKey() { /* Generate only the API-managed client key; existing identities are not replaced. */ const client = await api("/api/v1/client-keys", "POST", { client_id: safeID(element("client-id").value) }); state.client = client.enrollment || client; element("enrollment").textContent = JSON.stringify(state.client, null, 2); notice("Managed client identity generated. Transfer its public enrollment to the server."); });
bind("public-key", async function publicKey() { /* Read only public enrollment, never a private key/reference. */ const client = await api("/api/v1/client-keys/" + safeID(element("client-id").value) + "/public"); state.client = client; element("enrollment").textContent = JSON.stringify(client, null, 2); notice("Public enrollment loaded."); });
bind("load-profile", async function loadProfile() { /* Explicit load replaces unsaved edits, as stated by the button and operator contract. */ selectProfile(await api("/api/v1/profiles/" + safeID(element("profiles").value)), true); notice("Saved profile loaded; editor replaced."); });
bind("validate", async function validateProfile() { /* Validation does not save or change the selected run revision. */ const result = await api("/api/v1/profiles/validate", "POST", { profile_revision: "", profile: readyDraft() }); state.draft = clone(result.profile); renderEditor(); notice("Shared profile validation passed. Save edits explicitly before starting."); });
bind("save", async function saveProfile() { /* Send the loaded revision for the same ID; changing ID creates only, never blindly overwrites. */
  const draft = readyDraft(); const revision = draft.profile_id === state.selected.profile.profile_id && state.saved ? state.selected.profile_revision : "";
  const saved = await api("/api/v1/profiles/" + safeID(draft.profile_id), "PUT", { profile_revision: revision, profile: draft }); selectProfile(saved, true); await refreshCatalogs(); element("profiles").value = draft.profile_id; notice("Profile explicitly saved. Active run settings stay frozen.");
});
bind("add-phase", function addPhase() { /* Duplicate user settings without inventing traffic defaults; operator must choose a unique phase identity. */ if (state.invalid.size) throw new Error("Resolve invalid inputs before adding a phase."); if (state.draft.phases.length >= 64) throw new Error("Dashboard phase capacity reached."); const phase = clone(state.draft.phases.at(-1)); phase.phase_id = "phase-" + requestID().slice(3, 11); state.draft.phases.push(phase); renderEditor(); });
bind("show-json", function showJSON() { /* Serialize exact editor values into a local editable field. */ element("profile-json").value = JSON.stringify(readyDraft(), null, 2); notice("Editor copied to the JSON field."); });
bind("apply-json", async function applyJSON() { /* Validate original JSON bytes through the shared strict decoder before rebuilding fields. */
  const text = element("profile-json").value; if (new TextEncoder().encode(text).length > 1048576) throw new Error("Profile exceeds the document limit.");
  const result = await api("/api/v1/profiles/validate", "POST", { profile_revision: "" }, requestID(), text);
  if (result.profile.phases.length > 64) throw new Error("This dashboard edits at most 64 phases; use the CLI for larger profiles.");
  state.draft = clone(result.profile); renderEditor(); notice("Validated JSON applied to unsaved editor. Save explicitly.");
});
bind("start", async function startRun() { /* Only a clean saved revision starts; retain exact plan/id for retries after lost acknowledgments. */
  if (!state.saved || dirty() || state.pending) throw new Error("Save all edits and resolve any pending command before starting.");
  state.pending = { id: requestID(), session: state.session, path: "/api/v1/runs", fields: { profile_id: state.selected.profile.profile_id, profile_revision: state.selected.profile_revision }, frozen: clone(state.selected) }; await transmitPending();
});
bind("cancel", async function cancelRun() { /* Cancel the explicit handle only, including recovery after a page reload. */ if (state.pending) throw new Error("Resolve the retained command before sending another mutation."); state.pending = { id: requestID(), session: state.session, path: "/api/v1/runs/" + safeID(element("run-id").value) + "/cancel", fields: {} }; await transmitPending(); });
bind("retry", transmitPending);
bind("status", async function readStatus() { /* Recover the explicit handle; unknown IDs are errors, not terminal results. */ const run = await api("/api/v1/runs/" + safeID(element("run-id").value)); showRun(run); schedulePoll(); notice("Status read from the local runner."); });
controls();
