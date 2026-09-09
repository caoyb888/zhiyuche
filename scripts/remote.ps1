# 在远程编译机的项目目录里执行命令（已带 Go 的 PATH）。
# 用法：.\scripts\remote.ps1 "make test"
#       .\scripts\remote.ps1 "cd apps/web && npm run build"
param(
  [Parameter(Mandatory = $true, Position = 0)][string]$Command,
  [string]$Target = "devtest",
  [string]$Dir = "/home/xintong/zuche"
)
$ErrorActionPreference = "Stop"
$wrapped = "export PATH=`$PATH:/usr/local/go/bin:`$HOME/go/bin; cd $Dir && $Command"
ssh -o BatchMode=yes $Target $wrapped
exit $LASTEXITCODE
