"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const { webcrypto } = require("node:crypto");

// Node models only the DOM operations used by the real script; it does not claim layout/browser coverage.
class Node {
  // constructor starts one inert DOM field with independently retained listeners and attributes.
  constructor(tag) { this.tagName = tag; this.children = []; this.listeners = {}; this.dataset = {}; this.value = ""; this.textContent = ""; this.disabled = false; this.hidden = false; this.checked = false; }
  // append preserves ordered form children and the native select's initial choice for fixtures.
  append(...nodes) { for (const node of nodes) { if (node.parent) node.parent.children.splice(node.parent.children.indexOf(node),1); node.parent=this; this.children.push(node); } if (this.tagName === "select" && !this.value && nodes.length) this.value = nodes[0].value; }
  // replaceChildren replaces only owned form nodes when the UI explicitly rebuilds an editor.
  replaceChildren(...nodes) { for(const node of this.children) node.parent=null; this.children = []; this.textContent=""; if (this.tagName === "select") this.value = ""; this.append(...nodes); }
  // addEventListener records actions for explicit fixture activation without synthetic scheduling.
  addEventListener(name, action) { (this.listeners[name] ||= []).push(action); }
  // setAttribute retains native SVG attributes for bounded rendering assertions.
  setAttribute(name, value) { (this.attributes ||= {})[name] = value; }
  // click activates local buttons or records an explicit export without claiming browser rendering.
  click() { if(this.disabled) return; if(this.tagName==="a") downloads.push({ name:this.download, href:this.href }); else for(const action of this.listeners.click || []) action(); }
  // focus records keyboard navigation's selected control without claiming native focus rendering.
  focus() { focused=this; }
  // showModal models dialog ownership; browser geometry and native focus still require rendered checks.
  showModal() { this.open=true; }
  // close models a utility dialog ending without changing the selected measurement section.
  close() { this.open=false; }
  // contains follows modeled DOM ownership for selected reader reflow.
  contains(node) { return descendants(this).includes(node); }
  // firstElementChild exposes the owned label's input for override-field event wiring.
  get firstElementChild() { return this.children[0]; }
  // lastChild exposes the most recently appended label for descriptor/phase event wiring.
  get lastChild() { return this.children.at(-1); }
}
const html = fs.readFileSync(__dirname + "/index.html", "utf8");
const ids = Object.fromEntries(Array.from(html.matchAll(/id="([^"]+)"/g), function field(match) { /* Build fixed shell nodes from actual HTML IDs, detecting missing script bindings. */ const tag = new RegExp("<(\\w+)[^>]*id=\"" + match[1] + "\"").exec(html)[1]; return [match[1], new Node(tag)]; }));
assert.equal(Array.from(html.matchAll(/id="([^"]+)"/g)).length,Object.keys(ids).length,"duplicate HTML IDs");
for(const [id,node] of Object.entries(ids)) { node.id=id; node.hidden=/\bhidden\b/.test(new RegExp('<\\w+[^>]*id="'+id+'"[^>]*>').exec(html)[0]); }
for(const match of html.matchAll(/<section id="(view-[^"]+)"[\s\S]*?<\/section>/g)) { for(const child of match[0].matchAll(/id="([^"]+)"/g)) if(child[1]!==match[1]) ids[match[1]].append(ids[child[1]]); }
let focused=null;
const windowEvents={};
const fixtureWindow={innerWidth:1280,innerHeight:900,
  // addEventListener retains resize callbacks without claiming rendered viewport geometry.
  addEventListener(name,action) { (windowEvents[name] ||= []).push(action); }
};
const timers = new Map(); let timerID = 0; let dropStartReply = true; let dropConnectReply=true; let currentRun = "";
const requests = [];
const downloads = [];
let changedFingerprint=null;
// descendants recursively collects owned fields so disabling reflects the complete dynamic editor.
function descendants(node) { return [node, ...node.children.flatMap(descendants)]; }
const context = vm.createContext({ window:fixtureWindow, document: {
  // getElementById resolves only shell IDs actually present in the embedded HTML.
  getElementById(id) { const node=ids[id] || descendants(ids["section-tabs"]).find(function dynamicTab(item) { /* Resolve replacement tabs after keyboard activation. */ return item.id===id; }); assert.ok(node, "missing HTML binding: " + id); return node; },
  // createElement models safe text/field construction; no innerHTML interface is supplied.
  createElement(tag) { return new Node(tag); },
  // createElementNS models SVG construction without claiming browser geometry/layout coverage.
  createElementNS(namespace, tag) { assert.equal(namespace, "http://www.w3.org/2000/svg"); return new Node(tag); },
  // querySelectorAll returns all owned workspace controls for operation/dirty-state gating.
  querySelectorAll() { return Object.values(ids).flatMap(descendants); }
}, crypto: webcrypto, TextEncoder, AbortSignal, Blob, URL: {
  // createObjectURL models page-owned exported bytes without exposing the token in a URL.
  createObjectURL(blob) { assert.ok(blob.size>0); return "blob:fixture"; },
  // revokeObjectURL verifies release after each explicit download.
  revokeObjectURL(url) { assert.equal(url,"blob:fixture"); }
},
  // setTimeout retains declared cadence without background polling races in the scripted fixture.
  setTimeout(action, delay) { assert.equal(delay, 1000); timers.set(++timerID, action); return timerID; },
  // clearTimeout cancels the previous provisional read, never a measurement command.
  clearTimeout(id) { timers.delete(id); },
  // fetch uses the real owned API and simulates losing only a successful start acknowledgment.
  async fetch(path, options) {
    assert.ok(path.startsWith("/api/v1/")); assert.equal(options.credentials, "omit"); assert.equal(options.redirect, "error");
    const body = options.body && JSON.parse(options.body); requests.push({ path, body });
    if(changedFingerprint && path==="/api/v1/connection" && options.method==="POST") { const fingerprint=changedFingerprint; changedFingerprint=null; return new Response(JSON.stringify({schema_version:1,request_id:body.request_id,result:{status:"trust_required",server_ip:body.server_ip,ssh_port:body.ssh_port,ssh_username:body.ssh_username,host_key_fingerprint:fingerprint,previous_fingerprint:"SHA256:previous-fixture"}}),{status:200}); }
    const response = await fetch(process.env.DASHBOARD_TEST_URL + path, { ...options, headers: { ...options.headers, Origin: process.env.DASHBOARD_TEST_URL,"Sec-Fetch-Site":"same-origin" } });
    if(path==="/api/v1/connection" && options.method==="POST" && response.ok && dropConnectReply && (await response.clone().json()).result.status==="connected") { dropConnectReply=false; throw new Error("fixture lost Connect acknowledgment"); }
    if (path === "/api/v1/runs" && response.ok && dropStartReply) { dropStartReply = false; const data = await response.json(); currentRun = data.result.control_run_id; throw new Error("fixture lost acknowledgment"); }
    return response;
  }
});
vm.runInContext(fs.readFileSync(__dirname + "/navigation.js", "utf8"), context);
vm.runInContext(fs.readFileSync(__dirname + "/dashboard.js", "utf8"), context);
vm.runInContext(fs.readFileSync(__dirname + "/diagnostics.js", "utf8"), context);

vm.runInContext(fs.readFileSync(__dirname + "/workspace.js", "utf8"), context);
// evaluate inspects the actual script's lexical model instead of mirroring its implementation.
function evaluate(code) { return vm.runInContext(code, context); }
// click executes the bound UI handler and waits for its serialized asynchronous operation to finish.
async function click(id) {
  assert.equal(ids[id].disabled, false, "disabled control: " + id);
  for (const action of ids[id].listeners.click || []) action();
  const end = Date.now() + 6000;
  while (evaluate("state.busy")) { if (Date.now() > end) throw new Error("UI action timeout: " + id); await new Promise(function pause(resolve) { /* Yield until the finite HTTP action completes. */ setTimeout(resolve, 10); }); }
}
// navigationCheck exercises real tabs, keyboard focus and bounded field/detail pages without issuing API calls.
async function navigationCheck() {
  evaluate("navigation.advanced=true; navigationControls()");
  const count=requests.length, draft=evaluate("JSON.stringify(state.draft)");
  const groups=evaluate("sections");
  for(const [name,group] of Object.entries(groups)) {
    if(name==="dashboard") evaluate('navigate("dashboard","overview")'); else await click("nav-"+name);
    for(const [view] of group.views) {
      const button=ids["section-tabs"].children.find(function subtab(node) { /* Use the actual generated control. */ return node.id==="sub-"+name+"-"+view; }); if(button.disabled && view==="trust") continue; assert.equal(button.disabled,false,"locked tab: "+button.id); button.click();
      const visible=Object.entries(ids).filter(function visiblePanel(entry) { /* Inline settings belong to the main console; only activity sections count as independent views. */ return entry[0].startsWith("view-") && entry[1].tagName==="section" && !entry[1].hidden; });
      assert.deepEqual(visible.map(function panelID(entry) { /* Compare public panel identity. */ return entry[0]; }),["view-"+name+"-"+view]);
      assert.equal(ids["editor-actions"].hidden,name!=="workload");
    }
  }
  const last=ids["section-tabs"].children.at(-1); last.listeners.keydown[0]({key:"Home",
    // preventDefault supplies the native key event contract for secondary focus restoration.
    preventDefault() {}
  }); assert.equal(focused,ids["section-tabs"].firstElementChild); assert.equal(focused.attributes["aria-selected"],"true");
  evaluate('navigate("workload","phases"); phaseEditor.direction="client"; showPhase()');
  const fieldEntry=evaluate("navigation.fields").find(function directionalFields(entry) { /* Locate the selected phase's independently editable upstream fields. */ return entry.root.dataset.direction==="client"; });
  const label=fieldEntry.fields.find(function priorityField(node) { /* Keep field identity stable across pages. */ return node.dataset.field==="dscp"; });
  while(!fieldEntry.root.contains(label)) fieldEntry.root.lastChild.children[2].click();
  const input=label.firstElementChild; input.value="pending-bad"; input.listeners.input[0](); assert.equal(evaluate("state.invalid.size"),1);
  await click("phase-server"); evaluate('navigate("dashboard","overview")'); await click("nav-workload"); await click("phase-client"); assert.equal(input.value,"pending-bad"); assert.equal(input.attributes["aria-invalid"],"true");
  const baseline=evaluate("JSON.stringify(state.draft)"); fixtureWindow.innerHeight=640; fixtureWindow.innerWidth=700; for(const action of windowEvents.resize) action();
  assert.equal(input.value,"pending-bad"); assert.equal(evaluate("JSON.stringify(state.draft)"),baseline); assert.equal(evaluate("state.invalid.size"),1);
  while(!fieldEntry.root.contains(label)) fieldEntry.root.lastChild.children[2].click(); assert.equal(label.firstElementChild,input);
  input.value="0"; input.listeners.input[0](); fixtureWindow.innerHeight=900; fixtureWindow.innerWidth=1280; for(const action of windowEvents.resize) action();
  const source=Array.from({length:60},function detailLine(_,index) { /* Each page has independently identifiable content. */ return "line-"+index; }).join("\n");
  evaluate('showUtility("message")'); evaluate('readText("message-details",'+JSON.stringify(source)+')');
  let pages=ids["message-details"].textContent; let next=ids["message-details-pages"].lastChild.children[2]; while(!next.disabled) { next.click(); pages+="\n"+ids["message-details"].textContent; next=ids["message-details-pages"].lastChild.children[2]; }
  assert.equal(pages,source); evaluate("closeUtility()"); assert.equal(evaluate("JSON.stringify(state.draft)"),draft); assert.equal(requests.length,count);
}
// main exercises actual script/API pairing, exact edits, revision conflicts and idempotent control recovery.
async function main() {
  await evaluate("bootPromise"); assert.equal(ids.workspace.hidden,false); assert.equal(evaluate("state.token.length"),64); assert.equal(ids.token,undefined); assert.equal(ids.start.disabled,true); assert.equal(ids["nav-workload"].hidden,true);
  assert.equal(ids["nav-connection"],undefined); assert.equal(ids["server-executable"],undefined); assert.equal(ids["view-connection-session"],undefined);
  ids["server-ip"].value="127.0.0.1"; ids["ssh-port"].value=process.env.DASHBOARD_TEST_SSH_PORT; ids["ssh-user"].value="sneak"; ids["ssh-password"].value="fixture-password";
  await click("connect"); assert.equal(evaluate("state.trust"),null); assert.equal(ids["ssh-trust"].hidden,true); assert.equal(ids.notice.dataset.error,"true"); assert.equal(ids["ssh-password"].value,""); assert.equal(ids.connect.disabled,true); assert.equal(ids["connection-status"].dataset.state,"failed"); await click("refresh-connection"); assert.equal(evaluate("state.connection.status"),"connected"); assert.equal(ids.start.disabled,false); assert.equal(ids["create-key"].disabled,true);
  assert.equal(ids.notice.dataset.error,"false",ids.notice.textContent);
  assert.equal(evaluate("state.connection.helper_sha256.length"),64); assert.equal(evaluate("state.connection.server_executable"),undefined); assert.equal(ids["connection-status"].dataset.state,"connected");
  const connectRequests=requests.filter(function connectionRequest(request) { /* First use needs exactly one request and no manual fingerprint/path fields. */ return request.path==="/api/v1/connection" && request.body; });
  assert.equal(connectRequests.length,1); assert.equal(connectRequests[0].body.trust_fingerprint,undefined); assert.equal(connectRequests[0].body.server_executable,undefined);
  await navigationCheck();
  ids["client-id"].value = "dashboard-client"; await click("public-key"); assert.ok(ids.enrollment.textContent.includes("public_key")); assert.ok(!ids.enrollment.textContent.includes("key_ref")); assert.equal(ids["create-key"].disabled, true);
  ids["pairing-id"].value = "dashboard-peer"; ids.bundle.value = process.env.DASHBOARD_TEST_BUNDLE; await click("import-pairing"); assert.ok(ids["pairing-preview"].textContent.includes("host_key_fingerprint"));
  await click("import-pairing"); assert.equal(ids.notice.dataset.error, "true"); ids.replace.checked = true; await click("import-pairing"); assert.equal(ids.notice.dataset.error, "false"); assert.equal(ids.replace.checked, false);
  await click("use-pairing"); assert.equal(evaluate("state.draft.pairing_ref"), "dashboard-peer");
  evaluate("state.draft.pairing_ref=state.connection.pairing_ref");
  const originalProfile = evaluate("JSON.stringify(state.draft)");
  ids["profile-json"].value = originalProfile.replace('"schema_version":1', '"schema_version":1,"schema_version":1'); await click("apply-json"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("JSON.stringify(state.draft)"), originalProfile);
  ids["profile-json"].value = originalProfile; await click("apply-json"); assert.equal(ids.notice.dataset.error, "false");
  const tcp = JSON.parse(originalProfile); tcp.adapter_id = "tcp"; tcp.adapter_options = { tcp_no_delay: true }; ids["profile-json"].value = JSON.stringify(tcp); await click("apply-json"); assert.ok(ids.capabilities.textContent.includes("TCP")); assert.ok(descendants(ids["profile-fields"]).some(function optionLabel(node) { /* Verify registered option descriptions reach the common form without protocol-name UI switches. */ return node.textContent.includes("tcp_no_delay"); }));
  ids["profile-json"].value = originalProfile; await click("apply-json");
  const extension = JSON.parse(originalProfile); extension.adapter_id = "test-extension"; extension.adapter_options = { enabled: true }; ids["profile-json"].value = JSON.stringify(extension); await click("apply-json");
  assert.ok(ids.capabilities.textContent.includes("maximum authenticated frame 1024 bytes")); assert.ok(ids.capabilities.textContent.includes("TEST-EXTENSION"));
  assert.ok(descendants(ids["profile-fields"]).some(function extensionOption(node) { /* The common form must render the test-only descriptor option. */ return node.textContent.includes("enabled"); }));
  await click("validate"); assert.equal(ids.notice.dataset.error, "false");
  evaluate('state.draft.adapter_options.enabled = false'); await click("validate"); assert.equal(ids.notice.dataset.error, "true"); evaluate('state.draft.adapter_options.enabled = true');
  evaluate('state.draft.grace_ns = "9007199254740993"'); await click("show-json"); assert.ok(ids["profile-json"].value.includes('"9007199254740993"')); evaluate('state.draft.grace_ns = state.selected.profile.grace_ns');
  await click("add-phase"); assert.equal(evaluate("state.draft.phases.length"), 2);
  const second = ids.phases.children[1]; const move = second.children.at(-1).children[0]; const original = evaluate("state.draft.phases[1].phase_id"); move.listeners.click[0](); assert.equal(evaluate("state.draft.phases[0].phase_id"), original);
  ids.phases.children[1].children.at(-1).children[2].listeners.click[0](); assert.equal(evaluate("state.draft.phases.length"), 1);
  const dscp = evaluate("navigation.fields").flatMap(function fields(entry) { /* Include preserved fields on later pages. */ return entry.fields; }).find(function priority(node) { /* Stable schema binding despite friendly labels. */ return node.dataset.field==="dscp"; }).firstElementChild;
  dscp.value = "bad"; dscp.listeners.input[0](); await click("validate"); assert.equal(ids.notice.dataset.error, "true"); dscp.value = "0"; dscp.listeners.input[0]();
  await click("validate"); assert.equal(ids.notice.dataset.error, "false"); assert.equal(evaluate("dirty()"),true); assert.equal(ids.start.disabled,false);
  const savesBefore=requests.filter(function saves(r) { /* Record saves so Run Test must persist this unsaved edited workload itself. */ return r.path.startsWith("/api/v1/profiles/") && r.body?.profile_revision!==undefined && r.path!=="/api/v1/profiles/validate"; }).length;
  await click("start"); assert.equal(ids.retry.hidden, false); assert.equal(ids.start.disabled, true); const pendingID = evaluate("state.pending.id");
  assert.equal(evaluate("dirty()"),false); assert.equal(requests.filter(function saves(r) { /* Verify the save belongs to this explicit Run Test action. */ return r.path.startsWith("/api/v1/profiles/") && r.body?.profile_revision!==undefined && r.path!=="/api/v1/profiles/validate"; }).length,savesBefore+1);
  assert.equal(ids.disconnect.disabled,true); evaluate("disconnect()"); assert.equal(evaluate("state.token"), ""); assert.equal(evaluate("state.session"), ""); assert.equal(timers.size, 0);

  await evaluate("perform(bootstrapSession)"); await click("retry"); assert.equal(ids.retry.hidden, true); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  await click("status"); assert.ok(ids["live-status"].textContent.includes("Provisional")); assert.equal(ids["live-progress"].value,500);
  assert.ok(descendants(ids["live-counters"]).some(function exactLive(node) { /* Provisional counters retain >2^53 integer text. */ return node.textContent==="9007199254740993"; }));
  const starts = requests.filter(function startRequest(r) { /* Compare original and retried start correlation exactly. */ return r.path === "/api/v1/runs"; }); assert.equal(starts.length, 2); assert.equal(starts[0].body.request_id, pendingID); assert.deepEqual(starts[0].body, starts[1].body);
  const frozen = ids.frozen.textContent; evaluate('state.draft.phases[0].client.content_bytes += 1; controls()'); assert.equal(ids.start.disabled, true); await click("save"); assert.equal(ids.frozen.textContent, frozen);
  evaluate('state.selected.profile_revision = "stale"; state.draft.phases[0].client.content_bytes += 1; controls()'); await click("save"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("dirty()"), true); assert.equal(ids.frozen.textContent, frozen);
  await click("cancel"); assert.ok(["cancelling", "partial"].includes(evaluate("state.run.status"))); await click("status"); assert.equal(evaluate("state.run.status"), "partial"); assert.equal(evaluate("state.run.local_complete"), null);
  ids["run-id"].value = "unknown"; await click("status"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  changedFingerprint=evaluate("state.connection.host_key_fingerprint");
  await click("disconnect"); assert.equal(evaluate("state.connection.status"),"disconnected"); assert.equal(evaluate("state.token.length"),64); assert.equal(ids.start.disabled,true);
  ids["ssh-password"].value="fixture-password"; await click("connect"); assert.equal(evaluate("state.trust.status"),"trust_required"); assert.equal(ids["ssh-trust"].hidden,false); assert.equal(ids["trust-connect"].disabled,true); assert.ok(ids["ssh-fingerprint"].textContent.includes("Server identity changed"));
  ids["replace-ssh-host"].checked=true; ids["replace-ssh-host"].listeners.change[0](); await click("trust-connect"); assert.equal(evaluate("state.connection.status"),"connected"); assert.equal(ids["ssh-password"].value,""); assert.equal(ids["ssh-trust"].hidden,true);
  const replacement=requests.filter(function replaced(request) { /* The actual script must send the displayed key and an explicit replacement flag. */ return request.path==="/api/v1/connection" && request.body; }).at(-1).body;
  assert.equal(replacement.replace_host_key,true); assert.ok(replacement.trust_fingerprint.startsWith("SHA256:"));
  await click("disconnect"); evaluate("disconnect()"); assert.equal(ids.workspace.hidden, true);
  assert.equal(evaluate("navigation.readers.size"),0); assert.equal(evaluate('navigate("results","metrics")'),false);
  assert.equal(evaluate("diagnosticsState.view"), null); assert.equal(evaluate("diagnosticsState.preflight"), null);
  assert.ok(!fs.readFileSync(__dirname + "/dashboard.js", "utf8").match(/localStorage|sessionStorage|innerHTML|document\.cookie/));
}
// guidedSettingsCheck checks friendly units/rates, invalid editing and custom-plan retention against the actual script model.
async function guidedSettingsCheck() {
  const original=evaluate("JSON.stringify(state.draft)");
  assert.equal(evaluate('unitValue(".125",9)'),"125000000"); assert.equal(evaluate('unitText("9007199254740993",6)'),"9007199254.740993");
  assert.equal(evaluate('JSON.stringify(rateParts("59.94"))'),'["2997","50"]'); assert.equal(evaluate('rateText({mode:"steady",rate_numerator:"1",rate_denominator:"3"})'),"1/3");
  ids["test-duration"].value="1.5"; ids["test-duration"].listeners.input[0](); assert.equal(evaluate("state.draft.phases[0].duration_ns"),"1500000000");
  ids["test-up-rate"].value="59.94"; ids["test-up-rate"].listeners.input[0](); assert.equal(evaluate("state.draft.phases[0].client.rate_numerator"),"2997"); assert.equal(evaluate("state.draft.phases[0].client.rate_denominator"),"50");
  assert.ok(evaluate("state.draft.limits.ledger_messages")>=2000); const beforeBad=evaluate("JSON.stringify(state.draft)");
  ids["test-duration"].value="bad"; ids["test-duration"].listeners.input[0](); assert.equal(ids.start.disabled,true); assert.equal(evaluate("JSON.stringify(state.draft)"),beforeBad);
  evaluate('navigate("dashboard","overview")'); evaluate('navigate("dashboard","overview")'); assert.equal(ids["test-duration"].value,"bad"); assert.equal(evaluate("state.invalid.size"),1);
  ids["test-duration"].value="1.5"; ids["test-duration"].listeners.input[0]();
  evaluate("state.draft="+original+"; renderEditor(); controls()");
  const custom=evaluate("JSON.stringify(state.draft)"); evaluate('state.draft.phases.push(clone(state.draft.phases[0])); state.draft.phases[1].phase_id="custom-second"; renderEditor(); controls()');
  assert.equal(ids["test-duration"].disabled,false); assert.equal(ids["test-up-rate"].disabled,false); assert.equal(ids["test-up-dscp"].disabled,false); assert.equal(ids["custom-plan"].hidden,false);
  ids["friendly-phase"].value="1"; ids["friendly-phase"].listeners.change[0](); ids["test-duration"].value=".25"; ids["test-duration"].listeners.input[0](); assert.equal(evaluate("state.draft.phases[1].duration_ns"),"250000000"); assert.equal(evaluate("state.draft.phases[0].duration_ns"),JSON.parse(custom).phases[0].duration_ns);
  evaluate("workspace.phase=0");
  evaluate("state.draft="+custom+"; renderEditor(); workspace.editorDirty=false; controls()");
}
// inlineSettingsCheck exercises choice-dependent main-page controls, retained bad edits and exact independent phases.
async function inlineSettingsCheck() {
  assert.ok(!/<aside\b|class="sidebar"|class="section-tabs"|view-results-|view-compare-|nav-results|nav-compare|\/compare.js/.test(html),"extra dashboard section remains");
  const original=evaluate("JSON.stringify(state.draft)"),count=requests.length;
  await click("test-advanced"); assert.equal(ids["test-settings"].hidden,false); assert.equal(evaluate("navigation.section"),"dashboard");
  ids["test-protocol"].value="tcp"; ids["test-protocol"].listeners.change[0](); assert.equal(ids["test-no-delay-row"].hidden,false);
  ids["test-no-delay"].checked=false; ids["test-no-delay"].listeners.change[0](); assert.equal(evaluate("state.draft.adapter_options.tcp_no_delay"),false);
  ids["test-protocol"].value="udp"; ids["test-protocol"].listeners.change[0](); assert.equal(ids["test-no-delay-row"].hidden,true);
  ids["test-up-mode"].value="burst"; ids["test-up-mode"].listeners.change[0](); assert.equal(ids["test-up-burst"].hidden,false); assert.equal(ids["test-down-burst"].hidden,true);
  for(const [id,value] of [["test-up-active","5.5"],["test-up-gap","2"],["test-up-rate","59.94"]]) { ids[id].value=value; ids[id].listeners.input[0](); }
  assert.equal(evaluate("state.draft.phases[0].client.mode"),"burst"); assert.equal(evaluate("state.draft.phases[0].client.active_ns"),"5500000"); assert.equal(evaluate("state.draft.phases[0].client.gap_ns"),"2000000");
  ids["test-down-mode"].value="quiet"; ids["test-down-mode"].listeners.change[0](); assert.equal(ids["test-down-rate"].disabled,true); assert.equal(evaluate("state.draft.phases[0].client.mode"),"burst");
  ids["test-up-rate"].value="bad"; ids["test-up-rate"].listeners.input[0]();
  ids["settings-group"].value="network"; ids["settings-group"].listeners.change[0](); assert.equal(ids["settings-network"].hidden,false); assert.equal(ids["view-dashboard-traffic"].hidden,true); assert.equal(ids["test-up-rate"].value,"bad"); assert.equal(ids.start.disabled,true);
  ids["test-up-rate"].value="59.94"; ids["test-up-rate"].listeners.input[0]();
  ids["test-up-dscp"].value="46"; ids["test-up-dscp"].listeners.change[0](); assert.equal(evaluate("state.draft.phases[0].client.dscp"),46);
  ids["source-port-choice"].value="custom"; ids["source-port-choice"].listeners.change[0](); assert.equal(ids["source-port-field"].hidden,false); assert.equal(ids.start.disabled,true);
  ids["test-source-port"].value="54321"; ids["test-source-port"].listeners.input[0](); assert.equal(evaluate("state.draft.source_port"),"54321");
  ids["source-port-choice"].value="auto"; ids["source-port-choice"].listeners.change[0](); assert.equal(evaluate("state.draft.source_port"),"auto");
  await click("friendly-phase-add"); assert.equal(evaluate("state.draft.phases.length"),2); assert.equal(evaluate("workspace.phase"),1);
  ids["test-payload"].value="55"; ids["test-payload"].listeners.input[0](); const firstBytes=evaluate("state.draft.phases[0].client.content_bytes"); assert.equal(evaluate("state.draft.phases[1].client.content_bytes"),55);
  ids["friendly-phase-deadline"].value="12.125"; ids["friendly-phase-deadline"].listeners.input[0](); assert.equal(evaluate("state.draft.phases[1].deadline_ns"),"12125000");
  await click("friendly-phase-up"); assert.equal(evaluate("state.draft.phases[0].client.content_bytes"),55); assert.equal(evaluate("state.draft.phases[1].client.content_bytes"),firstBytes);
  ids["automatic-storage"].checked=false; ids["automatic-storage"].listeners.change[0]();
  const limits=evaluate("navigation.fields").find(function inlineLimits(entry) { /* Locate actual typed fields without depending on pagination. */ return entry.root.id==="friendly-limits"; });
  const queue=limits.fields.find(function queueCapacity(label) { /* Verify exact byte quantities above JavaScript's integer precision. */ return label.dataset.setting==="queue_bytes"; }).firstElementChild;
  queue.value="9007199254740993"; queue.listeners.input[0](); assert.equal(evaluate("state.draft.limits.queue_bytes"),"9007199254740993");
  const clocks=evaluate("navigation.fields").find(function inlineClock(entry) { /* Optional clock bounds remain typed and human-unit based. */ return entry.root.id==="friendly-clock"; });
  const cadence=clocks.fields.find(function sampleCadence(label) { /* Scale only the explicitly changed sampling interval. */ return label.dataset.setting==="cadence_ns"; }).firstElementChild;
  cadence.value="12.125"; cadence.listeners.input[0](); assert.equal(evaluate("state.draft.calibration.cadence_ns"),"12125000");
  assert.equal(requests.length,count,"display/settings edits sent unexpected requests");
  evaluate("state.draft="+original+"; workspace.phase=0; workspace.autoStorage=true; workspace.settings=false; workspace.group='traffic'; renderEditor(); controls()");
  const clock=evaluate("navigation.fields").find(function clockGroup(entry) { /* Run the existing local preflight through its new human-unit main-page controls. */ return entry.root.id==="friendly-clock"; });
  for(const [key,value] of [["preflight_duration_ns","2"],["preflight_interval_ns",".5"]]) { const field=clock.fields.find(function timingField(label) { /* Use actual generated controls rather than setting the model directly. */ return label.dataset.setting===key; }).firstElementChild; field.value=value; field.listeners.input[0](); }
  await click("preflight"); assert.ok(evaluate("diagnosticsState.preflight")); assert.equal(requests.at(-1).path,"/api/v1/preflight"); assert.equal(requests.at(-1).body.duration_ns,"2000000"); assert.equal(requests.at(-1).body.interval_ns,"500000"); assert.equal(ids["timer-view-option"].disabled,false);
  ids["settings-group"].value="timer"; ids["settings-group"].listeners.change[0](); assert.ok(ids["preflight-table"].firstElementChild.children.length<=3);
  evaluate("state.draft="+original+"; workspace.group='traffic'; renderEditor(); controls()");
}
// durationBudgetCheck reproduces the default boundary and verifies sizing/manual recovery without changing rates or cadence.
async function durationBudgetCheck() {
  const original=evaluate("JSON.stringify(state.draft)");
  evaluate('state.draft.warmup_ns="0"; state.draft.grace_ns="100000000"; state.draft.phases=state.draft.phases.slice(0,1); for(const direction of [state.draft.phases[0].client,state.draft.phases[0].server]) { direction.mode="steady"; direction.rate_numerator="60"; direction.rate_denominator="1"; direction.active_ns="0"; direction.gap_ns="0"; } state.draft.limits.heartbeat_interval_ns="1000000000"; state.draft.calibration.cadence_ns="1000000000"; state.draft.limits.control_requests=128; workspace.phase=0; renderEditor(); controls()');
  ids["test-duration"].value="60"; ids["test-duration"].listeners.input[0]();
  assert.equal(evaluate("state.draft.phases[0].duration_ns"),"60000000000"); assert.ok(evaluate("state.draft.limits.control_requests")>129); assert.equal(evaluate("state.draft.calibration.cadence_ns"),"1000000000"); assert.equal(evaluate("state.draft.phases[0].client.rate_numerator"),"60");
  const sized=evaluate("state.draft.limits.control_requests"),bytes=evaluate("state.draft.limits.file_bytes"); assert.ok(BigInt(sized)>=129n+(BigInt(bytes)+32767n)/32768n+3n);
  await evaluate('api("/api/v1/profiles/validate","POST",{profile_revision:"",profile:readyDraft()})');
  evaluate('workspace.autoStorage=false; state.draft.limits.control_requests=128; state.draft.phases[0].duration_ns="1000000000"; renderEditor(); controls()');
  ids["test-duration"].value="60"; ids["test-duration"].listeners.input[0](); assert.equal(ids.start.disabled,true); assert.ok(ids["test-duration"].title.includes("129 control requests")); assert.equal(evaluate("state.draft.phases[0].duration_ns"),"1000000000");
  ids["automatic-storage"].checked=true; ids["automatic-storage"].listeners.change[0](); assert.equal(evaluate("state.invalid.size"),0); assert.equal(evaluate("state.draft.phases[0].duration_ns"),"60000000000");
  evaluate('state.run={status:"failed",profile_id:"budget-test",reason:"finite run exceeds control/calibration request budget"}; renderWorkspace()'); assert.equal(ids["test-error"].hidden,false); assert.ok(ids["test-error"].textContent.includes("request budget"));
  evaluate("state.run=null; state.draft="+original+"; workspace.autoStorage=true; renderEditor(); controls()");
}
// guidedMain drives normal controls against a real loopback measurement runner without any manual machine identities.
async function guidedMain() {
  await evaluate("bootPromise"); assert.equal(evaluate("navigation.section"),"dashboard"); assert.equal(ids["nav-workload"].hidden,true);
  ids["server-ip"].value="127.0.0.1"; ids["ssh-port"].value=process.env.DASHBOARD_TEST_SSH_PORT; ids["ssh-user"].value="sneak"; ids["ssh-password"].value="fixture-password";
  await click("connect"); await click("refresh-connection"); assert.equal(evaluate("state.connection.status"),"connected");
  await guidedSettingsCheck();
  await inlineSettingsCheck();
  await durationBudgetCheck();
  const long=process.env.DASHBOARD_TEST_SECONDS==="60";
  const results=[];
  for(const protocol of ["udp","tcp"]) {
    if(results.length) await click("reset-test");
    ids["test-protocol"].value=protocol; ids["test-protocol"].listeners.change[0]();
    for(const [id,value] of [["test-name","Usable "+protocol+" baseline"],["test-duration",".12"],["test-warmup",".01"],["test-grace","100"],["test-deadline","20"],["test-up-rate","59.94"],["test-down-rate","30"],["test-payload","41"],["test-down-payload","53"],["test-port",protocol==="udp" ? process.env.DASHBOARD_UDP_PORT : process.env.DASHBOARD_TCP_PORT]]) { ids[id].value=value; ids[id].listeners.input[0](); }
    if(long) { for(const [id,value] of [["test-duration","60"],["test-warmup","0"],["test-deadline","100"],["test-up-rate","60"],["test-down-rate","60"]]) { ids[id].value=value; ids[id].listeners.input[0](); } }
    else {
    ids["test-up-mode"].value="burst"; ids["test-up-mode"].listeners.change[0]();
    for(const [id,value] of [["test-up-active","20"],["test-up-gap","10"],["test-duration",".06"]]) { ids[id].value=value; ids[id].listeners.input[0](); }
    await click("friendly-phase-add"); ids["friendly-phase-deadline"].value="12.125"; ids["friendly-phase-deadline"].listeners.input[0]();
    }
    if(protocol==="tcp") { ids["test-no-delay"].checked=true; ids["test-no-delay"].listeners.change[0](); }
    await click("start"); if(evaluate("!!state.pending")) await click("test-retry");
    const end=Date.now()+(long ? 85000 : 10000);
    while(!evaluate('state.run && !["executing","cancelling"].includes(state.run.status)')) { if(Date.now()>end) throw Error("real guided run timeout"); await click("test-refresh"); await new Promise(function statusYield(resolve) { /* Yield to real finite cleanup; long trials are polled once per second. */ setTimeout(resolve,long ? 1000 : 20); }); }
    await evaluate("workspace.loading");
    if(!evaluate("!!workspace.result")) { await click("test-refresh"); await evaluate("workspace.loading"); }
    assert.equal(evaluate("state.run.status"),"completed",ids["test-error"].textContent); assert.ok(evaluate("workspace.result"),ids.notice.textContent); assert.equal(evaluate("workspace.result.complete"),true); assert.ok(ids["card-tail"].textContent.endsWith(" ms")); assert.ok(ids["card-tail-note"].textContent.includes("samples"));
    assert.equal(evaluate("workspace.resultRun"),evaluate("state.run.control_run_id")); assert.equal(evaluate("state.draft.phases[0].client.rate_numerator"),long ? "60" : "2997");
    if(long) { assert.equal(evaluate("state.selected.profile.phases[0].duration_ns"),"60000000000"); assert.ok(BigInt(evaluate("state.run.progress.elapsed_ns"))>=60000000000n); }
    else { assert.equal(evaluate("state.selected.profile.phases.length"),2); assert.equal(evaluate("state.selected.profile.phases[1].deadline_ns"),"12125000"); assert.equal(evaluate("state.selected.profile.phases[0].client.mode"),"burst"); }
    results.push(evaluate("state.run.report_id"));
  }
  await click("reset-test"); assert.equal(evaluate("state.connection.status"),"connected"); assert.ok(!ids["test-name"].value.match(/[a-f0-9]{8}/)); assert.equal(evaluate("workspace.result"),null);
  await click("disconnect");
}
(process.env.DASHBOARD_GUIDED_REAL ? guidedMain() : main()).catch(function failure(error) { /* Surface fixture failures without printing test credentials or request headers. */ process.stderr.write(String(error.stack)); process.exitCode = 1; });
