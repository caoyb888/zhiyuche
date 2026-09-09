# 把本地工作区（git 跟踪 + 未忽略的新文件）同步到远程编译机，不需要先提交。
# 用法：.\scripts\sync-remote.ps1            默认 devtest:/home/xintong/zuche
#       .\scripts\sync-remote.ps1 -Target devtest -Dir /home/xintong/zuche
param(
  [string]$Target = "devtest",
  [string]$Dir = "/home/xintong/zuche"
)
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$list = Join-Path ([System.IO.Path]::GetTempPath()) "zuche-sync-files.txt"

# -c: 已跟踪  -o: 未跟踪  --exclude-standard: 尊重 .gitignore
$files = git -C $root ls-files -co --exclude-standard
[System.IO.File]::WriteAllLines($list, $files, (New-Object System.Text.UTF8Encoding($false)))
Write-Host ("sync {0} files -> {1}:{2}" -f $files.Count, $Target, $Dir)

# 用 cmd 做二进制管道，PowerShell 管道会破坏 tar 字节流
$cmd = "tar -C `"$root`" -cf - -T `"$list`" | ssh -o BatchMode=yes $Target `"mkdir -p $Dir && tar -xf - -C $Dir`""
cmd /c $cmd
if ($LASTEXITCODE -ne 0) { throw "sync failed (exit $LASTEXITCODE)" }
Write-Host "done"
