<# 
.SYNOPSIS
    家庭网络反代 Windows 服务安装脚本

.DESCRIPTION
    将家庭网络反代安装为 Windows 系统服务，实现开机自启
    需要以管理员身份运行 PowerShell

.PARAMETER InstallPath
    家庭网络反代程序安装路径，默认: C:\Program Files\HNProxy

.PARAMETER ConfigPath
    配置文件路径，默认: C:\Program Files\HNProxy\hnproxy.conf

.EXAMPLE
    .\install_service.ps1
    .\install_service.ps1 -InstallPath "D:\HNProxy" -ConfigPath "D:\HNProxy\hnproxy.conf"
#>

param(
    [string]$InstallPath = "C:\Program Files\HNProxy",
    [string]$ConfigPath = "C:\Program Files\HNProxy\hnproxy.conf"
)

# 检查管理员权限
if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Error "请以管理员身份运行 PowerShell"
    exit 1
}

$ServiceName = "HNProxy"
$DisplayName = "家庭网络反代 (Home Network Proxy)"
$Description = "Personal and Home Network Tool - DDNS, Reverse Proxy, Port Forwarding, etc."
$BinaryPath = Join-Path $InstallPath "hnproxy.exe"
$Args = "-c `"$ConfigPath`""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  家庭网络反代 Windows 服务安装程序" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查二进制文件
if (-NOT (Test-Path $BinaryPath)) {
    Write-Error "未找到程序文件: $BinaryPath"
    Write-Host "请先将 hnproxy.exe 放置在: $InstallPath" -ForegroundColor Yellow
    exit 1
}

# 检查配置文件
if (-NOT (Test-Path $ConfigPath)) {
    Write-Warning "配置文件不存在: $ConfigPath"
    Write-Host "将创建默认配置文件..." -ForegroundColor Yellow
    
    $ConfigDir = Split-Path $ConfigPath
    if (-NOT (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    
    @"
# 家庭网络反代 配置文件 - 自动生成
web:
  port: 16601
  username: "666"
  password: "666"
  safe_url: ""
  enable_https: false

ddns:
  enabled: false
  providers: {}
  interval: 300

reverse_proxy:
  enabled: false
  services: []

socket_proxy:
  enabled: false
  services: []

storage:
  path: "$ConfigDir"

security:
  ip_whitelist: []
  ip_blacklist: []
  enable_waf: false
"@ | Out-File -FilePath $ConfigPath -Encoding UTF8
    
    Write-Host "默认配置文件已创建: $ConfigPath" -ForegroundColor Green
    Write-Host "默认用户名/密码: 666/666 (请尽快修改!)" -ForegroundColor Red
}

# 检查服务是否已存在
$ExistingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($ExistingService) {
    Write-Warning "服务 [$ServiceName] 已存在"
    $Confirm = Read-Host "是否重新安装? (y/N)"
    if ($Confirm -ne 'y' -and $Confirm -ne 'Y') {
        Write-Host "取消安装" -ForegroundColor Yellow
        exit 0
    }
    
    # 停止并删除现有服务
    Write-Host "停止现有服务..." -ForegroundColor Yellow
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    
    Write-Host "删除现有服务..." -ForegroundColor Yellow
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 1
}

# 创建服务
Write-Host "创建 Windows 服务..." -ForegroundColor Cyan
$Result = sc.exe create $ServiceName `
    binPath= "`"$BinaryPath`" $Args" `
    DisplayName= "$DisplayName" `
    Description= "$Description" `
    start= auto `
    obj= "LocalSystem" `
    error= normal

if ($LASTEXITCODE -ne 0) {
    Write-Error "服务创建失败 (错误代码: $LASTEXITCODE)"
    exit 1
}

Write-Host "服务创建成功!" -ForegroundColor Green

# 设置恢复选项（失败时重启）
Write-Host "配置服务恢复选项..." -ForegroundColor Cyan
sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/60000 | Out-Null

# 设置服务描述
$svc = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
$svc.Change($null, $null, $null, $null, $null, $null, $Description, $null, $null, $null) | Out-Null

# 启动服务
Write-Host "启动服务..." -ForegroundColor Cyan
try {
    Start-Service -Name $ServiceName -ErrorAction Stop
    Start-Sleep -Seconds 3
    
    $Status = (Get-Service -Name $ServiceName).Status
    if ($Status -eq 'Running') {
        Write-Host "" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "  安装完成！服务正在运行" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host ""
        Write-Host "服务名称: $ServiceName" -ForegroundColor Cyan
        Write-Host "显示名称: $DisplayName" -ForegroundColor Cyan
        Write-Host "程序路径: $BinaryPath" -ForegroundColor Cyan
        Write-Host "配置文件: $ConfigPath" -ForegroundColor Cyan
        Write-Host "Web 界面: http://localhost:16601" -ForegroundColor Cyan
        Write-Host "默认账号: 666" -ForegroundColor Cyan
        Write-Host "默认密码: 666" -ForegroundColor Red
        Write-Host ""
        Write-Host "常用命令:" -ForegroundColor Yellow
        Write-Host "  启动服务:   net start $ServiceName" -ForegroundColor Gray
        Write-Host "  停止服务:   net stop $ServiceName" -ForegroundColor Gray
        Write-Host "  重启服务:   net stop $ServiceName && net start $ServiceName" -ForegroundColor Gray
        Write-Host "  查看状态:   sc query $ServiceName" -ForegroundColor Gray
        Write-Host "  卸载服务:   .\uninstall_service.ps1" -ForegroundColor Gray
    } else {
        Write-Warning "服务创建但启动失败，状态: $Status"
        Write-Host "请检查配置文件和日志: $ConfigPath" -ForegroundColor Yellow
    }
} catch {
    Write-Error "服务启动失败: $_"
    exit 1
}