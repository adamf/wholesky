package fleet

import "net/http"

func (c *Collector) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(pageHTML)) //nolint:errcheck
}

// The cluster board: every Jetway system in the world as a live row, and any
// message's actual bytes two clicks away.
const pageHTML = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wholesky — the fleet</title>
<style>
  :root { --bg:#06090d; --panel:#0b1118; --line:#1d2836; --ink:#9fb4c8; --dim:#5b6b7d;
           --hot:#5fd38d; --amber:#e0b93c; --warn:#e05a5a; --plane:#e8eef4; }
  html,body { margin:0; min-height:100%; background:var(--bg); color:var(--ink);
    font:13px/1.45 "SF Mono", ui-monospace, Menlo, monospace; }
  a { color:var(--dim); text-decoration:none; } a:hover { color:var(--ink); }
  #bar { position:sticky; top:0; display:flex; gap:22px; align-items:baseline;
    padding:12px 18px; background:#06090df2; border-bottom:1px solid var(--line); z-index:5; }
  #bar b { color:var(--plane); letter-spacing:.08em; }
  #bar span i { font-style:normal; color:var(--hot); }
  #bar .nav { margin-left:auto; display:flex; gap:16px; }
  #wrap { display:flex; gap:0; }
  #main { flex:1; min-width:0; padding:14px 18px 60px; }
  .cards { display:flex; gap:12px; margin-bottom:14px; flex-wrap:wrap; }
  .card { background:var(--panel); border:1px solid var(--line); border-radius:6px;
    padding:10px 16px; cursor:pointer; min-width:170px; }
  .card:hover { border-color:#2c3d52; }
  .card b { color:var(--plane); font-size:15px; }
  .card .k { color:var(--dim); font-size:11.5px; }
  .card .n { color:var(--hot); }
  #q { width:260px; background:var(--panel); border:1px solid var(--line); color:var(--ink);
    font:inherit; padding:6px 10px; border-radius:5px; margin-bottom:10px; }
  table { border-collapse:collapse; width:100%; font-variant-numeric:tabular-nums; }
  th { text-align:left; color:var(--dim); font-weight:400; font-size:11px; letter-spacing:.08em;
    text-transform:uppercase; padding:6px 14px 6px 0; border-bottom:1px solid var(--line);
    position:sticky; top:46px; background:var(--bg); }
  td { padding:5px 14px 5px 0; border-bottom:1px solid #0e1520; white-space:nowrap; }
  tr.row { cursor:pointer; } tr.row:hover td { background:#0b1118; }
  td.r, th.r { text-align:right; }
  .dot { display:inline-block; width:7px; height:7px; border-radius:50%; margin-right:7px;
    background:var(--hot); vertical-align:1px; }
  .dot.down { background:var(--warn); }
  .fmt { font-size:10.5px; padding:1px 7px; border-radius:9px; letter-spacing:.05em; }
  .fmt.typeb { background:#13251b; color:var(--hot); }
  .fmt.edifact { background:#241f10; color:var(--amber); }
  #drawer { width:0; overflow:hidden; transition:width .15s; background:var(--panel);
    border-left:1px solid var(--line); position:sticky; top:46px; height:calc(100vh - 46px); }
  #drawer.open { width:520px; }
  #drawer .inner { width:520px; padding:14px 16px; height:100%; overflow-y:auto; box-sizing:border-box; }
  #drawer h2 { margin:0; color:var(--plane); font-size:17px; }
  #drawer .sub { color:var(--dim); margin:2px 0 12px; }
  #drawer table td { font-size:12px; }
  #raw { position:fixed; inset:8% 12%; background:#04070a; border:1px solid #2c3d52;
    border-radius:8px; padding:18px 22px; z-index:9; display:none; overflow:auto; }
  #raw pre { color:#cfe3d6; margin:0; white-space:pre-wrap; word-break:break-all; }
  #raw .x { position:absolute; top:10px; right:16px; cursor:pointer; color:var(--dim); }
</style>
<div id="bar">
  <b>WHOLESKY FLEET</b>
  <span>nodes <i id="nnodes">0</i></span>
  <span>links up <i id="nlinks">0</i></span>
  <span>messages <i id="nmsgs">0</i></span>
  <span>rate <i id="nrate">0</i>/s</span>
  <div class="nav"><a href="/eye">globe →</a><a href="/stats">stats →</a><a href="/">switch console →</a></div>
</div>
<div id="wrap">
  <div id="main">
    <div class="cards" id="cards"></div>
    <input id="q" placeholder="filter carriers — code, name, hub">
    <table>
      <thead><tr>
        <th>link</th><th>code</th><th>name</th><th>fmt</th><th>hub</th>
        <th class="r">flights/day</th><th class="r">in</th><th class="r">out</th>
        <th>last</th><th>last kind</th>
      </tr></thead>
      <tbody id="rows"></tbody>
    </table>
  </div>
  <div id="drawer"><div class="inner" id="dinner"></div></div>
</div>
<div id="raw"><span class="x" onclick="raw.style.display='none'">✕ close</span><pre id="rawpre"></pre></div>
<script>
"use strict";
let nodes=[], prevTotal=0, prevAt=Date.now(), sel=null, msgTimer=null;
const rowsEl=document.getElementById("rows"), q=document.getElementById("q");

function fmtAgo(iso){ if(!iso) return "—";
  const s=(Date.now()-Date.parse(iso))/1000;
  return s<2?"now":s<60?(s|0)+"s":s<3600?((s/60)|0)+"m":((s/3600)|0)+"h"; }

async function poll(){
  const r=await fetch("/fleet/nodes.json"); nodes=await r.json();
  const total=nodes.reduce((a,n)=>a+n.in+n.out,0);
  const now=Date.now(), dt=(now-prevAt)/1000;
  document.getElementById("nnodes").textContent=nodes.length;
  document.getElementById("nlinks").textContent=nodes.filter(n=>n.link).length;
  document.getElementById("nmsgs").textContent=total;
  if(prevTotal) document.getElementById("nrate").textContent=Math.max(0,((total-prevTotal)/dt)).toFixed(0);
  prevTotal=total; prevAt=now;
  render();
}
function render(){
  const infra=nodes.filter(n=>n.kind!=="carrier");
  document.getElementById("cards").innerHTML=infra.map(n=>
    "<div class='card' onclick=\"openNode('"+n.code+"')\"><b>"+n.code+"</b> · "+n.name+
    (n.kind==="switch"?" <a href='/' target='_blank' style='color:#5fd38d'>console →</a>"
                      :" <a href='/node/"+n.code+"/' target='_blank' style='color:#5fd38d'>console →</a>")+
    "<div class='k'>in <span class='n'>"+n.in+"</span> · out <span class='n'>"+n.out+
    "</span> · last "+fmtAgo(n.last_at)+"</div></div>").join("");
  const f=q.value.trim().toUpperCase();
  const carriers=nodes.filter(n=>n.kind==="carrier")
    .filter(n=>!f||n.code.includes(f)||n.name.toUpperCase().includes(f)||(n.hub||"").includes(f));
  rowsEl.innerHTML=carriers.map(n=>
    "<tr class='row' onclick=\"openNode('"+n.code+"')\">"+
    "<td><span class='dot"+(n.link?"":" down")+"'></span></td>"+
    "<td style='color:#e8eef4'>"+n.code+"</td><td>"+esc(n.name)+"</td>"+
    "<td><span class='fmt "+n.format+"'>"+n.format+"</span></td>"+
    "<td>"+(n.hub||"")+"</td><td class='r'>"+(n.flights||"")+"</td>"+
    "<td class='r'>"+n.in+"</td><td class='r'>"+n.out+"</td>"+
    "<td>"+fmtAgo(n.last_at)+"</td><td style='color:#5b6b7d'>"+esc(n.last_kind||"")+"</td></tr>").join("");
}
const esc=s=>(s||"").replace(/[<>&]/g,c=>({"<":"&lt;",">":"&gt;","&":"&amp;"}[c]));

async function openNode(code){
  sel=code;
  document.getElementById("drawer").classList.add("open");
  await refreshDrawer();
  clearInterval(msgTimer); msgTimer=setInterval(refreshDrawer,3000);
}
async function refreshDrawer(){
  if(!sel) return;
  const n=nodes.find(x=>x.code===sel); if(!n) return;
  const [msgs,detail]=await Promise.all([
    fetch("/fleet/node/"+sel+"/messages.json").then(r=>r.json()),
    fetch("/fleet/node/"+sel+"/detail.json").then(r=>r.json())]);
  const queues=Object.entries(detail.queues||{}).map(([k,v])=>k+" <span style='color:#5fd38d'>"+v+"</span>").join(" · ")||"—";
  document.getElementById("dinner").innerHTML=
    "<div style='display:flex;justify-content:space-between'><h2>"+n.code+
    " <a href='/node/"+n.code+"/' target='_blank' style='font-size:12px;color:#5fd38d'>open console →</a></h2>"+
    "<a onclick=\"closeDrawer()\" style='cursor:pointer'>✕</a></div>"+
    "<div class='sub'>"+esc(n.name)+(n.hub?" · hub "+n.hub:"")+(n.format?" · "+n.format:"")+
    (n.kind==="carrier"?(n.link
      ?" · <a onclick=\"linkCtl('"+n.code+"','sever')\" style='color:#e05a5a;cursor:pointer'>✂ sever link</a>"
      :" · <a onclick=\"linkCtl('"+n.code+"','restore')\" style='color:#5fd38d;cursor:pointer'>⟳ restore link</a>"):"")+
    "</div>"+
    "<div class='sub'>records <span style='color:#5fd38d'>"+detail.records+"</span> · queues: "+queues+"</div>"+
    "<table><thead><tr><th>at</th><th>dir</th><th>peer</th><th>kind</th><th class='r'>bytes</th><th>status</th></tr></thead><tbody>"+
    msgs.map(m=>"<tr class='row' onclick=\"openRaw('"+sel+"','"+m.id+"')\">"+
      "<td>"+m.at+"</td><td>"+(m.dir==="in"?"◀":"▶")+"</td><td>"+esc(m.peer)+"</td>"+
      "<td>"+esc(m.kind||"—")+"</td><td class='r'>"+m.size+"</td>"+
      "<td style='color:"+(m.status==="undeliverable"?"#e05a5a":"#5b6b7d")+"'>"+m.status+"</td></tr>").join("")+
    "</tbody></table>";
}
function closeDrawer(){ sel=null; clearInterval(msgTimer);
  document.getElementById("drawer").classList.remove("open"); }
async function linkCtl(code,action){
  await fetch("/fleet/node/"+code+"/link",{method:"POST",
    headers:{"Content-Type":"application/json"},body:JSON.stringify({action})});
  await poll(); await refreshDrawer();
}
async function openRaw(code,id){
  const t=await fetch("/fleet/node/"+code+"/message/"+id).then(r=>r.text());
  document.getElementById("rawpre").textContent=t;
  document.getElementById("raw").style.display="block";
}
q.addEventListener("input",render);
poll(); setInterval(poll,2500);
</script>
`
