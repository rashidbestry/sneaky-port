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
  append(...nodes) { this.children.push(...nodes); if (this.tagName === "select" && !this.value && nodes.length) this.value = nodes[0].value; }
  // replaceChildren replaces only owned form nodes when the UI explicitly rebuilds an editor.
  replaceChildren(...nodes) { this.children = []; if (this.tagName === "select") this.value = ""; this.append(...nodes); }
  // addEventListener records actions for explicit fixture activation without synthetic scheduling.
  addEventListener(name, action) { (this.listeners[name] ||= []).push(action); }
  // firstElementChild exposes the owned label's input for override-field event wiring.
  get firstElementChild() { return this.children[0]; }
  // lastChild exposes the most recently appended label for descriptor/phase event wiring.
  get lastChild() { return this.children.at(-1); }
}
const html = fs.readFileSync(__dirname + "/index.html", "utf8");
const ids = Object.fromEntries(Array.from(html.matchAll(/id="([^"]+)"/g), function field(match) { /* Build fixed shell nodes from actual HTML IDs, detecting missing script bindings. */ const tag = new RegExp("<(\\w+)[^>]*id=\"" + match[1] + "\"").exec(html)[1]; return [match[1], new Node(tag)]; }));
const timers = new Map(); let timerID = 0; let dropStartReply = true; let currentRun = "";
const requests = [];
// descendants recursively collects owned fields so disabling reflects the complete dynamic editor.
function descendants(node) { return [node, ...node.children.flatMap(descendants)]; }
const context = vm.createContext({ document: {
  // getElementById resolves only shell IDs actually present in the embedded HTML.
  getElementById(id) { assert.ok(ids[id], "missing HTML binding: " + id); return ids[id]; },
  // createElement models safe text/field construction; no innerHTML interface is supplied.
  createElement(tag) { return new Node(tag); },
  // querySelectorAll returns all owned workspace controls for operation/dirty-state gating.
  querySelectorAll() { return Object.values(ids).flatMap(descendants); }
}, crypto: webcrypto, TextEncoder, AbortSignal,
  // setTimeout retains declared cadence without background polling races in the scripted fixture.
  setTimeout(action, delay) { assert.equal(delay, 1000); timers.set(++timerID, action); return timerID; },
  // clearTimeout cancels the previous provisional read, never a measurement command.
  clearTimeout(id) { timers.delete(id); },
  // fetch uses the real owned API and simulates losing only a successful start acknowledgment.
  async fetch(path, options) {
    assert.ok(path.startsWith("/api/v1/")); assert.equal(options.credentials, "omit"); assert.equal(options.redirect, "error");
    const body = options.body && JSON.parse(options.body); requests.push({ path, body });
    const response = await fetch(process.env.DASHBOARD_TEST_URL + path, { ...options, headers: { ...options.headers, Origin: process.env.DASHBOARD_TEST_URL } });
    if (path === "/api/v1/runs" && response.ok && dropStartReply) { dropStartReply = false; const data = await response.json(); currentRun = data.result.control_run_id; throw new Error("fixture lost acknowledgment"); }
    return response;
  }
});
vm.runInContext(fs.readFileSync(__dirname + "/dashboard.js", "utf8"), context);
// evaluate inspects the actual script's lexical model instead of mirroring its implementation.
function evaluate(code) { return vm.runInContext(code, context); }
// click executes the bound UI handler and waits for its serialized asynchronous operation to finish.
async function click(id) {
  assert.equal(ids[id].disabled, false, "disabled control: " + id);
  for (const action of ids[id].listeners.click || []) action();
  const end = Date.now() + 6000;
  while (evaluate("state.busy")) { if (Date.now() > end) throw new Error("UI action timeout: " + id); await new Promise(function pause(resolve) { /* Yield until the finite HTTP action completes. */ setTimeout(resolve, 10); }); }
}
// main exercises actual script/API pairing, exact edits, revision conflicts and idempotent control recovery.
async function main() {
  ids.token.value = process.env.DASHBOARD_TEST_TOKEN; await click("connect"); assert.equal(ids.workspace.hidden, false); assert.equal(ids.token.value, ""); assert.equal(ids.start.disabled, true);
  ids["client-id"].value = "dashboard-client"; await click("create-key"); assert.ok(ids.enrollment.textContent.includes("public_key")); assert.ok(!ids.enrollment.textContent.includes("key_ref")); assert.equal(ids["create-key"].disabled, true);
  ids["pairing-id"].value = "dashboard-peer"; ids.bundle.value = process.env.DASHBOARD_TEST_BUNDLE; await click("import-pairing"); assert.ok(ids["pairing-preview"].textContent.includes("host_key_fingerprint"));
  await click("import-pairing"); assert.equal(ids.notice.dataset.error, "true"); ids.replace.checked = true; await click("import-pairing"); assert.equal(ids.notice.dataset.error, "false"); assert.equal(ids.replace.checked, false);
  await click("use-pairing"); assert.equal(evaluate("state.draft.pairing_ref"), "dashboard-peer");
  const originalProfile = evaluate("JSON.stringify(state.draft)");
  ids["profile-json"].value = originalProfile.replace('"schema_version":1', '"schema_version":1,"schema_version":1'); await click("apply-json"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("JSON.stringify(state.draft)"), originalProfile);
  ids["profile-json"].value = originalProfile; await click("apply-json"); assert.equal(ids.notice.dataset.error, "false");
  const tcp = JSON.parse(originalProfile); tcp.adapter_id = "tcp"; tcp.adapter_options = { tcp_no_delay: true }; ids["profile-json"].value = JSON.stringify(tcp); await click("apply-json"); assert.ok(ids.capabilities.textContent.includes("tcp")); assert.ok(descendants(ids["profile-fields"]).some(function optionLabel(node) { /* Verify registered option descriptions reach the common form without protocol-name UI switches. */ return node.textContent.includes("tcp_no_delay"); }));
  ids["profile-json"].value = originalProfile; await click("apply-json");
  evaluate('state.draft.grace_ns = "9007199254740993"'); await click("show-json"); assert.ok(ids["profile-json"].value.includes('"9007199254740993"')); evaluate('state.draft.grace_ns = state.selected.profile.grace_ns');
  await click("add-phase"); assert.equal(evaluate("state.draft.phases.length"), 2);
  const second = ids.phases.children[1]; const move = second.children.at(-1).children[0]; const original = evaluate("state.draft.phases[1].phase_id"); move.listeners.click[0](); assert.equal(evaluate("state.draft.phases[0].phase_id"), original);
  ids.phases.children[1].children.at(-1).children[2].listeners.click[0](); assert.equal(evaluate("state.draft.phases.length"), 1);
  const dscp = descendants(ids.phases).find(function dscpLabel(node) { /* Activate an actual numeric form control with malformed input. */ return node.tagName === "label" && node.textContent === "dscp"; }).firstElementChild;
  dscp.value = "bad"; dscp.listeners.input[0](); await click("validate"); assert.equal(ids.notice.dataset.error, "true"); dscp.value = "0"; dscp.listeners.input[0]();
  await click("validate"); assert.equal(ids.notice.dataset.error, "false"); await click("save"); assert.equal(ids.start.disabled, false);
  await click("start"); assert.equal(ids.retry.hidden, false); assert.equal(ids.start.disabled, true); const pendingID = evaluate("state.pending.id");
  await click("disconnect"); assert.equal(evaluate("state.token"), ""); assert.equal(evaluate("state.session"), ""); assert.equal(timers.size, 0);
  ids.token.value = process.env.DASHBOARD_TEST_TOKEN; await click("connect"); await click("retry"); assert.equal(ids.retry.hidden, true); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  const starts = requests.filter(function startRequest(r) { /* Compare original and retried start correlation exactly. */ return r.path === "/api/v1/runs"; }); assert.equal(starts.length, 2); assert.equal(starts[0].body.request_id, pendingID); assert.deepEqual(starts[0].body, starts[1].body);
  const frozen = ids.frozen.textContent; evaluate('state.draft.phases[0].client.content_bytes += 1; controls()'); assert.equal(ids.start.disabled, true); await click("save"); assert.equal(ids.frozen.textContent, frozen);
  evaluate('state.selected.profile_revision = "stale"; state.draft.phases[0].client.content_bytes += 1; controls()'); await click("save"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("dirty()"), true); assert.equal(ids.frozen.textContent, frozen);
  await click("cancel"); assert.ok(["cancelling", "partial"].includes(evaluate("state.run.status"))); await click("status"); assert.equal(evaluate("state.run.status"), "partial"); assert.equal(evaluate("state.run.local_complete"), null);
  ids["run-id"].value = "unknown"; await click("status"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  await click("disconnect"); assert.equal(ids.workspace.hidden, true);
  assert.ok(!fs.readFileSync(__dirname + "/dashboard.js", "utf8").match(/localStorage|sessionStorage|innerHTML|document\.cookie/));
}
main().catch(function failure(error) { /* Surface fixture failures without printing test credentials or request headers. */ process.stderr.write(String(error.stack)); process.exitCode = 1; });
