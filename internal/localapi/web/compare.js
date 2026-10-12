"use strict";

// comparisonState retains the last successful explicit snapshot; local pages never replay or schedule traffic.
const comparisonState = { view: null, page: 0, rows: [] };
// comparisonRows selects repeated or paired scope/phase rows without recomputing statistics in JavaScript.
function comparisonRows() {
  const view = comparisonState.view; if (!view) return [];
  const chosen = element("comparison-detail").value;
  const rows = [];
  if (chosen === "repeated") {
    const append = function repeatedRows(scope, metrics) { /* Retain trial population beside exact delta ranges. */ for (const m of metrics) rows.push([scope, m.metric, m.unit, m.trials + " pairs", m.min_delta, m.max_delta, m.state, evidenceText(m.evidence)]); };
    append(view.selection.scope, view.repeated); view.phases.forEach(function phaseRows(metrics, i) { /* Main originating phases remain visible independently. */ append("phase " + i, metrics); });
  } else {
    const pair = view.pairs[Number(chosen)]; if (!pair) return [];
    const append = function pairedRows(scope, metrics) { /* Preserve unavailable values, exact B-A and descriptive state. */ for (const m of metrics) rows.push([scope, m.metric, m.unit, m.a, m.b, m.delta, m.state, evidenceText(m.evidence) + " / A: " + evidenceText(m.evidence_a) + " / B: " + evidenceText(m.evidence_b)]); };
    append(view.selection.scope, pair.metrics); pair.phases.forEach(function phaseRows(metrics, i) { /* Aggregate tails cannot conceal a phase failure. */ append("phase " + i, metrics); });
  }
  return rows;
}
// renderComparison displays only twenty rows and one selected pair's public provenance at a time.
function renderComparison() {
  const view = comparisonState.view; if (!view) return;
  element("comparison-status").textContent = "Loaded selection: " + JSON.stringify(view.selection) + " · " + view.pairs.map(function pairStatus(p, i) { /* Do not discard failed/incompatible pairs. */ return "pair " + i + ": " + p.state + (p.reasons.length ? " (" + p.reasons.join("; ") + ")" : ""); }).join(" · ");
  element("comparison-policy").textContent = view.policy + " Evidence policy: " + JSON.stringify(view.evidence_policy);
  const repeated = element("comparison-detail").value === "repeated";
  comparisonState.rows = comparisonRows();
  tablePage("comparison-table", repeated ? ["Scope", "Metric", "Unit", "Available trials", "Minimum B-A", "Maximum B-A", "State", "Evidence / reasons"] : ["Scope", "Metric", "Unit", "A", "B", "B-A", "State", "Evidence / reasons"], comparisonState.rows, comparisonState.page);
  element("comparison-page").textContent = "Page " + (comparisonState.page + 1) + " / " + Math.max(1, Math.ceil(comparisonState.rows.length / 20));
  const pair = view.pairs[Number(element("comparison-detail").value)];
  element("comparison-provenance").textContent = pair ? JSON.stringify({ a: pair.a, b: pair.b }, null, 2) : "Choose a pair to inspect frozen settings, exact populations, time labels, bindings, marking evidence and limitations. Repeated ranges describe per-trial deltas, not pooled p95/p99.";
  comparisonControls();
}
// comparisonControls bounds local pagination after common session/action gating.
function comparisonControls() {
  const blocked = state.busy || !state.token;
  element("comparison-prev").disabled = blocked || !comparisonState.view || comparisonState.page === 0;
  element("comparison-next").disabled = blocked || !comparisonState.view || (comparisonState.page + 1) * 20 >= comparisonState.rows.length;
}
// clearComparison removes previous-session evidence and selection labels when disconnecting.
function clearComparison() { comparisonState.view = null; comparisonState.rows = []; comparisonState.page = 0; for (const id of ["comparison-status", "comparison-policy", "comparison-table", "comparison-provenance", "comparison-page", "comparison-detail"]) element(id).replaceChildren(); }
// readComparison validates explicit report pairs and axes; failures retain labeled last-successful evidence.
async function readComparison() {
  const lines = element("comparison-pairs").value.trim().split(/\r?\n/);
  if (lines.length < 1 || lines.length > 8) throw new Error("Select one to eight A/B pairs, one pair per line.");
  const pairs = lines.map(function pairLine(line) { /* Safe IDs only; no file paths or repeated evidence. */ const ids = line.trim().split(/\s+/); if (ids.length !== 2) throw new Error("Each line must contain A-report-ID B-report-ID."); return { a: safeID(ids[0]), b: safeID(ids[1]) }; });
  const axes = ["adapter", "port", "client_dscp", "server_dscp"].filter(function selectedAxis(axis) { /* Selection must match each report's frozen comparison_axes. */ return element("axis-" + axis).checked; });
  const view = await api("/api/v1/reports/compare", "POST", { pairs, axes, endpoint: element("comparison-endpoint").value, scope: element("comparison-scope").value, evidence_policy:evidencePolicy() });
  comparisonState.view = view; comparisonState.page = 0;
  element("comparison-detail").replaceChildren();
  const option = document.createElement("option"); option.value = "repeated"; option.textContent = "Repeated delta ranges"; element("comparison-detail").append(option);
  view.pairs.forEach(function pairOption(pair, i) { /* Label both saved identities and unsuccessful pair states. */ const option = document.createElement("option"); option.value = String(i); option.textContent = "Pair " + i + ": " + pair.a.report_id + " / " + pair.b.report_id + " · " + pair.state; element("comparison-detail").append(option); });
  element("comparison-detail").value = "repeated"; renderComparison(); notice("Saved trials compared. Read each pair's compatibility, populations and limits before interpreting differences.");
}
bind("compare-reports", readComparison);
bind("comparison-prev", function previousComparison() { /* Page the admitted snapshot only. */ if (comparisonState.page > 0) comparisonState.page--; renderComparison(); });
bind("comparison-next", function nextComparison() { /* Keep rendering within twenty rows. */ if ((comparisonState.page + 1) * 20 < comparisonState.rows.length) comparisonState.page++; renderComparison(); });
element("comparison-detail").addEventListener("change", function comparisonDetail() { /* Reset pages when changing pair/repeated view. */ comparisonState.page = 0; renderComparison(); });
comparisonControls();
