package cmd

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type packageManagerKind string

const (
	packageManagerAPT packageManagerKind = "apt"
	packageManagerAPK packageManagerKind = "apk"
	packageManagerDNF packageManagerKind = "dnf"
	packageManagerYUM packageManagerKind = "yum"
)

// OS detection cache
var (
	cachedOsType *detectOsStruct
	osDetectOnce sync.Once
)

type detectOsStruct struct {
	deb         bool
	apk         bool
	dnf         bool
	yum         bool
	unsupported bool
	osID        string
	osIDLike    []string
	prettyName  string
	manager     packageManagerKind
}

func (o detectOsStruct) packageManagerName() string {
	if o.manager != "" {
		return string(o.manager)
	}
	switch {
	case o.deb:
		return string(packageManagerAPT)
	case o.apk:
		return string(packageManagerAPK)
	case o.dnf:
		return string(packageManagerDNF)
	case o.yum:
		return string(packageManagerYUM)
	default:
		return ""
	}
}

// detectOs identifies the Linux distribution and returns cached result on subsequent calls.
func detectOs() detectOsStruct {
	osDetectOnce.Do(func() {
		cachedOsType = detectOsUncached()
	})
	return *cachedOsType
}

// detectOsUncached performs the actual OS detection.
func detectOsUncached() *detectOsStruct {
	values, err := readOsRelease("/etc/os-release")
	if err != nil {
		log.Fatalf("Could not read /etc/os-release: %v", err)
	}

	osStruct := &detectOsStruct{
		osID:       strings.ToLower(values["ID"]),
		osIDLike:   lowerFields(values["ID_LIKE"]),
		prettyName: values["PRETTY_NAME"],
	}

	manager := selectPackageManager(*osStruct)
	switch manager {
	case packageManagerAPT:
		osStruct.deb = true
	case packageManagerAPK:
		osStruct.apk = true
	case packageManagerDNF:
		osStruct.dnf = true
	case packageManagerYUM:
		osStruct.yum = true
	default:
		osStruct.unsupported = true
		log.Fatalf("Sorry, this OS (%s) is not yet supported!", osStruct.osID)
	}
	osStruct.manager = manager

	return osStruct
}

func readOsRelease(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(value, `"`)
	}
	return values, scanner.Err()
}

func selectPackageManager(osInfo detectOsStruct) packageManagerKind {
	debIDs := []string{
		"ubuntu", "debian", "pop", "linuxmint", "elementary", "kali",
		"parrot", "raspbian", "zorin", "neon",
	}
	rpmIDs := []string{
		"almalinux", "amzn", "amazon", "centos", "fedora", "ol",
		"openeuler", "oracle", "rhel", "rocky",
	}
	alpineIDs := []string{"alpine"}

	if stringInSet(osInfo.osID, alpineIDs) || stringInSetAny(osInfo.osIDLike, alpineIDs) {
		if commandExistsFunc("apk") {
			return packageManagerAPK
		}
	}

	if stringInSet(osInfo.osID, debIDs) || stringInSetAny(osInfo.osIDLike, []string{"debian", "ubuntu"}) {
		if commandExistsFunc("apt-get") {
			return packageManagerAPT
		}
	}

	if stringInSet(osInfo.osID, rpmIDs) ||
		stringInSetAny(osInfo.osIDLike, []string{"rhel", "fedora", "centos"}) {
		if commandExistsFunc("dnf") {
			return packageManagerDNF
		}
		if commandExistsFunc("yum") {
			return packageManagerYUM
		}
	}

	// Last-resort fallback for derivative distributions with incomplete os-release data.
	if commandExistsFunc("apt-get") {
		return packageManagerAPT
	}
	if commandExistsFunc("dnf") {
		return packageManagerDNF
	}
	if commandExistsFunc("yum") {
		return packageManagerYUM
	}
	if commandExistsFunc("apk") {
		return packageManagerAPK
	}

	return ""
}

func stringInSet(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func stringInSetAny(values []string, candidates []string) bool {
	for _, value := range values {
		if stringInSet(value, candidates) {
			return true
		}
	}
	return false
}

func lowerFields(value string) []string {
	fields := strings.Fields(value)
	for i, field := range fields {
		fields[i] = strings.ToLower(field)
	}
	return fields
}

var commandExistsFunc = commandExists

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

type packageManager interface {
	Name() string
	CheckUpdates() systemUpdatesStruct
	ApplyCommand(ctx context.Context, pkg string) *exec.Cmd
	KernelCleanupCommand(oldKernels []string) (string, []string)
}

func getPackageManager(osType detectOsStruct) packageManager {
	switch osType.packageManagerKind() {
	case packageManagerAPT:
		return aptPackageManager{}
	case packageManagerAPK:
		return apkPackageManager{}
	case packageManagerDNF:
		return rpmPackageManager{binary: "dnf"}
	case packageManagerYUM:
		return rpmPackageManager{binary: "yum"}
	default:
		log.Fatal("Sorry, but your OS is not yet supported!")
	}
	return nil
}

func (o detectOsStruct) packageManagerKind() packageManagerKind {
	if o.manager != "" {
		return o.manager
	}
	switch {
	case o.deb:
		return packageManagerAPT
	case o.apk:
		return packageManagerAPK
	case o.dnf:
		return packageManagerDNF
	case o.yum:
		return packageManagerYUM
	default:
		return ""
	}
}

type aptPackageManager struct{}

func (aptPackageManager) Name() string {
	return string(packageManagerAPT)
}

// apkPackageManager implements Alpine's native package operations. Alpine upgrades its
// kernel package in place, so there are normally no parallel kernel packages to purge.
type apkPackageManager struct{}

func (apkPackageManager) Name() string {
	return string(packageManagerAPK)
}

func (apkPackageManager) CheckUpdates() systemUpdatesStruct {
	return apkCheck()
}

func (apkPackageManager) ApplyCommand(ctx context.Context, pkg string) *exec.Cmd {
	return newCommand(ctx, "apk", "--no-progress", "upgrade", pkg)
}

func (apkPackageManager) KernelCleanupCommand(oldKernels []string) (string, []string) {
	return "", nil
}

func (aptPackageManager) CheckUpdates() systemUpdatesStruct {
	return debCheck()
}

func (aptPackageManager) ApplyCommand(ctx context.Context, pkg string) *exec.Cmd {
	return newCommand(ctx, "apt-get", "install", "-y", pkg)
}

func (aptPackageManager) KernelCleanupCommand(oldKernels []string) (string, []string) {
	packages := findDebianPackagesToRemove(oldKernels)
	if len(packages) == 0 {
		return "", nil
	}
	return "apt-get", append([]string{"purge", "-y"}, packages...)
}

type rpmPackageManager struct {
	binary string
}

func (p rpmPackageManager) Name() string {
	return p.binary
}

func (p rpmPackageManager) CheckUpdates() systemUpdatesStruct {
	if p.binary == "yum" {
		return yumCheck()
	}
	return dnfCheck()
}

func (p rpmPackageManager) ApplyCommand(ctx context.Context, pkg string) *exec.Cmd {
	return newCommand(ctx, p.binary, "update", "-y", pkg)
}

func (p rpmPackageManager) KernelCleanupCommand(oldKernels []string) (string, []string) {
	packages := findRPMPackagesToRemove(oldKernels)
	if len(packages) == 0 {
		return "", nil
	}
	return p.binary, append([]string{"remove", "-y"}, packages...)
}

// runCommandWithTimeout executes a command with a context timeout.
func runCommandWithTimeout(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := newCommand(ctx, name, args...)
	return cmd.Output()
}

// runCommandWithTimeoutCombined executes a command and returns combined stdout/stderr.
func runCommandWithTimeoutCombined(ctx context.Context, name string, args ...string) ([]byte, int, error) {
	cmd := newCommand(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return out, exitCode, err
}

// runCommandWithTimeoutStdout executes a command and returns only stdout. stderr is
// discarded so package-manager warnings (e.g. dnf "Repository ... is listed more than
// once in the configuration") never leak into the parsed package list.
func runCommandWithTimeoutStdout(ctx context.Context, name string, args ...string) ([]byte, int, error) {
	cmd := newCommand(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return stdout.Bytes(), exitCode, err
}

func packageCommandTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Minute)
}

func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"LANG=C",
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
	)
	return cmd
}
