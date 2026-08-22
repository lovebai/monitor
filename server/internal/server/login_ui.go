package server

const loginPage = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='%2317233a'/%3E%3Cpath d='M5 17h5l2.5-6 4.5 12 3-9 2 3h5' fill='none' stroke='%232ed5c3' stroke-width='3' stroke-linecap='round' stroke-linejoin='round'/%3E%3Ccircle cx='24' cy='9' r='3' fill='%232ed5c3'/%3E%3C/svg%3E">
<title>登录 · Server Monitor</title>
<style>
body{margin:0;color:#17233a;font:14px system-ui;background:#edf3fa;display:flex;align-items:center;justify-content:center;min-height:100vh}
.box{background:#fff;border:1px solid #d9e4f0;border-radius:17px;box-shadow:0 5px 20px #5470910b;padding:34px 40px;width:320px}
h1{font-size:19px;margin:0 0 6px}
h1:before{content:'//';color:#2ed5c3;margin-right:8px}
p{color:#6680a5;margin:0 0 20px}
label{color:#647da0;display:block;margin:12px 0 6px}
input{width:100%;box-sizing:border-box;border:1px solid #d5e2f2;border-radius:9px;padding:9px 11px;font:inherit;outline:none}
input:focus{border-color:#2ed5c3}
button{margin-top:22px;width:100%;background:#2ed5c3;border:none;border-radius:9px;color:#fff;font:inherit;font-weight:750;padding:10px;cursor:pointer}
.err{color:#e95169;margin-top:12px;text-align:center}
</style>
</head>
<body>
<form class="box" method="post" action="/login">
<h1>Server Monitor</h1>
<p>请输入登录凭据</p>
<label>用户名</label><input name="username" autocomplete="username" required>
<label>密码</label><input type="password" name="password" autocomplete="current-password" required>
<button type="submit">登录</button>
{{if .Err}}<div class="err">用户名或密码错误</div>{{end}}
</form>
</body>
</html>`
