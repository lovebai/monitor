# 一键构建 Server / Agent 可执行文件（Windows 原生 + Linux amd64）
# 用法:
#   powershell -ExecutionPolicy Bypass -File build.ps1            # 常规构建
#   powershell -ExecutionPolicy Bypass -File build.ps1 -Legacy    # 额外构建兼容 Windows 7/Server 2008 的 agent-win2008.exe（Go 1.20）
param([switch]$Legacy)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$bin = Join-Path $root 'bin'
$ver = Get-Date -Format 'yyyy.MM.dd'
$ldflags = "-s -w -X main.Version=$ver"
New-Item -ItemType Directory -Force -Path $bin | Out-Null

function Build-One($dir, $name, $goos, $goarch) {
    Push-Location (Join-Path $root $dir)
    try {
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $env:CGO_ENABLED = '0'
        go build -trimpath -ldflags $ldflags -o (Join-Path $bin $name) ./cmd
        Write-Host "built $name (version $ver)"
    } finally {
        Pop-Location
    }
}

Build-One 'server' 'server.exe'          'windows' 'amd64'
Build-One 'agent'   'agent.exe'          'windows' 'amd64'
Build-One 'server'  'server-linux-amd64' 'linux'   'amd64'
Build-One 'agent'   'agent-linux-amd64'  'linux'   'amd64'

if ($Legacy) {
    $go120 = Join-Path $root '.tools\go1.20.14\bin\go.exe'
    if (-not (Test-Path $go120)) {
        $toolsDir = Join-Path $root '.tools'
        New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null
        $zip = Join-Path $env:TEMP 'go1.20.14.windows-amd64.zip'
        Write-Host 'downloading Go 1.20.14 toolchain ...'
        curl.exe -L --ssl-no-revoke --fail --silent --show-error -o $zip https://go.dev/dl/go1.20.14.windows-amd64.zip
        if ($LASTEXITCODE -ne 0) {
            curl.exe -L --ssl-no-revoke --fail --silent --show-error -o $zip https://golang.google.cn/dl/go1.20.14.windows-amd64.zip
        }
        tar.exe -xf $zip -C $toolsDir
        Move-Item -LiteralPath (Join-Path $toolsDir 'go') -Destination (Join-Path $toolsDir 'go1.20.14')
    }
    Push-Location (Join-Path $root 'agent')
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        & $go120 build -trimpath -ldflags $ldflags -modfile legacy.go.mod -o (Join-Path $bin 'agent-win2008.exe') ./cmd
        Write-Host "built agent-win2008.exe (Go 1.20, Windows 7 / Server 2008 R2+, version $ver)"
    } finally {
        Pop-Location
    }
}

Write-Host "done, binaries in: $bin"
