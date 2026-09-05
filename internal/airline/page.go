package airline

// The pages: a lobby to pick a carrier and the operations centre for one.
// Same palette as the eye; no framework; the API underneath is the one an
// agent uses, so what a person can do here is exactly what a model can.

const styleCSS = `
  :root { --bg:#06090d; --ink:#9fb4c8; --dim:#3d4c5c; --hot:#5fd38d; --warn:#e05a5a; --amber:#e0b93c; --plane:#e8eef4; --line:#1d2836; --card:#0b1118; }
  html,body { margin:0; background:var(--bg); color:var(--ink); font:13px/1.45 "SF Mono", ui-monospace, Menlo, monospace; }
  a { color:#8fa6bc; text-decoration:none; } a:hover { color:var(--plane); }
  header { display:flex; gap:18px; align-items:baseline; padding:12px 18px; border-bottom:1px solid var(--line); flex-wrap:wrap; }
  header b { color:var(--plane); letter-spacing:.08em; font-weight:600; }
  header i { font-style:normal; color:var(--hot); }
  main { padding:14px 18px; display:grid; gap:14px; grid-template-columns: 1.2fr 1fr; }
  @media (max-width:1000px){ main { grid-template-columns:1fr; } }
  section { background:var(--card); border:1px solid var(--line); border-radius:6px; padding:12px 14px; min-width:0; }
  section h2 { margin:0 0 8px; font-size:10px; letter-spacing:.14em; text-transform:uppercase; color:#5b6b7d; font-weight:600; }
  .wide { grid-column: 1 / -1; }
  .kpis { display:grid; grid-template-columns: repeat(auto-fit, minmax(120px,1fr)); gap:10px; }
  .kpi b { display:block; font-size:20px; color:var(--plane); font-weight:600; }
  .kpi span { color:#5b6b7d; font-size:11px; }
  .kpi.bad b { color:var(--warn); } .kpi.good b { color:var(--hot); }
  table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; color:#5b6b7d; font-weight:600; font-size:10px; letter-spacing:.1em; text-transform:uppercase; padding:4px 6px; border-bottom:1px solid var(--line); }
  td { padding:4px 6px; border-bottom:1px solid #111923; vertical-align:top; white-space:nowrap; }
  td.wrap { white-space:normal; color:#7d90a3; font-size:11px; }
  tr.cancelled td { color:#7a4a4a; } tr.departed td { color:#5b6b7d; } tr.late td.etd { color:var(--amber); }
  .scroll { overflow:auto; max-height:60vh; }
  button, input, select { font:inherit; background:#0f1721; color:var(--ink); border:1px solid #223042; border-radius:4px; padding:4px 9px; cursor:pointer; }
  button:hover { color:var(--plane); border-color:#3b4f66; } button.primary { border-color:var(--hot); color:var(--hot); }
  button.danger { border-color:var(--warn); color:var(--warn); } button:disabled { opacity:.4; cursor:default; }
  input { cursor:text; width:110px; }
  .decision { border:1px solid #2a3a2a; border-left:3px solid var(--hot); border-radius:4px; padding:8px 10px; margin:6px 0; background:#0a1410; }
  .decision b { color:var(--plane); } .decision .detail { color:#7d90a3; margin:3px 0 6px; white-space:pre-wrap; }
  .decision .opts { display:flex; gap:6px; flex-wrap:wrap; } .decision .due { float:right; color:#5b6b7d; font-size:11px; }
  .dept { display:flex; justify-content:space-between; align-items:center; gap:10px; padding:5px 0; border-bottom:1px solid #111923; }
  .dept span { color:#7d90a3; font-size:11px; } .dept em { font-style:normal; color:var(--plane); }
  .dept button.on { border-color:var(--amber); color:var(--amber); }
  #tape { font-size:11px; max-height:34vh; overflow:auto; } #tape div { padding:1px 0; border-bottom:1px solid #0e151d; }
  #tape .t { color:#5b6b7d; margin-right:8px; } #tape .k-decision { color:var(--hot); } #tape .k-action { color:var(--amber); } #tape .k-incident { color:var(--warn); }
  .lever { display:flex; gap:6px; align-items:center; flex-wrap:wrap; margin:6px 0; } .lever span { color:#7d90a3; font-size:11px; min-width:70px; }
  .muted { color:#5b6b7d; } .row { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }
  .lobby td a { color:var(--plane); } .lobby tr:hover td { background:#0f1721; }
  .rank { color:#5b6b7d; }
`

const lobbyHTML = `<!doctype html><meta charset="utf-8"><title>wholesky — run a carrier</title>
<meta name="viewport" content="width=device-width,initial-scale=1"><style>` + styleCSS + `</style>
<header><b>WHOLESKY</b> <span>run a carrier</span> <span class="muted">day <i id="clock">--:--</i> · warp <i id="warp">-</i></span>
<span style="margin-left:auto"><a href="/eye">the sky →</a> · <a href="/stats">instruments →</a> · <a href="https://github.com/adamf/wholesky/blob/main/docs/run-a-carrier.md">how this works →</a></span></header>
<main>
<section class="wide"><h2>The bar to beat</h2>
<p class="muted" style="margin:0 0 8px">Every carrier below is flying on autopilot. Take one and its departments become yours: what you leave on autopilot the machine keeps deciding; what you take, it asks you about, and falls to the default if you are slow. The scorecard is the same for everyone. An agent uses the same API (<code>/carrier/XX/…</code>, or <code>skyagent</code> as MCP tools).</p>
<div class="row"><input id="holder" placeholder="your name" style="width:180px"> <span class="muted">then pick a carrier</span></div>
</section>
<section class="wide lobby"><h2>Leaderboard · <span id="n"></span> carriers</h2>
<div class="scroll"><table><thead><tr><th>#</th><th>carrier</th><th>hub</th><th>flights</th><th>flown</th><th>cxl</th><th>OTP</th><th>LF</th><th>revenue</th><th>profit</th><th>score</th><th>seat</th><th></th></tr></thead><tbody id="rows"></tbody></table></div>
</section>
</main>
<script>
const $=s=>document.querySelector(s); const money=c=>"$"+(c/100).toLocaleString(undefined,{maximumFractionDigits:0}); const pct=x=>(100*x).toFixed(0)+"%";
const mmz=m=>String(Math.floor(m/60)%24).padStart(2,"0")+":"+String(Math.floor(m)%60).padStart(2,"0")+"z";
async function load(){
  const d=await fetch("/carriers.json").then(r=>r.json());
  $("#clock").textContent=mmz(d.pos); $("#warp").textContent=d.warp; $("#n").textContent=d.carriers.length;
  $("#rows").innerHTML=d.carriers.map((c,i)=>{ const s=c.score; return "<tr><td class='rank'>"+(i+1)+"</td><td><a href='/ops/"+c.code+"'>"+c.code+"</a> <span class='muted'>"+esc(c.name||"")+"</span></td><td>"+c.hub+"</td><td>"+c.flights+"</td><td>"+s.flown+"</td><td>"+s.cancelled+"</td><td>"+pct(s.otp)+"</td><td>"+pct(s.load_factor)+"</td><td>"+money(s.revenue)+"</td><td>"+money(s.profit)+"</td><td><b>"+s.score.toFixed(1)+"</b></td><td>"+(c.seat?esc(c.seat.holder):"<span class='muted'>autopilot</span>")+"</td><td>"+(c.seat?"":"<button onclick='take(\""+c.code+"\")'>take</button>")+"</td></tr>"; }).join("");
}
function esc(s){ return String(s).replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c])); }
async function take(code){
  const holder=$("#holder").value.trim(); if(!holder){ $("#holder").focus(); return; }
  const r=await fetch("/carrier/"+code+"/take",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({holder})});
  const d=await r.json(); if(!r.ok){ alert(d.error); return; }
  try{ localStorage.setItem("seat:"+code, d.token); localStorage.setItem("holder", holder); }catch(e){}
  location.href="/ops/"+code;
}
try{ $("#holder").value=localStorage.getItem("holder")||""; }catch(e){}
load(); setInterval(load, 10000);
</script>`

const opsHTML = `<!doctype html><meta charset="utf-8"><title>{{CARRIER}} — operations centre</title>
<meta name="viewport" content="width=device-width,initial-scale=1"><style>` + styleCSS + `</style>
<header><b>WHOLESKY</b> <span><a href="/ops/">carriers</a> / <b id="code">{{CARRIER}}</b> <span id="name" class="muted"></span></span>
<span class="muted">day <i id="clock">--:--</i> · warp <i id="warp">-</i></span>
<span id="seat" class="muted"></span>
<span style="margin-left:auto" class="row"><button id="takebtn" class="primary">take the seat</button><button id="relbtn" class="danger" hidden>release</button> <a href="/node/{{CARRIER}}/" target="_blank">the carrier's console →</a> <a href="/eye">the sky →</a></span></header>
<main>
<section class="wide"><h2>Scorecard</h2><div class="kpis" id="kpis"></div><div class="muted" id="costs" style="margin-top:8px;font-size:11px"></div></section>
<section><h2>Decisions <span id="inboxn" class="muted"></span></h2><div id="inbox"><div class="muted">Nothing open. Take a department off autopilot and the day will start asking.</div></div></section>
<section><h2>Departments</h2><div id="depts"></div>
<h2 style="margin-top:14px">Levers</h2>
<div class="lever"><span>fares</span><input id="mult" type="number" step="0.05" min="0" placeholder="1.00"> <button onclick="act({kind:'fares',multiplier:+$('#mult').value})">set multiplier</button> <span class="muted">over the filing; 0 restores it</span></div>
<div class="lever"><span>flight</span><input id="fl" placeholder="{{CARRIER}}0117" style="width:90px"> <input id="bd" placeholder="LHR" style="width:52px"></div>
<div class="lever"><span></span><input id="mins" type="number" placeholder="minutes" style="width:80px"> <button onclick="act({kind:'retime',flight:$('#fl').value,board:$('#bd').value,minutes:+$('#mins').value})">retime</button>
 <button onclick="act({kind:'substitute',flight:$('#fl').value,board:$('#bd').value})">substitute aircraft</button>
 <button onclick="act({kind:'ready',flight:$('#fl').value,board:$('#bd').value})">ask for a better slot</button>
 <button onclick="act({kind:'reserves',flight:$('#fl').value,board:$('#bd').value})">call reserves</button>
 <button class="danger" onclick="if(confirm('Cancel '+$('#fl').value+'?')) act({kind:'cancel',flight:$('#fl').value,board:$('#bd').value,reason:'cancelled by operations'})">cancel</button></div>
<div class="lever"><span>class</span><input id="cls" placeholder="K" style="width:40px"> <button onclick="act({kind:'class',flight:$('#fl').value,board:$('#bd').value,class:$('#cls').value,status:'C'})">close on flight</button> <button onclick="act({kind:'class',flight:$('#fl').value,board:$('#bd').value,class:$('#cls').value,status:''})">back to the ladder</button></div>
<div id="actresult" class="muted"></div>
</section>
<section class="wide"><h2>Departures today <span id="fln" class="muted"></span></h2>
<div class="scroll"><table><thead><tr><th>flight</th><th>sector</th><th>STD</th><th>ETD</th><th>status</th><th>booked/seats</th><th>revenue</th><th>the day</th></tr></thead><tbody id="flights"></tbody></table></div></section>
<section class="wide"><h2>Tape</h2><div id="tape"></div></section>
</main>
<script>
const CODE="{{CARRIER}}"; const $=s=>document.querySelector(s);
const money=c=>"$"+(c/100).toLocaleString(undefined,{maximumFractionDigits:0}); const pct=x=>(100*x).toFixed(0)+"%";
const mmz=m=>String(Math.floor(m/60)%24).padStart(2,"0")+":"+String(Math.floor(m)%60).padStart(2,"0")+"z";
function esc(s){ return String(s==null?"":s).replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c])); }
let token=""; try{ token=localStorage.getItem("seat:"+CODE)||""; }catch(e){}
const hdr=()=>({"content-type":"application/json","X-Seat-Token":token});
let state=null;
async function load(){
  state=await fetch("/carrier/"+CODE+"/state").then(r=>r.json());
  $("#clock").textContent=mmz(state.pos); $("#warp").textContent=state.warp;
  const s=state.score, seat=state.seat, mine=seat&&token;
  $("#seat").textContent=seat?("seat: "+seat.holder+" · answered "+seat.answered+" · defaulted "+seat.defaulted):"autopilot";
  $("#takebtn").hidden=!!seat; $("#relbtn").hidden=!mine;
  const k=(v,l,cls)=>"<div class='kpi "+(cls||"")+"'><b>"+v+"</b><span>"+l+"</span></div>";
  $("#kpis").innerHTML=k(s.score.toFixed(1),"score")+k(money(s.profit),"profit",s.profit<0?"bad":"good")+k(money(s.revenue),"revenue")+k(money(s.cost),"cost")+k(pct(s.otp),"on time (D15)",s.otp<0.8?"bad":"good")+k(s.flown+"/"+s.flights,"flown")+k(s.cancelled,"cancelled",s.cancelled?"bad":"")+k(pct(s.load_factor),"load factor")+k(s.delay_min+"m","delay minutes")+k(s.bags.Mishandled,"bags mishandled")+k(s.slots,"slots")+k(s.reserves,"reserve callouts");
  $("#costs").textContent="costs: "+Object.entries(s.costs||{}).map(([k,v])=>k+" "+money(v)).join(" · ");
  const inbox=state.inbox||[]; $("#inboxn").textContent=inbox.length?"· "+inbox.length+" open":"";
  if(inbox.length) $("#inbox").innerHTML=inbox.map(d=>"<div class='decision'><span class='due'>"+d.department+" · due "+new Date(d.deadline).toLocaleTimeString()+"</span><b>"+esc(d.title)+"</b><div class='detail'>"+esc(d.detail)+"</div><div class='opts'>"+d.options.map(o=>"<button "+(mine?"":"disabled ")+"onclick='decide(\""+d.id+"\",\""+o.key+"\")' class='"+(o.key===d.default?"primary":"")+"'>"+esc(o.label)+(o.cost?" <span class=muted>"+money(o.cost)+"</span>":"")+(o.key===d.default?" (default)":"")+"</button>").join("")+"</div></div>").join("");
  else $("#inbox").innerHTML="<div class='muted'>Nothing open."+(mine?" Take a department off autopilot and the day will start asking.":"")+"</div>";
  $("#depts").innerHTML=(state.departments||[]).map(d=>{ const on=seat&&seat.manual&&seat.manual[d.key]; return "<div class='dept'><div><em>"+esc(d.name)+"</em><br><span>"+esc(d.about)+"</span></div><button "+(mine?"":"disabled ")+"class='"+(on?"on":"")+"' onclick='dept(\""+d.key+"\","+(!on)+")'>"+(on?"manual":"autopilot")+"</button></div>"; }).join("");
  const fl=state.flights||[]; $("#fln").textContent="· "+fl.length;
  $("#flights").innerHTML=fl.map(f=>{ const day=[f.delay,f.slot,f.crew,f.retimed,f.substituted,f.rushed,f.cancelled].filter(Boolean).join(" · "); return "<tr class='"+f.status+(f.delay_min>=15?" late":"")+"'><td><a href='#' onclick='pick(\""+f.flight+"\",\""+f.from+"\");return false'>"+f.flight+"</a></td><td>"+f.from+"–"+f.to+"</td><td>"+f.std+"</td><td class='etd'>"+f.etd+(f.delay_min?" (+"+f.delay_min+")":"")+"</td><td>"+f.status+"</td><td>"+f.booked+"/"+f.seats+(f.boarded?" · "+f.boarded+" boarded":"")+"</td><td>"+money(f.revenue)+"</td><td class='wrap'>"+esc(day)+"</td></tr>"; }).join("");
}
function pick(f,b){ $("#fl").value=f; $("#bd").value=b; }
async function dept(key,manual){ const r=await fetch("/carrier/"+CODE+"/departments",{method:"POST",headers:hdr(),body:JSON.stringify({department:key,manual})}); if(!r.ok) alert((await r.json()).error); load(); }
async function decide(id,option){ const r=await fetch("/carrier/"+CODE+"/decide",{method:"POST",headers:hdr(),body:JSON.stringify({id,option})}); if(!r.ok) alert((await r.json()).error); load(); }
async function act(a){ a.flight=(a.flight||"").toUpperCase(); a.board=(a.board||"").toUpperCase(); const r=await fetch("/carrier/"+CODE+"/act",{method:"POST",headers:hdr(),body:JSON.stringify(a)}); const d=await r.json(); $("#actresult").textContent=(r.ok?"✓ ":"✕ ")+(d.result||d.error||""); load(); }
$("#takebtn").onclick=async()=>{ let holder=""; try{ holder=localStorage.getItem("holder")||""; }catch(e){} holder=prompt("Your name", holder)||""; if(!holder) return; const r=await fetch("/carrier/"+CODE+"/take",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({holder})}); const d=await r.json(); if(!r.ok){ alert(d.error); return; } token=d.token; try{ localStorage.setItem("seat:"+CODE, token); localStorage.setItem("holder", holder);}catch(e){} load(); };
$("#relbtn").onclick=async()=>{ if(!confirm("Hand "+CODE+" back to the autopilot?")) return; await fetch("/carrier/"+CODE+"/release",{method:"POST",headers:hdr()}); token=""; try{ localStorage.removeItem("seat:"+CODE);}catch(e){} load(); };
const es=new EventSource("/carrier/"+CODE+"/events");
es.onmessage=e=>{ const ev=JSON.parse(e.data); const t=$("#tape"); const div=document.createElement("div"); div.className="k-"+ev.kind; div.innerHTML="<span class='t'>"+new Date(ev.at).toLocaleTimeString()+"</span>"+esc(ev.text); t.prepend(div); while(t.children.length>200) t.lastChild.remove(); if(ev.kind==="decision"||ev.kind==="decided"||ev.kind==="seat") load(); };
load(); setInterval(load, 5000);
</script>`
