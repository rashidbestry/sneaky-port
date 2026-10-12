"use strict";

// diagnosticsState owns selected replay/preflight snapshots and bounded pages, never probe timing.
const diagnosticsState = { view: null, page: 0, phasePage: 0, preflight: null, wakePage: 0 };
// textNode builds plain-text content without parsing imported evidence as markup.
function textNode(tag, text) { const node = document.createElement(tag); node.textContent = text; return node; }
// exactValue preserves decimal strings and explicitly labels unavailable evidence.
function exactValue(value) { return value === null || value === undefined ? "unavailable" : String(value); }
// tablePage renders a viewport-bounded page; page controls never reload raw evidence.
function tablePage(target, headers, rows, page, size=tablePageSize()) {
  const table = document.createElement("table"), head = document.createElement("tr");
  for (const label of headers) { const cell=textNode("th",label); cell.setAttribute("scope","col"); head.append(cell); } table.append(head);
  for (const row of rows.slice(page * size, (page + 1) * size)) { const tr = document.createElement("tr"); for (const value of row) tr.append(textNode("td", exactValue(value))); table.append(tr); }
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
function plot(target, view, metric, cumulative, human=false) {
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
    const range = cumulative ? group : group[metric], label = human ? "Completed samples: "+range.count+" · "+unitText(range.min_ns,6)+"–"+unitText(range.max_ns,6)+" ms" : JSON.stringify(group);
    if (cumulative) {
      svg.append(svgNode("rect", { x: x(range.min_ns), y: y(group.last_rank), width: Math.max(1, x(range.max_ns) - x(range.min_ns)), height: Math.max(1, y(group.first_rank) - y(group.last_rank)), class: "sample-range" }, label));
    } else {
      if (range.count !== "0") svg.append(svgNode("rect", { x: x(group.start_ns), y: y(range.max_ns), width: Math.max(2, x(group.end_ns) - x(group.start_ns)), height: Math.max(2, y(range.min_ns) - y(range.max_ns)), class: "sample-range" }, label));
      if (group.missing !== "0" || group.local !== "0") svg.append(svgNode("circle", { cx: x(group.start_ns), cy: 225, r: 3, class: "missing-marker" }, "Missing/local outcomes: " + label));
    }
  }
  const bottom = svgNode("text", { x: 50, y: 245, class: "chart-label" }); bottom.textContent = cumulative ? "Duration ns (0 to " + maximum + ") · cumulative completed fraction (0 to 1)" : "Host-local scheduled ns (0 to " + cutoff + ") · duration ns (0 to " + maximum + ")"; if(human) bottom.textContent=cumulative ? "Receipt RTT · 0 to "+unitText(maximum,6)+" ms · completed fraction" : "Elapsed · 0 to "+unitText(cutoff,9)+" s · RTT 0 to "+unitText(maximum,6)+" ms"; svg.append(bottom);
  if (!groups.length) { const empty = svgNode("text", { x: 70, y: 110, class: "chart-label" }); empty.textContent = "No eligible samples / reached slots in this scope."; svg.append(empty); }
  element(target).replaceChildren(svg);
}
// wakePageSize keeps optional inline timer evidence compact without dropping any exact observation.
function wakePageSize() { return navigation.section==="dashboard" ? 2 : tablePageSize(); }
// renderPreflight paginates wake evidence with unverified clock quality and exact lag tails.
function renderPreflight() {
  const result = diagnosticsState.preflight; if (!result) return;
  diagnosticsState.wakePage=Math.min(diagnosticsState.wakePage,Math.max(0,Math.ceil(result.wakes.length/wakePageSize())-1));
  element("wake-page").textContent="Page "+(diagnosticsState.wakePage+1)+" / "+Math.max(1,Math.ceil(result.wakes.length/wakePageSize()));
  element("preflight-status").textContent = result.status + " · wakes=" + result.wakes.length + " · missed slots=" + result.missed_slots + " · lag p95/p99 ns=" + exactValue(result.lag_p95_ns) + "/" + exactValue(result.lag_p99_ns) + " · " + result.clock;
  tablePage("preflight-table", ["Scheduled ns", "Actual ns", "Lag ns", "Earlier due slots"], result.wakes.map(function wakeRow(wake) { /* Preserve exact times and missed counts. */ return [wake.scheduled_ns, wake.actual_ns, wake.lag_ns, wake.earlier_due_slots]; }), diagnosticsState.wakePage,wakePageSize());
  element("wakes-prev").disabled = diagnosticsState.wakePage === 0; element("wakes-next").disabled = (diagnosticsState.wakePage + 1) * wakePageSize() >= result.wakes.length;
}
// evidenceBrief keeps current metric coverage/reasons beside the unchanged measured value.
function evidenceBrief(label) { if(!label) return "See context"; return label.coverage+(label.reasons.length ? " · "+label.reasons[0].replaceAll("_"," ") : " · "+label.basis); }
// evidenceQuery uses authoritative server defaults for current-test evidence presentation.
function evidenceQuery() { return ""; }
// clearDiagnostics drops local timer observations on access loss; retained runs remain server-owned.
function clearDiagnostics() { diagnosticsState.view=null; diagnosticsState.preflight=null; for(const id of ["preflight-status","preflight-table","wake-page","live-status","live-counters","live-page"]) element(id).replaceChildren(); element("live-progress").value=0; }
// diagnosticControls gates local timer actions/pages without editing workload schedules.
function diagnosticControls() { const blocked=state.busy || !state.token || typeof workspace!=="undefined" && !!workspace.loading,flight=diagnosticsState.preflight; element("preflight").disabled=blocked || state.invalid.size>0; element("wakes-prev").disabled=blocked || !flight || diagnosticsState.wakePage===0; element("wakes-next").disabled=blocked || !flight || (diagnosticsState.wakePage+1)*wakePageSize()>=flight.wakes.length; }
bind("preflight", async function startPreflight() { /* Observe operator-supplied local timing without changing the workload or starting traffic. */
  const main=navigation.section==="dashboard",duration=main ? state.draft.preflight_duration_ns : element("preflight-duration").value,interval=main ? state.draft.preflight_interval_ns : element("preflight-interval").value;
  if(main && state.invalid.size) throw new Error("Check highlighted settings before checking the local timer.");
  if (!/^[1-9][0-9]*$/.test(duration) || !/^[1-9][0-9]*$/.test(interval)) throw new Error("Provide positive canonical nanosecond strings.");
  diagnosticsState.preflight = await api("/api/v1/preflight", "POST", { duration_ns: duration, interval_ns: interval }); diagnosticsState.wakePage = 0; renderPreflight(); notice("Local preflight observed; it does not establish network or calibration quality.");
});
bind("wakes-prev", function previousWakes() { /* Page bounded preflight samples. */ if (diagnosticsState.wakePage > 0) diagnosticsState.wakePage--; renderPreflight(); });
bind("wakes-next", function nextWakes() { /* Advance within the admitted snapshot. */ if (diagnosticsState.preflight && (diagnosticsState.wakePage + 1) * wakePageSize() < diagnosticsState.preflight.wakes.length) diagnosticsState.wakePage++; renderPreflight(); });
diagnosticControls();
