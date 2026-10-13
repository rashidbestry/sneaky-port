"use strict";

// workspace keeps display selection/history separate from immutable plans and server-owned run timing.
const workspace = { run: "", result: null, resultRun: "", loading: null, autoStorage: true, initial: null, phase:0, settings:false, group:"traffic", patterns:{} };
// prettyName turns stored names into readable labels without exposing generated run/report handles.
function prettyName(value) { return String(value).replace(/[-_]+/g," ").replace(/\b(udp|tcp)\b/gi,function protocolName(word) { /* Preserve familiar protocol acronyms in saved names. */ return word.toUpperCase(); }); }
// unitText formats exact decimal integers into human units without floating-point rounding.
function unitText(value, places) {
  if(value===null || value===undefined) return "—";
  const number=BigInt(value), scale=10n**BigInt(places), fraction=(number%scale).toString().padStart(places,"0").replace(/0+$/,"");
  return (number/scale).toString()+(fraction ? "."+fraction : "");
}
// unitValue accepts friendly unsigned decimals and returns exact schema integers, never JavaScript Numbers.
function unitValue(text, places) {
  const input=text.trim().replace(/^\./,"0."),match=/^(\d+)(?:\.(\d*))?$/.exec(input);
  if(!match || (match[2] || "").length>places) throw new Error("Use a nonnegative decimal with at most "+places+" decimal places.");
  const value=BigInt(match[1])*10n**BigInt(places)+BigInt((match[2] || "").padEnd(places,"0") || "0");
  if(value>18446744073709551615n) throw new Error("Value exceeds the supported range.");
  return value.toString();
}
// rateParts reduces exact packets-per-second decimals or loaded fractions to the scheduler's rational rate.
function rateParts(text) {
  const value=text.trim().replace(/^\./,"0."), ratio=/^(\d+)\/(\d+)$/.exec(value), decimal=/^(\d+)(?:\.(\d{0,9}))?$/.exec(value);
  if(!ratio && !decimal) throw new Error("Enter packets per second, for example 60 or 59.94.");
  let numerator=ratio ? BigInt(ratio[1]) : BigInt(decimal[1]+(decimal[2] || "")), denominator=ratio ? BigInt(ratio[2]) : 10n**BigInt((decimal[2] || "").length);
  if(denominator===0n) throw new Error("Packet rate denominator must be positive.");
  let a=numerator,b=denominator; while(b) { const remainder=a%b; a=b; b=remainder; }
  numerator/=a; denominator/=a;
  if(numerator>18446744073709551615n || denominator>18446744073709551615n) throw new Error("Packet rate exceeds the supported range.");
  return [numerator.toString(),denominator.toString()];
}
// rateText preserves nonterminating loaded rates as fractions instead of changing their offered schedule.
function rateText(direction) {
  if(direction.mode==="quiet") return "0";
  const numerator=BigInt(direction.rate_numerator), denominator=BigInt(direction.rate_denominator);
  if(denominator===0n) return "Unsupported rate";
  if(numerator%denominator===0n) return String(numerator/denominator);
  let divisor=denominator,places=0; while(divisor%2n===0n) { divisor/=2n; places++; } let fives=0; while(divisor%5n===0n) { divisor/=5n; fives++; }
  places=Math.max(places,fives);
  return divisor===1n && places<=9 ? unitText(numerator*10n**BigInt(places)/denominator,places) : numerator+"/"+denominator;
}
// draftStorage sizes bounded evidence/control capacity without changing traffic, clock cadence or heartbeat timing.
// The shared API/server validators remain authoritative; no network rate or measurement evidence is invented.
function draftStorage(plan) {
  const total=BigInt(plan.warmup_ns)+BigInt(plan.grace_ns)+plan.phases.reduce(function runDuration(sum,phase) { /* All ordered phases participate in the frozen control lifetime. */ return sum+BigInt(phase.duration_ns); },0n), heartbeat=BigInt(plan.limits.heartbeat_interval_ns),cadence=BigInt(plan.calibration.cadence_ns);
  if(total<=0n || total>9223372036854775807n || heartbeat<=0n || cadence<=0n) throw new Error("Check the test duration, heartbeat interval and clock cadence.");
  const clocks=(total+cadence-1n)/cadence, lifecycle=total/heartbeat+8n;
  if(!workspace.autoStorage) { if(lifecycle+clocks>BigInt(plan.limits.control_requests)) throw new Error("This duration needs at least "+(lifecycle+clocks)+" control requests. Enable automatic resources or increase the manual limit."); return; }
  let slots=0n;
  for(let index=0;index<plan.phases.length;index++) for(const direction of [plan.phases[index].client,plan.phases[index].server]) {
    if(direction.mode==="quiet") continue;
    const duration=BigInt(plan.phases[index].duration_ns)+(index===0 ? BigInt(plan.warmup_ns) : 0n), denominator=BigInt(direction.rate_denominator)*1000000000n;
    if(denominator===0n) throw new Error("Check the packet rate in Traffic settings.");
    slots+=(duration*BigInt(direction.rate_numerator)+denominator-1n)/denominator;
  }
  const ledger=slots+128n, clockBytes=clocks*4096n, events=ledger*4096n+16384n+clockBytes, files=events*2n+8388608n;
  const transferBytes=files>BigInt(plan.limits.file_bytes) ? files : BigInt(plan.limits.file_bytes), requests=lifecycle+clocks+(transferBytes+32767n)/32768n+3n;
  if(ledger>1048576n || events>1073741824n || transferBytes>1073741824n || requests>1048576n) throw new Error("This test exceeds automatic resource capacity. Reduce duration/rate or review Storage limits.");
  plan.limits.control_requests=Math.max(plan.limits.control_requests,Number(requests));
  plan.limits.ledger_messages=Math.max(plan.limits.ledger_messages,Number(ledger));
  for(const [key,needed] of [["event_bytes",events],["file_bytes",files]]) if(BigInt(plan.limits[key])<needed) plan.limits[key]=needed.toString();
  const payload=Math.max(...plan.phases.flatMap(function payloads(phase) { /* Both directions share explicit local queue reservations. */ return [phase.client.content_bytes,phase.server.content_bytes]; }));
  const queue=BigInt(payload+124)*BigInt(plan.limits.receipt_reserve+1);
  if(BigInt(plan.limits.queue_bytes)<queue) plan.limits.queue_bytes=queue.toString();
}
// guidedEdit retains incomplete text in its real input and updates only explicitly edited friendly fields.
function guidedEdit(id,apply) {
  const field=element(id);
  // editSetting blocks malformed edits without replacing the prior valid plan or stealing focus.
  function editSetting() {
    try { const draft=clone(state.draft); apply(draft,field.value); draftStorage(draft); apply(state.draft,field.value); draftStorage(state.draft); workspace.editorDirty=true; state.invalid.delete(field); field.title=""; }
    catch(error) { state.invalid.add(field); field.title=error.message; }
    field.setAttribute("aria-invalid",String(state.invalid.has(field))); controls();
  }
  field.addEventListener("input",editSetting);
  field.revalidate=editSetting;
  if(field.tagName.toLowerCase()==="select") field.addEventListener("change",editSetting);
}
// settingsObject resolves an explicit draft path; input handlers never retain an obsolete cloned plan.
function settingsObject(plan,path) { let object=plan; for(const key of path) object=object[key]; return object; }
// friendlyField builds one typed inline setting, preserving exact integers and malformed input text.
function friendlyField(root,path,key,title,places=null,kind=null) {
  const object=settingsObject(state.draft,path),value=object[key],label=document.createElement("label"),field=document.createElement(kind==="boolean" ? "select" : "input");
  label.textContent=title; label.dataset.setting=key;
  if(kind==="boolean") for(const [choice,text] of [["unset","Not set"],["true","Enabled"],["false","Disabled"]]) { const option=document.createElement("option"); option.value=choice; option.textContent=text; field.append(option); }
  field.value=kind==="boolean" ? (value===undefined ? "unset" : String(value)) : places===null ? String(value ?? "") : unitText(value ?? "0",places);
  field.maxLength=kind==="string" ? 4096 : 30; field.inputMode=kind==="string" ? "text" : "decimal";
  // editInline applies only valid typed values; the shared server validator remains authoritative.
  function editInline() {
    try {
      let parsed=field.value;
      if(kind==="boolean") { if(!["unset","true","false"].includes(parsed)) throw new Error("Choose Enabled or Disabled."); parsed=parsed==="unset" ? undefined : parsed==="true"; }
      else if(places!==null) parsed=unitValue(parsed,places);
      else if(typeof value==="number" || kind==="integer") { const integer=unitValue(parsed,0); if(BigInt(integer)>BigInt(Number.MAX_SAFE_INTEGER)) throw new Error("Integer exceeds the supported range."); parsed=Number(integer); }
      const trial=clone(state.draft),target=settingsObject(trial,path); if(parsed===undefined) delete target[key]; else target[key]=parsed; draftStorage(trial);
      const current=settingsObject(state.draft,path); if(parsed===undefined) delete current[key]; else current[key]=parsed; draftStorage(state.draft); workspace.editorDirty=true; state.invalid.delete(field); field.title="";
    } catch(error) { state.invalid.add(field); field.title=error.message; }
    field.setAttribute("aria-invalid",String(state.invalid.has(field))); controls();
  }
  field.addEventListener(kind==="boolean" ? "change" : "input",editInline); field.revalidate=editInline; label.append(field); root.append(label); return field;
}
// settingTitle adds familiar units to schema quantities without changing their values or timing semantics.
function settingTitle(key) { const names={ledger_messages:"Evidence records",receipt_reserve:"Reserved receipt slots",outstanding_data:"Outstanding data messages",event_bytes:"Event storage · bytes",file_bytes:"Report storage · bytes",queue_bytes:"Queue capacity · bytes",samples:"Minimum valid clock samples",window:"Clock sample window",bound_source:"Clock bound evidence source",drift_ppb:"Clock drift bound · ppb",environment:"Matched environment / path"}; return names[key] || prettyName(key.replace(/_ns$/,""))+(key.endsWith("_ns") ? " · ms" : ""); }
// settingsFields rebuilds optional fields only after an explicit load/structural edit, never on a keystroke.
function settingsFields(id,build) {
  const root=element(id); navigation.fields=navigation.fields.filter(function retainOtherGroups(entry) { /* Retain unrelated field pages and invalid nodes. */ return entry.root!==root; }); root.replaceChildren(); build(root);
  const notes=Array.from(root.children).filter(function settingNote(child) { /* Capability limitations must survive field pagination. */ return child.tagName.toLowerCase()!=="label"; });
  if(notes.length!==root.children.length) { pageFields(root); for(const note of notes) root.append(note); }
}
// syncSettings maps every supported test setting to typed inline groups and preserves loaded custom phases.
function syncSettings() {
  if(!state.draft) return;
  const plan=state.draft,phase=plan.phases[workspace.phase];
  for(const [prefix,key] of [["up","client"],["down","server"]]) { element("test-"+prefix+"-mode").value=phase[key].mode; for(const part of ["active","gap"]) element("test-"+prefix+"-"+part).value=unitText(phase[key][part+"_ns"],6); }
  for(const id of ["test-up-dscp","test-down-dscp"]) if(!element(id).children.length) for(let code=0;code<=63;code++) { const option=document.createElement("option"); option.value=String(code); option.textContent=code===0 ? "Default · DSCP 0" : "DSCP "+code; element(id).append(option); }
  element("test-up-dscp").value=String(phase.client.dscp); element("test-down-dscp").value=String(phase.server.dscp);
  element("source-port-choice").value=plan.source_port==="auto" ? "auto" : "custom";
  element("friendly-phase").replaceChildren(); plan.phases.forEach(function phaseChoice(item,index) { /* Keep stable phase identities internal while showing ordinal/readable names. */ const option=document.createElement("option"); option.value=String(index); option.textContent=(index+1)+" · "+prettyName(item.phase_id); element("friendly-phase").append(option); });
  element("friendly-phase").value=String(workspace.phase); element("friendly-phase-name").value=prettyName(phase.phase_id); element("friendly-phase-deadline").value=phase.deadline_ns===undefined ? "" : unitText(phase.deadline_ns,6);
  settingsFields("friendly-limits",function safetyFields(root) { /* Automatic allocation hides only managed capacities; queue behavior/timing remain explicit. */ for(const key of Object.keys(plan.limits)) if(!workspace.autoStorage || !["control_requests","ledger_messages","queue_bytes","event_bytes","file_bytes"].includes(key)) friendlyField(root,["limits"],key,settingTitle(key),key.endsWith("_ns") ? 6 : typeof plan.limits[key]==="string" ? 0 : null); });
  settingsFields("friendly-clock",function clockFields(root) { /* Clock model inputs are visible bounds/provenance, not a claim of calibrated production timing. */ for(const key of Object.keys(plan.calibration)) friendlyField(root,["calibration"],key,settingTitle(key),key.endsWith("_ns") ? 6 : key==="drift_ppb" ? 0 : null,key==="bound_source" ? "string" : null); for(const key of ["preflight_duration_ns","preflight_interval_ns","spike_threshold_ns"]) friendlyField(root,[],key,settingTitle(key),6); });
  settingsFields("friendly-content",function contentFields(root) { /* Simple zero-byte payloads need no encoding input; custom bytes remain exact and explicit. */ for(const [key,title] of [["client","Upstream"],["server","Downstream"]]) {
    const identity=phase.phase_id+":"+key,label=document.createElement("label"),choice=document.createElement("select"); label.textContent=title+" payload pattern";
    for(const [value,text] of [["zero","Zero bytes"],["custom","Custom repeated bytes"]]) { const option=document.createElement("option"); option.value=value; option.textContent=text; choice.append(option); }
    const custom=phase[key].content_pattern_base64!=="AA==" || workspace.patterns[identity]; choice.value=custom ? "custom" : "zero"; label.append(choice); root.append(label);
    const field=friendlyField(root,["phases",workspace.phase,key],"content_pattern_base64",title+" repeated bytes · Base64",null,"string"),patternLabel=root.lastChild; patternLabel.hidden=!custom;
    choice.addEventListener("change",function payloadChoice() { /* Changing content is explicit; opening Custom does not invent replacement bytes. */ if(state.invalid.size) { choice.value=patternLabel.hidden ? "zero" : "custom"; notice("Check highlighted settings before changing payload pattern.",true); return; } workspace.patterns[identity]=choice.value==="custom"; patternLabel.hidden=choice.value!=="custom"; if(choice.value==="zero") { state.draft.phases[workspace.phase][key].content_pattern_base64="AA=="; field.value="AA=="; workspace.editorDirty=true; } controls(); });
  } });
  settingsFields("friendly-protocol",function protocolFields(root) { /* Descriptor types drive carrier options; undeclared/complex options stay visible and unmodified for validation. */ const d=descriptor(),options=plan.adapter_options; for(const key of new Set([...Object.keys(d?.options || {}),...Object.keys(options)])) { if(key==="tcp_no_delay") continue; const declared=d?.options[key] || "",value=options[key]; if(typeof value==="object" || !/boolean|integer|string/.test(declared)) { const note=document.createElement("p"); note.className="hint"; note.textContent=prettyName(key)+": unsupported typed control; retained for shared validation. Use the CLI to edit this extension option."; root.append(note); } else friendlyField(root,["adapter_options"],key,prettyName(key),null,declared.includes("boolean") ? "boolean" : declared.includes("integer") ? "integer" : "string"); } if(!root.children.length) { const note=document.createElement("p"); note.className="hint"; note.textContent=plan.adapter_id==="tcp" ? "TCP no-delay is above, beside Run test." : "This protocol has no additional supported options."; root.append(note); } });
  showSettings();
}
// showSettings changes only inline group visibility; invalid edits and pending command identities survive.
function showSettings() {
  const groups={traffic:"view-dashboard-traffic",network:"settings-network",phases:"settings-phases",saved:"view-dashboard-saved",limits:"settings-limits",clock:"settings-clock",timer:"settings-timer",content:"settings-content",protocol:"settings-protocol"};
  element("test-settings").hidden=!workspace.settings; element("test-advanced").textContent=workspace.settings ? "Hide settings" : "More settings"; element("test-advanced").setAttribute("aria-expanded",String(workspace.settings)); element("settings-group").value=workspace.group;
  for(const [name,id] of Object.entries(groups)) element(id).hidden=name!==workspace.group;
  const hints={traffic:"Each direction has its own pattern. Burst timing appears only for Burst.",network:"SSH port is above. These ports and requested DSCP apply to measured data.",phases:"Main inputs edit this phase. Adding a phase copies its settings; test order is explicit.",saved:"Run test automatically saves and negotiates your current settings.",limits:"Automatic storage adjusts capacity only. Queue behavior and timeouts remain explicit.",clock:"Bounds need independent evidence; changing inputs does not prove calibration.",timer:"Local wake observations only. No network traffic or calibrated clock claim.",content:"Zero bytes needs no extra input. Custom repeated bytes use Base64; sizes are above.",protocol:"Only the selected protocol's declared options apply."}; element("settings-hint").textContent=hints[workspace.group];
}
// settingsControls reveals burst/source-port choices without changing hidden values or the frozen run.
function settingsControls(blocked) {
  const phase=state.draft?.phases[workspace.phase];
  for(const id of ["test-up-mode","test-down-mode","test-up-active","test-up-gap","test-down-active","test-down-gap","source-port-choice","friendly-phase","friendly-phase-name","friendly-phase-deadline","friendly-phase-add","friendly-phase-up","friendly-phase-down","friendly-phase-remove"]) element(id).disabled=blocked;
  if(phase) for(const [prefix,key] of [["up","client"],["down","server"]]) { element("test-"+prefix+"-burst").hidden=phase[key].mode!=="burst"; element("test-"+prefix+"-mode").value=phase[key].mode; element("test-"+prefix+"-rate").disabled=blocked || phase[key].mode==="quiet"; }
  element("source-port-field").hidden=element("source-port-choice").value!=="custom";
  element("friendly-phase-up").disabled=blocked || workspace.phase===0; element("friendly-phase-down").disabled=blocked || !state.draft || workspace.phase+1>=state.draft.phases.length; element("friendly-phase-remove").disabled=blocked || !state.draft || state.draft.phases.length===1;
  element("timer-view-option").disabled=typeof diagnosticsState==="undefined" || !diagnosticsState.preflight;
  element("preflight").disabled=blocked || state.invalid.size>0;
}
// syncWorkspace reads the exact loaded draft into normal controls only after explicit load/save/structural editing.
function syncWorkspace() {
  if(!state.draft) return;
  if(!workspace.initial) workspace.initial=clone(state.draft);
  workspace.phase=Math.min(workspace.phase,state.draft.phases.length-1);
  const plan=state.draft,phase=plan.phases[workspace.phase];
  element("test-name").value=prettyName(plan.profile_id);
  element("test-protocol").replaceChildren();
  for(const adapter of state.adapters) { const option=document.createElement("option"); option.value=adapter.adapter_id; option.textContent=adapter.adapter_id.toUpperCase(); element("test-protocol").append(option); }
  element("test-protocol").value=plan.adapter_id;
  const duration=plan.phases.reduce(function sumDuration(total,item) { /* Display full custom-phase duration without flattening its plan. */ return total+BigInt(item.duration_ns); },0n);
  for(const [id,value] of [["test-duration",unitText(phase.duration_ns,9)],["test-warmup",unitText(plan.warmup_ns,9)],["test-deadline",unitText(plan.receipt_deadline_ns,6)],["test-payload",phase.client.content_bytes],["test-down-payload",phase.server.content_bytes],["test-up-rate",rateText(phase.client)],["test-down-rate",rateText(phase.server)],["test-up-dscp",phase.client.dscp],["test-down-dscp",phase.server.dscp],["test-port",plan.destination_port],["test-source-port",plan.source_port],["test-grace",unitText(plan.grace_ns,6)]]) element(id).value=String(value);
  element("test-no-delay").checked=plan.adapter_options.tcp_no_delay===true;
  element("automatic-storage").checked=workspace.autoStorage;
  const custom=plan.phases.length!==1;
  element("custom-plan").hidden=!custom;
  element("custom-plan").textContent=custom ? "Editing phase "+(workspace.phase+1)+" of "+plan.phases.length+" · "+prettyName(phase.phase_id)+" · total test "+unitText(duration,9)+" s. Select a phase in More settings." : "";
  syncSettings();
  workspaceControls(); renderWorkspace();
}
// workspaceControls edits only the selected phase and reveals options relevant to the chosen carrier/pattern.
function workspaceControls() {
  const blocked=state.busy || !!workspace.loading || !state.token || !state.draft, running=state.run && ["executing","cancelling"].includes(state.run.status);
  for(const id of ["test-name","test-protocol","test-warmup","test-deadline","test-up-dscp","test-down-dscp","test-port","test-source-port","test-grace","automatic-storage"]) element(id).disabled=blocked;
  for(const id of ["test-duration","test-payload","test-down-payload","test-up-rate","test-down-rate","test-up-dscp","test-down-dscp"]) element(id).disabled=blocked;
  element("test-no-delay-row").hidden=state.draft?.adapter_id!=="tcp"; element("test-no-delay").disabled=blocked;
  element("reset-test").disabled=blocked || !!running || !!state.pending;
  element("cancel").disabled=state.busy || !state.token || !running || !!state.pending;
  element("test-refresh").disabled=state.busy || !state.token || !state.run;
  element("test-retry").hidden=!state.pending; element("test-retry").disabled=state.busy || !state.pending;
  element("test-recovery").hidden=!state.pending;
  element("test-pending").textContent=state.pending ? "Request acknowledgment unknown. Refresh status or retry the same request; do not start another test." : "";
  if(state.draft) {
    element("test-target").textContent=(state.connection?.status==="connected" ? state.connection.server_ip : "Connect a server above")+" · data port "+state.draft.destination_port;
    const phases=state.draft.phases,total=phases.reduce(function totalDuration(sum,phase) { /* Show the current valid draft duration without rewriting a partly typed field. */ return sum+BigInt(phase.duration_ns); },0n);
    element("custom-plan").hidden=phases.length===1; element("custom-plan").textContent=phases.length>1 ? "Editing phase "+(workspace.phase+1)+" of "+phases.length+" · "+prettyName(phases[workspace.phase].phase_id)+" · total test "+unitText(total,9)+" s. Select a phase in More settings." : "";
  }
  settingsControls(blocked);
}
// percentText reports small nonzero ratios as bounds; missing/zero populations never become false zero loss.
function percentText(numerator,denominator) {
  const n=BigInt(numerator),d=BigInt(denominator); if(d===0n) return "—";
  const scaled=n*100000n/d; return n>0n && scaled===0n ? "<0.001%" : unitText(scaled,3)+"%";
}
// metricCard assigns exact human display text and its population/status independently of other metrics.
function metricCard(id,value,note) { element(id).textContent=value; element(id+"-note").textContent=note; }

// sampleNote keeps per-metric sample/coverage limitations beside human-unit values rather than implying confidence.
function sampleNote(view,key) { const label=view.evidence_labels?.[key]; return label ? label.completed+" samples · "+evidenceBrief(label) : "Evidence unavailable"; }

// rememberProgress resets results on run changes; the engine supplies bounded provisional RTT envelopes.
function rememberProgress(run) {
  if(workspace.run!==run.control_run_id) { workspace.run=run.control_run_id; workspace.result=null; workspace.resultRun=""; }
  renderWorkspace();
}
// latencyPlot renders client attempt-to-receipt RTT in ms; missing receipts never create latency samples.
// Live one-second ranges retain min/max/count; authoritative saved ranges replace them after replay.
function latencyPlot(progress,view) {
  element("test-latency-note").textContent=view ? (view.complete ? "Final" : "Partial")+" client RTT · ms · completed-sample ranges" : "Provisional client RTT · ms · up to 128 one-second ranges";
  if(view) { plot("test-rtt-chart",view,"rtt",false,true); return; }
  const ranges=progress?.rtt_time || [],time=ranges.map(function liveRange(range) { /* Preserve the engine's exact envelope and omit unknown missing/local time bins. */ return {start_ns:range.start_ns,end_ns:range.end_ns,rtt:{count:range.count,min_ns:range.min_ns,max_ns:range.max_ns},missing:"0",local:"0"}; });
  plot("test-rtt-chart",{time,phases:[],cutoff_ns:progress?.elapsed_ns || "0",summary:{RTTSamples:progress?.main.completed || "0"}},"rtt",false,true);
}
// renderWorkspace keeps live counts, finalized replay and partial/unavailable states visibly distinct.
function renderWorkspace() {
  const run=state.run,p=run?.progress,view=workspace.result;
  const active=run && ["executing","cancelling"].includes(run.status);
  element("test-state").textContent=active ? (run.status==="cancelling" ? "Stopping test…" : "Running · "+prettyName(run.profile_id)+" · "+prettyName(p?.phase_id || "Preparing")) : run ? (run.status==="partial" ? "Stopped / partial" : run.status==="completed" ? "Finished" : prettyName(run.status))+" · "+prettyName(run.profile_id) : "Ready to test";
  element("test-error").hidden=!run?.reason; element("test-error").textContent=run?.reason ? "Test issue: "+run.reason : "";
  element("test-progress").value=p && BigInt(p.total_ns)>0n ? Number(BigInt(p.elapsed_ns)*1000n/BigInt(p.total_ns)) : 0;
  element("test-time").textContent=p ? unitText(p.elapsed_ns,9)+" / "+unitText(p.total_ns,9)+" s" : "";
  const summary=view?.summary,counts=p?.main,basis=summary ? (view.complete ? "Final saved results" : "Partial saved results") : counts ? "Provisional client-local counts" : "No observations yet";
  element("test-evidence").textContent=basis+(view ? " · "+(view.endpoint==="server" ? "Server origin" : "Client origin")+" · "+view.scope+" · "+view.integrity : "")+". Missed deadlines and pending receipts are not packet loss.";
  metricCard("card-rtt",summary && summary.RTTMedian!=null ? unitText(summary.RTTMedian,6)+" ms" : "—",summary ? sampleNote(view,"RTTMedian") : "Available after finalization");
  metricCard("card-tail",summary && summary.RTTP95!=null ? unitText(summary.RTTP95,6)+" ms" : "—",summary ? sampleNote(view,"RTTP95") : "Available after finalization");
  metricCard("card-variation",summary && summary.VariationP95!=null ? unitText(summary.VariationP95,6)+" ms" : "—",summary ? sampleNote(view,"VariationP95") : "Available after finalization");
  const planned=summary?.Planned ?? counts?.planned,misses=summary?.DeadlineMisses ?? counts?.deadline_misses;
  metricCard("card-misses",String(misses ?? "—"),misses!==undefined ? (summary || p.accounting_valid ? percentText(misses,planned)+" of "+planned+" reached slots" : "Accounting incomplete")+" · includes late/local misses" : "No reached slots yet");
  metricCard("card-on-time",String(summary?.OnTime ?? counts?.on_time ?? "—"),"On-time application receipts");
  metricCard("card-late",String(summary?.Late ?? counts?.late ?? "—"),"Also counted as missed deadlines");
  metricCard("card-pending",String(summary?.Pending ?? counts?.pending ?? "—"),"Pending deadlines · delivery unknown");
  latencyPlot(p,view);
}
// loadRunResult automatically loads the exact terminal report once; failures stay explicit and retries are read-only.
async function loadRunResult(run) {
  if(!run.report_id || ["executing","cancelling"].includes(run.status) || workspace.resultRun===run.control_run_id || workspace.loading) return;
  const epoch=state.epoch,target=run.control_run_id;
  const loading=(async function readFinalReport() { /* Serialize saved reads and retain identity after a terminal status, never retry a mutation. */
    const view=await api("/api/v1/runs/"+safeID(target)+"/diagnostics?endpoint=local&scope=main"+evidenceQuery());
    if(epoch!==state.epoch || state.run?.control_run_id!==target) return;
    workspace.result=view; workspace.resultRun=target; renderWorkspace();
  })(); workspace.loading=loading; controls();
  try { await loading; } catch(error) { if(epoch===state.epoch) notice("Test ended; saved results could not be loaded. Refresh results to retry. "+error.message,true); }
  finally { if(workspace.loading===loading) { workspace.loading=null; controls(); } }
}
// clearWorkspace releases prior observations/result labels on local access loss without cancelling admitted work.
function clearWorkspace() { workspace.run=""; workspace.result=null; workspace.resultRun=""; workspace.initial=null; workspace.phase=0; workspace.settings=false; workspace.group="traffic"; showSettings(); renderWorkspace(); }
guidedEdit("test-name",function testName(plan,value) { /* Name edits derive safe storage identities; no UID needs to be typed. */ const name=value.trim().toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"").slice(0,64); if(!name) throw new Error("Give the test a name using letters or numbers."); plan.profile_id=name; });
for(const [id,key,places] of [["test-duration","duration_ns",9],["test-warmup","warmup_ns",9],["test-deadline","receipt_deadline_ns",6],["test-grace","grace_ns",6]]) guidedEdit(id,function timing(plan,value) { /* Scale only the selected phase or explicitly edited global duration. */ const parsed=unitValue(value,places); if(["duration_ns","receipt_deadline_ns"].includes(key) && parsed==="0") throw new Error("Duration and deadline must be greater than zero."); if(key==="duration_ns") plan.phases[workspace.phase][key]=parsed; else plan[key]=parsed; });
for(const [id,key] of [["test-payload","client"],["test-down-payload","server"]]) guidedEdit(id,function payload(plan,value) { /* Independent selected-phase payloads preserve asymmetric and custom plans. */ const parsed=unitValue(value,0); const maximum=(descriptor()?.max_frame_bytes || 1048576)-124; if(BigInt(parsed)>BigInt(maximum)) throw new Error("Payload exceeds this protocol's frame limit ("+maximum+" bytes)."); plan.phases[workspace.phase][key].content_bytes=Number(parsed); });
for(const [id,key] of [["test-up-rate","client"],["test-down-rate","server"]]) guidedEdit(id,function packetRate(plan,value) { /* Preserve burst windows when editing its rate; zero explicitly selects quiet traffic. */ const [n,d]=rateParts(value),direction=plan.phases[workspace.phase][key]; direction.rate_numerator=n; direction.rate_denominator=d; if(n==="0") direction.mode="quiet"; else if(direction.mode==="quiet") direction.mode="steady"; if(direction.mode!=="burst") { direction.active_ns="0"; direction.gap_ns="0"; } });
for(const [id,key] of [["test-up-dscp","client"],["test-down-dscp","server"]]) guidedEdit(id,function priority(plan,value) { /* Apply selected-phase requested DSCP independently of captured marking evidence. */ const parsed=unitValue(value,0); if(BigInt(parsed)>63n) throw new Error("DSCP must be 0–63."); plan.phases[workspace.phase][key].dscp=Number(parsed); });
guidedEdit("test-port",function dataPort(plan,value) { /* SSH connection port and measured destination port remain different settings. */ const parsed=unitValue(value,0); if(BigInt(parsed)<1n || BigInt(parsed)>65535n) throw new Error("Data port must be 1–65535."); plan.destination_port=Number(parsed); });
guidedEdit("test-source-port",function sourcePort(plan,value) { /* Automatic selection is explicit; a selected custom port must be valid. */ if(element("source-port-choice").value==="auto") plan.source_port="auto"; else { const parsed=unitValue(value,0); if(BigInt(parsed)<1n || BigInt(parsed)>65535n) throw new Error("Source port must be 1–65535."); plan.source_port=parsed; } });
element("test-protocol").addEventListener("change",function protocolChoice() { /* Choosing a protocol visibly changes its real supported options; no carrier substitution is permitted. */
  if(state.invalid.size) { notice("Check highlighted settings before switching protocol.",true); return; }
  const adapter=state.adapters.find(function selected(item) { /* Resolve the actual registered adapter contract. */ return item.adapter_id===element("test-protocol").value; });
  if(!adapter) return;
  state.draft.adapter_id=adapter.adapter_id; state.draft.adapter_contract_version=adapter.adapter_contract_version;
  state.draft.adapter_options=adapter.adapter_id==="tcp" ? {tcp_no_delay:element("test-no-delay").checked} : {};
  renderEditor(); controls();
});
element("test-no-delay").addEventListener("change",function tcpOption() { /* A visible checkbox supplies the explicit native TCP boolean instead of JSON. */ if(state.draft.adapter_id==="tcp") state.draft.adapter_options.tcp_no_delay=element("test-no-delay").checked; controls(); });
element("automatic-storage").addEventListener("change",function storageChoice() { /* Enabling automatic capacity can resolve valid pending duration/resource edits; malformed text is retained. */ const automatic=element("automatic-storage").checked; if(state.invalid.size && !automatic) { element("automatic-storage").checked=workspace.autoStorage; notice("Check highlighted settings before disabling automatic resources.",true); return; } workspace.autoStorage=automatic; if(automatic) { for(const field of Array.from(state.invalid)) if(field.revalidate) field.revalidate(); try { draftStorage(state.draft); } catch(error) { notice(error.message,true); } } if(!state.invalid.size) syncSettings(); controls(); });
bind("reset-test",function resetTest() { /* Reset only the draft/display, keep saved evidence and the authenticated connection. */ const draft=clone(workspace.initial),names=new Set(Array.from(element("profiles").children).map(function savedName(option) { /* Generate a readable unused test name without requiring a machine handle. */ return option.value; })); let suffix=1,name=draft.adapter_id+"-test"; while(names.has(name) || name===state.selected.profile.profile_id) name=draft.adapter_id+"-test-"+(++suffix); draft.profile_id=name; if(state.connection?.status==="connected") { draft.server_address=state.connection.server_ip; draft.pairing_ref=state.connection.pairing_ref; draft.address_family=draft.server_address.includes(":") ? "ipv6" : "ipv4"; } state.draft=draft; state.saved=false; state.run=null;  workspace.result=null; workspace.resultRun=""; renderEditor(); controls(); notice("New test ready. Saved tests and results are retained."); });
bind("test-refresh",async function refreshTest() { /* Read the managed run handle; users never have to paste it into a normal form. */ if(state.run) { showRun(await api("/api/v1/runs/"+safeID(state.run.control_run_id))); await loadRunResult(state.run); schedulePoll(); } });
bind("test-retry",transmitPending);
element("test-advanced").addEventListener("click",function trafficSettings() { /* Expand settings in place without requests, field replacement or a separate view. */ workspace.settings=!workspace.settings; showSettings(); });
element("settings-group").addEventListener("change",function settingsGroup() { /* Group selection retains field nodes; timer pagination uses the already loaded snapshot. */ workspace.group=element("settings-group").value; showSettings(); if(workspace.group==="timer") { diagnosticsState.wakePage=0; renderPreflight(); } });
for(const [prefix,key] of [["up","client"],["down","server"]]) {
  element("test-"+prefix+"-mode").addEventListener("change",function trafficPattern() { /* Quiet clears its offered schedule; Burst exposes explicit active/pause durations. */ if(state.invalid.size) { element("test-"+prefix+"-mode").value=state.draft.phases[workspace.phase][key].mode; notice("Check highlighted settings before switching traffic pattern.",true); return; } const direction=state.draft.phases[workspace.phase][key],mode=element("test-"+prefix+"-mode").value; if(!["steady","burst","quiet"].includes(mode)) return; direction.mode=mode; if(mode!=="burst") { direction.active_ns="0"; direction.gap_ns="0"; } if(mode==="quiet") { direction.rate_numerator="0"; direction.rate_denominator="1"; } workspace.editorDirty=true; syncWorkspace(); controls(); });
  for(const part of ["active","gap"]) guidedEdit("test-"+prefix+"-"+part,function burstTime(plan,value) { /* Apply milliseconds exactly to only the selected direction/phase. */ plan.phases[workspace.phase][key][part+"_ns"]=unitValue(value,6); });
}
element("source-port-choice").addEventListener("change",function chooseSourcePort() { /* A custom port needs explicit valid input; selecting Automatic releases only that pending edit. */ const field=element("test-source-port"),choice=element("source-port-choice").value; if(Array.from(state.invalid).some(function otherInvalid(input) { /* Do not clear unrelated malformed edits. */ return input!==field; })) { element("source-port-choice").value=state.draft.source_port==="auto" ? "auto" : "custom"; notice("Check highlighted settings before changing source port mode.",true); return; } if(choice==="auto") { state.draft.source_port="auto"; field.value="auto"; state.invalid.delete(field); field.setAttribute("aria-invalid","false"); } else if(state.draft.source_port==="auto") { field.value=""; state.invalid.add(field); field.title="Enter a source port from 1–65535."; field.setAttribute("aria-invalid","true"); } workspace.editorDirty=true; controls(); });
element("friendly-phase").addEventListener("change",function chooseFriendlyPhase() { /* Switching phases cannot discard invalid text from the previous phase. */ if(state.invalid.size) { element("friendly-phase").value=String(workspace.phase); notice("Check highlighted settings before changing phase.",true); return; } const index=Number(element("friendly-phase").value); if(Number.isInteger(index) && index>=0 && index<state.draft.phases.length) { workspace.phase=index; syncWorkspace(); controls(); } });
guidedEdit("friendly-phase-name",function phaseName(plan,value) { /* Derive a safe readable phase identity only after an explicit name edit. */ const name=value.trim().toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"").slice(0,64); if(!name || plan.phases.some(function duplicate(phase,index) { /* Prevent two frozen phases sharing one accounting identity. */ return index!==workspace.phase && phase.phase_id===name; })) throw new Error("Use a unique phase name containing letters or numbers."); plan.phases[workspace.phase].phase_id=name; });
guidedEdit("friendly-phase-deadline",function phaseDeadline(plan,value) { /* Blank explicitly inherits the global deadline; overrides keep exact milliseconds. */ if(!value.trim()) delete plan.phases[workspace.phase].deadline_ns; else { const parsed=unitValue(value,6); if(parsed==="0") throw new Error("Deadline override must be positive."); plan.phases[workspace.phase].deadline_ns=parsed; } });
for(const [id,delta] of [["friendly-phase-add",2],["friendly-phase-up",-1],["friendly-phase-down",1],["friendly-phase-remove",0]]) bind(id,function changePhaseOrder() { /* Explicit phase structure changes preserve all untouched direction settings and the running frozen plan. */ if(state.invalid.size) throw new Error("Check highlighted settings before editing phase order."); const phases=state.draft.phases,index=workspace.phase; if(delta===2) { if(phases.length>=64) throw new Error("At most 64 phases can be edited here."); const phase=clone(phases[index]); let number=phases.length+1; while(phases.some(function usedName(item) { /* Keep generated ordinal names unique without asking for a UID. */ return item.phase_id==="phase-"+number; })) number++; phase.phase_id="phase-"+number; phases.splice(index+1,0,phase); workspace.phase=index+1; } else if(delta===0) { if(phases.length===1) throw new Error("Keep at least one phase."); phases.splice(index,1); workspace.phase=Math.min(index,phases.length-1); } else if(index+delta>=0 && index+delta<phases.length) { [phases[index],phases[index+delta]]=[phases[index+delta],phases[index]]; workspace.phase+=delta; } workspace.editorDirty=true; renderEditor(); controls(); });
renderWorkspace();
