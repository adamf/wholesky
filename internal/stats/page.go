package stats

import "net/http"

func (c *Collector) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(pageHTML)) //nolint:errcheck
}

// The instrument panel: the cluster as charts. Ten minutes of history,
// two-second resolution, drawn on plain canvas.
const pageHTML = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wholesky — stats</title>
<style>
  :root { --bg:#06090d; --panel:#0b1118; --line:#1d2836; --ink:#9fb4c8; --dim:#5b6b7d;
          --hot:#5fd38d; --amber:#e0b93c; --warn:#e05a5a; --blue:#5f96be; --plane:#e8eef4; }
  html,body { margin:0; min-height:100%; background:var(--bg); color:var(--ink);
    font:13px/1.45 "SF Mono", ui-monospace, Menlo, monospace; }
  a { color:#8fa6bc; text-decoration:none; } a:hover { color:#e8eef4; }
  #bar { position:sticky; top:0; display:flex; gap:22px; align-items:baseline;
    padding:12px 18px; background:#06090df2; border-bottom:1px solid var(--line); z-index:5; }
  #bar b { color:var(--plane); letter-spacing:.08em; }
  #bar span i { font-style:normal; color:var(--hot); }
  #bar .nav { margin-left:auto; display:flex; gap:16px; }
  #grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(340px,1fr));
    gap:14px; padding:16px 18px 60px; }
  .card { background:var(--panel); border:1px solid var(--line); border-radius:6px; padding:12px 14px; }
  .card h3 { margin:0 0 2px; font-size:12px; font-weight:400; letter-spacing:.08em;
    text-transform:uppercase; color:var(--dim); }
  .card .big { color:var(--plane); font-size:22px; margin-bottom:6px; }
  .card .big small { color:var(--dim); font-size:12px; }
  canvas { display:block; width:100%; height:88px; }
  .legend { display:flex; gap:14px; margin-top:6px; font-size:11px; color:var(--dim); }
  .legend i { font-style:normal; }
  .qrow { display:flex; justify-content:space-between; padding:3px 0; border-bottom:1px solid #0e1520; }
</style>
<div id="bar">
  <b>WHOLESKY STATS</b>
  <span>links <i id="hlinks">—</i></span>
  <span>messages <i id="hmsgs">—</i></span>
  <span>bookings <i id="hbkg">—</i></span>
  <span>movements <i id="hmvt">—</i></span>
  <span>uptime <i id="hup">—</i></span>
  <div class="nav"><a href="/eye">globe →</a><a href="/fleet">fleet →</a><a href="/">switch console →</a></div>
</div>
<div id="grid">
  <div class="card"><h3>messages / second</h3><div class="big" id="b-total"></div>
    <canvas id="c-total"></canvas>
    <div class="legend"><i style="color:#5fd38d">total</i></div></div>
  <div class="card"><h3>by wire format</h3><div class="big" id="b-fmt"></div>
    <canvas id="c-fmt"></canvas>
    <div class="legend"><i style="color:#5fd38d">type b</i><i style="color:#e0b93c">edifact</i></div></div>
  <div class="card"><h3>by traffic class</h3><div class="big" id="b-kind"></div>
    <canvas id="c-kind"></canvas>
    <div class="legend"><i style="color:#5f96be">avs</i><i style="color:#e0b93c">mvt</i>
      <i style="color:#5fd38d">res</i><i style="color:#e05a5a">asm</i>
      <i style="color:#b48fd9">pnl</i><i style="color:#8fd9c4">bag</i><i style="color:#f0a0c8">dcs</i></div></div>
  <div class="card"><h3>bookings / second</h3><div class="big" id="b-bkg"></div>
    <canvas id="c-bkg"></canvas>
    <div class="legend"><i style="color:#5fd38d">record events at the gds</i></div></div>
  <div class="card"><h3>aircraft airborne</h3><div class="big" id="b-air"></div>
    <canvas id="c-air"></canvas>
    <div class="legend"><i style="color:#e8eef4">flown by movement messages</i></div></div>
  <div class="card"><h3>movements / second</h3><div class="big" id="b-mvt"></div>
    <canvas id="c-mvt"></canvas>
    <div class="legend"><i style="color:#e0b93c">mvt · mva · div at the watcher</i></div></div>
  <div class="card"><h3>undeliverable / second</h3><div class="big" id="b-und"></div>
    <canvas id="c-und"></canvas>
    <div class="legend"><i style="color:#e05a5a">the number that should be zero</i></div></div>
  <div class="card"><h3>queue items held</h3><div class="big" id="b-q"></div>
    <canvas id="c-q"></canvas>
    <div id="qlist"></div></div>
</div>
<script>
"use strict";
const $=id=>document.getElementById(id);
function drawSeries(cv, series, colors, opts){
  const cx=cv.getContext("2d"), DPR=devicePixelRatio||1;
  const w=cv.clientWidth, h=cv.clientHeight;
  cv.width=w*DPR; cv.height=h*DPR; cx.setTransform(DPR,0,0,DPR,0,0);
  cx.clearRect(0,0,w,h);
  let max=0;
  for(const s of series) for(const v of s) if(v>max) max=v;
  if(max<=0) max=1;
  // faint grid
  cx.strokeStyle="#101820"; cx.lineWidth=1;
  for(let i=1;i<4;i++){ const y=h*i/4;
    cx.beginPath(); cx.moveTo(0,y); cx.lineTo(w,y); cx.stroke(); }
  series.forEach((s,si)=>{
    if(!s.length) return;
    cx.strokeStyle=colors[si]; cx.lineWidth=1.4; cx.beginPath();
    s.forEach((v,i)=>{
      const x=i/(Math.max(1,s.length-1))*w, y=h-2-(v/max)*(h-8);
      i?cx.lineTo(x,y):cx.moveTo(x,y);
    });
    cx.stroke();
    // emphasise the now-point
    const v=s[s.length-1], x=w, y=h-2-(v/max)*(h-8);
    cx.fillStyle=colors[si]; cx.beginPath(); cx.arc(x-2,y,2.2,0,7); cx.fill();
  });
  if(opts&&opts.maxLabel){ cx.fillStyle="#3d4c5c"; cx.font="10px ui-monospace";
    cx.fillText(max.toFixed(max<10?1:0),4,10); }
}
const last=s=>s.length?s[s.length-1]:0;
const fmtUp=s=>s<3600?((s/60)|0)+"m":((s/3600)|0)+"h"+(((s%3600)/60)|0)+"m";

async function poll(){
  const d=await fetch("/stats/data.json").then(r=>r.json());
  const S=d.series;
  $("hlinks").textContent=d.links??"—";
  $("hmsgs").textContent=d.totals.messages;
  $("hbkg").textContent=d.totals.bookings;
  $("hmvt").textContent=d.totals.movements;
  $("hup").textContent=fmtUp(d.uptime);

  $("b-total").innerHTML=last(S.total).toFixed(0)+" <small>msg/s</small>";
  drawSeries($("c-total"),[S.total],["#5fd38d"],{maxLabel:1});
  $("b-fmt").innerHTML=last(S.typeb).toFixed(0)+" <small>typeb</small> · "+
    last(S.edifact).toFixed(0)+" <small>edifact</small>";
  drawSeries($("c-fmt"),[S.typeb,S.edifact],["#5fd38d","#e0b93c"],{maxLabel:1});
  $("b-kind").innerHTML=last(S.mvt).toFixed(0)+" <small>mvt/s</small> · "+
    last(S.avs).toFixed(0)+" <small>avs/s</small>";
  drawSeries($("c-kind"),[S.avs,S.mvt,S.res,S.asm,S.pnl,S.bag,S.dcs||[]],
    ["#5f96be","#e0b93c","#5fd38d","#e05a5a","#b48fd9","#8fd9c4","#f0a0c8"],{maxLabel:1});
  $("b-bkg").innerHTML=last(S.bookings).toFixed(1)+" <small>/s</small>";
  drawSeries($("c-bkg"),[S.bookings],["#5fd38d"],{maxLabel:1});
  $("b-air").innerHTML=last(S.airborne).toFixed(0)+" <small>aircraft</small>";
  drawSeries($("c-air"),[S.airborne],["#e8eef4"],{maxLabel:1});
  $("b-mvt").innerHTML=last(S.movements).toFixed(0)+" <small>/s</small>";
  drawSeries($("c-mvt"),[S.movements],["#e0b93c"],{maxLabel:1});
  const und=last(S.undeliverable);
  $("b-und").innerHTML="<span style='color:"+(und>0?"#e05a5a":"#5fd38d")+"'>"+und.toFixed(1)+"</span> <small>/s · "+d.totals.undeliverable+" total</small>";
  drawSeries($("c-und"),[S.undeliverable],["#e05a5a"],{maxLabel:1});
  $("b-q").innerHTML=last(S.queued).toFixed(0)+" <small>items</small>";
  drawSeries($("c-q"),[S.queued],["#e0b93c"],{maxLabel:1});
  $("qlist").innerHTML=Object.entries(d.queues||{}).sort((a,b)=>b[1]-a[1])
    .map(([k,v])=>"<div class='qrow'><span>"+k+"</span><span style='color:#5fd38d'>"+v+"</span></div>").join("");
}
poll(); setInterval(poll,2000);
</script>
`
