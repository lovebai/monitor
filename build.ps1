# 一键构建 Server / Agent 可执行文件（Windows 原生 + Linux amd64）
# 用法: powershell -ExecutionPolicy Bypass -File build.ps1
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$bin = Join-Path $root 'bin'
New-Item -ItemType Directory -Force -Path $bin | Out-Null

function Build-One($dir, $name, $goos, $goarch) {
    Push-Location (Join-Path $root $dir)
    try {
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $env:CGO_ENABLED = '0'
        go build -trimpath -ldflags '-s -w' -o (Join-Path $bin $name) ./cmd
        Write-Host "built $name"
    } finally {
        Pop-Location
    }
}

Build-One 'server' 'server.exe'          'windows' 'amd64'
Build-One 'agent'   'agent.exe'          'windows' 'amd64'
Build-One 'server'  'server-linux-amd64' 'linux'   'amd64'
Build-One 'agent'   'agent-linux-amd64'  'linux'   'amd64'
Write-Host "done, binaries in: $bin"
