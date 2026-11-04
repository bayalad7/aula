package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// inspectTimeout bounds the PowerShell run: listing the firewall filters
// takes a few seconds on a machine with many rules.
const inspectTimeout = 30 * time.Second

func newNetworkInspector() (NetworkInspector, error) { return winNetwork{}, nil }

type winNetwork struct{}

// networkScript reads the network profiles (NetConnection cmdlets) and the
// firewall through its COM API, HNetCfg.FwPolicy2, which a standard user can
// read (the NetSecurity cmdlets ask for an administrator). It prints JSON
// with numbers and enum names, which do not depend on the Windows display
// language (netsh output does).
//
// Profile bits: 1 Domain, 2 Private, 4 Public. Rule direction 1 is inbound,
// action 1 allows, protocol 6 is TCP.
const networkScript = `
$nets = @(Get-NetConnectionProfile | ForEach-Object {
  [pscustomobject]@{name=[string]$_.Name; interface=[string]$_.InterfaceAlias; category=[string]$_.NetworkCategory} })
$fw = New-Object -ComObject HNetCfg.FwPolicy2
$profiles = @(@(1,'Domain'),@(2,'Private'),@(4,'Public') | ForEach-Object {
  [pscustomobject]@{name=$_[1]; enabled=[bool]$fw.FirewallEnabled($_[0]); inboundAllowed=($fw.DefaultInboundAction($_[0]) -eq 1)} })
$rules = @($fw.Rules | Where-Object { $_.Enabled -and $_.Direction -eq 1 -and (
  ($_.ApplicationName -and [Environment]::ExpandEnvironmentVariables($_.ApplicationName) -ieq $prog) -or
  (-not $_.ApplicationName -and $_.Protocol -eq 6 -and (@($_.LocalPorts -split ',') -contains $port)) ) } | ForEach-Object {
  [pscustomobject]@{name=[string]$_.Name; allow=($_.Action -eq 1); profiles=[int]$_.Profiles} })
[pscustomobject]@{networks=$nets; firewall=$profiles; rules=$rules} | ConvertTo-Json -Depth 4 -Compress
`

// psQuote quotes s as a PowerShell single-quoted string.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (winNetwork) Inspect(ctx context.Context, program string, port int) (NetworkState, error) {
	ctx, cancel := context.WithTimeout(ctx, inspectTimeout)
	defer cancel()
	script := "$ErrorActionPreference='Stop'; $prog=" + psQuote(program) + "; $port=" + psQuote(strconv.Itoa(port)) + ";" + networkScript
	exe := filepath.Join(windowsDir(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	// The console is a GUI app: without this a console window would flash.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return NetworkState{}, fmt.Errorf("network inspect: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return parseNetworkState(out)
}
