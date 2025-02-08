package sshutil

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vma/model"
	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"golang.org/x/sys/cpu"
	"io"
	"io/fs"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	sshc "github.com/lima-vm/sshocker/pkg/ssh"
	sshk "golang.org/x/crypto/ssh"
)

func NewSSHMgr(inst, addr string, port int) *SSHMgr {
	return &SSHMgr{
		vmName:  inst,
		port:    port,
		address: addr,
	}
}

type SSHMgr struct {
	port    int
	address string
	vmName  string
	config  *sshc.SSHConfig
}

func (ssh *SSHMgr) GetPort() int {
	return ssh.port
}

func (ssh *SSHMgr) SetPort(port int) {
	ssh.port = port
}

func (ssh *SSHMgr) GetAddr() string {
	return ssh.address
}

func (ssh *SSHMgr) SetAddr(addr string) {
	ssh.address = addr
}

func (ssh *SSHMgr) RunCommand(cmd string) (string, error) {
	configDir, err := model.MdConfigDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(configDir, v1.UserPrivateKey))
	if err != nil {
		return "", err
	}
	signer, err := sshk.ParsePrivateKey(data)
	if err != nil {
		return "", err
	}
	config := &sshk.ClientConfig{
		User: ssh.vmName,
		Auth: []sshk.AuthMethod{
			sshk.PublicKeys(signer),
		},
		HostKeyCallback: sshk.InsecureIgnoreHostKey(),
	}
	klog.Infof("debug ssh %s@%s %s", ssh.vmName, ssh.address, cmd)

	client, err := sshk.Dial("tcp", ssh.address, config)
	if err != nil {
		return "", errors.Wrapf(err, "failed to connect to %s", ssh.address)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", errors.Wrapf(err, "failed to create session")
	}
	defer session.Close()
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(cmd); err != nil {
		return "", errors.Wrapf(err, "failed to run %s", cmd)
	}
	return b.String(), nil
}

func (ssh *SSHMgr) SSHConfig(instDir string) (*sshc.SSHConfig, error) {
	if ssh.config != nil {
		return ssh.config, nil
	}
	sshOpts, err := SSHOpts(instDir, true, true, true, true)
	if err != nil {
		return nil, err
	}

	ssh.config = &sshc.SSHConfig{
		AdditionalArgs: SSHArgsFromOpts(sshOpts),
	}
	return ssh.config, nil
}

func (ssh *SSHMgr) WriteSSHConfigFile(name, instDir string) error {

	sshOpts, err := SSHOpts(instDir, true, true, true, true)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	if _, err := fmt.Fprintf(&b, `# This SSH config file can be passed to 'ssh -F'.
# This file is created by Lima, but not used by Lima itself currently.
# Modifications to this file will be lost on restarting the Lima instance.
`); err != nil {
		return err
	}
	if err := Format(&b, name, FormatConfig,
		append(sshOpts,
			fmt.Sprintf("Hostname=%s", ssh.address),
			fmt.Sprintf("Port=%d", ssh.port),
		)); err != nil {
		return err
	}
	fileName := filepath.Join(instDir, v1.SSHConfig)
	return os.WriteFile(fileName, b.Bytes(), 0o600)
}

// LoadPubKey returns the public key from $MD_HOME/_config/user.pub.
// The key will be created if it does not yet exist.
//
// When loadDotSSH is true, ~/.ssh/*.pub will be appended to make the VM accessible without specifying
// an identity explicitly.
func (ssh *SSHMgr) LoadPubKey() ([]PubKey, error) {
	var pubkeys []PubKey
	configDir, err := model.MdConfigDir()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath.Join(configDir, v1.UserPrivateKey))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return nil, fmt.Errorf("could not create %q directory: %w", configDir, err)
		}
		args := []string{
			"-t", "ed25519",
			"-q", "-N", "",
			"-C", "meridian",
			"-f", filepath.Join(configDir, v1.UserPrivateKey),
		}
		// no passphrase, no user@host comment
		keygenCmd := exec.Command("ssh-keygen", args...)
		if out, err := keygenCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to run %v: %q: %w", keygenCmd.Args, string(out), err)
		}
		klog.Infof("ssh public key generated for %q", ssh.address)
	}
	entry, err := readPublicKey(filepath.Join(configDir, v1.UserPublicKey))
	if err != nil {
		return nil, err
	}
	pubkeys = append(pubkeys, entry)

	// Append all of ~/.ssh/*.pub
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(homeDir, ".ssh/*.pub"))
	if err != nil {
		panic(err) // Only possible error is ErrBadPattern, so this should be unreachable.
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".pub") {
			klog.Infof("skipping public key %q", f)
			continue
		}
		entry, err := readPublicKey(f)
		if err != nil {
			klog.Warningf("read public key %q: %v", f, err)
			continue
		}
		if !detectValidPublicKey(entry.Content) {
			klog.Warningf("public key %q doesn't seem to be in ssh format", entry.Filename)
			continue
		}
		pubkeys = append(pubkeys, entry)
	}
	return pubkeys, nil
}

var sshInfo struct {
	sync.Once
	// aesAccelerated is set to true when AES acceleration is available.
	// Available on almost all modern Intel/AMD processors.
	aesAccelerated bool
	// openSSHVersion is set to the version of OpenSSH, or semver.New("0.0.0") if the version cannot be determined.
	openSSHVersion semver.Version
}

type PubKey struct {
	Filename string
	Content  string
}

func readPublicKey(f string) (PubKey, error) {
	entry := PubKey{
		Filename: f,
	}
	content, err := os.ReadFile(f)
	if err == nil {
		entry.Content = strings.TrimSpace(string(content))
	} else {
		err = fmt.Errorf("failed to read ssh public key %q: %w", f, err)
	}
	return entry, err
}

// CommonOpts returns ssh option key-value pairs like {"IdentityFile=/path/to/id_foo"}.
// The result may contain different values with the same key.
//
// The result always contains the IdentityFile option.
// The result never contains the Port option.
func CommonOpts(useDotSSH bool) ([]string, error) {
	configDir, err := model.MdConfigDir()
	if err != nil {
		return nil, err
	}
	privateKeyPath := filepath.Join(configDir, v1.UserPrivateKey)
	_, err = os.Stat(privateKeyPath)
	if err != nil {
		return nil, err
	}
	var opts []string
	if runtime.GOOS == "windows" {
		opts = []string{fmt.Sprintf(`IdentityFile='%s'`, privateKeyPath)}
	} else {
		opts = []string{fmt.Sprintf(`IdentityFile="%s"`, privateKeyPath)}
	}

	// Append all private keys corresponding to ~/.ssh/*.pub to keep old instances working
	// that had been created before lima started using an internal identity.
	if useDotSSH {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		files, err := filepath.Glob(filepath.Join(homeDir, ".ssh/*.pub"))
		if err != nil {
			panic(err) // Only possible error is ErrBadPattern, so this should be unreachable.
		}
		for _, f := range files {
			if !strings.HasSuffix(f, ".pub") {
				panic(fmt.Errorf("unexpected ssh public key filename %q", f))
			}
			privateKeyPath := strings.TrimSuffix(f, ".pub")
			_, err = os.Stat(privateKeyPath)
			if errors.Is(err, fs.ErrNotExist) {
				// Skip .pub files without a matching private key. This is reasonably common,
				// due to major projects like Vault recommending the ${name}-cert.pub format
				// for SSH certificate files.
				//
				// e.g. https://www.vaultproject.io/docs/secrets/ssh/signed-ssh-certificates
				continue
			}
			if err != nil {
				// Fail on permission-related and other path errors
				return nil, err
			}
			if runtime.GOOS == "windows" {
				opts = append(opts, fmt.Sprintf(`IdentityFile='%s'`, privateKeyPath))
			} else {
				opts = append(opts, fmt.Sprintf(`IdentityFile="%s"`, privateKeyPath))
			}
		}
	}

	opts = append(opts,
		"StrictHostKeyChecking=no",
		"UserKnownHostsFile=/dev/null",
		"NoHostAuthenticationForLocalhost=yes",
		"GSSAPIAuthentication=no",
		"PreferredAuthentications=publickey",
		"Compression=no",
		"BatchMode=yes",
		"IdentitiesOnly=yes",
	)

	sshInfo.Do(func() {
		sshInfo.aesAccelerated = detectAESAcceleration()
		sshInfo.openSSHVersion = DetectOpenSSHVersion()
	})

	// Only OpenSSH version 8.1 and later support adding ciphers to the front of the default set
	if !sshInfo.openSSHVersion.LessThan(*semver.New("8.1.0")) {
		// By default, `ssh` choose chacha20-poly1305@openssh.com, even when AES accelerator is available.
		// (OpenSSH_8.1p1, macOS 11.6, MacBookPro 2020, Core i7-1068NG7)
		//
		// We prioritize AES algorithms when AES accelerator is available.
		if sshInfo.aesAccelerated {
			klog.Infof("AES accelerator seems available, prioritizing aes128-gcm@openssh.com and aes256-gcm@openssh.com")
			if runtime.GOOS == "windows" {
				opts = append(opts, "Ciphers=^aes128-gcm@openssh.com,aes256-gcm@openssh.com")
			} else {
				opts = append(opts, "Ciphers=\"^aes128-gcm@openssh.com,aes256-gcm@openssh.com\"")
			}
		} else {
			klog.Infof("AES accelerator does not seem available, prioritizing chacha20-poly1305@openssh.com")
			if runtime.GOOS == "windows" {
				opts = append(opts, "Ciphers=^chacha20-poly1305@openssh.com")
			} else {
				opts = append(opts, "Ciphers=\"^chacha20-poly1305@openssh.com\"")
			}
		}
	}
	return opts, nil
}

// SSHOpts adds the following options to CommonOptions: User, ControlMaster, ControlPath, ControlPersist.
func SSHOpts(instDir string, useDotSSH, forwardAgent, forwardX11, forwardX11Trusted bool) ([]string, error) {
	controlSock := filepath.Join(instDir, v1.SSHSock)
	if len(controlSock) >= model.UnixPathMax {
		return nil, fmt.Errorf("socket path %q is too long: >= UNIX_PATH_MAX=%d", controlSock, model.UnixPathMax)
	}
	u, err := model.MdUser(false)
	if err != nil {
		return nil, err
	}
	opts, err := CommonOpts(useDotSSH)
	if err != nil {
		return nil, err
	}
	controlPath := fmt.Sprintf(`ControlPath="%s"`, controlSock)
	if runtime.GOOS == "windows" {
		controlPath = fmt.Sprintf(`ControlPath='%s'`, controlSock)
	}
	opts = append(opts,
		fmt.Sprintf("User=%s", u.Username), // guest and host have the same username, but we should specify the username explicitly (#85)
		"ControlMaster=auto",
		controlPath,
		"ControlPersist=yes",
	)
	if forwardAgent {
		opts = append(opts, "ForwardAgent=yes")
	}
	if forwardX11 {
		opts = append(opts, "ForwardX11=yes")
	}
	if forwardX11Trusted {
		opts = append(opts, "ForwardX11Trusted=yes")
	}
	return opts, nil
}

// SSHArgsFromOpts returns ssh args from opts.
// The result always contains {"-F", "/dev/null} in addition to {"-o", "KEY=VALUE", ...}.
func SSHArgsFromOpts(opts []string) []string {
	args := []string{"-F /dev/null"}
	for _, o := range opts {
		args = append(args, fmt.Sprintf("-o %s", o))
	}
	return args
}

func ParseOpenSSHVersion(version []byte) *semver.Version {
	regex := regexp.MustCompile(`^OpenSSH_(\d+\.\d+)(?:p(\d+))?\b`)
	matches := regex.FindSubmatch(version)
	if len(matches) == 3 {
		if len(matches[2]) == 0 {
			matches[2] = []byte("0")
		}
		return semver.New(fmt.Sprintf("%s.%s", matches[1], matches[2]))
	}
	return &semver.Version{}
}

func DetectOpenSSHVersion() semver.Version {
	var (
		v      semver.Version
		stderr bytes.Buffer
	)
	cmd := exec.Command("ssh", "-V")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		klog.Warningf("failed to run %v: stderr=%q, %s", cmd.Args, stderr.String(), err.Error())
	} else {
		v = *ParseOpenSSHVersion(stderr.Bytes())
		klog.Infof("OpenSSH version %s detected", v)
	}
	return v
}

// detectValidPublicKey returns whether content represent a public key.
// OpenSSH public key format have the structure of '<algorithm> <key> <comment>'.
// By checking 'algorithm' with signature format identifier in 'key' part,
// this function may report false positive but provide better compatibility.
func detectValidPublicKey(content string) bool {
	if strings.ContainsRune(content, '\n') {
		return false
	}
	spaced := strings.SplitN(content, " ", 3)
	if len(spaced) < 2 {
		return false
	}
	algo, base64Key := spaced[0], spaced[1]
	decodedKey, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(decodedKey) < 4 {
		return false
	}
	sigLength := binary.BigEndian.Uint32(decodedKey)
	if uint32(len(decodedKey)) < sigLength {
		return false
	}
	sigFormat := string(decodedKey[4 : 4+sigLength])
	return algo == sigFormat
}

func detectAESAcceleration() bool {
	if !cpu.Initialized {
		if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
			// cpu.Initialized seems to always be false, even when the cpu.ARM64 struct is filled out
			// it is only being set by readARM64Registers, but not by readHWCAP or readLinuxProcCPUInfo
			return cpu.ARM64.HasAES
		}
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			// golang.org/x/sys/cpu supports darwin/amd64, linux/amd64, and linux/arm64,
			// but apparently lacks support for darwin/arm64: https://github.com/golang/sys/blob/v0.5.0/cpu/cpu_arm64.go#L43-L60
			//
			// According to https://gist.github.com/voluntas/fd279c7b4e71f9950cfd4a5ab90b722b ,
			// aes-128-gcm is faster than chacha20-poly1305 on Apple M1.
			//
			// So we return `true` here.
			//
			// This workaround will not be needed when https://go-review.googlesource.com/c/sys/+/332729 is merged.
			klog.Infof("Failed to detect CPU features. Assuming that AES acceleration is available on this Apple silicon.")
			return true
		}
		klog.Infof("Failed to detect CPU features. Assuming that AES acceleration is not available.")
		return false
	}
	return cpu.ARM.HasAES || cpu.ARM64.HasAES || cpu.S390X.HasAES || cpu.X86.HasAES
}

// FormatT specifies the format type.
type FormatT = string

const (
	// FormatCmd prints the full ssh command line.
	//
	//	ssh -o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022 lima-default
	FormatCmd = FormatT("cmd")

	// FormatArgs is similar to FormatCmd but omits "ssh" and the destination address.
	//
	//	-o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022
	FormatArgs = FormatT("args")

	// FormatOptions prints the ssh option key value pairs.
	//
	//	IdentityFile="/Users/example/.lima/_config/user"
	//	User=example
	//	Hostname=127.0.0.1
	//	Port=60022
	FormatOptions = FormatT("options")

	// FormatConfig uses the ~/.ssh/config format
	//
	//	Host lima-default
	//	  IdentityFile "/Users/example/.lima/_config/user "
	//	  User example
	//	  Hostname 127.0.0.1
	//	  Port 60022
	FormatConfig = FormatT("config")

	// TODO: consider supporting "url" format (ssh://USER@HOSTNAME:PORT)
	//
	// TODO: consider supporting "json" format
	// It is unclear whether we can just map ssh "config" into JSON, as "config" has duplicated keys.
	// (JSON supports duplicated keys too, but not all JSON implementations expect JSON with duplicated keys)
)

// Formats is the list of the supported formats.
var Formats = []FormatT{FormatCmd, FormatArgs, FormatOptions, FormatConfig}

func quoteOption(o string) string {
	// make sure the shell doesn't swallow quotes in option values
	if strings.ContainsRune(o, '"') {
		o = "'" + o + "'"
	}
	return o
}

// Format formats the ssh options.
func Format(w io.Writer, instName string, format FormatT, opts []string) error {
	fakeHostname := "md-" + instName // corresponds to the default guest hostname
	switch format {
	case FormatCmd:
		args := []string{"ssh"}
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		args = append(args, fakeHostname)
		// the args are similar to `limactl shell` but not exactly same. (e.g., lacks -t)
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case FormatArgs:
		var args []string
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case FormatOptions:
		for _, o := range opts {
			fmt.Fprintln(w, o)
		}
	case FormatConfig:
		fmt.Fprintf(w, "Host %s\n", fakeHostname)
		for _, o := range opts {
			kv := strings.SplitN(o, "=", 2)
			if len(kv) != 2 {
				return fmt.Errorf("unexpected option %q", o)
			}
			fmt.Fprintf(w, "  %s %s\n", kv[0], kv[1])
		}
	default:
		return fmt.Errorf("unknown format: %q", format)
	}
	return nil
}

func GetSSHAddr(instName, vmType string) (string, error) {
	if vmType != string(v1.WSL2) {
		return "127.0.0.1", nil
	}
	return getWslSSHAddress(instName)
}

// GetWslSSHAddress runs a hostname command to get the IP from inside of a wsl2 VM.
//
// Expected output (whitespace preserved, [] for optional):
// PS > wsl -d <distroName> bash -c hostname -ii | cut -d' ' -f1
// 168.1.1.1 [10.0.0.1]
func getWslSSHAddress(instName string) (string, error) {
	distroName := "md-" + instName
	cmd := exec.Command("wsl.exe", "-d", distroName, "bash", "-c", `hostname -ii | cut -d ' ' -f1`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname for instance %q, err: %w (out=%q)", instName, err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}
