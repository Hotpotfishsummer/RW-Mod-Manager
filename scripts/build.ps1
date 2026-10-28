# 一键全量生产打包 RW-Mod-Manager 单文件原生可执行程序
$env:PATH = "C:\Users\HOTPOTFISH\go\go\bin;C:\Users\HOTPOTFISH\go\bin;$env:PATH"
$env:GOPROXY = "https://goproxy.cn,direct"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " 打包铁锈战争模组与地图管理器 (Wails 生产构建)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

wails build -clean

if ($LASTEXITCODE -eq 0) {
    $exeItem = Get-ChildItem "build\bin\*.exe" | Select-Object -First 1
    if ($exeItem -and (Test-Path $exeItem.FullName)) {
        $fileSize = [math]::Round($exeItem.Length / 1MB, 2)
        Write-Host ""
        Write-Host "构建成功！" -ForegroundColor Green
        Write-Host "输出路径: $($exeItem.FullName)" -ForegroundColor Green
        Write-Host "产物体积: $fileSize MB" -ForegroundColor Green
    }
} else {
    Write-Host "构建失败！" -ForegroundColor Red
    exit 1
}
