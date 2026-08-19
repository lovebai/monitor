package server

const detailPage2 = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Node.Hostname}} · Monitor</title>
<style>
body{margin:0;color:#17233a;font:14px system-ui;background:#edf3fa;background-image:linear-gradient(#dce8f455 1px,transparent 1px),linear-gradient(90deg,#dce8f455 1px,transparent 1px);background-size:55px 55px}
.w{max-width:1420px;margin:auto;padding:20px 4%}
a{color:#3b719d;text-decoration:none}
.top{margin-bottom:20px}
.name{font-size:21px;font-weight:750;margin-top:18px}
.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
.card{background:white;border:1px solid #d9e4f0;border-radius:13px;padding:16px}
.card label{color:#647da0}
.card b{display:block;font-size:17px;margin-top:8px}
.section{font-size:17px;font-weight:750;margin:27px 0 13px}
.section:before{content:'//';color:#2ed5c3;margin-right:8px}
.row{display:flex;gap:12px;flex-wrap:wrap}
.pill{background:#eef7f8;border-radius:8px;padding:8px;color:#42607f}
.pill b{display:block;margin-bottom:4px}
.disk{width:240px}
.meta{color:#5c7898;font-size:13px;margin-top:3px}
.bar{height:7px;background:#e7edf5;border-radius:5px;margin-top:8px}
.bar i{display:block;height:100%;background:#31cdbc;border-radius:5px}
.danger{color:#e95169}
.bar i.danger{background:#e95169}
.warn{color:#d84e67;margin-top:8px}
.checks{background:white;border:1px solid #d9e4f0;border-radius:13px;overflow:hidden}
.checks table{width:100%;border-collapse:collapse}
.checks th,.checks td{text-align:left;padding:10px 14px;border-bottom:1px solid #e9eff8;font-size:13px}
.checks th{color:#647da0;background:#f6fafd;white-space:nowrap}
.checks tr:last-child td{border-bottom:none}
.toprow{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.topcol table{margin:0}
.tophead{padding:12px 14px 0;font-weight:750}
.empty{color:#647da0;text-align:center;padding:18px}
.tag{display:inline-block;border:1px solid #d5e2f2;border-radius:10px;padding:1px 8px;font-size:12px;color:#5d769a;background:#f7fafd}
.st{font-weight:750}
.ok{color:#2dcebc}
.bad{color:#e95169}
@media(max-width:850px){.grid{grid-template-columns:repeat(2,1fr)}.toprow{grid-template-columns:1fr}}
@media(max-width:500px){.grid{grid-template-columns:1fr}}
</style>
</head>
<body data-node="{{.Node.NodeID}}" data-memthr="{{.MemThreshold}}" data-diskthr="{{.DiskThreshold}}">
<main class="w">
<div class="top"><a href="/">‹ 返回总览</a><div class="name">{{.Node.Hostname}} <span id="d-on" style="color:#29ba9b;font-size:13px">● {{if .Node.Online}}在线{{else}}离线{{end}}</span></div><div style="color:#6981a1;margin-top:6px">{{.Node.NodeID}} · {{.Node.OS.Name}} {{.Node.OS.Version}} · {{.Node.Hardware.CPUModel}}</div></div>
<section class="grid">
<div class="card"><label>CPU</label><b id="d-cpu">{{printf "%.1f" .Node.Resources.CPUPercent}}%</b></div>
<div class="card"><label>网络延迟</label><b id="d-lat">{{if .Node.Network.Reachable}}{{printf "%.1f ms" .Node.Network.LatencyMS}}{{else}}不可达{{end}}</b></div>
<div class="card"><label>负载</label><b id="d-load">{{printf "%.2f" .Node.Resources.Load1}} / {{printf "%.2f" .Node.Resources.Load5}}</b></div>
<div class="card"><label>运行时长</label><b id="d-uptime">{{.Node.OS.UptimeSeconds}} 秒</b></div>
</section>
<div class="section">内存</div>
<div class="card"><label>内存</label><b id="d-mem" class="{{if ge (pct .Node.Resources.MemoryUsedBytes .Node.Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}">{{bytes .Node.Resources.MemoryUsedBytes}} / {{bytes .Node.Resources.MemoryTotalBytes}}</b><div class="meta" id="d-mempct" class="{{if ge (pct .Node.Resources.MemoryUsedBytes .Node.Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}">{{printf "%.1f" (pct .Node.Resources.MemoryUsedBytes .Node.Resources.MemoryTotalBytes)}}%</div><div class="bar"><i id="d-membar" class="{{if ge (pct .Node.Resources.MemoryUsedBytes .Node.Resources.MemoryTotalBytes) $.MemThreshold}}danger{{end}}" style="width:{{printf "%.0f" (pct .Node.Resources.MemoryUsedBytes .Node.Resources.MemoryTotalBytes)}}%"></i></div></div>
<div class="section">磁盘</div>
<div class="row" id="d-disks">{{range .Node.Resources.Disks}}<div class="pill disk"><b>{{.Mountpoint}}</b>{{bytes .UsedBytes}} / {{bytes .TotalBytes}}<div class="meta {{if ge .UsedPercent $.DiskThreshold}}danger{{end}}">{{printf "%.1f" .UsedPercent}}%</div><div class="bar"><i class="{{if ge .UsedPercent $.DiskThreshold}}danger{{end}}" style="width:{{printf "%.0f" .UsedPercent}}%"></i></div></div>{{end}}</div>
<div class="section">网卡</div>
<div class="row" id="d-nics">{{range .Node.Interfaces}}{{if isUp .}}<div class="pill"><b>{{.Name}}</b>{{.MAC}}<div class="meta">{{range ipv4s .Addresses}}{{.}} {{end}}</div></div>{{end}}{{end}}</div>
<div class="section">服务与进程</div>
<div id="d-checks">{{if .Node.Checks}}<div class="checks"><table><tr><th>类型</th><th>名称</th><th>状态与详情</th></tr>{{range .Node.Checks}}<tr><td><span class="tag">{{if eq .Type "process"}}进程{{else}}服务{{end}}</span></td><td>{{.Name}}</td><td>{{if .Healthy}}<span class="st ok">● {{.Detail}}{{if .PIDs}}（PID {{range .PIDs}}{{.}} {{end}}）{{end}}</span>{{else}}<span class="st bad">⚠ {{.Detail}}</span>{{end}}</td></tr>{{end}}</table></div>{{else}}<div class="card" style="color:#647da0">未配置服务与进程检查</div>{{end}}</div>
<div class="section">进程资源 Top 5</div>
<div class="toprow">
<div class="checks topcol" id="d-topcpu">{{if .Node.TopCPU}}<div class="tophead">CPU 占用 Top 5</div><table><tr><th>排名</th><th>应用</th><th>PID</th><th>CPU</th></tr>{{range $i, $p := .Node.TopCPU}}<tr><td>{{add $i 1}}</td><td>{{.Name}}</td><td>{{.PID}}</td><td>{{printf "%.1f" .CPUPercent}}%</td></tr>{{end}}</table>{{else}}<div class="tophead">CPU 占用 Top 5</div><table><tr><th>排名</th><th>应用</th><th>PID</th><th>CPU</th></tr><tr><td colspan="4" class="empty">暂无数据</td></tr></table>{{end}}</div>
<div class="checks topcol" id="d-topmem">{{if .Node.TopMemory}}<div class="tophead">内存占用 Top 5</div><table><tr><th>排名</th><th>应用</th><th>PID</th><th>内存</th></tr>{{range $i, $p := .Node.TopMemory}}<tr><td>{{add $i 1}}</td><td>{{.Name}}</td><td>{{.PID}}</td><td>{{bytes .MemoryBytes}}（{{printf "%.1f" .MemoryPct}}%）</td></tr>{{end}}</table>{{else}}<div class="tophead">内存占用 Top 5</div><table><tr><th>排名</th><th>应用</th><th>PID</th><th>内存</th></tr><tr><td colspan="4" class="empty">暂无数据</td></tr></table>{{end}}</div>
</div>
<div id="d-alerts">{{range .Node.Alerts}}<div class="warn">● {{.Message}}（{{ago .CreatedAt}}）</div>{{end}}</div>
</main>
<script>
function esc(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function fmtBytes(v){v=+v||0;if(v<1024)return v+' B';const u=['KiB','MiB','GiB','TiB'];let i=-1;do{v/=1024;i++}while(v>=1024&&i<3);return v.toFixed(1)+' '+u[i]}
function fmtAgo(t){if(!t)return '';const s=(Date.now()-new Date(t).getTime())/1000;return s<60?Math.max(0,Math.round(s))+' 秒前':Math.round(s/60)+' 分钟前'}
function isUp(i){return i.flags&&i.flags.indexOf('up')>=0&&!!i.mac}
function checksHTML(checks){
  if(!checks||!checks.length)return '<div class="card" style="color:#647da0">未配置服务与进程检查</div>';
  let h='<div class="checks"><table><tr><th>类型</th><th>名称</th><th>状态与详情</th></tr>';
  for(const c of checks){
    const pids=(c.pids||[]).length?('（PID '+c.pids.join(' ')+'）'):'';
    h+='<tr><td><span class="tag">'+(c.type==='process'?'进程':'服务')+'</span></td><td>'+esc(c.name)+'</td><td><span class="st '+(c.healthy?'ok':'bad')+'">'+(c.healthy?'●':'⚠')+' '+esc(c.detail||'')+pids+'</span></td></tr>';
  }
  return h+'</table></div>';
}
function procsTopHTML(list, mode){
  const title='<div class="tophead">'+(mode==='cpu'?'CPU 占用 Top 5':'内存占用 Top 5')+'</div>';
  const head='<tr><th>排名</th><th>应用</th><th>PID</th><th>'+(mode==='cpu'?'CPU':'内存')+'</th></tr>';
  if(!list||!list.length)return title+'<table>'+head+'<tr><td colspan="4" class="empty">暂无数据</td></tr></table>';
  const rows=list.map(function(p,i){
    const cell=mode==='cpu'?(p.cpu_percent||0).toFixed(1)+'%':fmtBytes(p.memory_bytes||0)+'（'+(p.memory_percent||0).toFixed(1)+'%）';
    return '<tr><td>'+(i+1)+'</td><td>'+esc(p.name)+'</td><td>'+p.pid+'</td><td>'+cell+'</td></tr>';
  }).join('');
  return title+'<table>'+head+rows+'</table>';
}
async function refresh(){
  try{
    const list=await (await fetch('/api/v1/nodes',{cache:'no-store'})).json();
    const memThr=parseFloat(document.body.dataset.memthr||'80'),diskThr=parseFloat(document.body.dataset.diskthr||'80');
    const n=(list||[]).find(x=>x.node_id===document.body.dataset.node);
    if(!n)return;
    const on=document.getElementById('d-on');
    if(on){on.textContent='● '+(n.online?'在线':'离线');on.style.color=n.online?'#29ba9b':'#d84e67'}
    const r=n.resources||{},h=n.hardware||{},nw=n.network||{};
    const set=(id,fn)=>{const el=document.getElementById(id);if(el)el.textContent=fn()};
    set('d-cpu',()=>(r.cpu_percent||0).toFixed(1)+'%');
    set('d-lat',()=>nw.reachable?(nw.latency_ms||0).toFixed(1)+' ms':'不可达');
    set('d-load',()=>(r.load_1||0).toFixed(2)+' / '+(r.load_5||0).toFixed(2));
    set('d-uptime',()=>(n.os&&n.os.uptime_seconds?n.os.uptime_seconds:0)+' 秒');
    set('d-mem',()=>fmtBytes(r.memory_used_bytes||0)+' / '+fmtBytes(r.memory_total_bytes||0));
    const mp=r.memory_total_bytes?(r.memory_used_bytes||0)*100/r.memory_total_bytes:0;
    const mpe=document.getElementById('d-mempct');
    if(mpe){mpe.textContent=mp.toFixed(1)+'%';mpe.classList.toggle('danger',mp>=memThr)}
    const mbe=document.getElementById('d-mem');
    if(mbe)mbe.classList.toggle('danger',mp>=memThr);
    const mb=document.getElementById('d-membar');
    if(mb){mb.style.width=Math.min(100,mp)+'%';mb.classList.toggle('danger',mp>=memThr)}
    const disks=document.getElementById('d-disks');
    if(disks)disks.innerHTML=(r.disks||[]).map(d=>{const danger=(d.used_percent||0)>=diskThr;return '<div class="pill disk"><b>'+esc(d.mountpoint)+'</b>'+fmtBytes(d.used_bytes)+' / '+fmtBytes(d.total_bytes)+'<div class="meta'+(danger?' danger':'')+'">'+((d.used_percent||0).toFixed(1))+'%</div><div class="bar"><i'+(danger?' class="danger"':'')+' style="width:'+Math.min(100,d.used_percent||0)+'%"></i></div></div>'}).join('');
    const nics=document.getElementById('d-nics');
    if(nics)nics.innerHTML=(n.interfaces||[]).filter(isUp).map(i=>{const v4=(i.addresses||[]).filter(a=>a.indexOf(':')<0).join(' ');return '<div class="pill"><b>'+esc(i.name)+'</b>'+esc(i.mac)+'<div class="meta">'+esc(v4)+'</div></div>'}).join('');
    const checks=document.getElementById('d-checks');
    if(checks)checks.innerHTML=checksHTML(n.checks);
    const topcpu=document.getElementById('d-topcpu');
    if(topcpu)topcpu.innerHTML=procsTopHTML(n.top_cpu,'cpu');
    const topmem=document.getElementById('d-topmem');
    if(topmem)topmem.innerHTML=procsTopHTML(n.top_memory,'mem');
    const al=document.getElementById('d-alerts');
    if(al)al.innerHTML=(n.alerts||[]).map(a=>'<div class="warn">● '+esc(a.message)+'（'+fmtAgo(a.created_at)+'）</div>').join('');
  }catch(e){}
}
refresh();setInterval(refresh,5000);
</script>
</body>
</html>`
