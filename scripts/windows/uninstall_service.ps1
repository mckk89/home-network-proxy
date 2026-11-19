<# 
.SYNOPSIS
    家庭网络反代 Windows 服务卸载脚本

.DESCRIPTION
    卸载 家庭网络反代 Windows 系统服务
    需要以管理员身份运行 PowerShell
#>

param(
    [string]$ServiceName = "HNProxy"
)

# 检查管理员权限
if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Error "请以管理员身份运行 PowerShell"
    exit 1
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  家庭网络反代 Windows 服务卸载程序" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查服务是否存在
$Service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-NOT $Service) {
    Write-Warning "服务 [$ServiceName] 不存在"
    exit 0
}

Write-Host "发现服务: $($Service.DisplayName) ($ServiceName)" -ForegroundColor Yellow
Write-Host "当前状态: $($Service.Status)" -ForegroundColor Yellow
Write-Host ""

$Confirm = Read-Host "确定要卸载该服务吗? (y/N)"
if ($Confirm -ne 'y' -and $Confirm -ne 'Y') {
    Write-Host "取消卸载" -ForegroundColor Yellow
    exit 0
}

# 停止服务
if ($Service.Status -eq 'Running') {
    Write-Host "停止服务..." -ForegroundColor Cyan
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

# 删除服务
Write-Host "删除服务..." -ForegroundColor Cyan
$Result = sc.exe delete $ServiceName

if ($LASTEXITCODE -eq 0) {
    Write-Host "" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  卸载完成！" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
} else {
    Write-Error "卸载失败 (错误代码: $LASTEXITCODE)"
    exit 1
}