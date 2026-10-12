"use strict";

// sections defines public navigation only; it never stores tokens, plans or API request identities.
const sections = {
  connection: { label: "Connection", views: [["session", "SSH connection", "Enter the Linux server details, connect, then configure your test."], ["trust", "Verify server", "Accept the observed SSH host identity before password authentication."], ["details", "Connection details", "Server identity, helper state and resource ceilings."], ["server", "Saved pairings", "Advanced: inspect saved measurement pairing records."], ["import", "Manual pairing", "Advanced: import a public bundle through a trusted channel."], ["identity", "Client identity", "Local identity is managed automatically for SSH setup."], ["message", "Message details", "Read the complete last operation message."]] },
  workload: { label: "Workload", views: [["profiles", "Saved workloads", "Load a profile or name the workload you are editing."], ["protocol", "Protocol", "Choose a registered carrier and its explicit options."], ["ports", "Address & ports", "Choose the data endpoint and client source port."], ["timing", "Run timing", "Warmup, receipt deadlines and the late-observation window."], ["phases", "Traffic phases", "Edit one ordered phase at a time: timing, upstream or downstream."], ["limits", "Resource limits", "Explicit limits bound local queues, records and storage."], ["calibration", "Clock settings", "Review clock sampling/model settings separately from traffic."], ["metadata", "Comparison setup", "Declare comparison axes, environment and additional settings."], ["json", "Complete JSON", "Advanced import/editing of the full profile."]] },
  run: { label: "Run", views: [["live", "Live measurement", "Follow provisional local counts or recover an existing run handle."], ["frozen", "Started workload", "These are the frozen settings from your start command."], ["details", "Status details", "Exact API status; unknown evidence remains unknown."], ["preflight", "Timer preflight", "Check local timer behavior before sending network traffic."], ["wakes", "Timer results", "Inspect the saved local preflight snapshot."]] },
  results: { label: "Results", views: [["select", "Choose report", "Choose an origin and population, then explicitly load saved evidence."], ["metrics", "Quality metrics", "Deadline outcomes first, then latency tails and sample counts."], ["time", "Time plot", "Inspect phase-aligned completion ranges and missing/local markers."], ["distribution", "Distribution", "Inspect the completed-sample empirical distribution."], ["phases", "Phase boundaries", "Host-local phase timing; endpoints are not synchronized."], ["evidence", "Metric evidence", "Read a metric's complete counts, basis and limitations."], ["context", "Measurement context", "Clock, marking, environment and plot limitations remain independent."], ["policy", "Sample targets", "Advisory display preferences; measured values and traffic stay unchanged."], ["export", "Download report", "Export the complete report in JSON, JSONL or CSV."]] },
  compare: { label: "Compare", views: [["select", "Choose trials", "Choose distinct matched A/B runs and predeclared axes."], ["metrics", "Differences", "Exact B minus A, ties and inconclusive results; no winner."], ["evidence", "Metric evidence", "Inspect complete per-arm or repeated evidence labels."], ["details", "Trial settings", "Inspect the selected pair's public settings and provenance."], ["policy", "Comparison rules", "Read compatibility rules and the loaded presentation policy."]] }
};
const navigation = { section: "connection", views: {}, fields: [], readers: new Map(), authenticated:false, serverConnected:false, serverRequired:false };

// navElement resolves only fixed public shell IDs; no URL/history state is used for navigation.
function navElement(id) { return document.getElementById(id); }
// connected reads a boolean set by session controls; navigation never retains a second credential.
function connected() { return navigation.authenticated; }
// tabKeys implements vertical tab keyboard movement without sending measurement commands.
function tabKeys(event, buttons, index) {
  if (!["ArrowUp", "ArrowDown", "Home", "End"].includes(event.key)) return;
  event.preventDefault();
  const available = buttons.filter(function enabled(button) { /* Skip locked setup views. */ return !button.disabled; });
  const current = available.indexOf(buttons[index]); if (!available.length) return;
  const next = event.key === "Home" ? 0 : event.key === "End" ? available.length - 1 : (current + (event.key === "ArrowDown" ? 1 : -1) + available.length) % available.length;
  const target=available[next]; target.click(); (document.getElementById(target.id) || target).focus();
}
// navigate switches a single visible view, preserving every draft field, pending command and active poll.
function navigate(section, view = navigation.views[section] || sections[section]?.views[0][0]) {
  const group = sections[section], chosen = group && group.views.find(function match(item) { /* Reject arbitrary panel IDs. */ return item[0] === view; });
  if (!chosen || !connected() && !(section === "connection" && ["session", "message"].includes(view))) return false;
  if(["workload","run"].includes(section) && navigation.serverRequired && !navigation.serverConnected) return false;
  navigation.section = section; navigation.views[section] = view;
  for (const [name, config] of Object.entries(sections)) {
    navElement("nav-" + name).setAttribute("aria-selected", String(name === section));
    navElement("nav-" + name).setAttribute("aria-controls","view-"+name+"-"+(navigation.views[name] || config.views[0][0]));
    navElement("nav-" + name).tabIndex = name === section ? 0 : -1;
    for (const [id] of config.views) navElement("view-" + name + "-" + id).hidden = name !== section || id !== view;
  }
  const tabs = navElement("section-tabs"); tabs.replaceChildren(); tabs.setAttribute("aria-label", group.label + " views");
  for (const [id, title] of group.views) {
    const button = document.createElement("button"); button.id = "sub-" + section + "-" + id; button.textContent = title; button.setAttribute("role", "tab"); button.setAttribute("aria-controls", "view-" + section + "-" + id); button.setAttribute("aria-selected", String(id === view)); button.tabIndex = id === view ? 0 : -1;
    button.addEventListener("click", function selectView() { /* Local visibility only; no requests or draft replacement. */ navigate(section, id); });
    button.addEventListener("keydown", function subtabKey(event) { /* Move between the current section's vertical tabs. */ const buttons = Array.from(tabs.children); tabKeys(event, buttons, buttons.indexOf(button)); });
    tabs.append(button); navElement("view-" + section + "-" + id).setAttribute("aria-labelledby", button.id);
  }
  navElement("breadcrumb").textContent = group.label + " / " + chosen[1]; navElement("view-title").textContent = chosen[1]; navElement("view-description").textContent = chosen[2];
  navElement("editor-actions").hidden = section !== "workload" || !connected();
  if(section==="connection" && view==="message") readText("message-details",navElement("notice").textContent);
  navigationControls(); refreshReaders(); return true;
}
// navigationControls leaves navigation usable during requests while locking protected views before connect.
function navigationControls() {
  for (const name of Object.keys(sections)) navElement("nav-" + name).disabled = name !== "connection" && !connected() || ["workload","run"].includes(name) && navigation.serverRequired && !navigation.serverConnected;
  for (const button of navElement("section-tabs").children) button.disabled = !connected() && !["sub-connection-session", "sub-connection-message"].includes(button.id) || button.id==="sub-connection-trust" && !navigation.trustPending;
  navElement("notice-details").disabled = false;
  navElement("session-badge").textContent = navigation.serverConnected ? "SSH server connected" : connected() ? "Dashboard ready" : "Opening dashboard";
  navElement("connection-indicator").textContent = navigation.serverConnected ? "● Server connected" : "○ Server disconnected";
}
// fieldPageSize selects small form pages so controls fit beside the two vertical navigation columns.
function fieldPageSize() { return window.innerWidth < 760 ? 2 : window.innerHeight < 760 ? 4 : 6; }
// tablePageSize budgets table rows conservatively; complete evidence moves to its own detail view.
function tablePageSize() { return Math.max(2, Math.min(8, Math.floor((window.innerHeight - 410) / 58))); }
// livePageSize reserves extra vertical room for status, cancellation/retry and provisional warnings.
function livePageSize() { return Math.max(2,tablePageSize()-3); }
// pageFields moves existing inputs into pages once; page changes never discard malformed pending edits.
function pageFields(root) {
  const fields = []; for (const child of Array.from(root.children)) { if (child.tagName.toLowerCase() === "label") fields.push(child); else if (child.className === "grid") fields.push(...Array.from(child.children)); }
  root.replaceChildren(); const entry = { root, fields, page: 0 }; navigation.fields.push(entry); drawFields(entry);
}
// drawFields reuses original input nodes and bounds each page to the current viewport.
function drawFields(entry) {
  const size = fieldPageSize(), count = Math.max(1, Math.ceil(entry.fields.length / size)); entry.page = Math.min(entry.page, count - 1);
  entry.root.replaceChildren(); const grid = document.createElement("div"); grid.className = "field-page";
  for (const field of entry.fields.slice(entry.page * size, (entry.page + 1) * size)) grid.append(field);
  entry.root.append(grid);
  if (count > 1) entry.root.append(pager(entry.page, count, function fieldPage(page) { /* Reattach the same input nodes, preserving pending text/errors. */ entry.page = page; drawFields(entry); if (typeof controls === "function") controls(); }));
}
// pager constructs local previous/next controls with an explicit total; it never reloads data.
function pager(page, count, select) {
  const row = document.createElement("div"); row.className = "pager";
  const previous = document.createElement("button"), label = document.createElement("span"), next = document.createElement("button"); previous.textContent = "Previous"; next.textContent = "Next"; previous.disabled = page === 0; next.disabled = page + 1 >= count; label.textContent = "Page " + (page + 1) + " / " + count;
  previous.addEventListener("click", function previousPage() { /* Local bounded page change. */ if (page > 0) select(page - 1); }); next.addEventListener("click", function nextPage() { /* Local bounded page change. */ if (page + 1 < count) select(page + 1); }); row.append(previous, label, next); return row;
}
// readText retains the complete public source text while showing bounded pages of wrapped lines.
function readText(id, value) {
  const text = String(value), previous = navigation.readers.get(id), entry = { id, text, page: previous && previous.text === text ? previous.page : 0 };
  navigation.readers.set(id, entry); drawReader(entry);
}
// drawReader wraps display lines without changing source data; no large JSON block requires page scrolling.
function drawReader(entry) {
  const node = navElement(entry.id), columns = Math.max(24, Math.min(100, Math.floor((node.clientWidth || (window.innerWidth - 460)) / 8)));
  const lines = []; for (const line of entry.text.split("\n")) { if (!line.length) lines.push(""); for (let i = 0; i < line.length; i += columns) lines.push(line.slice(i, i + columns)); }
  const size = Math.max(1, Math.min(20, Math.floor(((node.clientHeight || (window.innerHeight - 330)) - 32) / 19))), count = Math.max(1, Math.ceil(lines.length / size)); entry.page = Math.min(entry.page, count - 1);
  entry.pageCount=count;
  node.textContent = lines.slice(entry.page * size, (entry.page + 1) * size).join("\n");
  const controls = navElement(entry.id + "-pages"); controls.replaceChildren(); controls.append(pager(entry.page, count, function textPage(page) { /* Full text remains available on subsequent pages. */ entry.page = page; drawReader(entry); }));
}
// refreshReaders reflows only the selected view's public detail reader on navigation/resize.
function refreshReaders() { const panel=navElement("view-" + navigation.section + "-" + navigation.views[navigation.section]); for (const entry of navigation.readers.values()) if (panel.contains(navElement(entry.id))) drawReader(entry); }
// clearReaders drops retained public snapshots on disconnect, preventing prior-session details from resurfacing.
function clearReaders() { for (const entry of navigation.readers.values()) { navElement(entry.id).textContent = ""; navElement(entry.id + "-pages").replaceChildren(); } navigation.readers.clear(); }
// metricName translates common report names while retaining exact values/units in the original data.
function metricName(name) { const names = { DeadlineMisses: "Missed deadlines", DeadlineMissFraction: "Missed deadline fraction", Planned: "Reached slots", LocalMisses: "Local failures", AcceptedMisses: "Admitted deadline misses", Pending: "Pending deadlines", OnTime: "On-time receipts", Late: "Late receipts", RTTP95: "Receipt RTT p95", RTTP99: "Receipt RTT p99", RTTSamples: "Completed RTT samples", RTTMedian: "Receipt RTT median", VariationP95: "RTT variation p95", VariationP99: "RTT variation p99" }; return names[name] || name.replace(/([a-z])([A-Z])/g, "$1 $2").replaceAll("_", " "); }
for (const [name, group] of Object.entries(sections)) {
  const button = navElement("nav-" + name); button.setAttribute("aria-label", group.label); button.setAttribute("title", group.label);
  button.addEventListener("click", function primaryTab() { /* Navigation does not start/cancel runs or save edits. */ navigate(name); });
  button.addEventListener("keydown", function primaryKey(event) { /* Follow vertical-tab keyboard conventions. */ const buttons = Object.keys(sections).map(function mainButton(id) { /* Stable ordered main tabs. */ return navElement("nav-" + id); }); tabKeys(event, buttons, buttons.indexOf(button)); });
}
navElement("notice-details").addEventListener("click", function messageDetails() { /* Complete errors stay reachable beyond the compact global notice. */ readText("message-details", navElement("notice").textContent); navigate("connection", "message"); });
window.addEventListener("resize", function resizeDashboard() { /* Reflow retained inputs/readers and paginate existing data only; never issue a request. */ for (const entry of navigation.fields) drawFields(entry); refreshReaders(); if (typeof renderDiagnostic === "function") renderDiagnostic(); if (typeof renderComparison === "function") renderComparison(); if (typeof renderPreflight === "function") renderPreflight(); if (connected() && state.run) renderLive(state.run); if (typeof controls === "function") controls(); });
navigate("connection", "session");
