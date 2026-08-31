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
  #bar { position:fixed; top:0; left:0; right:0; display:flex; gap:22px; align-items:baseline;
    padding:10px 16px; background:linear-gradient(#06090df2,#06090d00); z-index:2; pointer-events:none; }
  #bar b { color:var(--plane); font-weight:600; letter-spacing:.08em; }
  #bar span i { font-style:normal; color:var(--hot); }
  #bar a { color:var(--dim); text-decoration:none; pointer-events:auto; }
  #bar a:first-of-type { margin-left:auto; }
  #bar a:hover { color:var(--ink); }
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
  <span>airborne <i id="air">0</i></span>
  <span>movements <i id="mvt">0</i></span>
  <span>bookings <i id="bkg">0</i></span>
  <span>messages <i id="msg">0</i></span>
  <a href="/fleet" style="pointer-events:auto">fleet →</a>
  <a href="/stats" style="pointer-events:auto">stats →</a>
  <a href="/">switch console →</a>
</div>
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
addEventListener("resize",size); size();

let world=null, globe=false, anchor=null, mode="sky", land=null;
fetch("/eye/land.json").then(r=>r.json()).then(l=>{ land=l; });
const rates=new Map(); let netPos=null;
const planes=new Map(), pulses=[], blips=[];
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

fetch("/eye/world.json").then(r=>r.json()).then(w=>{
  world=w;
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
    else if(d.t==="close"){ closed.set(d.airport, closed.get(d.airport)||0); renderClosed(); }
    else if(d.t==="halo"){ closed.set(d.airport, d.count); renderClosed(); }
    else if(d.t==="reopen"){ closed.delete(d.airport); renderClosed(); }
    else if(d.t==="stats"){ air.textContent=d.airborne; mvt.textContent=d.movements;
      bkg.textContent=d.bookings; msg.textContent=d.messages;
      if(d.rates){ rates.clear(); for(const [p,n] of Object.entries(d.rates)) rates.set(p,n/2); }
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
  if(code==="1G") return anchor;
  const c=(world.carriers||[]).find(c=>c.code===code);
  return c?[c.lat,c.lon]:null;
};

document.getElementById("m-sky").onclick=()=>setMode("sky");
document.getElementById("m-net").onclick=()=>setMode("net");
function setMode(m){ mode=m; panel.style.display="none";
  document.getElementById("m-sky").style.color=m==="sky"?"#e8eef4":"#3d4c5c";
  document.getElementById("m-net").style.color=m==="net"?"#e8eef4":"#3d4c5c"; }

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
  pos.set("1G",[cxp,cyp+R*0.45]);
  return pos;
}

/* interaction: drag to rotate the globe (pan does nothing flat), wheel zooms, click selects */
let drag=null;
cv.addEventListener("mousedown",e=>{ drag={x:e.clientX,y:e.clientY,moved:false}; cv.classList.add("drag"); });
addEventListener("mousemove",e=>{
  if(!drag) return;
  const dx=e.clientX-drag.x, dy=e.clientY-drag.y;
  if(Math.abs(dx)+Math.abs(dy)>3) drag.moved=true;
  if(globe){ auto=false;
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
  zoom=Math.max(.6,Math.min(6,zoom*(e.deltaY<0?1.12:0.9))); },{passive:false});

function pick(x,y){
  if(!world) return;
  if(mode==="net"){ pickNode(x,y); return; }
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
  let best=null,bd=14*14;
  for(const [code,p] of netPos){
    const d=(p[0]-x)**2+(p[1]-y)**2;
    if(d<bd){ bd=d; best=code; }
  }
  if(!best){ panel.style.display="none"; return; }
  const c=(world.carriers||[]).find(c=>c.code===best);
  const name=best==="1X"?"the switch":best==="1G"?"the gds":best;
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

function draw(){
  requestAnimationFrame(draw);
  if(!world) return;
  cx.clearRect(0,0,W,H);
  const t=now();
  if(mode==="net"){ drawNet(t); return; }
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

  /* aircraft */
  const nowMs=Date.now();
  for(const p of planes.values()){
    const dep=Date.parse(p.departed), arr=Date.parse(p.arriving);
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
function drawNet(t){
  netPos=layoutNet(); if(!netPos) return;
  const centre=netPos.get("1X"), gds=netPos.get("1G");

  /* edges: one per link, weighted by the true rate; the pulses riding them
     are sampled, the brightness is not */
  for(const [code,p] of netPos){
    if(code==="1X") continue;
    const r=rates.get(code)||0;
    const w=Math.min(3,0.3+Math.log1p(r)*0.7);
    const al=Math.min(.55,.05+Math.log1p(r)*.12);
    cx.strokeStyle="rgba(95,150,190,"+al+")"; cx.lineWidth=w;
    cx.beginPath(); cx.moveTo(p[0],p[1]); cx.lineTo(centre[0],centre[1]); cx.stroke();
  }

  /* pulses along the spokes */
  for(let i=pulses.length-1;i>=0;i--){
    const q=pulses[i], f=(t-q.t)/700;
    if(f>=1){ pulses.splice(i,1); continue; }
    const h=netPos.get(q.peer); if(!h) continue;
    const from=q.dir==="in"?h:centre, to=q.dir==="in"?centre:h;
    const x=from[0]+(to[0]-from[0])*f, y=from[1]+(to[1]-from[1])*f;
    cx.fillStyle=q.kind==="MVT"?"rgba(224,185,60,.9)":
      (q.kind==="ASM"||q.kind==="SSM")?"rgba(224,90,90,.95)":
      q.kind==="AVS"?"rgba(95,150,190,.8)":"rgba(95,211,141,.9)";
    cx.beginPath(); cx.arc(x,y,1.8,0,7); cx.fill();
  }

  /* nodes: carriers sized by rate, labels for the loud ones */
  for(const [code,p] of netPos){
    if(code==="1X"||code==="1G") continue;
    const r=rates.get(code)||0;
    const rad=1.5+Math.min(5,Math.log1p(r)*1.6);
    cx.fillStyle="#3a4d63"; cx.beginPath(); cx.arc(p[0],p[1],rad,0,7); cx.fill();
    if(r>2){ cx.fillStyle="#7d93aa"; cx.font="10px ui-monospace";
      const out=p[0]<W/2?-24:6;
      cx.fillText(code,p[0]+out,p[1]+3); }
  }
  /* infra */
  cx.fillStyle="#e8eef4"; cx.beginPath(); cx.arc(centre[0],centre[1],7,0,7); cx.fill();
  cx.strokeStyle="#5fd38d"; cx.lineWidth=1.4;
  cx.beginPath(); cx.arc(centre[0],centre[1],11+2*Math.sin(t/400),0,7); cx.stroke();
  cx.fillStyle="#9fb4c8"; cx.font="11px ui-monospace";
  cx.fillText("1X · switch",centre[0]+16,centre[1]+4);
  cx.fillStyle="#5fd38d"; cx.beginPath(); cx.arc(gds[0],gds[1],5.5,0,7); cx.fill();
  cx.fillStyle="#9fb4c8"; cx.fillText("1G · gds",gds[0]+12,gds[1]+4);
}
draw();
</script>
`
