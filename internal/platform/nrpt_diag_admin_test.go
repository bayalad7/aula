//go:build windows && admintest

package platform

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestAdminNRPTDiagnostics is temporary: it logs what Windows does with an
// NRPT rule on the CI runner, to understand why the listing came out empty.
func TestAdminNRPTDiagnostics(t *testing.T) {
	requireAdmin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	exe := filepath.Join(windowsDir(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	script := `$ErrorActionPreference='Continue'
'--- adapters'; Get-DnsClientServerAddress | Format-Table InterfaceAlias, AddressFamily, ServerAddresses -AutoSize | Out-String -Width 200
'--- nrpt before'; Get-DnsClientNrptRule | Format-List * | Out-String -Width 200
'--- add'; Add-DnsClientNrptRule -Namespace '.' -NameServers '127.0.0.1' -Comment 'aula-diag' -PassThru 2>&1 | Format-List * | Out-String -Width 200
"--- add exit: $?"
'--- nrpt after'; Get-DnsClientNrptRule | Format-List * | Out-String -Width 200
'--- policy'; Get-DnsClientNrptPolicy | Format-List * | Out-String -Width 200
'--- registry'; reg query HKLM\SYSTEM\CurrentControlSet\Services\Dnscache\Parameters\DnsPolicyConfig /s 2>&1 | Out-String
'--- registry gpo'; reg query "HKLM\SOFTWARE\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig" /s 2>&1 | Out-String
'--- resolve'; Resolve-DnsName example.com -Type A -DnsOnly 2>&1 | Format-Table -AutoSize | Out-String -Width 200
'--- remove'; Get-DnsClientNrptRule | Where-Object { $_.Comment -eq 'aula-diag' } | ForEach-Object { Remove-DnsClientNrptRule -Name $_.Name -Force }
'--- nrpt end'; Get-DnsClientNrptRule | Format-List * | Out-String -Width 200`
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	t.Logf("exit %v\n%s", err, out)
}
