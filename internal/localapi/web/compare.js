"use strict";

// comparisonState retains the last successful explicit snapshot; local pages never replay or schedule traffic.
const comparisonState = { view: null, page: 0, rows: [] };
// comparisonRows selects repeated or paired scope/phase rows without recomputing statistics in JavaScript.
function comparisonRows() {
  const view = comparisonState.view; if (!view) return [];
  const chosen = element("comparison-detail").value;
  const rows = [];
  if (chosen === "repeated") {
    const append = function repeatedRows(scope, metrics) { /* Retain trial population beside exact delta ranges; full labels remain in the evidence view. */ for (const m of metrics) rows.push([scope, metricName(m.metric), m.unit, m.trials + " pairs", m.min_delta, m.max_delta, m.state, evidenceBrief(m.evidence)]); };
    append(view.selection.scope, view.repeated); view.phases.forEach(function phaseRows(metrics, i) { /* Main originating phases remain visible independently. */ append("phase " + i, metrics); });
  } else {
    const pair = view.pairs[Number(chosen)]; if (!pair) return [];
    const append = function pairedRows(scope, metrics) { /* Preserve exact deltas and state; full arm populations remain in Metric evidence. */ for (const m of metrics) rows.push([scope, metricName(m.metric), m.unit, m.a, m.b, m.delta, m.state, evidenceBrief(m.evidence)]); };
    append(view.selection.scope, pair.metrics); pair.phases.forEach(function phaseRows(metrics, i) { /* Aggregate tails cannot conceal a phase failure. */ append("phase " + i, metrics); });
  }
  return rows;
}
// renderComparison displays viewport-bounded rows and separately paged full public provenance.
function renderComparison() {
  const view = comparisonState.view; if (!view) return;
  element("comparison-status").textContent = "Loaded "+view.pairs.length+" pair(s). States: "+Array.from(new Set(view.pairs.map(function pairState(pair) { /* Preserve every distinct unsuccessful state in the summary. */ return pair.state; }))).join(", ")+". Complete reasons in Comparison rules.";
  readText("comparison-policy",view.policy+"\n\n"+JSON.stringify({selection:view.selection,pairs:view.pairs.map(function pairReasons(pair,index) { /* Retain original identities and every compatibility reason. */ return {index,a:pair.a.report_id,b:pair.b.report_id,state:pair.state,reasons:pair.reasons}; }),evidence_policy:view.evidence_policy},null,2));
  const repeated = element("comparison-detail").value === "repeated";
  comparisonState.rows = comparisonRows();
  comparisonState.page=Math.min(comparisonState.page,Math.max(0,Math.ceil(comparisonState.rows.length/tablePageSize())-1));
  tablePage("comparison-table", repeated ? ["Scope", "Metric", "Unit", "Available trials", "Minimum B-A", "Maximum B-A", "State", "Evidence / reasons"] : ["Scope", "Metric", "Unit", "A", "B", "B-A", "State", "Evidence / reasons"], comparisonState.rows, comparisonState.page);
  element("comparison-page").textContent = "Page " + (comparisonState.page + 1) + " / " + Math.max(1, Math.ceil(comparisonState.rows.length / tablePageSize()));
  const pair = view.pairs[Number(element("comparison-detail").value)];
  readText("comparison-provenance",pair ? JSON.stringify({ a: pair.a, b: pair.b }, null, 2) : "Choose a pair in Differences to inspect frozen settings and provenance. Repeated ranges describe per-trial deltas, not pooled p95/p99.");
  const evidence=selectedComparisonEvidence(), previous=element("comparison-evidence-select").value; element("comparison-evidence-select").replaceChildren();
  evidence.forEach(function evidenceOption(row,index) { /* Preserve every selected/phase metric without pooling populations. */ const option=document.createElement("option"); option.value=String(index); option.textContent=row.scope+" / "+metricName(row.metric); element("comparison-evidence-select").append(option); }); if(Number(previous)<evidence.length) element("comparison-evidence-select").value=previous || "0"; renderComparisonEvidence();
  comparisonControls();
}
// selectedComparisonEvidence projects original label objects; displayed deltas and calculations stay unchanged.
function selectedComparisonEvidence() {
  const view=comparisonState.view; if(!view) return []; const repeated=element("comparison-detail").value==="repeated", source=repeated?view:view.pairs[Number(element("comparison-detail").value)]; if(!source) return [];
  const out=[]; const append=function evidenceRows(scope,metrics) { /* Keep independently labeled original scope and phases. */ for(const m of metrics) out.push({scope,...m}); }; append(view.selection.scope,repeated?source.repeated:source.metrics); source.phases.forEach(function phaseEvidence(metrics,index) { /* Phase tails are never pooled. */ append("phase "+index,metrics); }); return out;
}
// renderComparisonEvidence shows full per-arm/repeated counts and reasons beyond compact table summaries.
function renderComparisonEvidence() { const row=selectedComparisonEvidence()[Number(element("comparison-evidence-select").value)]; readText("comparison-evidence-detail",row?JSON.stringify(row,null,2):"Load a comparison and choose a metric."); }
element("comparison-evidence-select").addEventListener("change",renderComparisonEvidence);
// comparisonControls bounds local pagination after common session/action gating.
function comparisonControls() {
  const blocked = state.busy || !state.token || typeof workspace!=="undefined" && !!workspace.loading;
  element("comparison-prev").disabled = blocked || !comparisonState.view || comparisonState.page === 0;
  element("comparison-next").disabled = blocked || !comparisonState.view || (comparisonState.page + 1) * tablePageSize() >= comparisonState.rows.length;
}
// clearComparison removes previous-session evidence and selection labels when disconnecting.
function clearComparison() { comparisonState.view = null; comparisonState.rows = []; comparisonState.page = 0; for (const id of ["comparison-status", "comparison-policy", "comparison-table", "comparison-provenance", "comparison-page", "comparison-detail", "comparison-evidence-select"]) element(id).replaceChildren(); }
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
  element("comparison-detail").value = "repeated"; renderComparison(); navigate("compare","metrics"); notice("Saved trials compared. Read each pair's compatibility and evidence before interpreting differences.");
}
bind("compare-reports", readComparison);
bind("comparison-prev", function previousComparison() { /* Page the admitted snapshot only. */ if (comparisonState.page > 0) comparisonState.page--; renderComparison(); });
bind("comparison-next", function nextComparison() { /* Keep rendering within the viewport's page budget. */ if ((comparisonState.page + 1) * tablePageSize() < comparisonState.rows.length) comparisonState.page++; renderComparison(); });
element("comparison-detail").addEventListener("change", function comparisonDetail() { /* Reset pages when changing pair/repeated view. */ comparisonState.page = 0; renderComparison(); });
comparisonControls();
