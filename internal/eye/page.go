package eye

import "net/http"

func (e *Eye) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(pageHTML)) //nolint:errcheck
}

// The scope. One canvas, no dependencies. A world-spanning manifest renders as
// an orthographic globe -- drag to spin it, scroll to zoom, and the continents
// draw themselves from airport density alone; a regional manifest renders
// flat. Aircraft are flown by movement messages, halos are queue items, and
// clicking an airport offers the one control the sky answers to.
const pageHTML = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wholesky — the eye</title>
<style>
  :root { --bg:#06090d; --ink:#9fb4c8; --dim:#3d4c5c; --hot:#5fd38d; --warn:#e05a5a; --plane:#e8eef4; }
  html,body { margin:0; height:100%; background:var(--bg); color:var(--ink);
    font:13px/1.4 "SF Mono", ui-monospace, Menlo, monospace; overflow:hidden; }
  #bar { position:fixed; top:0; left:0; right:0; display:flex; gap:16px; align-items:baseline;
    padding:10px 16px; background:linear-gradient(#06090df2,#06090d00); z-index:2; pointer-events:none;
    white-space:nowrap; flex-wrap:nowrap; overflow:hidden; }
  @media (max-width:1100px){ #bar .tot { display:none; } }
  #bar b { color:var(--plane); font-weight:600; letter-spacing:.08em; }
  #bar span i { font-style:normal; color:var(--hot); }
  #bar .qps i { font-size:16px; }
  #bar .tot { color:#5b6b7d; } #bar .tot i { color:#8fa6bc; }
  #bar a { color:#8fa6bc; text-decoration:none; pointer-events:auto; }
  #bar a.nav:first-of-type { margin-left:auto; }
  #bar a:hover { color:#e8eef4; }
  #simctl a { padding:0 3px; } #simctl a.on { color:var(--hot); }
  #legend { position:fixed; right:16px; top:44px; z-index:3; background:#0b1118ee;
    border:1px solid #1d2836; border-radius:6px; padding:10px 14px; max-width:290px;
    font-size:12px; line-height:1.55; }
  #legend b { color:var(--plane); }
  #legend .k { display:inline-block; width:14px; text-align:center; }
  #legend .x { float:right; cursor:pointer; color:#5b6b7d; } #legend .x:hover { color:#e8eef4; }
  canvas { display:block; width:100vw; height:100vh; cursor:grab; }
  canvas.drag { cursor:grabbing; }
  #panel { position:fixed; left:16px; bottom:16px; background:#0b1118ee; border:1px solid #1d2836;
    border-radius:6px; padding:12px 14px; z-index:3; display:none; min-width:210px; }
  #panel b { color:var(--plane); font-size:15px; }
  #panel .sub { color:var(--dim); margin:2px 0 10px; }
  #panel button { font:inherit; padding:5px 14px; border-radius:4px; border:1px solid #2a3a4e;
    background:#101a26; color:var(--ink); cursor:pointer; }
  #panel button.close { border-color:#5a2a2a; color:#e08a8a; }
  #panel button:hover { filter:brightness(1.3); }
  #closedlist { position:fixed; right:16px; bottom:16px; z-index:2; text-align:right; }
  #closedlist div { color:var(--warn); margin-top:4px; }
</style>
<div id="bar">
  <b>WHOLESKY</b>
  <span id="modes" style="pointer-events:auto"><a id="m-sky" style="color:#e8eef4">sky</a> · <a id="m-net">net</a></span>
  <span id="netmodes" style="pointer-events:auto;display:none"><a id="l-web" style="color:#e8eef4">web</a> · <a id="l-ring">ring</a></span>
  <span class="qps" title="live messages per second crossing the switch">msg/s <i id="qps">–</i></span>
  <span title="aircraft currently in the air, flown by movement messages">airborne <i id="air">0</i></span>
  <span id="simctl" title="the sim day's clock and speed; reservations keep flowing while paused">
    day <i id="simclk">--:--</i>
    <a id="w0" title="pause the day">⏸</a><a id="w60" title="one hour per minute">▶</a><a id="w300" title="five hours per minute">▶▶</a><a id="w600" title="ten hours per minute">▶▶▶</a></span>
  <span class="tot" title="running totals since this world booted">since boot — msgs <i id="msg">0</i> · mvts <i id="mvt">0</i> · bkgs <i id="bkg">0</i></span>
  <a class="nav" id="legendlink">what is this? →</a>
  <a class="nav" href="/fleet">fleet →</a>
  <a class="nav" href="/stats">stats →</a>
  <a class="nav" href="/">console →</a>
</div>
<div id="legend" style="display:none"></div>
<div id="panel"></div>
<div id="closedlist"></div>
<canvas id="c"></canvas>
<script>
"use strict";
const cv=document.getElementById("c"), cx=cv.getContext("2d");
const panel=document.getElementById("panel"), closedEl=document.getElementById("closedlist");
let W=0,H=0,DPR=1;
function size(){ DPR=devicePixelRatio||1; W=innerWidth; H=innerHeight;
  cv.width=W*DPR; cv.height=H*DPR; cx.setTransform(DPR,0,0,DPR,0,0); }
addEventListener("resize",()=>{ size(); edgesDirty=true; settleFrames=0; }); size();

let world=null, globe=false, anchor=null, mode="sky", land=null;
fetch("/eye/land.json").then(r=>r.json()).then(l=>{ land=l; });
const rates=new Map(); let netPos=null;
let netLayout="web";
const edgesE=new Map();   // "SRC>DST" -> {w} smoothed conversation weight
const flows=[];           // sampled conversations in flight
const phys=new Map();     // code -> {x,y,vx,vy} for the web layout
let physSeeded=false;
const planes=new Map(), pulses=[], blips=[];
let prevMsgTotal=null, prevMsgAt=0, qpsEMA=null;
let simWarp=null, pausedMs=0, lastFrameW=0;
let edgesDirty=true, settleFrames=0;
let netZoom=1, netPanX=0, netPanY=0;
const fmtN=n=> n>=1e6?(n/1e6).toFixed(1)+"m" : n>=1e4?(n/1e3).toFixed(0)+"k" : n;

function setWarpUI(w){
  if(simWarp===0 && w>0 && pausedMs>0){
    for(const p of planes.values()){
      if(p._dep!==undefined){ p._dep+=pausedMs; p._arr+=pausedMs; }
    }
    pausedMs=0;
  }
  simWarp=w;
  for(const id of ["w0","w60","w300","w600"])
    document.getElementById(id).classList.toggle("on", "w"+w===id);
}
for(const w of [0,60,300,600]){
  document.getElementById("w"+w).onclick=()=>{
    fetch("/eye/time",{method:"POST",headers:{"Content-Type":"application/json"},
      body:JSON.stringify({warp:w})}).then(()=>setWarpUI(w));
  };
}

/* The explainer. Shown until dismissed once; the link brings it back. */
const LEGEND_SKY=
  "<span class='x' onclick='hideLegend()'>✕</span><b>the sky</b><br>"+
  "Every mark is caused by a real message on the wire; nothing is animated for show.<br>"+
  "<span class='k' style='color:#e8eef4'>➤</span> an aircraft — it departs and lands because its carrier's MVT movement messages crossed the switch<br>"+
  "<span class='k' style='color:#e0b93c'>➤</span> a diverted aircraft (a DIV message named its alternate)<br>"+
  "<span class='k' style='color:#5fd38d'>·</span> sparks — messages travelling to and from a carrier's home: <i style='color:#5fd38d'>green</i> reservations · <i style='color:#e0b93c'>amber</i> movements · <i style='color:#e05a5a'>red</i> schedule<br>"+
  "<span class='k' style='color:#e8eef4'>◌</span> an expanding ring — an arrival<br>"+
  "<span class='k' style='color:#e05a5a'>◎</span> a red halo — a closed airport; its size counts real queue items: bookings an agent now has to rework<br>"+
  "<b>click</b> an airport to close or reopen it · <b>click</b> an aircraft to see the bookings riding it · drag to spin, scroll to zoom";
const LEGEND_NET=
  "<span class='x' onclick='hideLegend()'>✕</span><b>the logical web</b><br>"+
  "The same world as a network diagram: systems that talk, not places.<br>"+
  "<span class='k' style='color:#5fd38d'>●</span> a distribution system (GDS)<br>"+
  "<span class='k' style='color:#5b7a95'>●</span> a carrier, sized by how many conversations it is holding; quiet carriers park on the outer ring<br>"+
  "<span class='k' style='color:#82afd7'>—</span> a conversation — bright lines are carrier↔carrier interline, faint ones lead to a GDS<br>"+
  "<span class='k'>·</span> moving dots — sampled real messages riding their edges: <i style='color:#5fd38d'>green</i> reservations · <i style='color:#e0b93c'>amber</i> movements · <i style='color:#5f96be'>blue</i> availability · <i style='color:#e05a5a'>red</i> schedule<br>"+
  "<b>drag</b> to pan · <b>scroll</b> to zoom · <b>click</b> a node for its console";
function showLegend(){
  document.getElementById("legend").innerHTML = mode==="net"?LEGEND_NET:LEGEND_SKY;
  document.getElementById("legend").style.display="block";
}
function hideLegend(){
  document.getElementById("legend").style.display="none";
  try{ localStorage.setItem("eye-legend-seen","1"); }catch(e){}
}
document.getElementById("legendlink").onclick=showLegend;
let seen=false; try{ seen=!!localStorage.getItem("eye-legend-seen"); }catch(e){}
if(!seen) addEventListener("load",showLegend);
const closed=new Map(); // iata -> halo count
const D=Math.PI/180;

/* view state */
let rot={lam:10*D, phi:-45*D};      // globe rotation (centre lon/lat)
let zoom=1, auto=true, flatFit=null;

function span(apts){
  let lo=180,hi=-180;
  for(const a of apts){ lo=Math.min(lo,a.lon); hi=Math.max(hi,a.lon); }
  return hi-lo;
}
function fitFlat(apts){
  let lo=[90,180],hi=[-90,-180];
  for(const a of apts){ lo[0]=Math.min(lo[0],a.lat); lo[1]=Math.min(lo[1],a.lon);
    hi[0]=Math.max(hi[0],a.lat); hi[1]=Math.max(hi[1],a.lon); }
  const mid=(lo[0]+hi[0])/2, k=Math.cos(mid*D);
  const pad=0.07, sX=(hi[1]-lo[1])*k, sY=hi[0]-lo[0];
  const s=Math.min(W*(1-2*pad)/sX,(H-60)*(1-2*pad)/sY);
  const ox=(W-sX*s)/2, oy=(H-sY*s)/2+20;
  return (lat,lon)=>[ox+(lon-lo[1])*k*s, oy+(hi[0]-lat)*s, 1];
}
/* orthographic: returns [x,y,visible] */
function projGlobe(lat,lon){
  const R=Math.min(W,H-40)*0.46*zoom;
  const phi=lat*D, lam=lon*D;
  const cosc=Math.sin(-rot.phi)*Math.sin(phi)+Math.cos(-rot.phi)*Math.cos(phi)*Math.cos(lam-rot.lam);
  const x=R*Math.cos(phi)*Math.sin(lam-rot.lam);
  const y=R*(Math.cos(-rot.phi)*Math.sin(phi)-Math.sin(-rot.phi)*Math.cos(phi)*Math.cos(lam-rot.lam));
  return [W/2+x, H/2+20-y, cosc>0?1:0];
}
const proj=(lat,lon)=> globe?projGlobe(lat,lon):flatFit(lat,lon);

let HUBS=new Set(["1G"]);
fetch("/eye/world.json").then(r=>r.json()).then(w=>{
  world=w;
  if(w.hubs&&w.hubs.length) HUBS=new Set(w.hubs);
  globe=span(w.airports)>150;
  flatFit=fitFlat(w.airports);
  const lhr=w.airports.find(a=>a.iata==="LHR");
  anchor=lhr?[lhr.lat,lhr.lon]:[w.airports[0].lat,w.airports[0].lon];
  return fetch("/eye/planes.json");
}).then(r=>r.json()).then(ps=>{ for(const p of ps) planes.set(p.flight,p); connect(); });

function connect(){
  const es=new EventSource("/eye/stream");
  es.onmessage=ev=>{
    const d=JSON.parse(ev.data);
    if(d.t==="dep") planes.set(d.plane.flight,d.plane);
    else if(d.t==="arr"){ const p=planes.get(d.flight);
      if(p){ const q=proj(p.to_lat,p.to_lon); if(q[2]) blips.push({x:q[0],y:q[1],t:now()}); }
      planes.delete(d.flight); }
    else if(d.t==="pulse"){ if(pulses.length<500) pulses.push({peer:d.peer,dir:d.dir,kind:d.kind,t:now()}); }
    else if(d.t==="flow"){ if(flows.length<300) flows.push({src:d.src,dst:d.dst,kind:d.kind,t:now()}); }
    else if(d.t==="close"){ closed.set(d.airport, closed.get(d.airport)||0); renderClosed(); }
    else if(d.t==="halo"){ closed.set(d.airport, d.count); renderClosed(); }
    else if(d.t==="reopen"){ closed.delete(d.airport); renderClosed(); }
    else if(d.t==="stats"){ air.textContent=d.airborne; mvt.textContent=fmtN(d.movements);
      bkg.textContent=fmtN(d.bookings); msg.textContent=fmtN(d.messages);
      /* live rate: the delta between two-second snapshots, lightly smoothed */
      const nowW=Date.now();
      if(prevMsgTotal!==null && nowW>prevMsgAt){
        const r=Math.max(0,(d.messages-prevMsgTotal)/((nowW-prevMsgAt)/1000));
        qpsEMA = qpsEMA===null ? r : qpsEMA*0.4+r*0.6;
        qps.textContent=qpsEMA.toFixed(0);
      }
      prevMsgTotal=d.messages; prevMsgAt=nowW;
      if(d.sim!==undefined) simclk.textContent=d.sim;
      if(d.warp!==undefined) setWarpUI(d.warp);
      if(d.rates){ rates.clear(); for(const [p,n] of Object.entries(d.rates)) rates.set(p,n/2); }
      if(d.edges){
        /* Conversations pulse -- an AVS cycle every minute, bookings in
           bursts -- so the web remembers minutes, not seconds: a slow decay
           with fresh counts folded on top. Keys are parsed once, here, so
           the render loop never splits a string. */
        for(const v of edgesE.values()) v.w*=0.96;
        for(const [k,n] of Object.entries(d.edges)){
          let e=edgesE.get(k);
          if(!e){ const gt=k.indexOf(">"); e={w:0,a:k.slice(0,gt),b:k.slice(gt+1)}; edgesE.set(k,e); }
          e.w=Math.min(60,Math.max(e.w,n));
        }
        for(const [k,v] of edgesE) if(v.w<0.08) edgesE.delete(k);
        edgesDirty=true; settleFrames=0;
      }
      if(d.closed){ closed.clear(); for(const [a,n] of Object.entries(d.closed)) closed.set(a,n); renderClosed(); } }
  };
  es.onerror=()=>{ es.close(); setTimeout(connect,2000); };
}

function renderClosed(){
  closedEl.innerHTML=[...closed.entries()]
    .map(([a,n])=>"<div>"+a+" CLOSED · "+n+" bookings queued</div>").join("");
}

const now=()=>performance.now();
const hubPos=code=>{
  if(!world) return null;
  if(HUBS.has(code)) return anchor;
  const c=(world.carriers||[]).find(c=>c.code===code);
  return c?[c.lat,c.lon]:null;
};

document.getElementById("m-sky").onclick=()=>setMode("sky");
document.getElementById("m-net").onclick=()=>setMode("net");
document.getElementById("l-web").onclick=()=>setNetLayout("web");
document.getElementById("l-ring").onclick=()=>setNetLayout("ring");
function setMode(m){ mode=m; panel.style.display="none";
  document.getElementById("m-sky").style.color=m==="sky"?"#e8eef4":"#3d4c5c";
  document.getElementById("m-net").style.color=m==="net"?"#e8eef4":"#3d4c5c";
  document.getElementById("netmodes").style.display=m==="net"?"":"none";
  settleFrames=0;
  if(document.getElementById("legend").style.display!=="none") showLegend(); }
function setNetLayout(l){ netLayout=l; settleFrames=0;
  document.getElementById("l-web").style.color=l==="web"?"#e8eef4":"#3d4c5c";
  document.getElementById("l-ring").style.color=l==="ring"?"#e8eef4":"#3d4c5c"; }

/* net layout: the star the network actually is. Carriers on a ring ordered by
   hub longitude (the globe's geography, flattened), the switch at the centre,
   the GDS on its own short spoke. */
function layoutNet(){
  if(!world) return null;
  const cxp=W/2, cyp=(H+30)/2, R=Math.min(W,H-60)*0.40;
  const cs=[...(world.carriers||[])].sort((a,b)=>a.lon-b.lon);
  const pos=new Map();
  cs.forEach((c,i)=>{
    const a=-Math.PI/2 + i/cs.length*2*Math.PI;
    pos.set(c.code,[cxp+R*Math.cos(a), cyp+R*Math.sin(a)]);
  });
  pos.set("1X",[cxp,cyp]);
  const hubs=[...HUBS];
  hubs.forEach((h,i)=>{
    const a=Math.PI/2 + (i-(hubs.length-1)/2)*0.5;
    pos.set(h,[cxp+R*0.45*Math.cos(a), cyp+R*0.45*Math.sin(a)]);
  });
  return pos;
}

/* interaction: drag to rotate the globe (pan does nothing flat), wheel zooms, click selects */
let drag=null;
cv.addEventListener("mousedown",e=>{ drag={x:e.clientX,y:e.clientY,moved:false}; cv.classList.add("drag"); });
addEventListener("mousemove",e=>{
  if(!drag) return;
  const dx=e.clientX-drag.x, dy=e.clientY-drag.y;
  if(Math.abs(dx)+Math.abs(dy)>3) drag.moved=true;
  if(mode==="net"){ netPanX+=dx; netPanY+=dy; }
  else if(globe){ auto=false;
    rot.lam-=dx/(140*zoom); rot.phi+=dy/(140*zoom);
    rot.phi=Math.max(-1.4,Math.min(1.4,rot.phi)); }
  drag.x=e.clientX; drag.y=e.clientY;
});
addEventListener("mouseup",e=>{
  cv.classList.remove("drag");
  if(drag&&!drag.moved) pick(e.clientX,e.clientY);
  drag=null;
});
cv.addEventListener("wheel",e=>{ e.preventDefault(); auto=false;
  if(mode==="net"){
    const f=e.deltaY<0?1.12:0.9;
    const nz=Math.max(.4,Math.min(8,netZoom*f)), r=nz/netZoom;
    // zoom about the cursor, so the point under it stays put
    netPanX=e.clientX-(e.clientX-netPanX)*r;
    netPanY=e.clientY-(e.clientY-netPanY)*r;
    netZoom=nz;
    return;
  }
  zoom=Math.max(.6,Math.min(6,zoom*(e.deltaY<0?1.12:0.9))); },{passive:false});

function planeAt(x,y){
  const nowMs=Date.now()-pausedMs;
  let best=null,bd=11*11;
  for(const p of planes.values()){
    if(p._dep===undefined){ p._dep=Date.parse(p.departed); p._arr=Date.parse(p.arriving); }
    const dep=p._dep, arr=p._arr;
    let f=(nowMs-dep)/Math.max(1,arr-dep); f=Math.max(0,Math.min(1,f));
    const lat=p.from_lat+(p.to_lat-p.from_lat)*f, lon=p.from_lon+(p.to_lon-p.from_lon)*f;
    const q=proj(lat,lon); if(!q[2]) continue;
    const d=(q[0]-x)**2+(q[1]-y)**2;
    if(d<bd){ bd=d; best=p; }
  }
  return best;
}
async function showFlight(p){
  panel.innerHTML="<b>"+p.flight+"</b><div class='sub'>"+p.reg+" · "+p.from+" → "+p.to+
    (p.diverted?" · <span style='color:#e0b93c'>DIVERTED</span>":"")+"</div>"+
    "<div class='sub'>fetching the souls on board…</div>";
  panel.style.display="block";
  let recs=[], dcs=null;
  try{ const d=await fetch("/eye/flight/"+p.flight).then(r=>r.json()); recs=d.records||[]; dcs=d.dcs||null; }catch(e){}
  const c=dcs&&dcs.counts;
  const ground=dcs? "<div class='sub' style='margin-top:4px;color:#e8eef4'>departure control · "+dcs.state.replace("_"," ")+
    "</div><div class='sub'>"+(c.accepted+c.boarded)+" accepted · "+c.boarded+" boarded"+
    (c.noshow?" · "+c.noshow+" no-show":"")+(c.standby?" · "+c.standby+" standby":"")+
    " · "+c.bags+" bags "+c.bag_kilos+"kg · "+c.seats+" seats"+(dcs.alerts?" · ⚠ "+dcs.alerts:"")+
    " · <a href='/node/"+p.flight.slice(0,2)+"/' target='_blank' style='color:#5fd38d'>departures board</a></div>" : "";
  const rows=recs.slice(0,14).map(r=>
    "<div class='sub'><a href='/node/"+r.gds+"/' target='_blank' style='color:#5fd38d'>"+r.locator+
    "</a> "+r.surname+(r.party>1?" ×"+r.party:"")+" · "+r.status+" · "+r.gds+"</div>").join("");
  panel.innerHTML="<b>"+p.flight+"</b><div class='sub'>"+p.reg+" · "+p.from+" → "+p.to+
    (p.diverted?" · <span style='color:#e0b93c'>DIVERTED</span>":"")+"</div>"+
    ground+
    (recs.length? "<div class='sub' style='margin-top:4px'>"+recs.length+" records on board</div>"+rows
                : "<div class='sub'>no records on this flight</div>");
}
function pick(x,y){
  if(!world) return;
  if(mode==="net"){ pickNode(x,y); return; }
  const pl=planeAt(x,y);
  if(pl){ showFlight(pl); return; }
  let best=null,bd=12*12;
  for(const a of world.airports){
    const p=proj(a.lat,a.lon); if(!p[2]) continue;
    const d=(p[0]-x)**2+(p[1]-y)**2;
    if(d<bd){ bd=d; best=a; }
  }
  if(!best){ panel.style.display="none"; return; }
  const isClosed=closed.has(best.iata);
  panel.innerHTML="<b>"+best.iata+"</b><div class='sub'>"+best.n+" daily flight movements"+
    (isClosed?" · <span style='color:#e05a5a'>CLOSED</span>":"")+"</div>"+
    "<button class='"+(isClosed?"":"close")+"' onclick=\"chaos('"+(isClosed?"reopen":"close")+"','"+best.iata+"')\">"+
    (isClosed?"REOPEN "+best.iata:"CLOSE "+best.iata)+"</button>";
  panel.style.display="block";
}
function pickNode(x,y){
  if(!netPos) return;
  x=(x-netPanX)/netZoom; y=(y-netPanY)/netZoom;
  let best=null,bd=14*14;
  for(const [code,p] of netPos){
    const d=(p[0]-x)**2+(p[1]-y)**2;
    if(d<bd){ bd=d; best=code; }
  }
  if(!best){ panel.style.display="none"; return; }
  const c=(world.carriers||[]).find(c=>c.code===best);
  const name=best==="1X"?"the switch":HUBS.has(best)?best+" · gds":best;
  const rate=(rates.get(best)||0).toFixed(1);
  const consoleHref=best==="1X"?"/":"/node/"+best+"/";
  panel.innerHTML="<b>"+name+"</b><div class='sub'>"+rate+" msg/s on this link</div>"+
    "<a href='"+consoleHref+"' target='_blank' style='color:#5fd38d;pointer-events:auto'>open console →</a> · "+
    "<a href='/fleet' target='_blank' style='color:#3d4c5c;pointer-events:auto'>fleet</a>";
  panel.style.display="block";
}
window.chaos=(action,iata)=>{
  fetch("/eye/chaos",{method:"POST",headers:{"content-type":"application/json"},
    body:JSON.stringify({action,airport:iata})}).then(()=>{ panel.style.display="none"; });
};

let drawNet=null;
function draw(){
  requestAnimationFrame(draw);
  if(!world) return;
  cx.clearRect(0,0,W,H);
  const t=now();
  if(mode==="net"&&drawNet){ drawNet(t); return; }
  if(globe&&auto) rot.lam+=0.00035;

  /* the ocean disc, then the shore */
  if(globe){
    const R=Math.min(W,H-40)*0.46*zoom;
    const g=cx.createRadialGradient(W/2-R*0.3,H/2-R*0.3,R*0.1,W/2,H/2+20,R);
    g.addColorStop(0,"#0a1220"); g.addColorStop(1,"#060b12");
    cx.fillStyle=g; cx.beginPath(); cx.arc(W/2,H/2+20,R,0,7); cx.fill();
    cx.strokeStyle="#1a2634"; cx.lineWidth=1.2;
    cx.beginPath(); cx.arc(W/2,H/2+20,R,0,7); cx.stroke();
  }
  if(land){
    cx.strokeStyle=globe?"#2c3f52":"#22323f"; cx.lineWidth=1;
    for(const line of land){
      cx.beginPath(); let pen=false;
      for(const [lon,lat] of line){
        const p=proj(lat,lon);
        if(p[2]){ pen?cx.lineTo(p[0],p[1]):cx.moveTo(p[0],p[1]); pen=true; } else pen=false;
      }
      cx.stroke();
    }
  }
  cx.strokeStyle="#101820"; cx.lineWidth=1;
  for(let lon=-180;lon<180;lon+=15){
    cx.beginPath(); let pen=false;
    for(let lat=-85;lat<=85;lat+=3){
      const p=proj(lat,lon);
      if(p[2]){ pen?cx.lineTo(p[0],p[1]):cx.moveTo(p[0],p[1]); pen=true; } else pen=false;
    } cx.stroke();
  }
  for(let lat=-75;lat<=75;lat+=15){
    cx.beginPath(); let pen=false;
    for(let lon=-180;lon<=180;lon+=3){
      const p=proj(lat,lon);
      if(p[2]){ pen?cx.lineTo(p[0],p[1]):cx.moveTo(p[0],p[1]); pen=true; } else pen=false;
    } cx.stroke();
  }

  /* airports: density draws the continents */
  for(const a of world.airports){
    const p=proj(a.lat,a.lon); if(!p[2]) continue;
    const r=Math.min(3.5,0.8+Math.log1p(a.n)/2.6);
    cx.fillStyle="#26364a"; cx.beginPath(); cx.arc(p[0],p[1],r,0,7); cx.fill();
    if(a.n>140&&zoom>0.9){ cx.fillStyle="#4a5b6d"; cx.font="10px ui-monospace"; cx.fillText(a.iata,p[0]+5,p[1]+3); }
  }

  /* closed airports: pulsing rings sized by their queue halo */
  for(const [iata,n] of closed){
    const a=world.airports.find(x=>x.iata===iata); if(!a) continue;
    const p=proj(a.lat,a.lon); if(!p[2]) continue;
    const base=6+Math.min(34,Math.log1p(n)*6), pulse=1+0.12*Math.sin(t/300);
    cx.strokeStyle="rgba(224,90,90,.8)"; cx.lineWidth=1.6;
    cx.beginPath(); cx.arc(p[0],p[1],base*pulse,0,7); cx.stroke();
    cx.strokeStyle="rgba(224,90,90,.25)";
    cx.beginPath(); cx.arc(p[0],p[1],base*pulse+7,0,7); cx.stroke();
    cx.fillStyle="#e05a5a"; cx.font="11px ui-monospace";
    cx.fillText(iata+" ✕ "+n, p[0]+base+10, p[1]+4);
  }

  /* message pulses along hub -> anchor */
  for(let i=pulses.length-1;i>=0;i--){
    const q=pulses[i], f=(t-q.t)/900;
    if(f>=1){ pulses.splice(i,1); continue; }
    const h=hubPos(q.peer); if(!h) continue;
    const from=q.dir==="in"?h:anchor, to=q.dir==="in"?anchor:h;
    const lat=from[0]+(to[0]-from[0])*f, lon=from[1]+(to[1]-from[1])*f;
    const p=proj(lat,lon); if(!p[2]) continue;
    cx.fillStyle=q.kind==="MVT"?"rgba(224,185,60,.8)":
      (q.kind==="ASM"||q.kind==="SSM")?"rgba(224,90,90,.9)":"rgba(95,211,141,.8)";
    cx.beginPath(); cx.arc(p[0],p[1],1.6,0,7); cx.fill();
  }

  /* landing blips */
  for(let i=blips.length-1;i>=0;i--){
    const b=blips[i], f=(t-b.t)/1200;
    if(f>=1){ blips.splice(i,1); continue; }
    cx.strokeStyle="rgba(232,238,244,"+(1-f)*.7+")";
    cx.beginPath(); cx.arc(b.x,b.y,3+f*14,0,7); cx.stroke();
  }

  /* aircraft. While the day is paused, sim time stands still but wall time
     does not: the pause accumulates, and on resume every airborne plane's
     schedule slides forward by exactly the time the world stood still. */
  const wallNow=Date.now();
  if(lastFrameW && simWarp===0) pausedMs+=wallNow-lastFrameW;
  lastFrameW=wallNow;
  const nowMs=wallNow-pausedMs;
  for(const p of planes.values()){
    if(p._dep===undefined){ p._dep=Date.parse(p.departed); p._arr=Date.parse(p.arriving); }
    const dep=p._dep, arr=p._arr;
    let f=(nowMs-dep)/Math.max(1,arr-dep); f=Math.max(0,Math.min(1,f));
    const lat=p.from_lat+(p.to_lat-p.from_lat)*f, lon=p.from_lon+(p.to_lon-p.from_lon)*f;
    const q=proj(lat,lon); if(!q[2]) continue;
    const q2=proj(p.to_lat,p.to_lon);
    const ang=q2[2]?Math.atan2(q2[1]-q[1],q2[0]-q[0]):0;
    const back=proj(p.from_lat+(lat-p.from_lat)*0.72, p.from_lon+(lon-p.from_lon)*0.72);
    if(back[2]){ cx.strokeStyle="rgba(232,238,244,.12)";
      cx.beginPath(); cx.moveTo(back[0],back[1]); cx.lineTo(q[0],q[1]); cx.stroke(); }
    cx.save(); cx.translate(q[0],q[1]); cx.rotate(ang);
    cx.fillStyle=p.diverted?"#e0b93c":"#e8eef4"; cx.beginPath();
    cx.moveTo(4,0); cx.lineTo(-3,2.4); cx.lineTo(-1.4,0); cx.lineTo(-3,-2.4); cx.closePath(); cx.fill();
    cx.restore();
  }
}
/* --- the logical web ---------------------------------------------------
   Nodes are systems that converse: the GDS and the carriers. The switch is
   plumbing and never appears. Edge weight is the smoothed conversation rate;
   the dots riding the edges are sampled real conversations. Two layouts of
   the same graph: a force-directed sprawl, and a ring with chords. */

function ringPositions(){
  const cxp=W/2, cyp=(H+30)/2, R=Math.min(W,H-60)*0.40;
  const cs=[...(world.carriers||[])].sort((a,b)=>a.lon-b.lon);
  const pos=new Map();
  cs.forEach((c,i)=>{
    const a=-Math.PI/2 + i/cs.length*2*Math.PI;
    pos.set(c.code,[cxp+R*Math.cos(a), cyp+R*Math.sin(a)]);
  });
  const hubs=[...HUBS];
  hubs.forEach((h,i)=>{
    const a=(i/Math.max(1,hubs.length))*2*Math.PI;
    pos.set(h,[cxp+40*Math.cos(a), cyp+40*Math.sin(a)]);
  });
  return pos;
}

function seedPhys(){
  const ring=ringPositions();
  for(const [code,p] of ring){
    if(!phys.has(code)) phys.set(code,{x:p[0]+(Math.random()-.5)*40,y:p[1]+(Math.random()-.5)*40,vx:0,vy:0});
  }
  physSeeded=true;
}

/* The web's cast is recomputed only when the edges change (every stats
   tick at most), never per frame: membership, the active array, the degree
   count and the parked ring are all cached. The physics itself cools -- once
   the sprawl stops moving it stops being simulated, and fresh conversations
   reheat it. This page once split three thousand edge keys per node per
   frame; a full net view now costs a settled layout almost nothing. */
let webCache=null;
function rebuildWeb(){
  const cxp=W/2, cyp=(H+30)/2;
  const inWeb=new Set(HUBS);
  const deg=new Map();
  for(const e of edgesE.values()){
    inWeb.add(e.a); inWeb.add(e.b);
    deg.set(e.a,(deg.get(e.a)||0)+1);
    deg.set(e.b,(deg.get(e.b)||0)+1);
  }
  const active=[...phys.entries()].filter(([c])=>inWeb.has(c));
  const parked=new Map();
  const all=(world.carriers||[]).filter(c=>!inWeb.has(c.code)).sort((a,b)=>a.lon-b.lon);
  const R=Math.min(W,H-60)*0.47;
  all.forEach((c,i)=>{
    const a=-Math.PI/2 + i/Math.max(1,all.length)*2*Math.PI;
    parked.set(c.code,[cxp+R*Math.cos(a), cyp+R*Math.sin(a)]);
  });
  webCache={inWeb,deg,active,parked};
  edgesDirty=false;
}

function stepPhys(){
  if(!physSeeded) seedPhys();
  if(edgesDirty||!webCache) rebuildWeb();
  const {inWeb,active,parked}=webCache;
  const cxp=W/2, cyp=(H+30)/2;
  // Cooled? Serve the settled layout without simulating anything.
  const settled = settleFrames>30;
  if(!settled){
  const K=2100;
  let maxV=0;
  for(let it=0;it<2;it++){
    // repulsion
    for(let i=0;i<active.length;i++){
      const a=active[i][1];
      for(let j=i+1;j<active.length;j++){
        const b=active[j][1];
        let dx=a.x-b.x, dy=a.y-b.y;
        let d2=dx*dx+dy*dy; if(d2<25) d2=25;
        if(d2>90000) continue; // beyond 300px repulsion is noise
        const f=K/d2, d=Math.sqrt(d2);
        dx/=d; dy/=d;
        a.vx+=dx*f; a.vy+=dy*f; b.vx-=dx*f; b.vy-=dy*f;
      }
    }
    // springs along conversations, stronger for louder ones
    for(const e of edgesE.values()){
      const a=phys.get(e.a), b=phys.get(e.b);
      if(!a||!b) continue;
      const hub=HUBS.has(e.a)||HUBS.has(e.b);
      const dx=b.x-a.x, dy=b.y-a.y, d=Math.max(1,Math.hypot(dx,dy));
      const rest=hub?170:80-Math.min(40,Math.log1p(e.w)*12);
      // A GDS holds a spring to nearly every carrier; unnormalised, five
      // hundred springs crush the web into a star. Hub springs are softened
      // so the carrier-to-carrier springs get to shape the clusters.
      let f=(d-rest)*0.004*Math.min(3,0.5+Math.log1p(e.w));
      if(hub) f*=0.18;
      a.vx+=dx/d*f; a.vy+=dy/d*f; b.vx-=dx/d*f; b.vy-=dy/d*f;
    }
    // gravity, heavier for the hubs so the web hangs off them
    for(const [code,a] of active){
      const g=HUBS.has(code)?0.05:0.02;
      a.vx+=(cxp-a.x)*g; a.vy+=(cyp-a.y)*g;
      a.vx*=0.66; a.vy*=0.66;
      a.x+=a.vx; a.y+=a.vy;
      const v=Math.abs(a.vx)+Math.abs(a.vy);
      if(v>maxV) maxV=v;
    }
  }
  if(maxV<0.06) settleFrames++; else settleFrames=0;
  }
  const pos=new Map(parked);
  for(const [code,a] of phys){ if(inWeb.has(code)) pos.set(code,[a.x,a.y]); }
  return pos;
}

function edgePoint(p1,p2,f,curved){
  if(!curved){ return [p1[0]+(p2[0]-p1[0])*f, p1[1]+(p2[1]-p1[1])*f]; }
  const cxp=W/2, cyp=(H+30)/2;
  const mx=(p1[0]+p2[0])/2, my=(p1[1]+p2[1])/2;
  const qx=mx+(cxp-mx)*0.5, qy=my+(cyp-my)*0.5; // pull chords toward centre
  const u=1-f;
  return [u*u*p1[0]+2*u*f*qx+f*f*p2[0], u*u*p1[1]+2*u*f*qy+f*f*p2[1]];
}

const kindColor=k=> k==="MVT"||k==="MVA"||k==="DIV" ? "rgba(224,185,60," :
  k==="ASM"||k==="SSM" ? "rgba(224,90,90," :
  k==="AVS" ? "rgba(95,150,190," : "rgba(95,211,141,";

drawNet=function(t){
  const curved = netLayout==="ring";
  netPos = curved ? ringPositions() : stepPhys();
  cx.save();
  cx.translate(netPanX,netPanY);
  cx.scale(netZoom,netZoom);

  /* edges: the conversations. Everyone talks to the GDS -- that is the known
     truth, drawn as faint wallpaper; the ink goes to the carrier-to-carrier
     web, which is the part worth reading. */
  for(const e of edgesE.values()){
    const p1=netPos.get(e.a), p2=netPos.get(e.b);
    if(!p1||!p2) continue;
    const hub=HUBS.has(e.a)||HUBS.has(e.b);
    let al=Math.min(.5,.04+Math.log1p(e.w)*.09);
    let wd=Math.min(3,.3+Math.log1p(e.w)*.55);
    if(hub){ al*=0.22; wd=Math.min(wd,0.8); }
    else { al=Math.min(.8,al*1.9); }
    cx.strokeStyle=hub?"rgba(95,120,150,"+al+")":"rgba(130,175,215,"+al+")";
    cx.lineWidth=wd;
    cx.beginPath();
    if(curved){
      const cxp=W/2, cyp=(H+30)/2;
      const mx=(p1[0]+p2[0])/2, my=(p1[1]+p2[1])/2;
      cx.moveTo(p1[0],p1[1]);
      cx.quadraticCurveTo(mx+(cxp-mx)*0.5, my+(cyp-my)*0.5, p2[0],p2[1]);
    } else {
      cx.moveTo(p1[0],p1[1]); cx.lineTo(p2[0],p2[1]);
    }
    cx.stroke();
  }

  /* sampled conversations riding their edges */
  for(let i=flows.length-1;i>=0;i--){
    const q=flows[i], f=(t-q.t)/800;
    if(f>=1){ flows.splice(i,1); continue; }
    const p1=netPos.get(q.src), p2=netPos.get(q.dst);
    if(!p1||!p2) continue;
    const p=edgePoint(p1,p2,f,curved);
    cx.fillStyle=kindColor(q.kind)+".9)";
    cx.beginPath(); cx.arc(p[0],p[1],1.9,0,7); cx.fill();
  }

  /* nodes: sized by how many conversations they hold */
  const degs=(webCache&&webCache.deg)||new Map();
  for(const [code,p] of netPos){
    if(HUBS.has(code)) continue;
    const deg=degs.get(code)||0, r=1.4+Math.min(6,Math.log1p(deg)*1.7);
    const rate=rates.get(code)||0;
    cx.fillStyle=rate>0.5?"#5b7a95":"#3a4d63";
    cx.beginPath(); cx.arc(p[0],p[1],r,0,7); cx.fill();
    if(deg>2||rate>3){
      cx.fillStyle="#7d93aa"; cx.font="10px ui-monospace";
      cx.fillText(code,p[0]+r+3,p[1]+3);
    }
  }
  for(const code of HUBS){
    const g=netPos.get(code);
    if(!g) continue;
    cx.fillStyle="#5fd38d"; cx.beginPath(); cx.arc(g[0],g[1],8,0,7); cx.fill();
    cx.strokeStyle="rgba(95,211,141,.5)"; cx.lineWidth=1.4;
    cx.beginPath(); cx.arc(g[0],g[1],12+2*Math.sin(t/400),0,7); cx.stroke();
    cx.fillStyle="#9fb4c8"; cx.font="11px ui-monospace";
    cx.fillText(code+" · gds",g[0]+16,g[1]+4);
  }
  cx.restore();
};
draw();
</script>
`
