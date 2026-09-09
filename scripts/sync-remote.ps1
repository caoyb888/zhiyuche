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
# 必须写 LF 行尾：远程 comm/sort 逐行比对，CRLF 会让每一行都不匹配
[System.IO.File]::WriteAllText($list, (($files -join "`n") + "`n"), (New-Object System.Text.UTF8Encoding($false)))
Write-Host ("sync {0} files -> {1}:{2}" -f $files.Count, $Target, $Dir)

# 用 cmd 做二进制管道，PowerShell 管道会破坏 tar 字节流
$cmd = "tar -C `"$root`" -cf - -T `"$list`" | ssh -o BatchMode=yes $Target `"mkdir -p $Dir && tar -xf - -C $Dir`""
cmd /c $cmd
if ($LASTEXITCODE -ne 0) { throw "sync failed (exit $LASTEXITCODE)" }

# 删除远程仍被 git 跟踪、但本地已不存在的文件（tar 只增不删，否则旧文件会让编译失败）
scp -q -o BatchMode=yes $list "${Target}:/tmp/zuche-sync-list.txt"
scp -q -o BatchMode=yes (Join-Path $PSScriptRoot "remote-prune.sh") "${Target}:/tmp/zuche-prune.sh"
ssh -o BatchMode=yes $Target "sed -i 's/\r$//' /tmp/zuche-prune.sh && bash /tmp/zuche-prune.sh $Dir /tmp/zuche-sync-list.txt"
Write-Host "done"
