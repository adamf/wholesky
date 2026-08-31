package eye

import "net/http"

func (e *Eye) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(pageHTML)) //nolint:errcheck
}

// The scope. One canvas, no dependencies: graticule, airports, aircraft flown
// by movement messages, and message traffic pulsing along the network star.
const pageHTML = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wholesky — the eye</title>
<style>
  :root { --bg:#06090d; --ink:#9fb4c8; --dim:#3d4c5c; --hot:#5fd38d; --pulse:#e0b93c; --plane:#e8eef4; }
  html,body { margin:0; height:100%; background:var(--bg); color:var(--ink);
    font:13px/1.4 "SF Mono", ui-monospace, Menlo, monospace; overflow:hidden; }
  #bar { position:fixed; top:0; left:0; right:0; display:flex; gap:22px; align-items:baseline;
    padding:10px 16px; background:linear-gradient(#06090df2,#06090d00); z-index:2; }
  #bar b { color:var(--plane); font-weight:600; letter-spacing:.08em; }
  #bar span i { font-style:normal; color:var(--hot); }
  #bar a { color:var(--dim); text-decoration:none; margin-left:auto; }
  #bar a:hover { color:var(--ink); }
  canvas { display:block; width:100vw; height:100vh; }
</style>
<div id="bar">
  <b>WHOLESKY</b>
  <span>airborne <i id="air">0</i></span>
  <span>movements <i id="mvt">0</i></span>
  <span>bookings <i id="bkg">0</i></span>
  <span>messages <i id="msg">0</i></span>
  <a href="/">switch console →</a>
</div>
<canvas id="c"></canvas>
<script>
"use strict";
const cv = document.getElementById("c"), cx = cv.getContext("2d");
let W=0,H=0,DPR=1;
function size(){ DPR=devicePixelRatio||1; W=innerWidth; H=innerHeight;
  cv.width=W*DPR; cv.height=H*DPR; cx.setTransform(DPR,0,0,DPR,0,0); }
addEventListener("resize",size); size();

let world=null, proj=null, center=null;
const planes=new Map(), pulses=[], blips=[];

// Equirectangular, fit to the airports with padding; latitude compressed by
// cos(mid) so Europe does not look squashed.
function fit(apts){
  let lo=[90,180],hi=[-90,-180];
  for(const a of apts){ lo[0]=Math.min(lo[0],a.lat); lo[1]=Math.min(lo[1],a.lon);
    hi[0]=Math.max(hi[0],a.lat); hi[1]=Math.max(hi[1],a.lon); }
  const mid=(lo[0]+hi[0])/2, k=Math.cos(mid*Math.PI/180);
  const pad=0.07, spanX=(hi[1]-lo[1])*k, spanY=hi[0]-lo[0];
  const s=Math.min(W*(1-2*pad)/spanX, (H-60)*(1-2*pad)/spanY);
  const ox=(W-spanX*s)/2, oy=(H-spanY*s)/2+20;
  return (lat,lon)=>[ox+(lon-lo[1])*k*s, oy+(hi[0]-lat)*s];
}

fetch("/eye/world.json").then(r=>r.json()).then(w=>{
  world=w; proj=fit(w.airports);
  let sx=0,sy=0;
  for(const a of w.airports){ const p=proj(a.lat,a.lon); sx+=p[0]; sy+=p[1]; }
  center=[sx/w.airports.length, sy/w.airports.length];
  return fetch("/eye/planes.json");
}).then(r=>r.json()).then(ps=>{ for(const p of ps) planes.set(p.flight,p); connect(); });

function connect(){
  const es=new EventSource("/eye/stream");
  es.onmessage=ev=>{
    const d=JSON.parse(ev.data);
    if(d.t==="dep"){ planes.set(d.plane.flight,d.plane); }
    else if(d.t==="arr"){ const p=planes.get(d.flight);
      if(p){ const q=proj(p.to_lat,p.to_lon); blips.push({x:q[0],y:q[1],t:now()}); }
      planes.delete(d.flight); }
    else if(d.t==="pulse"){ if(pulses.length<400) pulses.push({peer:d.peer,dir:d.dir,kind:d.kind,t:now()}); }
    else if(d.t==="stats"){ air.textContent=d.airborne; mvt.textContent=d.movements;
      bkg.textContent=d.bookings; msg.textContent=d.messages; }
  };
  es.onerror=()=>{ es.close(); setTimeout(connect,2000); };
}

const hub=code=>{
  if(!world) return center;
  if(code==="1G") return center;
  const c=(world.carriers||[]).find(c=>c.code===code);
  return c?proj(c.lat,c.lon):center;
};
const now=()=>performance.now();

function draw(){
  requestAnimationFrame(draw);
  if(!world||!proj) return;
  cx.clearRect(0,0,W,H);
  const t=now();

  // graticule
  cx.strokeStyle="#101820"; cx.lineWidth=1; cx.beginPath();
  for(let lon=-30;lon<=45;lon+=5){ const a=proj(30,lon),b=proj(72,lon); cx.moveTo(a[0],a[1]); cx.lineTo(b[0],b[1]); }
  for(let lat=30;lat<=72;lat+=5){ const a=proj(lat,-30),b=proj(lat,45); cx.moveTo(a[0],a[1]); cx.lineTo(b[0],b[1]); }
  cx.stroke();

  // airports
  for(const a of world.airports){
    const p=proj(a.lat,a.lon), r=Math.min(4,1+Math.log1p(a.n)/2);
    cx.fillStyle="#22303e"; cx.beginPath(); cx.arc(p[0],p[1],r,0,7); cx.fill();
    if(a.n>60){ cx.fillStyle="#4a5b6d"; cx.font="10px ui-monospace"; cx.fillText(a.iata,p[0]+5,p[1]+3); }
  }

  // message pulses: hub -> network centre (in) or back (out), 900ms of life
  for(let i=pulses.length-1;i>=0;i--){
    const q=pulses[i], f=(t-q.t)/900;
    if(f>=1){ pulses.splice(i,1); continue; }
    const h=hub(q.peer), from=q.dir==="in"?h:center, to=q.dir==="in"?center:h;
    const x=from[0]+(to[0]-from[0])*f, y=from[1]+(to[1]-from[1])*f;
    cx.fillStyle=q.kind==="MVT"?"rgba(224,185,60,.8)":"rgba(95,211,141,.8)";
    cx.beginPath(); cx.arc(x,y,1.6,0,7); cx.fill();
  }

  // landing blips
  for(let i=blips.length-1;i>=0;i--){
    const b=blips[i], f=(t-b.t)/1200;
    if(f>=1){ blips.splice(i,1); continue; }
    cx.strokeStyle="rgba(232,238,244,"+(1-f)*.7+")";
    cx.beginPath(); cx.arc(b.x,b.y,3+f*14,0,7); cx.stroke();
  }

  // aircraft: position by wall time between departure and arrival
  const nowMs=Date.now();
  for(const p of planes.values()){
    const dep=Date.parse(p.departed), arr=Date.parse(p.arriving);
    let f=(nowMs-dep)/Math.max(1,arr-dep); f=Math.max(0,Math.min(1,f));
    const a=proj(p.from_lat,p.from_lon), b=proj(p.to_lat,p.to_lon);
    const x=a[0]+(b[0]-a[0])*f, y=a[1]+(b[1]-a[1])*f;
    const ang=Math.atan2(b[1]-a[1],b[0]-a[0]);
    // trail
    cx.strokeStyle="rgba(232,238,244,.12)"; cx.beginPath();
    cx.moveTo(a[0]+(x-a[0])*0.72,a[1]+(y-a[1])*0.72); cx.lineTo(x,y); cx.stroke();
    // the aircraft
    cx.save(); cx.translate(x,y); cx.rotate(ang);
    cx.fillStyle="#e8eef4"; cx.beginPath();
    cx.moveTo(4,0); cx.lineTo(-3,2.4); cx.lineTo(-1.4,0); cx.lineTo(-3,-2.4); cx.closePath(); cx.fill();
    cx.restore();
  }
}
draw();
</script>
`
