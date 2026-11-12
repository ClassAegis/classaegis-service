param([string]$InstallRoot = 'D:\ClassAegisService')
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class InstalledWindowProbe {
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
}
"@
function Get-AppRoot {
    $script:app = @(Get-Process ClassAegisService | Where-Object { $_.Path -eq (Join-Path $InstallRoot 'ClassAegisService.exe') })
    if ($script:app.Count -ne 1) { throw "Expected one installed app; found $($script:app.Count)" }
    $condition = [System.Windows.Automation.AndCondition]::new(
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $script:app[0].Id),
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'Tauri Window'))
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($windows.Count -ne 1) { throw "Expected one real Tauri window; found $($windows.Count)" }
    if ($windows[0].Current.IsOffscreen) { throw 'The installed application window is offscreen or minimized' }
    return $windows[0]
}
function Get-Text {
    $root = Get-AppRoot
    return ($root.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition) | ForEach-Object { $_.Current.Name }) -join "`n"
}
function Click-Button([string]$Name) {
    $root = Get-AppRoot
    $condition = [System.Windows.Automation.AndCondition]::new(
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, $Name),
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::Button))
    $buttons = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)
    if ($buttons.Count -ne 1) { throw "Expected one button '$Name'; found $($buttons.Count)" }
    ([System.Windows.Automation.InvokePattern]$buttons[0].GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)).Invoke()
    Start-Sleep -Milliseconds 700
}
function Assert-Text([string]$Expected) {
    $text = Get-Text
    if (!$text.Contains($Expected) -or $text.Contains('ERR_FILE_NOT_FOUND')) { throw "Window text missing '$Expected': $text" }
    Write-Output "PASS: $Expected"
}
foreach ($name in @('ClassAegisCore.exe', 'ClassAegisAbility.exe', 'ClassAegisLauncher.exe', 'ClassAegisCtl.exe', 'ClassAegisDebug.exe')) {
    if (!(Test-Path (Join-Path $InstallRoot "bin/$name") -PathType Leaf)) { throw "Installed component missing: $name" }
}
if (!(Test-Path (Join-Path $InstallRoot 'conf/config.example.toml') -PathType Leaf)) { throw 'Configuration template missing' }
Write-Output 'PASS: all five Service components and configuration template installed'
$initialId = (Get-AppRoot).Current.ProcessId
Assert-Text '三层防护链路'
Click-Button '安全日志'
Assert-Text '运行日志'
Click-Button '关于课佑'
Assert-Text ([Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $InstallRoot 'ClassAegisService.exe')).ProductVersion)
Click-Button '首页'
Assert-Text '三层防护链路'
$mainHandle = [IntPtr](Get-AppRoot).Current.NativeWindowHandle
Click-Button '×'
Start-Sleep -Milliseconds 500
if ([InstalledWindowProbe]::IsWindowVisible($mainHandle)) { throw 'Custom close button did not hide the window' }
Write-Output 'PASS: custom close hides the window while keeping the process alive'
Start-Process (Join-Path $InstallRoot 'ClassAegisService.exe')
Start-Sleep -Seconds 2
Assert-Text '三层防护链路'
if ($script:app[0].Id -ne $initialId) { throw 'Original process was replaced' }
Write-Output 'PASS: launching again restores the same instance'
$mainHandle = [IntPtr](Get-AppRoot).Current.NativeWindowHandle
Click-Button '—'
if (![InstalledWindowProbe]::IsIconic($mainHandle)) { throw 'Minimize button did not minimize the window' }
Write-Output 'PASS: custom minimize button'
Start-Process (Join-Path $InstallRoot 'ClassAegisService.exe')
Start-Sleep -Seconds 2
if ([InstalledWindowProbe]::IsIconic($mainHandle)) { throw 'Launching again did not restore the minimized window' }
Assert-Text '三层防护链路'
Write-Output 'PASS: launching again restores the minimized instance'