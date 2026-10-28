# 一键启动 Rusted Warfare Mod & Map Manager 开发模式 (支持前后端热重载)
$env:PATH = "C:\Users\HOTPOTFISH\go\go\bin;C:\Users\HOTPOTFISH\go\bin;$env:PATH"
$env:GOPROXY = "https://goproxy.cn,direct"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " 启动铁锈战争模组与地图管理器 (Go + Wails 开发模式)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

wails dev
