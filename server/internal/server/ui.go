package server

const page = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Server Monitor</title>
<style>
body{margin:0;color:#17233a;font:14px system-ui;background-color:#edf3fa;background-image:linear-gradient(#dce8f455 1px,transparent 1px),linear-gradient(90deg,#dce8f455 1px,transparent 1px);background-size:55px 55px}
.w{max-width:1740px;margin:auto;padding:20px 4.3%}
header{height:55px;display:flex;align-items:center;justify-content:space-between}
.brand{font-size:19px;font-weight:750}
.dot{display:inline-block;width:14px;height:14px;border-radius:50%;background:#2ed5c3;box-shadow:0 0 18px #2ed5c3;margin-right:12px}
.live{border:1px solid #aee9dc;background:white;border-radius:22px;padding:7px 12px;color:#20b99c}
.sound{border:1px solid #aee9dc;background:white;border-radius:22px;padding:7px 12px;color:#20b99c;cursor:pointer;font:inherit;font-size:13px}
.sound.off{color:#a0b0c5;border-color:#e2eaf4}
.sound.active{border-color:#f3a6b3;background:#fff0f2;color:#e95169}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:18px;margin:16px 0 38px}
.card,.node{background:#fff;border:1px solid #d9e4f0;border-radius:17px;box-shadow:0 5px 20px #5470910b}
.stat{padding:21px 24px;border-top:2px solid #39d7c4}
.stat label,.sub{color:#6680a5}
.stat b{display:block;font-size:27px;margin-top:10px}
.title{font-size:17px;font-weight:750;margin:42px 0 17px}
.stats + .title{margin-top:0}
.title:before{content:'//';color:#2ed5c3;margin-right:8px}
.nodes{display:grid;grid-template-columns:repeat(4,1fr);gap:18px}
.node{padding:22px;cursor:pointer;text-decoration:none;color:inherit;min-height:325px;transition:.18s}
.node:hover{transform:translateY(-4px);box-shadow:0 12px 30px #54709125}
.head{display:flex;justify-content:space-between;align-items:center}
.name{font-weight:750;font-size:17px}
.state{width:11px;height:11px;border-radius:50%;background:#fa6c82}
.on{background:#2ed5a7;box-shadow:0 0 8px #2ed5a7}
.node.off{background:#fff9fa;border-color:#f3c1c8;cursor:not-allowed}
.node.off .name{color:#d84e67}
.offtag{display:inline-block;border:1px solid #f3c1c8;background:#fff0f2;color:#d84e67;border-radius:10px;padding:0 7px;font-size:12px;margin-left:6px}
.nodeid{color:#8ba0bf;font-size:12px;margin-left:6px}
.chips span{display:inline-block;border:1px solid #d5e2f2;border-radius:12px;padding:3px 7px;margin:12px 4px 8px 0;color:#5d769a;font-size:12px}
.line{margin:15px 0}
.line label{display:flex;justify-content:space-between;color:#617a9c}
.bar{height:8px;background:#e9eff8;border-radius:7px;margin-top:6px}
.bar i{height:100%;display:block;background:#2dcebc;border-radius:7px}
.danger{color:#e95169}
.bar i.danger{background:#e95169}
.net{border-top:1px solid #e5edf7;margin-top:18px;padding-top:13px;color:#536e92;line-height:1.8}
.net.up{margin-top:10px;padding-top:8px;color:#9db1cf;font-size:12px}
.warn{margin-top:12px;color:#e95169;font-size:12px}
@media(max-width:1050px){.nodes{grid-template-columns:repeat(2,1fr)}.stats{grid-template-columns:repeat(2,1fr)}}
@media(max-width:600px){.nodes,.stats{grid-template-columns:1fr}.w{padding:12px}.brand{font-size:16px}}
</style>
</head>
<body data-memthr="{{.MemThreshold}}" data-diskthr="{{.DiskThreshold}}">
<main class="w">
<header><div class="brand"><i class="dot"></i>Server Monitor</div><div><button id="sound-btn" class="sound" type="button">🔔 停止报警</button> <span class="live">● 实时 · 5 秒更新</span></div></header>
<section class="stats">
<div class="card stat"><label>在线</label><b id="stat-online">{{len .Nodes}} 台</b><span class="sub"></span></div>
<div class="card stat"><label>下行</label><b id="stat-rx">{{rate .RxRate}}</b><span class="sub">已启用网卡汇总</span></div>
<div class="card stat"><label>上行</label><b id="stat-tx">{{rate .TxRate}}</b><span class="sub">已启用网卡汇总</span></div>
<div class="card stat"><label>活动告警</label><b id="stat-alerts">{{.AlertCount}}</b><span class="sub">延迟阈值 {{printf "%.0f" .Threshold}} ms</span></div>
</section>
{{range .Groups}}<div class="title" data-grp="{{.Name}}">{{.Name}} <span class="gcount">{{len .Nodes}} 台</span></div>
<section class="nodes">
{{range .Nodes}}<a class="node {{if not .Online}}off{{end}}" data-id="{{.NodeID}}" {{if .Online}}href="/nodes/{{.NodeID}}"{{else}}title="节点已离线，无法查看详情"{{end}}>
<div class="head"><div><i class="state {{if .Online}}on{{end}}" data-role="dot"></i> <span class="name">{{.NodeID}}</span><span class="nodeid">{{.Hostname}}</span><span class="offtag" data-role="offtag" style="display:{{if .Online}}none{{else}}inline-block{{end}}">已离线</span></div><span class="sub" data-role="ago">{{ago .Timestamp}}</span></div>
<div class="chips"><span>{{.OS.Name}}</span><span>{{.OS.Architecture}}</span><span>{{.Hardware.LogicalCPUs}} CPU</span></div>
<div class="line"><label>CPU <b data-role="cpu">{{printf "%.1f" .Resources.CPUPercent}}%</b></label><div class="bar"><i data-role="cpubar" style="width:{{printf "%.0f" .Resources.CPUPercent}}%"></i></div></div>
<div class="line"><label>内存 <b data-role="mem" class="{{if ge (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}">{{printf "%.1f" (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes)}}%</b></label><div class="bar"><i data-role="membar" class="{{if ge (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}" style="width:{{printf "%.0f" (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes)}}%"></i></div></div>
{{if .Resources.Disks}}{{with index .Resources.Disks 0}}<div class="line"><label>磁盘 <b data-role="disk" class="{{if ge .UsedPercent $.DiskThreshold}}danger{{end}}">{{printf "%.1f" .UsedPercent}}%</b></label><div class="bar"><i data-role="diskbar" class="{{if ge .UsedPercent $.DiskThreshold}}danger{{end}}" style="width:{{printf "%.0f" .UsedPercent}}%"></i></div></div>{{end}}{{end}}
<div class="net" data-role="net">负载　{{printf "%.0f" (loadPct .Resources.Load1 .Hardware.LogicalCPUs)}}%（{{printf "%.2f" .Resources.Load1}} / {{printf "%.2f" .Resources.Load5}} / {{printf "%.2f" .Resources.Load15}}）<br>网络　↓ {{rate .NetRxBps}}　↑ {{rate .NetTxBps}}<br>探测　{{if .Network.Reachable}}{{printf "%.0f ms" .Network.LatencyMS}}{{else}}不可达{{end}}</div>
<div class="net up" data-role="sys-time">系统时间:　{{sysTime .SystemTime}}</div>
<div data-role="alerts">{{range .Alerts}}<div class="warn">● {{.Message}}</div>{{end}}</div>
</a>{{end}}
</section>
{{else}}<div class="card stat">尚未收到 Agent 上报。</div>{{end}}
</main>
<script>
function esc(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function fmtRate(b){b=+b||0;if(b<1024)return b.toFixed(1)+' B/s';if(b<1048576)return (b/1024).toFixed(1)+' KB/s';if(b<1073741824)return (b/1048576).toFixed(1)+' MB/s';return (b/1073741824).toFixed(1)+' GB/s'}
function fmtAgo(t){if(!t)return '';const s=(Date.now()-new Date(t).getTime())/1000;return s<60?Math.max(0,Math.round(s))+' 秒前':Math.round(s/60)+' 分钟前'}
function fmtSysTime(t){return t?String(t).replace('T',' ').slice(0,19):'-'}
function loadPct(l,c){return c>0?l*100/c:0}
var prevOnline={},ack={},lastAlarmAt=Date.now(),audioCtx=null,latest={};
function beep(){
  try{
    audioCtx=audioCtx||new (window.AudioContext||window.webkitAudioContext)();
    if(audioCtx.state==='suspended')audioCtx.resume();
    const t=audioCtx.currentTime;
    [[880,t],[880,t+0.18],[660,t+0.36]].forEach(function(x){
      const f=x[0],st=x[1],o=audioCtx.createOscillator(),g=audioCtx.createGain();
      o.type='square';o.frequency.value=f;
      g.gain.setValueAtTime(0.12,st);
      g.gain.exponentialRampToValueAtTime(0.001,st+0.16);
      o.connect(g);g.connect(audioCtx.destination);
      o.start(st);o.stop(st+0.18);
    });
  }catch(e){}
}
function alarm(){lastAlarmAt=Date.now();beep()}
function unackedCount(){let n=0;for(const id in latest)if(!latest[id]&&!ack[id])n++;return n}
function refreshSoundBtn(){
  const b=document.getElementById('sound-btn');
  if(!b)return;
  if(unackedCount()>0){b.textContent='🔔 报警中 · 点击停止';b.classList.add('active')}
  else{b.textContent='🔔 停止报警';b.classList.remove('active')}
}
document.getElementById('sound-btn').addEventListener('click',function(){
  for(const id in latest)if(!latest[id])ack[id]=true;
  lastAlarmAt=Date.now();
  refreshSoundBtn();
});
document.addEventListener('click',function unlock(){try{audioCtx=audioCtx||new (window.AudioContext||window.webkitAudioContext)();if(audioCtx.state==='suspended')audioCtx.resume()}catch(e){}},{once:true});
async function refresh(){
  try{
    const list=await (await fetch('/api/v1/nodes',{cache:'no-store'})).json();
    const memThr=parseFloat(document.body.dataset.memthr||'80'),diskThr=parseFloat(document.body.dataset.diskthr||'80');
    let online=0,rx=0,tx=0,alerts=0;
    const byId={};
    for(const n of list){byId[n.node_id]=n;if(n.online)online++;rx+=n.net_rx_bps||0;tx+=n.net_tx_bps||0;alerts+=(n.alerts||[]).length}
    const next={};
    for(const id in byId)next[id]=!!byId[id].online;
    latest=next;
    for(const id in ack)if(next[id])delete ack[id];
    let newOffline=false;
    for(const id in prevOnline)if(prevOnline[id]&&!next[id])newOffline=true;
    let unacked=0;
    for(const id in next)if(!next[id]&&!ack[id])unacked++;
    prevOnline=next;
    if(newOffline||(unacked>0&&Date.now()-lastAlarmAt>30000))alarm();
    refreshSoundBtn();
    document.getElementById('stat-online').textContent=online+' 台';
    document.getElementById('stat-rx').textContent=fmtRate(rx);
    document.getElementById('stat-tx').textContent=fmtRate(tx);
    document.getElementById('stat-alerts').textContent=alerts;
    const groups={};
    for(const n of list){const g=n.group||'DEFAULT';(groups[g]=groups[g]||[]).push(n)}
    document.querySelectorAll('.title[data-grp]').forEach(t=>{const s=t.querySelector('.gcount');if(s)s.textContent=(groups[t.getAttribute('data-grp')]||[]).length+' 台'});
    document.querySelectorAll('.node[data-id]').forEach(card=>{
      const n=byId[card.getAttribute('data-id')];
      if(!n)return;
      const r=n.resources||{},h=n.hardware||{};
      const dot=card.querySelector('[data-role="dot"]');
      if(dot)dot.classList.toggle('on',!!n.online);
      card.classList.toggle('off',!n.online);
      if(n.online){
        if(!card.getAttribute('href'))card.setAttribute('href','/nodes/'+card.getAttribute('data-id'));
        card.removeAttribute('title');
      }else{
        card.removeAttribute('href');
        card.setAttribute('title','节点已离线，无法查看详情');
      }
      const ot=card.querySelector('[data-role="offtag"]');
      if(ot)ot.style.display=n.online?'none':'inline-block';
      const ago=card.querySelector('[data-role="ago"]');
      if(ago)ago.textContent=fmtAgo(n.timestamp);
      const set=(role,fn)=>{const el=card.querySelector('[data-role="'+role+'"]');if(el)el.textContent=fn()};
      const bar=(role,pct)=>{const el=card.querySelector('[data-role="'+role+'"]');if(el)el.style.width=pct+'%'};
      const cpu=r.cpu_percent||0;
      set('cpu',()=>cpu.toFixed(1)+'%');bar('cpubar',Math.min(100,cpu));
      const mp=r.memory_total_bytes?(r.memory_used_bytes||0)*100/r.memory_total_bytes:0;
      const memEl=card.querySelector('[data-role="mem"]');
      if(memEl){memEl.textContent=mp.toFixed(1)+'%';memEl.classList.toggle('danger',mp>=memThr)}
      const memBar=card.querySelector('[data-role="membar"]');
      if(memBar){memBar.classList.toggle('danger',mp>=memThr)}
      const d=(r.disks&&r.disks.length)?r.disks[0]:null;
      const dp=d?(d.used_percent||0):0;
      const diskEl=card.querySelector('[data-role="disk"]');
      if(diskEl){diskEl.textContent=d?dp.toFixed(1)+'%':'-';diskEl.classList.toggle('danger',!!d&&dp>=diskThr)}
      const diskBar=card.querySelector('[data-role="diskbar"]');
      if(diskBar){diskBar.classList.toggle('danger',!!d&&dp>=diskThr)}
      const net=card.querySelector('[data-role="net"]');
      if(net){
        const nw=n.network||{};
        net.innerHTML='负载　'+loadPct(r.load_1||0,h.logical_cpus||0).toFixed(0)+'%（'+(r.load_1||0).toFixed(2)+' / '+(r.load_5||0).toFixed(2)+' / '+(r.load_15||0).toFixed(2)+'）<br>网络　↓ '+fmtRate(n.net_rx_bps||0)+'　↑ '+fmtRate(n.net_tx_bps||0)+'<br>探测　'+(nw.reachable?Math.round(nw.latency_ms||0)+' ms':'不可达');
      }
      const st=card.querySelector('[data-role="sys-time"]');
      if(st)st.textContent='系统时间:　'+fmtSysTime(n.system_time);
      const aw=card.querySelector('[data-role="alerts"]');
      if(aw)aw.innerHTML=(n.alerts||[]).map(a=>'<div class="warn">● '+esc(a.message)+'</div>').join('');
    });
  }catch(e){}
}
refresh();setInterval(refresh,5000);
</script>
</body>
</html>`
