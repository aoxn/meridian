// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// startVM calls WSL to start a vm.
func startVM(ctx context.Context, distroName string) error {
	cmd := []string{
		"wsl.exe",
		"--distribution",
		distroName,
	}
	out, err := RunUTF16leCommand(cmd, WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --distribution %s`: %w (out=%q)",
			distroName, err, string(out))
	}
	return nil
}

// initVM calls WSL to import a new vm specifically for Lima.
func initVM(ctx context.Context, instanceDir, distroName string) error {
	baseDisk := filepath.Join(instanceDir, v1.BaseDisk)
	klog.Infof("Importing distro from %q to %q", baseDisk, instanceDir)
	cmd := []string{
		"wsl.exe",
		"--import",
		distroName,
		instanceDir,
		baseDisk,
	}
	out, err := RunUTF16leCommand(cmd, WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --import %s %s %s`: %w (out=%q)",
			distroName, instanceDir, baseDisk, err, string(out))
	}
	return nil
}

// stopVM calls WSL to stop a running vm.
func stopVM(ctx context.Context, distroName string) error {
	cmd := []string{
		"wsl.exe",
		"--terminate",
		distroName,
	}
	out, err := RunUTF16leCommand(cmd, WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --terminate %s`: %w (out=%q)",
			distroName, err, string(out))
	}
	return nil
}

//go:embed lima-init.TEMPLATE
var limaBoot string

// provisionVM starts Lima's boot process inside an already imported vm.
func provisionVM(ctx context.Context, instanceDir, instanceName, distroName string, errCh *chan error) error {
	m := map[string]string{
		"CIDataPath": filepath.Join(instanceDir, v1.CIDataISODir),
	}
	limaBootB, err := ExecuteTemplate(limaBoot, m)
	if err != nil {
		return fmt.Errorf("failed to construct wsl boot.sh script: %w", err)
	}
	bootFile, err := os.CreateTemp("", "lima-wsl2-boot-*.sh")
	if err != nil {
		return err
	}
	defer bootFile.Close()
	_, err = bootFile.Write(limaBootB)
	if err != nil {
		return err
	}
	fullPath := bootFile.Name()
	// path should be quoted and use \\ as separator
	wslBootFilePath := strconv.Quote(fullPath)
	args := []string{
		"-d",
		distroName,
		"bash",
		"-c",
		fmt.Sprintf("wslpath -u %s", wslBootFilePath),
		wslBootFilePath,
	}
	linuxBootFilePath, err := exec.Command("wsl.exe", args...).Output()
	if err != nil {
		_ = os.RemoveAll(fullPath)
		// this can return an error with an exit code, which causes it not to be logged
		// because main.handleExitCoder() traps it, so wrap the error
		return fmt.Errorf("failed to run wslpath command: %w", err)
	}
	limaBootFileLinuxPath := strings.TrimSpace(string(linuxBootFilePath))
	go func() {
		args := []string{
			"-d",
			distroName,
			"bash",
			"-c",
			limaBootFileLinuxPath,
		}
		cmd := exec.CommandContext(ctx, "wsl.exe", args...)
		out, err := cmd.CombinedOutput()
		_ = os.RemoveAll(wslBootFilePath)
		klog.Infof("debug cmd: %v: %q", cmd.Args, string(out))
		if err != nil {
			*errCh <- fmt.Errorf(
				"error running wslCommand that executes boot.sh (%v): %w, "+
					"check /var/log/lima-init.log for more details (out=%q)", cmd.Args, err, string(out))
		}

		for {
			<-ctx.Done()
			klog.Info("Context closed, stopping vm")
			if status, err := GetWslStatus(instanceName); err == nil &&
				status == StatusRunning {
				_ = stopVM(ctx, distroName)
			}
		}
	}()

	return err
}

// keepAlive runs a background process which in order to keep the WSL2 vm running in the background after launch.
func keepAlive(ctx context.Context, distroName string, errCh *chan error) {
	keepAliveCmd := exec.CommandContext(
		ctx,
		"wsl.exe",
		"-d",
		distroName,
		"bash",
		"-c",
		"nohup sleep 2147483647d >/dev/null 2>&1",
	)

	go func() {
		if err := keepAliveCmd.Run(); err != nil {
			*errCh <- fmt.Errorf(
				"error running wsl keepAlive command: %w", err)
		}
	}()
}

// unregisterVM calls WSL to unregister a vm.
func unregisterVM(ctx context.Context, distroName string) error {
	klog.Info("Unregistering WSL2 vm")
	cmd := []string{
		"wsl.exe",
		"--unregister",
		distroName,
	}
	out, err := RunUTF16leCommand(cmd, WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --unregister %s`: %w (out=%q)",
			distroName, err, string(out))
	}
	return nil
}

// ExecuteTemplate executes a text/template template.
func ExecuteTemplate(tmpl string, args interface{}) ([]byte, error) {
	x, err := template.New("").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := x.Execute(&b, args); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

const (
	guestCommunicationsPrefix = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
	magicVSOCKSuffix          = "-facb-11e6-bd58-64006a7986d3"
	wslDistroInfoPrefix       = `SOFTWARE\Microsoft\Windows\CurrentVersion\Lxss`
)

// vmIDRegex is a regular expression to extract the VM ID from the command line of wslhost.exe.
var vmIDRegex = regexp.MustCompile(`--vm-id\s\{(?P<vmID>.{36})\}`)

// GetInstanceVMID returns the VM ID of a running WSL instance.
func GetInstanceVMID(instanceName string) (string, error) {
	distroID, err := GetDistroID(instanceName)
	if err != nil {
		return "", err
	}

	cmdLines, err := GetProcessCommandLine("wslhost.exe")
	if err != nil {
		return "", err
	}

	vmID := ""
	for _, cmdLine := range cmdLines {
		if strings.Contains(cmdLine, distroID) {
			if matches := vmIDRegex.FindStringSubmatch(cmdLine); matches != nil {
				vmID = matches[vmIDRegex.SubexpIndex("vmID")]
				break
			}
		}
	}

	if vmID == "" {
		return "", fmt.Errorf("failed to find VM ID for instance %q", instanceName)
	}

	return vmID, nil
}

// GetDistroID returns a DistroId GUID corresponding to a Lima instance name.
func GetDistroID(name string) (string, error) {
	rootKey, err := registry.OpenKey(
		registry.CURRENT_USER,
		wslDistroInfoPrefix,
		registry.READ,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to open Lxss key (%s): %w",
			wslDistroInfoPrefix,
			err,
		)
	}
	defer rootKey.Close()

	keys, err := rootKey.ReadSubKeyNames(-1)
	if err != nil {
		return "", fmt.Errorf("failed to read subkey names for %s: %w", wslDistroInfoPrefix, err)
	}

	var out string
	for _, k := range keys {
		subKey, err := registry.OpenKey(
			registry.CURRENT_USER,
			fmt.Sprintf(`%s\%s`, wslDistroInfoPrefix, k),
			registry.READ,
		)
		if err != nil {
			return "", fmt.Errorf("failed to read subkey %q for key %q: %w", k, wslDistroInfoPrefix, err)
		}
		dn, _, err := subKey.GetStringValue("DistributionName")
		if err != nil {
			return "", fmt.Errorf("failed to read 'DistributionName' value for subkey %q of %q: %w", k, wslDistroInfoPrefix, err)
		}
		if dn == name {
			out = k
			break
		}
	}

	if out == "" {
		return "", fmt.Errorf("failed to find matching DistroID for %q", name)
	}

	return out, nil
}

type CommandLineJSON []struct {
	CommandLine string
}

// GetProcessCommandLine returns a slice of string containing all commandlines for a given process name.
func GetProcessCommandLine(name string) ([]string, error) {
	args := []string{
		"-nologo",
		"-noprofile",
		fmt.Sprintf(
			`Get-CimInstance Win32_Process -Filter "name = '%s'" | Select CommandLine | ConvertTo-Json`,
			name,
		),
	}
	out, err := exec.Command("powershell.exe", args...).CombinedOutput()
	if err != nil {
		return nil, err
	}

	var outJSON CommandLineJSON
	if err = json.Unmarshal(out, &outJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %q as %T: %w", out, outJSON, err)
	}

	var ret []string
	for _, s := range outJSON {
		ret = append(ret, s.CommandLine)
	}

	return ret, nil
}

type Status = string

const (
	StatusUnknown       Status = ""
	StatusUninitialized Status = "Uninitialized"
	StatusInstalling    Status = "Installing"
	StatusBroken        Status = "Broken"
	StatusStopped       Status = "Stopped"
	StatusRunning       Status = "Running"
)

// GetWslStatus runs `wsl --list --verbose` and parses its output.
// There are several possible outputs, all listed with their whitespace preserved output below.
//
// (1) Expected output if at least one distro is installed:
// PS > wsl --list --verbose
//
//	NAME      STATE           VERSION
//
// * Ubuntu    Stopped         2
//
// (2) Expected output when no distros are installed, but WSL is configured properly:
// PS > wsl --list --verbose
// Windows Subsystem for Linux has no installed distributions.
//
// Use 'wsl.exe --list --online' to list available distributions
// and 'wsl.exe --install <Distro>' to install.
//
// Distributions can also be installed by visiting the Microsoft Store:
// https://aka.ms/wslstore
// Error code: Wsl/WSL_E_DEFAULT_DISTRO_NOT_FOUND
//
// (3) Expected output when no distros are installed, and WSL2 has no kernel installed:
//
// PS > wsl --list --verbose
// Windows Subsystem for Linux has no installed distributions.
// Distributions can be installed by visiting the Microsoft Store:
// https://aka.ms/wslstore
func GetWslStatus(instName string) (string, error) {
	distroName := "md-" + instName
	cmd := []string{
		"wsl.exe",
		"--list",
		"--verbose",
	}
	out, err := RunUTF16leCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to run `wsl --list --verbose`, err: %w (out=%q)", err, string(out))
	}

	if len(out) == 0 {
		return StatusBroken, fmt.Errorf("failed to read instance state for instance %q, try running `wsl --list --verbose` to debug, err: %w", instName, err)
	}

	// Check for edge cases first
	outString := string(out)
	if strings.Contains(outString, "Windows Subsystem for Linux has no installed distributions.") {
		if strings.Contains(outString, "Wsl/WSL_E_DEFAULT_DISTRO_NOT_FOUND") {
			return StatusBroken, fmt.Errorf(
				"failed to read instance state for instance %q because no distro is installed,"+
					"try running `wsl --install -d Ubuntu` and then re-running Lima", instName)
		}
		return StatusBroken, fmt.Errorf(
			"failed to read instance state for instance %q because there is no WSL kernel installed,"+
				"this usually happens when WSL was installed for another user, but never for your user."+
				"Try running `wsl --install -d Ubuntu` and `wsl --update`, and then re-running Lima", instName)
	}

	var instState string
	wslListColsRegex := regexp.MustCompile(`\s+`)
	// wsl --list --verbose may have different headers depending on localization, just split by line
	for _, rows := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		cols := wslListColsRegex.Split(strings.TrimSpace(rows), -1)
		nameIdx := 0
		// '*' indicates default instance
		if cols[0] == "*" {
			nameIdx = 1
		}
		if cols[nameIdx] == distroName {
			instState = cols[nameIdx+1]
			break
		}
	}

	if instState == "" {
		return StatusUninitialized, nil
	}

	return instState, nil
}

func RunUTF16leCommand(args []string, opts ...Opt) (string, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return "", err
		}
	}

	var cmd *exec.Cmd
	if o.ctx != nil {
		cmd = exec.CommandContext(o.ctx, args[0], args[1:]...)
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	outString := ""
	out, err := cmd.CombinedOutput()
	if out != nil {
		s, err := FromUTF16leToString(bytes.NewReader(out))
		if err != nil {
			return "", fmt.Errorf("failed to convert output from UTF16 when running command %v, err: %w", args, err)
		}
		outString = s
	}
	return outString, err
}

// WithContext runs the command with CommandContext.
func WithContext(ctx context.Context) Opt {
	return func(o *options) error {
		o.ctx = ctx
		return nil
	}
}

type options struct {
	ctx context.Context
}

type Opt func(*options) error

// FromUTF16le returns an io.Reader for UTF16le data.
// Windows uses little endian by default, use unicode.UseBOM policy to retrieve BOM from the text,
// and unicode.LittleEndian as a fallback.
func FromUTF16le(r io.Reader) io.Reader {
	o := transform.NewReader(r, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
	return o
}

// FromUTF16leToString reads from Unicode 16 LE encoded data from an io.Reader and returns a string.
func FromUTF16leToString(r io.Reader) (string, error) {
	out, err := io.ReadAll(FromUTF16le(r))
	if err != nil {
		return "", err
	}

	return string(out), nil
}
