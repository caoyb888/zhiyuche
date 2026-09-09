# 本机对 GitHub 没有推送权限（HTTPS 403、无 SSH key），远程编译机有。
# 本脚本把本地尚未推送的提交打成 bundle 传到远程，由远程 push，并让远程工作区对齐该提交。
# 用法：先在本地 git commit，然后 .\scripts\push-via-remote.ps1
param(
  [string]$Target = "devtest",
  [string]$Dir = "/home/xintong/zuche",
  [string]$Branch = "main"
)
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$bundle = Join-Path ([System.IO.Path]::GetTempPath()) "zuche-push.bundle"

git -C $root fetch -q origin
$base = git -C $root rev-parse "origin/$Branch"
$head = git -C $root rev-parse $Branch
if ($base -eq $head) { Write-Host "nothing to push (local $Branch == origin/$Branch)"; exit 0 }

git -C $root bundle create $bundle "origin/$Branch..$Branch" | Out-Null
scp -o BatchMode=yes $bundle "${Target}:/tmp/zuche-push.bundle"
if ($LASTEXITCODE -ne 0) { throw "scp failed" }

$remoteCmd = "cd $Dir && git fetch -q /tmp/zuche-push.bundle $Branch && git merge -q --ff-only FETCH_HEAD && git push origin $Branch && rm -f /tmp/zuche-push.bundle && git log --oneline -1"
ssh -o BatchMode=yes $Target $remoteCmd
if ($LASTEXITCODE -ne 0) { throw "remote push failed" }

git -C $root fetch -q origin
Write-Host ("origin/{0} is now {1}" -f $Branch, (git -C $root rev-parse --short "origin/$Branch"))
