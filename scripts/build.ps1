# Build RW-Mod-Manager executable
$env:PATH = "C:\Users\HOTPOTFISH\go\go\bin;C:\Users\HOTPOTFISH\go\bin;$env:PATH"
$env:GOPROXY = "https://goproxy.cn,direct"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Building RW-Mod-Manager (Wails Production Build) " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

wails build -clean

if ($LASTEXITCODE -eq 0) {
    $exeItem = Get-ChildItem "build\bin\*.exe" | Select-Object -First 1
    if ($exeItem) {
        $fileSize = [math]::Round($exeItem.Length / 1MB, 2)
        Write-Host ""
        Write-Host "Build Successful!" -ForegroundColor Green
        Write-Host "Output: $($exeItem.FullName)" -ForegroundColor Green
        Write-Host "File Size: $fileSize MB" -ForegroundColor Green

        # 自动同步分发到常用目录 E:\Rusted Warfare
        $targetDir = "E:\Rusted Warfare"
        if (Test-Path $targetDir) {
            $destPath = Join-Path $targetDir $exeItem.Name
            Copy-Item -Path $exeItem.FullName -Destination $destPath -Force
            Write-Host "Auto-synced executable to: $destPath" -ForegroundColor Cyan

            $guideFile = "UserGuide.html"
            if (Test-Path $guideFile) {
                $guideDest = Join-Path $targetDir "RW-Mod-Manager-UserGuide.html"
                Copy-Item -Path $guideFile -Destination $guideDest -Force
                Write-Host "Auto-synced user guide to: $guideDest" -ForegroundColor Cyan
            }
        }
    }
} else {
    Write-Host "Build Failed!" -ForegroundColor Red
    exit 1
}
