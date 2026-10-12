"use strict";

// diagnosticsState owns selected replay/preflight snapshots and bounded pages, never probe timing.
const diagnosticsState = { view: null, page: 0, phasePage: 0, preflight: null, wakePage: 0 };
// textNode builds plain-text content without parsing imported evidence as markup.
function textNode(tag, text) { const node = document.createElement(tag); node.textContent = text; return node; }
// exactValue preserves decimal strings and explicitly labels unavailable evidence.
function exactValue(value) { return value === null || value === undefined ? "unavailable" : String(value); }
// tablePage renders at most twenty rows; page controls never reload raw evidence.
function tablePage(target, headers, rows, page) {
  const table = document.createElement("table"), head = document.createElement("tr");
  for (const label of headers) head.append(textNode("th", label)); table.append(head);
  for (const row of rows.slice(page * 20, (page + 1) * 20)) { const tr = document.createElement("tr"); for (const value of row) tr.append(textNode("td", exactValue(value))); table.append(tr); }
  element(target).replaceChildren(table);
}
// svgNode creates native SVG with fixed attribute names and safe textual tooltips.
function svgNode(tag, attributes, title = "") {
  const node = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [key, value] of Object.entries(attributes)) node.setAttribute(key, String(value));
  if (title) { const label = document.createElementNS("http://www.w3.org/2000/svg", "title"); label.textContent = title; node.append(label); }
  return node;
}
// ratio converts only bounded coordinate ratios after exact BigInt arithmetic; evidence stays strings.
function ratio(value, maximum) { return maximum === 0n ? 0 : Number(BigInt(value) * 1000000n / maximum) / 1000000; }
// plot draws range envelopes and missing/local markers without connecting absent responses.
// ECDF groups retain rank envelopes, not synthetic samples or recomputed final quantiles.
function plot(target, view, metric, cumulative) {
  const groups = cumulative ? view[metric + "_ecdf"] : view.time;
  const svg = svgNode("svg", { viewBox: "0 0 800 250", role: "img", "aria-label": cumulative ? "Completed-sample ECDF rank envelopes" : "Host-local scheduled time and duration ranges" });
  let maximum = 0n;
  for (const group of groups) { const range = cumulative ? group : group[metric]; if (range.max_ns !== null) maximum = maximum > BigInt(range.max_ns) ? maximum : BigInt(range.max_ns); }
  const cutoff = BigInt(view.cutoff_ns), samples = BigInt(view.summary.RTTSamples);
  // Coordinate conversion occurs only after scaling exact nanoseconds/ranks to bounded ratios.
  const x = function xCoordinate(value) { return 50 + 720 * ratio(value, cumulative ? maximum : cutoff); };
  const y = function yCoordinate(value) { /* Map the exact duration/rank ratio to bounded vertical coordinates. */ return 210 - 180 * ratio(value, cumulative ? samples : maximum); };
  svg.append(svgNode("line", { x1: 50, y1: 210, x2: 770, y2: 210, class: "axis" }));
  svg.append(svgNode("line", { x1: 50, y1: 30, x2: 50, y2: 210, class: "axis" }));
  if (!cumulative) {
    for (const phase of view.phases) {
      if (BigInt(phase.start_ns) > cutoff) continue;
      const boundary = x(phase.start_ns); svg.append(svgNode("line", { x1: boundary, y1: 30, x2: boundary, y2: 210, class: "phase-boundary" }, phase.phase_id + ": [" + phase.start_ns + ", " + phase.end_ns + ") ns"));
    }
  }
  for (const group of groups) {
    const range = cumulative ? group : group[metric], label = JSON.stringify(group);
    if (cumulative) {
      svg.append(svgNode("rect", { x: x(range.min_ns), y: y(group.last_rank), width: Math.max(1, x(range.max_ns) - x(range.min_ns)), height: Math.max(1, y(group.first_rank) - y(group.last_rank)), class: "sample-range" }, label));
    } else {
      if (range.count !== "0") svg.append(svgNode("rect", { x: x(group.start_ns), y: y(range.max_ns), width: Math.max(2, x(group.end_ns) - x(group.start_ns)), height: Math.max(2, y(range.min_ns) - y(range.max_ns)), class: "sample-range" }, label));
      if (group.missing !== "0" || group.local !== "0") svg.append(svgNode("circle", { cx: x(group.start_ns), cy: 225, r: 3, class: "missing-marker" }, "Missing/local outcomes: " + label));
    }
  }
  const bottom = svgNode("text", { x: 50, y: 245, class: "chart-label" }); bottom.textContent = cumulative ? "Duration ns (0 to " + maximum + ") · cumulative completed fraction (0 to 1)" : "Host-local scheduled ns (0 to " + cutoff + ") · duration ns (0 to " + maximum + ")"; svg.append(bottom);
  if (!groups.length) { const empty = svgNode("text", { x: 70, y: 110, class: "chart-label" }); empty.textContent = "No eligible samples / reached slots in this scope."; svg.append(empty); }
  element(target).replaceChildren(svg);
}
// renderDiagnostic retains exact tables beside labeled bounded plots and endpoint-only evidence status.
function renderDiagnostic() {
  const view = diagnosticsState.view; if (!view) return;
  element("diagnostic-status").textContent = view.report_id + " · " + view.endpoint + " · originating scope=" + view.scope + " · " + view.status + " · complete=" + view.complete + " · " + view.reason + " · integrity=" + view.integrity;
  element("plot-policy").textContent = view.plot_policy;
  // Deadline misses and their populations precede tails; no ranking or combined score.
  const first = ["DeadlineMisses", "Planned", "LocalMisses", "AcceptedMisses", "Pending", "Skipped", "OnTime", "Late", "Unresolved", "RTTSamples", "RTTP95", "RTTP99"];
  const keys = [...first, ...Object.keys(view.summary).filter(function remaining(key) { /* Retain remaining scalar fields once. */ return !first.includes(key); }).sort()];
  const rows = keys.map(function summaryRow(key) { /* State timing units; never convert null to zero. */ return [key + (/Median|P95|P99|Min|Max/.test(key) ? " (ns)" : ""), view.summary[key]]; });
  tablePage("diagnostic-table", ["Metric", "Exact value"], rows, diagnosticsState.page);
  element("metric-page").textContent = "Page " + (diagnosticsState.page + 1) + " / " + Math.ceil(rows.length / 20);
  element("metrics-prev").disabled = diagnosticsState.page === 0; element("metrics-next").disabled = (diagnosticsState.page + 1) * 20 >= rows.length;
  const metric = element("plot-metric").value || "rtt"; plot("time-plot", view, metric, false); plot("ecdf-plot", view, metric, true);
  tablePage("phase-table", ["Index / phase", "Start ns", "End ns (exclusive)"], view.phases.map(function phaseRow(phase, index) { /* Display exact phase boundaries. */ return [index + " / " + phase.phase_id, phase.start_ns, phase.end_ns]; }), diagnosticsState.phasePage);
  element("phase-page").textContent = "Page " + (diagnosticsState.phasePage + 1) + " / " + Math.max(1, Math.ceil(view.phases.length / 20));
  element("phases-prev").disabled = diagnosticsState.phasePage === 0; element("phases-next").disabled = (diagnosticsState.phasePage + 1) * 20 >= view.phases.length;
}
// readDiagnostic loads one endpoint/originating scope and freezes it for local pagination/rendering.
async function readDiagnostic() {
  const report = safeID(element("reports").value), scope = element("report-scope").value;
  if (!/^(main|warmup|0|[1-9][0-9]*)$/.test(scope)) throw new Error("Choose main, warmup, or a zero-based originating phase index.");
  const view = await api("/api/v1/reports/" + report + "/diagnostics?endpoint=" + element("report-endpoint").value + "&scope=" + scope);
  diagnosticsState.view = view; diagnosticsState.page = 0; diagnosticsState.phasePage = 0; renderDiagnostic(); notice("Validated saved evidence loaded. Counts and quantiles retain exact values.");
}
// renderPreflight paginates wake evidence with unverified clock quality and exact lag tails.
function renderPreflight() {
  const result = diagnosticsState.preflight; if (!result) return;
  element("preflight-status").textContent = result.status + " · wakes=" + result.wakes.length + " · missed slots=" + result.missed_slots + " · lag p95/p99 ns=" + exactValue(result.lag_p95_ns) + "/" + exactValue(result.lag_p99_ns) + " · " + result.clock;
  tablePage("preflight-table", ["Scheduled ns", "Actual ns", "Lag ns", "Earlier due slots"], result.wakes.map(function wakeRow(wake) { /* Preserve exact times and missed counts. */ return [wake.scheduled_ns, wake.actual_ns, wake.lag_ns, wake.earlier_due_slots]; }), diagnosticsState.wakePage);
  element("wakes-prev").disabled = diagnosticsState.wakePage === 0; element("wakes-next").disabled = (diagnosticsState.wakePage + 1) * 20 >= result.wakes.length;
}
// clearDiagnostics clears provenance on session changes so earlier evidence cannot appear current.
function clearDiagnostics() { diagnosticsState.view = null; diagnosticsState.preflight = null; for (const id of ["diagnostic-status", "plot-policy", "diagnostic-table", "time-plot", "ecdf-plot", "phase-table", "preflight-status", "preflight-table", "metric-page", "phase-page"]) element(id).replaceChildren(); }

// diagnosticControls preserves page bounds after common controls update without rerendering charts on edits.
function diagnosticControls() {
  const blocked = state.busy || !state.token, view = diagnosticsState.view, flight = diagnosticsState.preflight;
  element("metrics-prev").disabled = blocked || !view || diagnosticsState.page === 0;
  element("metrics-next").disabled = blocked || !view || (diagnosticsState.page + 1) * 20 >= Object.keys(view.summary).length;
  element("phases-prev").disabled = blocked || !view || diagnosticsState.phasePage === 0;
  element("phases-next").disabled = blocked || !view || (diagnosticsState.phasePage + 1) * 20 >= view.phases.length;
  element("wakes-prev").disabled = blocked || !flight || diagnosticsState.wakePage === 0;
  element("wakes-next").disabled = blocked || !flight || (diagnosticsState.wakePage + 1) * 20 >= flight.wakes.length;
}

bind("refresh-reports", async function refreshReports() { /* Explicit catalog refresh does not schedule a replay. */ fillSelect("reports", await catalog("reports")); notice("Saved report list refreshed."); });
bind("read-report", readDiagnostic);
bind("metrics-prev", function previousMetrics() { /* Bound local scalar pagination. */ if (diagnosticsState.page > 0) diagnosticsState.page--; renderDiagnostic(); });
bind("metrics-next", function nextMetrics() { /* Advance within the fixed scalar summary. */ if (diagnosticsState.view && (diagnosticsState.page + 1) * 20 < Object.keys(diagnosticsState.view.summary).length) diagnosticsState.page++; renderDiagnostic(); });
bind("phases-prev", function previousPhases() { /* Page boundaries without replaying files. */ if (diagnosticsState.phasePage > 0) diagnosticsState.phasePage--; renderDiagnostic(); });
bind("phases-next", function nextPhases() { /* Retain the phase capacity. */ if (diagnosticsState.view && (diagnosticsState.phasePage + 1) * 20 < diagnosticsState.view.phases.length) diagnosticsState.phasePage++; renderDiagnostic(); });
element("plot-metric").addEventListener("change", renderDiagnostic);
bind("preflight", async function startPreflight() { /* Observe operator-supplied local timing without changing the workload or starting traffic. */
  const duration = element("preflight-duration").value, interval = element("preflight-interval").value;
  if (!/^[1-9][0-9]*$/.test(duration) || !/^[1-9][0-9]*$/.test(interval)) throw new Error("Provide positive canonical nanosecond strings.");
  diagnosticsState.preflight = await api("/api/v1/preflight", "POST", { duration_ns: duration, interval_ns: interval }); diagnosticsState.wakePage = 0; renderPreflight(); notice("Local preflight observed; it does not establish network or calibration quality.");
});
bind("wakes-prev", function previousWakes() { /* Page bounded preflight samples. */ if (diagnosticsState.wakePage > 0) diagnosticsState.wakePage--; renderPreflight(); });
bind("wakes-next", function nextWakes() { /* Advance within the admitted snapshot. */ if (diagnosticsState.preflight && (diagnosticsState.wakePage + 1) * 20 < diagnosticsState.preflight.wakes.length) diagnosticsState.wakePage++; renderPreflight(); });
diagnosticControls();
