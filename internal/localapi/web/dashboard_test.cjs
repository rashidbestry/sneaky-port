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
  // setAttribute retains native SVG attributes for bounded rendering assertions.
  setAttribute(name, value) { (this.attributes ||= {})[name] = value; }
  // click records only the explicit temporary export link; no browser download/layout is claimed.
  click() { downloads.push({ name:this.download, href:this.href }); }
  // firstElementChild exposes the owned label's input for override-field event wiring.
  get firstElementChild() { return this.children[0]; }
  // lastChild exposes the most recently appended label for descriptor/phase event wiring.
  get lastChild() { return this.children.at(-1); }
}
const html = fs.readFileSync(__dirname + "/index.html", "utf8");
const ids = Object.fromEntries(Array.from(html.matchAll(/id="([^"]+)"/g), function field(match) { /* Build fixed shell nodes from actual HTML IDs, detecting missing script bindings. */ const tag = new RegExp("<(\\w+)[^>]*id=\"" + match[1] + "\"").exec(html)[1]; return [match[1], new Node(tag)]; }));
const timers = new Map(); let timerID = 0; let dropStartReply = true; let currentRun = "";
const requests = [];
const downloads = [];
// descendants recursively collects owned fields so disabling reflects the complete dynamic editor.
function descendants(node) { return [node, ...node.children.flatMap(descendants)]; }
const context = vm.createContext({ document: {
  // getElementById resolves only shell IDs actually present in the embedded HTML.
  getElementById(id) { assert.ok(ids[id], "missing HTML binding: " + id); return ids[id]; },
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
    const response = await fetch(process.env.DASHBOARD_TEST_URL + path, { ...options, headers: { ...options.headers, Origin: process.env.DASHBOARD_TEST_URL } });
    if (path === "/api/v1/runs" && response.ok && dropStartReply) { dropStartReply = false; const data = await response.json(); currentRun = data.result.control_run_id; throw new Error("fixture lost acknowledgment"); }
    return response;
  }
});
vm.runInContext(fs.readFileSync(__dirname + "/dashboard.js", "utf8"), context);
vm.runInContext(fs.readFileSync(__dirname + "/diagnostics.js", "utf8"), context);
vm.runInContext(fs.readFileSync(__dirname + "/compare.js", "utf8"), context);
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
  await click("refresh-reports"); ids.reports.value = "diagnostic"; ids["report-endpoint"].value = "local"; ids["report-scope"].value = "main";
  await click("read-report"); assert.equal(ids.notice.dataset.error, "false"); assert.ok(ids["diagnostic-status"].textContent.includes("complete=false"));
  assert.ok(ids["evidence-policy"].textContent.includes('"p99_target":"1000"'));
  assert.ok(descendants(ids["diagnostic-table"]).some(function evidenceCell(node) { /* Independent missing receiver evidence remains explicitly labeled. */ return node.textContent.includes("small sample"); }));
  const frozenWorkload=evaluate("JSON.stringify(state.draft)");
  ids["target-p95"].value="0"; const beforeInvalid=requests.length; await click("read-report"); assert.equal(ids.notice.dataset.error,"true"); assert.equal(requests.length,beforeInvalid);
  ids["target-median"].value="1"; ids["target-p95"].value="1"; ids["target-p99"].value="1";
  await click("read-report"); assert.ok(ids["evidence-policy"].textContent.includes('"p99_target":"1"')); assert.equal(evaluate("JSON.stringify(state.draft)"),frozenWorkload);
  for (const format of ["json","jsonl","csv"]) { ids["report-format"].value=format; await click("export-report"); assert.equal(ids.notice.dataset.error,"false"); assert.equal(downloads.at(-1).name,"diagnostic."+format); }
  ids["target-median"].value="20"; ids["target-p95"].value="200"; ids["target-p99"].value="1000";
  assert.equal(ids["time-plot"].children[0].tagName, "svg"); assert.ok(descendants(ids["diagnostic-table"]).some(function unavailableCell(node) { /* Null evidence must stay visibly unavailable. */ return node.textContent === "unavailable"; }));
  const emptyView = evaluate("JSON.stringify(diagnosticsState.view)");
  evaluate('diagnosticsState.view.cutoff_ns = "9007199254741993"; diagnosticsState.view.summary.RTTSamples = "2"; diagnosticsState.view.summary.RTTP99 = "9007199254740993"; diagnosticsState.view.time = [{start_ns:"1",end_ns:"2",slots:"3",missing:"1",local:"0",rtt:{count:"2",min_ns:"0",max_ns:"9007199254740993"},scheduled:{count:"2",min_ns:"1",max_ns:"9007199254740994"}}]; diagnosticsState.view.rtt_ecdf = [{count:"2",min_ns:"0",max_ns:"9007199254740993",first_rank:"1",last_rank:"2"}]; diagnosticsState.view.scheduled_ecdf = [{count:"2",min_ns:"1",max_ns:"9007199254740994",first_rank:"1",last_rank:"2"}]; renderDiagnostic()');
  assert.ok(descendants(ids["time-plot"]).some(function missingMarker(node) { /* Missing population retains a distinct SVG marker. */ return node.attributes?.class === "missing-marker"; }));
  assert.ok(descendants(ids["diagnostic-table"]).some(function exactTail(node) { /* Final tails must retain decimal values beyond Number precision. */ return node.textContent === "9007199254740993"; }));
  assert.ok(descendants(ids["ecdf-plot"]).some(function rankTitle(node) { /* ECDF tooltips retain exact tied-sample ranks and extrema. */ return node.tagName === "title" && node.textContent.includes('"last_rank":"2"'); }));
  assert.equal(evaluate('ratio("9007199254740993", 9007199254740993n)'), 1);
  evaluate("diagnosticsState.view = " + emptyView + "; renderDiagnostic()");
  assert.equal(ids["metrics-prev"].disabled, true); await click("metrics-next"); assert.ok(ids["metric-page"].textContent.includes("Page 2")); await click("metrics-prev");
  ids["report-endpoint"].value = "server"; await click("read-report"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("diagnosticsState.view.endpoint"), "client");
  ids["preflight-duration"].value = "1000000"; ids["preflight-interval"].value = "100000"; await click("preflight"); assert.ok(ids["preflight-status"].textContent.includes("source/resolution unverified")); assert.equal(ids["wakes-prev"].disabled, true);
  ids["preflight-duration"].value = "1000000001"; await click("preflight"); assert.equal(ids.notice.dataset.error, "true");
  ids["comparison-pairs"].value = "diagnostic comparison"; ids["comparison-endpoint"].value = "local"; ids["comparison-scope"].value = "main";
  await click("compare-reports"); assert.equal(ids.notice.dataset.error, "false"); assert.ok(ids["comparison-status"].textContent.includes("inconclusive")); assert.equal(ids["comparison-prev"].disabled, true);
  await click("comparison-next"); assert.ok(ids["comparison-page"].textContent.includes("Page 2")); await click("comparison-prev");
  ids["comparison-detail"].value = "0"; evaluate("renderComparison()"); assert.ok(ids["comparison-provenance"].textContent.includes("Offline consistency is not authentication"));
  const savedComparison = evaluate("JSON.stringify(comparisonState.view)");
  evaluate('comparisonState.view.pairs[0].metrics[9].a = "9007199254740993"; comparisonState.view.pairs[0].metrics[9].b = "9007199254740994"; comparisonState.view.pairs[0].metrics[9].delta = "1"; renderComparison()');
  assert.ok(descendants(ids["comparison-table"]).some(function exactComparison(node) { /* Actual rendering must preserve exact large values. */ return node.textContent === "9007199254740993"; }));
  ids["comparison-pairs"].value = "diagnostic diagnostic"; await click("compare-reports"); assert.equal(ids.notice.dataset.error, "true"); assert.ok(ids["comparison-status"].textContent.includes("diagnostic"));
  evaluate("comparisonState.view = " + savedComparison + "; renderComparison()");
  ids["client-id"].value = "dashboard-client"; await click("create-key"); assert.ok(ids.enrollment.textContent.includes("public_key")); assert.ok(!ids.enrollment.textContent.includes("key_ref")); assert.equal(ids["create-key"].disabled, true);
  ids["pairing-id"].value = "dashboard-peer"; ids.bundle.value = process.env.DASHBOARD_TEST_BUNDLE; await click("import-pairing"); assert.ok(ids["pairing-preview"].textContent.includes("host_key_fingerprint"));
  await click("import-pairing"); assert.equal(ids.notice.dataset.error, "true"); ids.replace.checked = true; await click("import-pairing"); assert.equal(ids.notice.dataset.error, "false"); assert.equal(ids.replace.checked, false);
  await click("use-pairing"); assert.equal(evaluate("state.draft.pairing_ref"), "dashboard-peer");
  const originalProfile = evaluate("JSON.stringify(state.draft)");
  ids["profile-json"].value = originalProfile.replace('"schema_version":1', '"schema_version":1,"schema_version":1'); await click("apply-json"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("JSON.stringify(state.draft)"), originalProfile);
  ids["profile-json"].value = originalProfile; await click("apply-json"); assert.equal(ids.notice.dataset.error, "false");
  const tcp = JSON.parse(originalProfile); tcp.adapter_id = "tcp"; tcp.adapter_options = { tcp_no_delay: true }; ids["profile-json"].value = JSON.stringify(tcp); await click("apply-json"); assert.ok(ids.capabilities.textContent.includes("tcp")); assert.ok(descendants(ids["profile-fields"]).some(function optionLabel(node) { /* Verify registered option descriptions reach the common form without protocol-name UI switches. */ return node.textContent.includes("tcp_no_delay"); }));
  ids["profile-json"].value = originalProfile; await click("apply-json");
  const extension = JSON.parse(originalProfile); extension.adapter_id = "test-extension"; extension.adapter_options = { enabled: true }; ids["profile-json"].value = JSON.stringify(extension); await click("apply-json");
  assert.ok(ids.capabilities.textContent.includes("transport framing 4 bytes")); assert.ok(ids.capabilities.textContent.includes("test-extension"));
  assert.ok(descendants(ids["profile-fields"]).some(function extensionOption(node) { /* The common form must render the test-only descriptor option. */ return node.textContent.includes("enabled"); }));
  await click("validate"); assert.equal(ids.notice.dataset.error, "false");
  evaluate('state.draft.adapter_options.enabled = false'); await click("validate"); assert.equal(ids.notice.dataset.error, "true"); evaluate('state.draft.adapter_options.enabled = true');
  evaluate('state.draft.grace_ns = "9007199254740993"'); await click("show-json"); assert.ok(ids["profile-json"].value.includes('"9007199254740993"')); evaluate('state.draft.grace_ns = state.selected.profile.grace_ns');
  await click("add-phase"); assert.equal(evaluate("state.draft.phases.length"), 2);
  const second = ids.phases.children[1]; const move = second.children.at(-1).children[0]; const original = evaluate("state.draft.phases[1].phase_id"); move.listeners.click[0](); assert.equal(evaluate("state.draft.phases[0].phase_id"), original);
  ids.phases.children[1].children.at(-1).children[2].listeners.click[0](); assert.equal(evaluate("state.draft.phases.length"), 1);
  const dscp = descendants(ids.phases).find(function dscpLabel(node) { /* Activate an actual numeric form control with malformed input. */ return node.tagName === "label" && node.textContent === "dscp"; }).firstElementChild;
  dscp.value = "bad"; dscp.listeners.input[0](); await click("validate"); assert.equal(ids.notice.dataset.error, "true"); dscp.value = "0"; dscp.listeners.input[0]();
  await click("validate"); assert.equal(ids.notice.dataset.error, "false"); await click("save"); assert.equal(ids.start.disabled, false);
  await click("start"); assert.equal(ids.retry.hidden, false); assert.equal(ids.start.disabled, true); const pendingID = evaluate("state.pending.id");
  await click("disconnect"); assert.equal(evaluate("state.token"), ""); assert.equal(evaluate("state.session"), ""); assert.equal(timers.size, 0);
  assert.equal(evaluate("comparisonState.view"), null); assert.equal(ids["comparison-status"].children.length, 0);
  ids.token.value = process.env.DASHBOARD_TEST_TOKEN; await click("connect"); await click("retry"); assert.equal(ids.retry.hidden, true); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  await click("status"); assert.ok(ids["live-status"].textContent.includes("Provisional")); assert.equal(ids["live-progress"].value,500);
  assert.ok(descendants(ids["live-counters"]).some(function exactLive(node) { /* Provisional counters retain >2^53 integer text. */ return node.textContent==="9007199254740993"; }));
  const starts = requests.filter(function startRequest(r) { /* Compare original and retried start correlation exactly. */ return r.path === "/api/v1/runs"; }); assert.equal(starts.length, 2); assert.equal(starts[0].body.request_id, pendingID); assert.deepEqual(starts[0].body, starts[1].body);
  const frozen = ids.frozen.textContent; evaluate('state.draft.phases[0].client.content_bytes += 1; controls()'); assert.equal(ids.start.disabled, true); await click("save"); assert.equal(ids.frozen.textContent, frozen);
  evaluate('state.selected.profile_revision = "stale"; state.draft.phases[0].client.content_bytes += 1; controls()'); await click("save"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("dirty()"), true); assert.equal(ids.frozen.textContent, frozen);
  await click("cancel"); assert.ok(["cancelling", "partial"].includes(evaluate("state.run.status"))); await click("status"); assert.equal(evaluate("state.run.status"), "partial"); assert.equal(evaluate("state.run.local_complete"), null);
  ids["run-id"].value = "unknown"; await click("status"); assert.equal(ids.notice.dataset.error, "true"); assert.equal(evaluate("state.run.control_run_id"), currentRun);
  await click("disconnect"); assert.equal(ids.workspace.hidden, true);
  assert.equal(evaluate("diagnosticsState.view"), null); assert.equal(evaluate("diagnosticsState.preflight"), null);
  assert.ok(!fs.readFileSync(__dirname + "/dashboard.js", "utf8").match(/localStorage|sessionStorage|innerHTML|document\.cookie/));
}
main().catch(function failure(error) { /* Surface fixture failures without printing test credentials or request headers. */ process.stderr.write(String(error.stack)); process.exitCode = 1; });
