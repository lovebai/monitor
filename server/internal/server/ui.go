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
.logout{border:1px solid #d9e4f0;background:white;border-radius:22px;padding:7px 12px;color:#5d769a;cursor:pointer;font:inherit;font-size:13px}
.logout:hover{color:#e95169;border-color:#f3a6b3}
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
.server-info{margin-top:1rem;margin-bottom:1rem}
.server-info .card{padding:18px 22px}
.si-head{font-size:17px;font-weight:750;margin-bottom:16px}
.si-head:before{content:'//';color:#2ed5c3;margin-right:8px}
.si-grid{display:grid;grid-template-columns:repeat(6,1fr);gap:18px}
.si-item label{color:#6680a5;display:block;font-size:12px}
.si-item b{display:block;font-size:19px;margin:6px 0 4px}
.si-item .sub{color:#8ba0bf;font-size:12px}
@media(max-width:1050px){.nodes{grid-template-columns:repeat(2,1fr)}.stats{grid-template-columns:repeat(2,1fr)}}
@media(max-width:1050px){.si-grid{grid-template-columns:repeat(3,1fr)}}
@media(max-width:600px){.nodes,.stats,.si-grid{grid-template-columns:1fr}.w{padding:12px}.brand{font-size:16px}.si-item{border-top:1px solid #e5edf7;padding-top:12px}}
</style>
</head>
<body data-memthr="{{.MemThreshold}}" data-diskthr="{{.DiskThreshold}}">
<main class="w">
<header><div class="brand"><i class="dot"></i>Server Monitor</div><div><button id="sound-btn" class="sound" type="button">🔔 停止报警</button> <span class="live">● 实时 · 5 秒更新</span>{{if .AuthEnabled}} <form method="post" action="/logout" style="display:inline"><button type="submit" class="logout">注销</button></form>{{end}}</div></header>
<section class="stats">
<div class="card stat"><label>在线</label><b id="stat-online">{{len .Nodes}} 台</b><span class="sub"></span></div>
<div class="card stat"><label>下行</label><b id="stat-rx">{{rate .RxRate}}</b><span class="sub">已启用网卡汇总</span></div>
<div class="card stat"><label>上行</label><b id="stat-tx">{{rate .TxRate}}</b><span class="sub">已启用网卡汇总</span></div>
<div class="card stat"><label>活动告警</label><b id="stat-alerts">{{.AlertCount}}</b><span class="sub">延迟阈值 {{printf "%.0f" .Threshold}} ms</span></div>
</section>
<div id="nodes-area">{{template "nodes" .}}</div>
</main>
<script>
function fmtRate(b){b=+b||0;if(b<1024)return b.toFixed(1)+' B/s';if(b<1048576)return (b/1024).toFixed(1)+' KB/s';if(b<1073741824)return (b/1048576).toFixed(1)+' MB/s';return (b/1073741824).toFixed(1)+' GB/s'}
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
    const [listRes,fragRes]=await Promise.all([
      fetch('/api/v1/nodes',{cache:'no-store'}),
      fetch('/api/v1/nodes-html',{cache:'no-store'})
    ]);
    const list=await listRes.json();
    const frag=await fragRes.text();
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
    const area=document.getElementById('nodes-area');
    if(area)area.innerHTML=frag;
  }catch(e){}
}
refresh();setInterval(refresh,5000);
</script>
</body>
</html>
{{define "nodes"}}
{{range .Groups}}<div class="title" data-grp="{{.Name}}">{{.Name}} <span class="gcount">{{len .Nodes}} 台</span></div>
<section class="nodes">
{{range .Nodes}}<a class="node {{if not .Online}}off{{end}}" data-id="{{.NodeID}}" {{if .Online}}href="/nodes/{{.NodeID}}"{{else}}title="节点已离线，无法查看详情"{{end}}>
<div class="head"><div><i class="state {{if .Online}}on{{end}}" data-role="dot"></i> <span class="name">{{.NodeID}}</span><span class="nodeid">{{.Hostname}}</span><span class="offtag" data-role="offtag" style="display:{{if .Online}}none{{else}}inline-block{{end}}">已离线</span></div><span class="sub" data-role="ago">{{ago .Timestamp}}</span></div>
<div class="chips"><span>{{.OS.Name}}</span><span>{{.OS.Architecture}}</span><span>{{.Hardware.LogicalCPUs}} CPU</span>{{if .Alias}}<span class="alias">{{.Alias}}</span>{{end}}</div>
<div class="line"><label>CPU <b data-role="cpu">{{printf "%.1f" .Resources.CPUPercent}}%</b></label><div class="bar"><i data-role="cpubar" style="width:{{printf "%.0f" .Resources.CPUPercent}}%"></i></div></div>
<div class="line"><label>内存 <b data-role="mem" class="{{if ge (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}">{{printf "%.1f" (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes)}}%</b></label><div class="bar"><i data-role="membar" class="{{if ge (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}" style="width:{{printf "%.0f" (pct .Resources.MemoryUsedBytes .Resources.MemoryTotalBytes)}}%"></i></div></div>
{{if .Resources.Disks}}{{with index .Resources.Disks 0}}<div class="line"><label>磁盘 <b data-role="disk" class="{{if ge .UsedPercent $.DiskThreshold}}danger{{end}}">{{printf "%.1f" .UsedPercent}}%</b></label><div class="bar"><i data-role="diskbar" class="{{if ge .UsedPercent $.DiskThreshold}}danger{{end}}" style="width:{{printf "%.0f" .UsedPercent}}%"></i></div></div>{{end}}{{end}}
<div class="net" data-role="net">负载　{{printf "%.0f" (loadPct .Resources.Load1 .Hardware.LogicalCPUs)}}%（{{printf "%.2f" .Resources.Load1}} / {{printf "%.2f" .Resources.Load5}} / {{printf "%.2f" .Resources.Load15}}）<br>网络　↓ {{rate .NetRxBps}}　↑ {{rate .NetTxBps}}<br>探测　{{if .Network.Reachable}}{{printf "%.0f ms" .Network.LatencyMS}}{{else}}不可达{{end}}</div>
<div class="net up" data-role="sys-time">系统时间:　{{sysTime .SystemTime}}</div>
<div data-role="alerts">{{range .Alerts}}<div class="warn">● {{.Message}}</div>{{end}}</div>
</a>{{end}}
</section>
{{else}}<div class="card stat">尚未收到 Agent 上报。</div>{{end}}
<footer class="server-info">
<div class="card">
<div class="si-head">Server 主机状态</div>
<div class="si-grid">
<div class="si-item"><label>系统</label><b>{{.Server.OSName}}</b><span class="sub">{{.Server.Hostname}} · {{.Server.Arch}}</span></div>
<div class="si-item"><label>负载</label><b>{{printf "%.2f" .Server.Load1}} / {{printf "%.2f" .Server.Load5}} / {{printf "%.2f" .Server.Load15}}</b><span class="sub">1 / 5 / 15 分钟</span></div>
<div class="si-item"><label>CPU</label><b>{{printf "%.1f" .Server.CPUPercent}}%</b><span class="sub">Server 主机</span></div>
<div class="si-item"><label>内存</label><b>{{printf "%.1f" (pct .Server.MemUsedBytes .Server.MemTotalBytes)}}%</b><span class="sub">{{bytes .Server.MemUsedBytes}} / {{bytes .Server.MemTotalBytes}}</span></div>
<div class="si-item"><label>磁盘</label><b>{{printf "%.1f" .Server.DiskUsedPct}}%</b><span class="sub">{{bytes .Server.DiskUsedBytes}} / {{bytes .Server.DiskTotalBytes}}</span></div>
<div class="si-item"><label>数据库文件</label><b>{{bytes .Server.DBFileSize}}</b><span class="sub">{{.Server.DBPath}}</span></div>
</div>
</div>
</footer>
{{end}}`
