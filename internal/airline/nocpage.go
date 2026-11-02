package airline

// nocHTML is a carrier's operations centre: the day's signals, the crews,
// and projections of a disruption's cost under each policy. The charts
// are one series each, in one hue; every number a chart draws is also in
// a table beside it.
const nocHTML = `<!doctype html><meta charset="utf-8"><title>wholesky — operations centre {{CODE}}</title>
<meta name="viewport" content="width=device-width,initial-scale=1"><style>` + styleCSS + `
  .viz { width:100%; height:auto; display:block; }
  .viz .grid { stroke:#1d2836; stroke-width:1; }
  .viz .axis { fill:#5b6b7d; font-size:10px; }
  .viz .bar { fill:#3987e5; } .viz .bar:hover, .viz .bar:focus { fill:#5598e7; outline:none; }
  .viz .mid { stroke:#c9d8e6; stroke-width:2; }
  .viz .lbl { fill:#9fb4c8; font-size:11px; }
  #tip { position:fixed; pointer-events:none; background:#0f1721; border:1px solid #223042; border-radius:4px; padding:5px 8px; font-size:11px; display:none; z-index:9; }
  #tip b { color:var(--plane); display:block; font-size:13px; }
  tr.pick td { background:#0f1721; } tr.sel td { background:#122030; } tr.best td:first-child { color:var(--hot); }
  details { margin-top:6px; } summary { cursor:pointer; color:#5b6b7d; font-size:11px; }
  .form { display:grid; grid-template-columns: repeat(auto-fit, minmax(130px,1fr)); gap:8px; align-items:end; }
  .form label { display:flex; flex-direction:column; gap:3px; color:#5b6b7d; font-size:11px; } .form input, .form select { width:auto; }
</style>
<header><b>WHOLESKY</b> <span>operations centre</span> <i>{{CODE}}</i> <span class="muted">day <i id="clock">--:--</i></span> <span class="muted" id="manual"></span>
<span style="margin-left:auto"><a href="/ops/{{CODE}}">ops →</a> · <a href="/eye">the sky →</a> · <a href="https://github.com/adamf/wholesky/blob/main/docs/noc.md">how this works →</a></span></header>
<main>
<section class="wide"><h2>The day so far</h2><div class="kpis" id="day"></div></section>

<section><h2>Weather over your stations</h2><table><thead><tr><th>system</th><th>airports</th><th>window</th><th>rate</th><th>state</th><th>your arrivals</th></tr></thead><tbody id="wx"></tbody></table>
<h2 style="margin-top:12px">Regulations on your flights</h2><table><thead><tr><th>regulation</th><th>window</th><th>rate</th><th>cause</th><th>flights</th><th>delay</th></tr></thead><tbody id="regs"></tbody></table>
<p class="muted" id="live"></p></section>

<section><h2>Add an event to the day</h2>
<p class="muted" style="margin:0 0 8px">Every operations centre it touches sees it as it would a forecast. One event per visitor per half-minute.</p>
<div class="form">
<label>kind<select id="ekind"><option>weather</option><option>groundstop</option><option>closure</option><option>sickout</option><option>aog</option></select></label>
<label>airport<input id="eap" placeholder="LHR"></label>
<label>starts in (min)<input id="ein" value="200"></label>
<label>lasts (min)<input id="edur" value="240"></label>
<label>rate kept (weather)<input id="efac" value="0.5"></label>
<label>sick share (sick-out)<input id="erate" value="0.12"></label>
<label>flight (aog)<input id="efl" placeholder="{{CODE}}123"></label>
<label><button id="ego" class="primary">add it</button></label>
</div><p class="muted" id="eres"></p>
<table><thead><tr><th>events added today</th></tr></thead><tbody id="events"></tbody></table></section>

<section class="wide"><h2>Projection: what the forecast will cost</h2>
<div class="row" style="margin-bottom:8px">
<label class="muted">plays <input id="trials" value="200" style="width:60px"></label>
<label class="muted">sampling <select id="method"><option value="lhs">Latin hypercube</option><option value="mc">Monte Carlo</option></select></label>
<label class="muted"><input type="checkbox" id="sens" checked style="width:auto;vertical-align:middle"> sensitivity and stress cases</label>
<button id="go" class="primary">project the forecast</button>
<label class="muted">or real forecasts for <input id="taf" placeholder="KORD,KEWR" style="width:140px"></label><button id="gotaf">project them</button>
<span class="muted" id="pmsg"></span></div>
<div class="kpis" id="tiles"></div>
<p class="muted" id="pdesc"></p>
<div class="scroll"><table><thead><tr><th>policy</th><th>cancelled ahead</th><th>expected cost</th><th>median</th><th>1 day in 20 worse than</th><th>against absorb (95%)</th><th>delay min</th><th>missed connections</th><th>crews</th><th>revenue lost</th></tr></thead><tbody id="pols"></tbody></table></div>
</section>

<section><h2 id="htitle">Cost of the disruption, play by play</h2><svg class="viz" id="hist" viewBox="0 0 560 220" role="img" aria-label="histogram of the disruption's cost across plays"></svg>
<details><summary>table view</summary><table><thead><tr><th>cost from</th><th>to</th><th>plays</th></tr></thead><tbody id="histt"></tbody></table></details></section>

<section><h2>What the answer turns on</h2><svg class="viz" id="torn" viewBox="0 0 560 220" role="img" aria-label="sensitivity of the cost to each uncertainty"></svg>
<details><summary>table view</summary><table><thead><tr><th>uncertainty</th><th>at its 10th percentile</th><th>at its 90th</th></tr></thead><tbody id="tornt"></tbody></table></details>
<table style="margin-top:8px"><thead><tr><th>stress case</th><th>cost</th><th>cancelled</th><th>delay min</th><th>crew timeouts</th></tr></thead><tbody id="stress"></tbody></table></section>

<section class="wide"><h2>Crews and agreements</h2><div class="kpis" id="crewk"></div>
<div class="row" style="align-items:flex-start;gap:18px;margin-top:10px">
<div style="flex:1;min-width:320px"><h2>Pilots nearest a limit after today's duty</h2><table><thead><tr><th>pilot</th><th>base</th><th>fleet</th><th>status</th><th>tightest limit</th><th>left</th></tr></thead><tbody id="watch"></tbody></table></div>
<div style="flex:1;min-width:320px"><h2>Agreements</h2><table><thead><tr><th>unit</th><th>standing</th><th>amendable</th><th>open ask</th><th>sick calls</th></tr></thead><tbody id="contracts"></tbody></table>
<h2 style="margin-top:10px">By base</h2><div class="scroll" style="max-height:30vh"><table><thead><tr><th>base</th><th>fleet</th><th>position</th><th>line</th><th>on today</th><th>flying</th><th>reserves on call</th><th>called</th><th>sick</th></tr></thead><tbody id="heads"></tbody></table></div></div>
</div></section>
</main>
<div id="tip"></div>
<script>
const CODE = "{{CODE}}";
const $ = id => document.getElementById(id);
const money = c => { const d = c/100, s = d < 0 ? "-" : "", a = Math.abs(d); return a >= 1e6 ? s+"$"+(a/1e6).toFixed(2)+"M" : a >= 1e4 ? s+"$"+Math.round(a/1e3)+"k" : s+"$"+Math.round(a); };
const hm = m => { m = Math.round(m); return String(Math.floor(m/60)%24).padStart(2,"0") + String(m%60).padStart(2,"0"); };
function cell(tr, v, cls) { const td = document.createElement("td"); td.textContent = v; if (cls) td.className = cls; tr.appendChild(td); return td; }
function rows(tbody, list, fn) { tbody.replaceChildren(); for (const x of list) { const tr = document.createElement("tr"); fn(tr, x); tbody.appendChild(tr); } }
function tile(box, label, value, cls) { const d = document.createElement("div"); d.className = "kpi " + (cls||""); const b = document.createElement("b"); b.textContent = value; const s = document.createElement("span"); s.textContent = label; d.append(b, s); box.appendChild(d); }
const tip = $("tip");
function showTip(e, head, line) { tip.replaceChildren(); const b = document.createElement("b"); b.textContent = head; const s = document.createElement("span"); s.textContent = line; tip.append(b, s); tip.style.display = "block"; const r = e.target.getBoundingClientRect ? e.target.getBoundingClientRect() : {left:e.clientX, top:e.clientY}; tip.style.left = (e.clientX || r.left) + 12 + "px"; tip.style.top = (e.clientY || r.top) + 12 + "px"; }
function hideTip() { tip.style.display = "none"; }
const NS = "http://www.w3.org/2000/svg";
function el(name, attrs, parent) { const e = document.createElementNS(NS, name); for (const k in attrs) e.setAttribute(k, attrs[k]); if (parent) parent.appendChild(e); return e; }
function niceStep(range, n) { const raw = range / n, p = Math.pow(10, Math.floor(Math.log10(raw || 1))), m = raw / p; return (m < 1.5 ? 1 : m < 3.5 ? 2 : m < 7.5 ? 5 : 10) * p; }
// A column with a 4px rounded top, square at the baseline.
function column(g, x, y, w, h) { const r = Math.min(4, w/2, h); return el("path", {d: "M"+x+","+(y+h)+"V"+(y+r)+"Q"+x+","+y+" "+(x+r)+","+y+"H"+(x+w-r)+"Q"+(x+w)+","+y+" "+(x+w)+","+(y+r)+"V"+(y+h)+"Z", class:"bar", tabindex:"0"}, g); }

function histogram(res) {
  const svg = $("hist"); svg.replaceChildren(); $("histt").replaceChildren();
  if (!res || !res.histogram) return;
  $("htitle").textContent = "Cost of the disruption, play by play: " + res.policy;
  const bins = res.histogram, W = 560, H = 220, L = 46, B = 30, T = 10;
  const maxN = Math.max(...bins.map(b => b.n), 1), step = niceStep(maxN, 4), top = Math.ceil(maxN/step)*step;
  for (let v = 0; v <= top; v += step) { const y = T + (H-T-B)*(1 - v/top); el("line", {x1:L, x2:W-6, y1:y, y2:y, class:"grid"}, svg); el("text", {x:L-6, y:y+3, "text-anchor":"end", class:"axis"}, svg).textContent = v; }
  const slot = (W - L - 6) / bins.length, w = Math.min(24, slot - 2);
  bins.forEach((b, i) => {
    const h = (H-T-B) * b.n / top, x = L + i*slot + (slot-w)/2, y = H - B - h;
    if (b.n > 0) { const c = column(svg, x, y, w, h); const head = b.n + " of " + res.trials + " plays", line = money(b.lo) + " to " + money(b.hi);
      c.addEventListener("pointermove", e => showTip(e, head, line)); c.addEventListener("pointerleave", hideTip); c.addEventListener("focus", e => showTip({clientX: x+40, clientY: y+80, target: c}, head, line)); c.addEventListener("blur", hideTip); }
    const tr = document.createElement("tr"); cell(tr, money(b.lo)); cell(tr, money(b.hi)); cell(tr, b.n); $("histt").appendChild(tr);
  });
  for (const i of [0, Math.floor(bins.length/2), bins.length-1]) el("text", {x: L + (i+0.5)*slot, y: H-B+16, "text-anchor":"middle", class:"axis"}, svg).textContent = money(i === bins.length-1 ? bins[i].hi : bins[i].lo);
}

function tornado(p) {
  const svg = $("torn"); svg.replaceChildren(); $("tornt").replaceChildren();
  const sw = (p && p.swings) || [];
  if (!sw.length) { el("text", {x:10, y:20, class:"axis"}, svg).textContent = "run a projection with sensitivity to see what the cost turns on"; return; }
  const list = sw.slice(0, 8), W = 560, L = 170, R = 14, rowH = 24, H = 30 + list.length*rowH;
  svg.setAttribute("viewBox", "0 0 " + W + " " + H);
  const base = p.results[0].baseline.total, mid = p.stress && p.stress.expected ? p.stress.expected.total - base : null;
  let lo = Math.min(...list.map(s => Math.min(s.low, s.high)), mid ?? Infinity), hi = Math.max(...list.map(s => Math.max(s.low, s.high)), mid ?? -Infinity);
  if (hi === lo) hi = lo + 1;
  const X = v => L + (W-L-R) * (v - lo) / (hi - lo);
  const step = niceStep(hi - lo, 4);
  for (let v = Math.ceil(lo/step)*step; v <= hi; v += step) { el("line", {x1:X(v), x2:X(v), y1:10, y2:H-18, class:"grid"}, svg); el("text", {x:X(v), y:H-4, "text-anchor":"middle", class:"axis"}, svg).textContent = money(v); }
  list.forEach((s, i) => {
    const y = 14 + i*rowH, a = X(Math.min(s.low, s.high)), b = X(Math.max(s.low, s.high));
    el("text", {x:L-8, y:y+12, "text-anchor":"end", class:"lbl"}, svg).textContent = s.event + " · " + s.factor;
    const r = el("rect", {x:a, y:y+2, width:Math.max(2, b-a), height:14, rx:3, class:"bar", tabindex:"0"}, svg);
    const head = money(Math.abs(s.high - s.low)) + " swing", line = s.factor + " at its 10th percentile: " + money(s.low) + "; at its 90th: " + money(s.high);
    r.addEventListener("pointermove", e => showTip(e, head, line)); r.addEventListener("pointerleave", hideTip);
    const tr = document.createElement("tr"); cell(tr, s.event + " · " + s.factor); cell(tr, money(s.low)); cell(tr, money(s.high)); $("tornt").appendChild(tr);
  });
  if (mid !== null) { el("line", {x1:X(mid), x2:X(mid), y1:8, y2:H-18, class:"mid"}, svg); el("text", {x:X(mid)+4, y:10, class:"axis"}, svg).textContent = "expected"; }
}

let proj = null, selected = 0;
function showProjection(p) {
  proj = p; if (!p || !p.results) return;
  const bi = Math.max(0, p.results.findIndex(r => r.policy === p.best));
  if (selected >= p.results.length) selected = bi;
  const best = p.results[bi], absorb = p.results[0];
  const tiles = $("tiles"); tiles.replaceChildren();
  tile(tiles, "expected cost, " + best.policy, money(best.cost.mean));
  tile(tiles, "1 day in 20 worse than", money(best.cost.p95), "bad");
  tile(tiles, "the desk recommends", best.policy, "good");
  if (bi > 0) tile(tiles, "against absorbing it (95%)", money(best.delta.mean) + " ± " + money(1.96*best.delta.se));
  tile(tiles, "plays, " + (best.method === "lhs" ? "Latin hypercube" : "Monte Carlo"), best.trials);
  $("pdesc").textContent = "As of " + p.clock + ". " + (p.describe || p.note || "") + (p.note && p.describe ? " " + p.note : "");
  rows($("pols"), p.results, (tr, r) => {
    const i = p.results.indexOf(r); tr.className = (i === selected ? "sel " : "") + (r.policy === p.best ? "best" : ""); tr.style.cursor = "pointer";
    tr.onclick = () => { selected = i; showProjection(proj); };
    cell(tr, r.policy); cell(tr, r.cancels); cell(tr, money(r.cost.mean)); cell(tr, money(r.cost.p50)); cell(tr, money(r.cost.p95));
    cell(tr, i === 0 ? "—" : money(r.delta.mean) + " ± " + money(1.96*r.delta.se));
    cell(tr, Math.round(r.metrics.delay_min.mean)); cell(tr, Math.round(r.metrics.pax_misconnected.mean));
    cell(tr, money(r.metrics.crew.mean + r.metrics.crew_premium.mean)); cell(tr, money(r.metrics.revenue_lost.mean));
  });
  histogram(p.results[selected]);
  tornado(p);
  const st = p.stress || {};
  rows($("stress"), ["expected", "bad", "severe"].filter(k => st[k]), (tr, k) => { const im = st[k]; cell(tr, k + (k === "bad" ? " (P90)" : k === "severe" ? " (P99)" : " (P50)")); cell(tr, money(im.total - absorb.baseline.total)); cell(tr, im.cancelled); cell(tr, im.delay_min); cell(tr, im.timeouts); });
}

async function state() {
  const r = await fetch("/carrier/" + CODE + "/noc"); if (!r.ok) { $("day").textContent = "this world runs no operations centre for " + CODE; return; }
  const v = await r.json();
  $("clock").textContent = v.clock; $("manual").textContent = v.manual ? "· you run this desk" : "· on autopilot";
  const d = $("day"); d.replaceChildren();
  tile(d, "flights", v.day.flights); tile(d, "flown", v.day.flown); tile(d, "late (>15 min)", v.day.late, v.day.late ? "bad" : "");
  tile(d, "cancelled", v.day.cancelled, v.day.cancelled ? "bad" : ""); tile(d, "of them for crew", v.day.crew_cancelled); tile(d, "slotted", v.day.slotted);
  tile(d, "reserves called", v.day.reserves); tile(d, "on the extension", v.day.extended);
  rows($("wx"), v.weather || [], (tr, w) => { cell(tr, w.name); cell(tr, w.airports.join(" "), "wrap"); cell(tr, w.start + "-" + w.end); cell(tr, "×" + w.factor.toFixed(2)); cell(tr, w.state); cell(tr, w.arrivals); });
  rows($("regs"), v.regulations || [], (tr, g) => { cell(tr, g.name); cell(tr, g.window); cell(tr, g.rate); cell(tr, g.cause); cell(tr, g.flights); cell(tr, g.delay_min + "m"); });
  rows($("events"), (v.events || []).slice().reverse(), (tr, e) => cell(tr, (e.source ? e.source + ": " : "") + (e.result || e.kind), "wrap"));
  $("live").textContent = v.live_weather ? "Live weather: " + v.live_weather + "." : "";
  if (v.projection && (!proj || v.projection.at !== proj.at)) showProjection(v.projection);
  const c = v.crew;
  if (c) {
    const k = $("crewk"); k.replaceChildren();
    tile(k, "crew members", c.members.toLocaleString()); tile(k, "duties today", c.duties); tile(k, "reserves called", c.called); tile(k, "relieved part way", c.relieved);
    tile(k, "flying on a day off", c.junior, c.junior ? "bad" : ""); tile(k, "grievances", c.grievances, c.grievances ? "bad" : ""); tile(k, "standby brought in", c.standby);
    tile(k, "crew cost so far", money(c.costs.total));
    rows($("watch"), c.watch || [], (tr, m) => { const t = (m.limits || []).reduce((a, l) => !a || l.remaining_min < a.remaining_min ? l : a, null);
      cell(tr, m.group === "captain" ? "CA " + m.id : "FO " + m.id); cell(tr, m.base); cell(tr, m.fleet); cell(tr, m.status); cell(tr, t ? t.name : ""); cell(tr, t ? Math.floor(t.remaining_min/60) + "h" + String(t.remaining_min%60).padStart(2,"0") : ""); });
    rows($("contracts"), c.contracts || [], (tr, a) => { cell(tr, a.unit); cell(tr, a.status); cell(tr, a.amendable.slice(0,10)); cell(tr, a.proposal ? (a.proposal*100).toFixed(1) + "%" : "—"); cell(tr, (a.sick_rate*100).toFixed(1) + "% base"); });
    rows($("heads"), c.headcount || [], (tr, h) => { cell(tr, h.base); cell(tr, h.fleet); cell(tr, h.group); cell(tr, h.line); cell(tr, h.on_today); cell(tr, h.flying); cell(tr, h.on_call); cell(tr, h.called); cell(tr, h.sick); });
  }
}

let busy = false;
async function project(body, btn) {
  if (busy) return; busy = true; btn.classList.add("busy"); btn.disabled = true; $("pmsg").textContent = "playing the day " + (body.trials||200) + " times under each policy…";
  try {
    const r = await fetch("/carrier/" + CODE + "/noc/project", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(body)});
    const v = await r.json();
    if (!r.ok) { $("pmsg").textContent = v.error || "the projection failed"; return; }
    $("pmsg").textContent = ""; selected = 0; showProjection(v);
  } catch (e) { $("pmsg").textContent = String(e); } finally { busy = false; btn.classList.remove("busy"); btn.disabled = false; }
}
const opts = () => ({trials: parseInt($("trials").value) || 200, method: $("method").value, sensitivity: $("sens").checked});
$("go").onclick = () => project(opts(), $("go"));
$("gotaf").onclick = () => { const t = $("taf").value.split(/[ ,]+/).filter(Boolean).map(s => s.toUpperCase()); if (t.length) project(Object.assign(opts(), {taf: t}), $("gotaf")); };
$("ego").onclick = async () => {
  const b = $("ego"); if (b.disabled) return; b.disabled = true;
  const kind = $("ekind").value, ev = {kind, in: parseInt($("ein").value)||0, duration: parseInt($("edur").value)||180};
  if (kind === "sickout") { ev.carrier = CODE; ev.rate = parseFloat($("erate").value)||0.12; }
  else if (kind === "aog") { ev.flight = $("efl").value; }
  else { ev.airport = $("eap").value; ev.factor = parseFloat($("efac").value)||0.5; }
  try { const r = await fetch("/noc/inject", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(ev)}); const t = await r.text(); let v; try { v = JSON.parse(t); } catch { v = {error: t}; } $("eres").textContent = v.ok || v.error; state(); }
  finally { setTimeout(() => b.disabled = false, 800); }
};
state(); setInterval(state, 10000);
</script>`
