"use strict";

// state keeps session secrets, edits and command retries in memory only; no URL/cookie/storage bootstrap.
const state = { token: "", session: "", draft: null, selected: null, saved: false, adapters: [], client: null, busy: false, invalid: new Set(), run: null, frozen: null, pending: null, timer: null, epoch: 0, sshEnabled:false, connection:null, trust:null };
const phaseEditor = { index: 0, direction: "timing" };
let livePage=0;
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
  if (!state.token) throw new Error("Reload the local dashboard to recover access.");
  const epoch = state.epoch;
  let body = fields === null ? undefined : JSON.stringify({ schema_version: 1, request_id: id, ...fields });
  if (rawProfile !== null) body = body.slice(0, -1) + ',"profile":' + rawProfile + "}";
  if (body && new TextEncoder().encode(body).length > 1048576) throw new Error("Document exceeds the 1 MiB API body limit.");
  const response = await fetch(path, { method, headers: { Authorization: "Bearer " + state.token, "X-Request-ID": id, "Content-Type": "application/json" }, body, credentials: "omit", cache: "no-store", redirect: "error", signal: AbortSignal.timeout(path==="/api/v1/connection" ? 25000 : 5000) });
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
  navigation.authenticated=!!state.token;
  for (const node of document.querySelectorAll("#workspace button, #workspace input, #workspace select, #workspace textarea")) node.disabled = state.busy || !state.token;
  const linked=state.connection?.status==="connected", running=state.run && ["executing","cancelling"].includes(state.run.status);
  navigation.serverConnected=linked; navigation.serverRequired=state.sshEnabled;
  navigation.trustPending=!!state.trust;
  element("connect").disabled = state.busy || !state.token || linked || state.connection?.status==="unknown" || !state.sshEnabled;
  element("disconnect").disabled = state.busy || !linked || !!running || !!state.pending;
  for(const id of ["server-ip","ssh-port","ssh-user","ssh-password"]) element(id).disabled=state.busy || linked || !state.token;
  element("trust-connect").disabled=state.busy || !state.trust;
  element("replace-ssh-host").disabled=state.busy || !state.trust;
  element("refresh-connection").disabled=state.busy || !state.token || !state.sshEnabled;
  element("create-key").disabled = state.busy || !state.token || !!state.client;
  element("validate").disabled=state.busy || !state.token; element("save").disabled=state.busy || !state.token;
  element("live-prev").disabled=state.busy || !state.token || !state.run?.progress || livePage===0; element("live-next").disabled=state.busy || !state.token || !state.run?.progress || (livePage+1)*livePageSize()>=10;
  element("start").disabled = state.busy || !state.token || state.invalid.size>0 || state.sshEnabled && !linked || !!state.pending || !!running;
  element("retry").hidden = !state.pending;
  element("pending").textContent = state.pending ? "Acknowledgment unknown. Retry uses the same command ID and frozen target. Do not start another run." : "";
  element("edit-state").textContent = state.draft ? (dirty() || !state.saved ? "Run Test validates and saves these edits, then negotiates with the server." : "Saved workload. Run Test agrees these exact settings with the server.") : "";
  if (typeof diagnosticControls === "function") diagnosticControls();
  if (typeof comparisonControls === "function") comparisonControls();
  navigationControls();
  for (const entry of navigation.fields) { const row=entry.root.lastChild; if (row && row.className==="pager") { row.children[0].disabled=state.busy || entry.page===0; row.children[2].disabled=state.busy || entry.page+1>=Math.ceil(entry.fields.length/fieldPageSize()); } }
  for (const entry of navigation.readers.values()) { const row=element(entry.id+"-pages").lastChild; if(row) { row.children[0].disabled=entry.page===0; row.children[2].disabled=entry.page+1>=entry.pageCount; } }
}
// perform serializes explicit UI actions; status polling remains independent and never overlaps itself.
async function perform(action) {
  if (state.busy) return;
  state.busy = true; controls();
  try { await action(); } catch (error) { notice(error.message, true); }
  finally { state.busy = false; controls(); }
}
// labelText makes schema field names legible while retaining exact units for canonical inputs.
function labelText(key) { const names={profile_id:"Workload name",pairing_ref:"Saved server pairing",adapter_id:"Protocol",adapter_contract_version:"Measurement contract version",address_family:"IP version",server_address:"Server data address",destination_port:"Server data port",source_port:"Client source port",warmup_ns:"Warmup duration (ns)",grace_ns:"Wait for late receipts (ns)",receipt_deadline_ns:"Receipt deadline (ns)",phase_id:"Phase name",duration_ns:"Phase duration (ns)",deadline_ns:"Deadline override (ns; blank inherits)",mode:"Traffic pattern",rate_numerator:"Messages per rate period",rate_denominator:"Rate period (seconds)",content_bytes:"Payload size (bytes)",content_pattern_base64:"Payload pattern (Base64)",active_ns:"Burst duration (ns)",gap_ns:"Pause between bursts (ns)",dscp:"Traffic priority (DSCP)"}; return names[key] || key.replaceAll("_", " ").replace(/ ns$/, " (ns)"); }
// inputField builds one editable typed field; integer fields are bounded by the shared API validator.
function inputField(parent, object, key, choices = null) {
  const label = document.createElement("label"); label.textContent = labelText(key);
  label.dataset.field = key;
  const value = object[key]; const field = document.createElement(choices ? "select" : "input");
  if (choices) { for (const choice of new Set([String(value), ...choices])) { const option = document.createElement("option"); option.value = choice; option.textContent = choice; field.append(option); } }
  else { field.type = "text"; field.spellcheck = false; if (typeof value === "number") field.inputMode = "numeric"; }
  field.value = String(value);
  field.addEventListener("input", function editField() { /* Keep decimal schema fields as text; record invalid integer edits rather than coercing them. */
    const next = typeof value === "number" ? Number(field.value) : field.value;
    if (typeof value === "number" && (!/^-?\d+$/.test(field.value) || !Number.isSafeInteger(next))) state.invalid.add(field);
    else { state.invalid.delete(field); object[key] = next; }
    field.setAttribute("aria-invalid",String(state.invalid.has(field)));
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
    field.setAttribute("aria-invalid",String(state.invalid.has(field)));
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
      field.setAttribute("aria-invalid",String(state.invalid.has(field)));
      controls();
    });
    label.append(field); grid.append(label);
  }
  parent.append(grid);
  element("capabilities").textContent = d ? d.adapter_id.toUpperCase()+" · "+d.address_families.join(" / ")+" · maximum authenticated frame "+d.max_frame_bytes+" bytes. Payload excludes measurement framing. Adjustable headers/fragmentation are unavailable." : "Unsupported protocol. Choose a registered adapter and validate its options.";
}
// renderPhases provides ordered common phase controls and independent directional fields.
function renderPhases() {
  const root = element("phases"); root.replaceChildren();
  phaseEditor.index=Math.min(phaseEditor.index,state.draft.phases.length-1);
  element("phase-select").replaceChildren();
  state.draft.phases.forEach(function phaseForm(phase, index) { /* Preserve phase order and identities until the operator explicitly changes them. */
    const option=document.createElement("option"); option.value=String(index); option.textContent=(index+1)+" · "+phase.phase_id; element("phase-select").append(option);
    const card = document.createElement("div"); card.className = "phase";
    const title = document.createElement("h3"); title.textContent = "Phase " + (index + 1); card.append(title);
    const head = document.createElement("div"); head.className = "grid"; inputField(head, phase, "phase_id"); inputField(head, phase, "duration_ns");
    const override = { deadline_ns: phase.deadline_ns === undefined ? "" : phase.deadline_ns };
    inputField(head, override, "deadline_ns");
    head.lastChild.firstElementChild.addEventListener("input", function phaseDeadline(event) { /* Empty override inherits the run deadline; nonempty decimal text is validated by the API. */ if (event.target.value === "") delete phase.deadline_ns; else phase.deadline_ns = event.target.value; controls(); });
    head.dataset.direction="timing"; card.append(head); pageFields(head);
    for (const direction of ["client", "server"]) { const pane=document.createElement("div"); pane.dataset.direction=direction; objectFields(pane,phase[direction]); card.append(pane); pageFields(pane); }
    for(const pane of card.children) if(pane.dataset.direction) { pane.id="phase-pane-"+index+"-"+pane.dataset.direction; pane.setAttribute("role","tabpanel"); pane.setAttribute("aria-labelledby","phase-"+pane.dataset.direction); }
    const row = document.createElement("div"); row.className = "row";
    for (const [text, delta] of [["Move up", -1], ["Move down", 1], ["Remove", 0]]) {
      const button = document.createElement("button"); button.textContent = text;
      button.addEventListener("click", function reorderPhase() { /* Phase editing never mutates the frozen selection or active run. */
        if (state.invalid.size) { notice("Resolve invalid inputs before reordering phases.", true); return; }
        const phases = state.draft.phases;
        if (delta === 0) { if (phases.length === 1) { notice("At least one phase is required.", true); return; } phases.splice(index, 1); }
        else if (index + delta >= 0 && index + delta < phases.length) [phases[index], phases[index + delta]] = [phases[index + delta], phases[index]];
        phaseEditor.index=Math.max(0,Math.min(phases.length-1,index+delta)); renderEditor(); controls();
      });
      row.append(button);
    }
    card.append(row); root.append(card);
  });
  element("phase-select").value=String(phaseEditor.index); showPhase();
}
// showPhase switches existing phase/direction controls without discarding pending invalid inputs.
function showPhase() {
  Array.from(element("phases").children).forEach(function selectedPhase(card,index) { /* Only one phase occupies the viewport. */ card.hidden=index!==phaseEditor.index; for(const pane of card.children) if(pane.dataset.direction) pane.hidden=pane.dataset.direction!==phaseEditor.direction; });
  for(const direction of ["timing","client","server"]) { const button=element("phase-"+direction); button.setAttribute("aria-controls","phase-pane-"+phaseEditor.index+"-"+direction); button.setAttribute("aria-selected",String(direction===phaseEditor.direction)); button.tabIndex=direction===phaseEditor.direction?0:-1; }
}
// renderEditor builds fields from the normalized schema, keeping adapter choices registry-driven.
function renderEditor() {
  state.invalid.clear(); navigation.fields=[];
  const groups={"identity-fields":["profile_id","pairing_ref"],"profile-fields":["adapter_id","adapter_contract_version","address_family"],"port-fields":["server_address","destination_port","source_port"],"timing-fields":["warmup_ns","grace_ns","receipt_deadline_ns"]};
  const keys=Object.values(groups).flat();
  for(const [id,fields] of Object.entries(groups)) { const common=element(id); common.replaceChildren();
  for (const key of fields) {
    inputField(common, state.draft, key, key === "adapter_id" ? state.adapters.map(function adapterID(d) { /* Enumerate only registered adapter IDs. */ return d.adapter_id; }) : key === "address_family" ? (descriptor() ? descriptor().address_families : []) : null);
    if (key === "adapter_id") common.lastChild.firstElementChild.addEventListener("change", function chooseAdapter() { /* Keep options explicit; changing protocol does not rewrite a workload silently. */
      if (state.invalid.size) { notice("Resolve invalid inputs before changing adapters.", true); return; }
      const d = descriptor(); if (d) state.draft.adapter_contract_version = d.adapter_contract_version;
      renderEditor(); controls();
    });
  } }
  adapterOptions(element("profile-fields"));
  for(const id of Object.keys(groups)) pageFields(element(id));
  const advanced=element("advanced-fields"); advanced.replaceChildren();
  for(const [key,id] of [["limits","limit-fields"],["calibration","calibration-fields"]]) { const root=element(id); root.replaceChildren(); objectFields(root,state.draft[key]); pageFields(root); }
  for (const [key, value] of Object.entries(state.draft)) {
    if (keys.includes(key) || ["schema_version", "phases", "adapter_options","limits","calibration"].includes(key)) continue;
    if (value !== null && typeof value === "object") jsonField(advanced, state.draft, key);
    else inputField(advanced, state.draft, key);
  }
  pageFields(advanced);
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
  readText("pairing-preview",pairing ? JSON.stringify(pairing.bundle, null, 2) : "No pairing selected.");
}
// readyDraft blocks malformed local input; the authoritative shared validator remains the API.
function readyDraft() { if (state.invalid.size) throw new Error("Resolve invalid integer/JSON fields before validating or saving."); return clone(state.draft); }
// showRun presents actual API states and nulls; final completeness is never inferred from HTTP success.
function showRun(run) {
  state.run = run; element("run-id").value = run.control_run_id; element("run-state").textContent = "Run " + run.control_run_id + " · " + run.status;
  readText("run-detail",JSON.stringify(run, null, 2)); renderLive(run); controls();
}

// renderLive presents exact provisional local counters; its progress bar alone uses a bounded display ratio.
function renderLive(run) {
  const p = run.progress;
  if (!p) { element("live-status").textContent = "Run " + run.status + ": waiting for client-local measurement progress."; element("live-progress").value = 0; element("live-counters").replaceChildren(); element("live-page").textContent="No counts yet"; return; }
  const terminal = !["executing", "cancelling"].includes(run.status);
  element("live-status").textContent = (terminal ? "Last provisional snapshot. " : "Provisional ") + p.stage + (p.phase_id ? " / " + p.phase_id : "") + " · elapsed " + p.elapsed_ns + " / " + p.total_ns + " ns · " + (p.accounting_valid ? "accounting active" : "accounting incomplete") + ". Full snapshot in Status details.";
  const total = BigInt(p.total_ns); element("live-progress").value = total > 0n ? Number(BigInt(p.elapsed_ns) * 1000n / total) : 0;
  const names = ["deadline_misses", "planned", "local_misses", "pending", "on_time", "late", "completed", "attempted", "written", "write_failed"];
  livePage=Math.min(livePage,Math.max(0,Math.ceil(names.length/livePageSize())-1));
  tablePage("live-counters", ["Provisional metric", "Main", "Warmup"], names.map(function liveRow(name) { /* Main/warmup are independent reached-slot populations, with no remote-loss inference. */ return [metricName(name), p.main[name], p.warmup[name]]; }),livePage,livePageSize());
  element("live-page").textContent="Page "+(livePage+1)+" / "+Math.ceil(names.length/livePageSize());
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
    if (command.frozen) { state.frozen = clone(command.frozen); readText("frozen",JSON.stringify(state.frozen, null, 2)); }
    state.pending = null; showRun(run); schedulePoll(); navigate("run","live"); notice("Run command acknowledged: " + run.status + ".");
  } catch (error) { if (error.definite) state.pending = null; else { navigate("run","live"); error.message = "Command acknowledgment unknown. Retry the retained command or read status. " + error.message; } throw error; }
}
// disconnect forgets the token and stops reads; it does not send a run cancellation or alter saved files.
function disconnect() {
  if (typeof clearDiagnostics === "function") clearDiagnostics();
  if (typeof clearComparison === "function") clearComparison();
  clearReaders(); state.epoch++; state.token = ""; state.session = ""; state.connection=null; state.trust=null; navigation.authenticated=false; clearTimeout(state.timer); element("ssh-password").value = ""; element("workspace").hidden = true; navigate("connection","session"); notice("Local dashboard access interrupted. Reload to recover; an admitted test may still be active."); controls();
}
// bind wraps explicit actions with serialization and visible errors, avoiding unhandled promises.
function bind(id, action) { element(id).addEventListener("click", function clicked() { /* One explicit UI action at a time. */ void perform(action); }); }

// bootstrapSession opens the local dashboard automatically through an exact-origin guarded grant.
async function bootstrapSession() {
  const id=requestID(), response=await fetch("/api/v1/dashboard-session",{method:"POST",headers:{"X-Dashboard-Bootstrap":"1","X-Request-ID":id},credentials:"omit",cache:"no-store",redirect:"error",signal:AbortSignal.timeout(5000)});
  const envelope=await response.json(); if(!response.ok || envelope.schema_version!==1 || envelope.request_id!==id || !/^[a-f0-9]{64}$/.test(envelope.result?.session_token)) throw new Error("Could not open the local dashboard. Reload its exact local URL.");
  state.token=envelope.result.session_token; state.epoch++;
  state.session=Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",new TextEncoder().encode(state.token))),function fingerprintByte(b) { /* Retain a nonsecret API-process retry identity. */ return b.toString(16).padStart(2,"0"); }).join("");
  try { const setup=await api("/api/v1/setup"); state.adapters=await catalog("adapters"); state.client=setup.client_enrollment; state.sshEnabled=setup.ssh_enabled; state.connection=setup.connection; selectProfile(setup.selected_profile,setup.saved); await refreshCatalogs(); readText("enrollment",state.client ? JSON.stringify(state.client,null,2) : "Connect creates a managed local identity automatically."); if(state.client) element("client-id").value=state.client.client_id; element("workspace").hidden=false; showConnection(); notice("Dashboard ready. Enter your server details and connect over SSH."); }
  catch(error) { disconnect(); throw error; }
}
// showConnection displays only public SSH/helper identity; credentials never enter a reader or profile.
function showConnection() { const linked=state.connection?.status==="connected"; if(linked) { element("server-ip").value=state.connection.server_ip; element("ssh-port").value=String(state.connection.ssh_port); element("ssh-user").value=state.connection.ssh_username; } element("connection-status").textContent=linked ? "Connected to "+state.connection.ssh_username+"@"+state.connection.server_ip+":"+state.connection.ssh_port+". Choose protocol settings in Workload." : "Enter the server IP, SSH port, username and password."; element("ssh-trust").hidden=!state.trust; if(state.trust) element("ssh-fingerprint").textContent="Server identity: "+state.trust.host_key_fingerprint+(state.trust.previous_fingerprint ? " · Previously trusted: "+state.trust.previous_fingerprint : ""); readText("connection-detail",JSON.stringify(state.connection || {status:"disconnected"},null,2)); controls(); }
// connectServer sends a password once to the local connector and applies only the chosen server target.
async function connectServer(accept=false) {
  const address=element("server-ip").value.trim(),port=element("ssh-port").value,user=element("ssh-user").value.trim(); if(!/^[1-9][0-9]{0,4}$/.test(port) || Number(port)>65535) throw new Error("SSH port must be 1–65535.");
  const fields={server_ip:address,ssh_port:Number(port),ssh_username:user,ssh_password:element("ssh-password").value};
  if(accept) { if(!state.trust || state.trust.server_ip!==address || state.trust.ssh_port!==Number(port) || state.trust.ssh_username!==user) throw new Error("Server details changed. Press Connect again."); fields.trust_fingerprint=state.trust.host_key_fingerprint; fields.replace_host_key=element("replace-ssh-host").checked; }
  let result;
  try { result=await api("/api/v1/connection","POST",fields); }
  catch(error) { element("ssh-password").value=""; state.trust=null; if(!error.definite) { state.connection={status:"unknown"}; error.message+=". Connection acknowledgment unknown. Press Refresh connection before connecting again."; } showConnection(); navigate("connection","session"); throw error; }
  if(result.status==="trust_required") { state.trust=result; element("replace-ssh-label").hidden=!result.previous_fingerprint; element("replace-ssh-host").checked=false; showConnection(); navigate("connection","trust"); notice("Verify the displayed SSH fingerprint, then trust this server to connect."); return; }
  element("ssh-password").value=""; state.trust=null; state.connection=result;
  const setup=await api("/api/v1/setup"); state.client=setup.client_enrollment; if(state.client) { element("client-id").value=state.client.client_id; readText("enrollment",JSON.stringify(state.client,null,2)); }
  state.draft.server_address=result.server_ip; state.draft.address_family=result.server_ip.includes(":") ? "ipv6" : "ipv4"; state.draft.pairing_ref=result.pairing_ref; renderEditor(); await refreshCatalogs(); showConnection(); navigate("workload","protocol"); notice("SSH connected. Configure your test, then press Run Test.");
}
bind("connect",connectServer);
bind("trust-connect",async function trustServer() { /* Accept only the actual observed fingerprint and an explicit changed-key replacement. */ await connectServer(true); });
bind("disconnect",async function disconnectServer() { /* End the owned helper only after active work/retries are resolved. */ await api("/api/v1/connection","DELETE"); state.connection={status:"disconnected"}; state.trust=null; element("ssh-password").value=""; showConnection(); navigate("connection","session"); notice("SSH server disconnected."); });
bind("refresh-connection",async function refreshConnection() { /* Recover helper/SSH state after a lost Connect acknowledgment; preserve every pending invalid field. */ state.connection=await api("/api/v1/connection"); if(state.connection.status==="connected" && !state.client) { const setup=await api("/api/v1/setup"); state.client=setup.client_enrollment; if(state.client) { element("client-id").value=state.client.client_id; readText("enrollment",JSON.stringify(state.client,null,2)); } } showConnection(); if(state.connection.status==="connected" && !state.invalid.size) { state.draft.server_address=state.connection.server_ip; state.draft.address_family=state.connection.server_ip.includes(":") ? "ipv6" : "ipv4"; state.draft.pairing_ref=state.connection.pairing_ref; renderEditor(); } notice("Server connection status refreshed."); });
bind("refresh-catalogs", async function refreshSaved() { /* Refresh catalogs without replacing unsaved editor fields. */ await refreshCatalogs(); notice("Saved lists refreshed; editor unchanged."); });
element("pairings").addEventListener("change", previewPairing);
bind("use-pairing", function usePairing() { /* Assign only the pairing reference; preserve all pending inputs during explicit selection. */ if (state.invalid.size) throw new Error("Resolve invalid inputs before selecting a pairing."); state.draft.pairing_ref = safeID(element("pairings").value); renderEditor(); notice("Pairing reference selected. Review the data address separately and save edits."); });
bind("import-pairing", async function importPairing() { /* Replacement requires the explicit checkbox; bundle identity is previewed as plain JSON. */
  const bundle = JSON.parse(element("bundle").value); const id = safeID(element("pairing-id").value);
  await api("/api/v1/pairings/import", "POST", { pairing_id: id, replace: element("replace").checked, bundle }); element("replace").checked = false; await refreshCatalogs(); element("pairings").value = id; previewPairing(); notice("Public pairing saved. Review and select its reference explicitly.");
});
bind("create-key", async function createKey() { /* Generate only the API-managed client key; existing identities are not replaced. */ const client = await api("/api/v1/client-keys", "POST", { client_id: safeID(element("client-id").value) }); state.client = client.enrollment || client; readText("enrollment",JSON.stringify(state.client, null, 2)); notice("Managed client identity generated. Transfer its public enrollment to the server."); });
bind("public-key", async function publicKey() { /* Read only public enrollment, never a private key/reference. */ const client = await api("/api/v1/client-keys/" + safeID(element("client-id").value) + "/public"); state.client = client; readText("enrollment",JSON.stringify(client, null, 2)); notice("Public enrollment loaded."); });
bind("load-profile", async function loadProfile() { /* Explicit load replaces unsaved edits, as stated by the button and operator contract. */ selectProfile(await api("/api/v1/profiles/" + safeID(element("profiles").value)), true); notice("Saved profile loaded; editor replaced."); });
bind("validate", async function validateProfile() { /* Validation does not save or change the selected run revision. */ const result = await api("/api/v1/profiles/validate", "POST", { profile_revision: "", profile: readyDraft() }); state.draft = clone(result.profile); renderEditor(); notice("Shared profile validation passed. Save edits explicitly before starting."); });
bind("save", async function saveProfile() { /* Send the loaded revision for the same ID; changing ID creates only, never blindly overwrites. */
  const draft = readyDraft(); const revision = draft.profile_id === state.selected.profile.profile_id && state.saved ? state.selected.profile_revision : "";
  const saved = await api("/api/v1/profiles/" + safeID(draft.profile_id), "PUT", { profile_revision: revision, profile: draft }); selectProfile(saved, true); await refreshCatalogs(); element("profiles").value = draft.profile_id; notice("Profile explicitly saved. Active run settings stay frozen.");
});
bind("add-phase", function addPhase() { /* Duplicate user settings without inventing traffic defaults; operator must choose a unique phase identity. */ if (state.invalid.size) throw new Error("Resolve invalid inputs before adding a phase."); if (state.draft.phases.length >= 64) throw new Error("Dashboard phase capacity reached."); const phase = clone(state.draft.phases.at(-1)); phase.phase_id = "phase-" + requestID().slice(3, 11); state.draft.phases.push(phase); phaseEditor.index=state.draft.phases.length-1; phaseEditor.direction="timing"; renderEditor(); navigate("workload","phases"); });
bind("show-json", function showJSON() { /* Serialize exact editor values into a local editable field. */ element("profile-json").value = JSON.stringify(readyDraft(), null, 2); notice("Editor copied to the JSON field."); });
bind("apply-json", async function applyJSON() { /* Validate original JSON bytes through the shared strict decoder before rebuilding fields. */
  const text = element("profile-json").value; if (new TextEncoder().encode(text).length > 1048576) throw new Error("Profile exceeds the document limit.");
  const result = await api("/api/v1/profiles/validate", "POST", { profile_revision: "" }, requestID(), text);
  if (result.profile.phases.length > 64) throw new Error("This dashboard edits at most 64 phases; use the CLI for larger profiles.");
  state.draft = clone(result.profile); renderEditor(); notice("Validated JSON applied to unsaved editor. Save explicitly.");
});
bind("start", async function startRun() { /* Run Test validates/saves the current draft, then retains the exact negotiated revision and command identity. */
  if(state.pending) throw new Error("Resolve the pending command before running another test.");
  if(state.sshEnabled) { state.connection=await api("/api/v1/connection"); if(state.connection.status!=="connected") { showConnection(); throw new Error("Connect the SSH server first."); } if(state.draft.server_address!==state.connection.server_ip || state.draft.pairing_ref!==state.connection.pairing_ref) throw new Error("This workload targets another server. Reconnect its SSH server before testing."); }
  if(!state.saved || dirty()) { const validated=await api("/api/v1/profiles/validate","POST",{profile_revision:"",profile:readyDraft()}); const draft=validated.profile,revision=draft.profile_id===state.selected.profile.profile_id && state.saved ? state.selected.profile_revision : ""; const saved=await api("/api/v1/profiles/"+safeID(draft.profile_id),"PUT",{profile_revision:revision,profile:draft}); selectProfile(saved,true); await refreshCatalogs(); }
  state.pending = { id: requestID(), session: state.session, path: "/api/v1/runs", fields: { profile_id: state.selected.profile.profile_id, profile_revision: state.selected.profile_revision }, frozen: clone(state.selected) }; await transmitPending();
});
bind("cancel", async function cancelRun() { /* Cancel the explicit handle only, including recovery after a page reload. */ if (state.pending) throw new Error("Resolve the retained command before sending another mutation."); state.pending = { id: requestID(), session: state.session, path: "/api/v1/runs/" + safeID(element("run-id").value) + "/cancel", fields: {} }; await transmitPending(); });
bind("retry", transmitPending);
bind("status", async function readStatus() { /* Recover the explicit handle; unknown IDs are errors, not terminal results. */ const run = await api("/api/v1/runs/" + safeID(element("run-id").value)); showRun(run); schedulePoll(); notice("Status read from the local runner."); });
element("phase-select").addEventListener("change",function choosePhase() { /* Select existing phase controls without resetting edits. */ const index=Number(element("phase-select").value); if(Number.isInteger(index) && index>=0 && index<state.draft.phases.length) { phaseEditor.index=index; showPhase(); } });
bind("live-prev",function previousLiveCounts() { /* Paginate the last immutable status only. */ if(livePage>0) livePage--; if(state.run) renderLive(state.run); });
bind("live-next",function nextLiveCounts() { /* Never fetch or schedule traffic from a count page. */ if((livePage+1)*livePageSize()<10) livePage++; if(state.run) renderLive(state.run); });
for(const direction of ["timing","client","server"]) { const button=element("phase-"+direction); button.addEventListener("click",function phaseDirection() { /* Directional page changes never mutate traffic. */ phaseEditor.direction=direction; showPhase(); }); button.addEventListener("keydown",function phaseTabKey(event) { /* Vertical keyboard navigation for the selected phase. */ const buttons=["timing","client","server"].map(function phaseButton(name) { /* Resolve fixed direction buttons. */ return element("phase-"+name); }); tabKeys(event,buttons,buttons.indexOf(button)); }); }
controls();
const bootPromise=perform(bootstrapSession);
