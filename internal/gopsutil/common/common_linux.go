//go:build linux
// +build linux

package common

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func DoSysctrl(mib string) ([]string, error) {
	sysctl, err := exec.LookPath("sysctl")
	if err != nil {
		return []string{}, err
	}
	cmd := exec.Command(sysctl, "-n", mib)
	cmd.Env = getSysctrlEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}
	v := strings.Replace(string(out), "{ ", "", 1)
	v = strings.Replace(string(v), " }", "", 1)
	values := strings.Fields(string(v))

	return values, nil
}

func NumProcs() (uint64, error) {
	f, err := os.Open(HostProc())
	if err != nil {
		return 0, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(f)

	list, err := f.Readdirnames(-1)
	if err != nil {
		return 0, err
	}
	var cnt uint64

	for _, v := range list {
		if _, err = strconv.ParseUint(v, 10, 64); err == nil {
			cnt++
		}
	}

	return cnt, nil
}

func BootTimeWithContext(ctx context.Context) (uint64, error) {

	system, role, err := Virtualization()
	if err != nil {
		return 0, err
	}

	statFile := "stat"
	if system == "lxc" && role == "guest" {
		// if lxc, /proc/uptime is used.
		statFile = "uptime"
	} else if system == "docker" && role == "guest" {
		// also docker, guest
		statFile = "uptime"
	}

	filename := HostProc(statFile)
	lines, err := ReadLines(filename)
	if err != nil {
		return 0, err
	}

	if statFile == "stat" {
		for _, line := range lines {
			if strings.HasPrefix(line, "btime") {
				f := strings.Fields(line)
				if len(f) != 2 {
					return 0, fmt.Errorf("wrong btime format")
				}
				b, err := strconv.ParseInt(f[1], 10, 64)
				if err != nil {
					return 0, err
				}
				t := uint64(b)
				return t, nil
			}
		}
	} else if statFile == "uptime" {
		if len(lines) != 1 {
			return 0, fmt.Errorf("wrong uptime format")
		}
		f := strings.Fields(lines[0])
		b, err := strconv.ParseFloat(f[0], 64)
		if err != nil {
			return 0, err
		}
		t := uint64(time.Now().Unix()) - uint64(b)
		return t, nil
	}

	return 0, fmt.Errorf("could not find btime")
}

func Virtualization() (string, string, error) {
	return VirtualizationWithContext(context.Background())
}

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	var system string
	var role string

	filename := HostProc("xen")
	if PathExists(filename) {
		system = "xen"
		role = "guest" // assume guest

		if PathExists(filepath.Join(filename, "capabilities")) {
			contents, err := ReadLines(filepath.Join(filename, "capabilities"))
			if err == nil {
				if StringsContains(contents, "control_d") {
					role = "host"
				}
			}
		}
	}

	filename = HostProc("modules")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "kvm") {
				system = "kvm"
				role = "host"
			} else if StringsContains(contents, "vboxdrv") {
				system = "vbox"
				role = "host"
			} else if StringsContains(contents, "vboxguest") {
				system = "vbox"
				role = "guest"
			} else if StringsContains(contents, "vmware") {
				system = "vmware"
				role = "guest"
			}
		}
	}

	filename = HostProc("cpuinfo")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "QEMU Virtual CPU") ||
				StringsContains(contents, "Common KVM processor") ||
				StringsContains(contents, "Common 32-bit KVM processor") {
				system = "kvm"
				role = "guest"
			}
		}
	}

	filename = HostProc("bus/pci/devices")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "virtio-pci") {
				role = "guest"
			}
		}
	}

	filename = HostProc()
	if PathExists(filepath.Join(filename, "bc", "0")) {
		system = "openvz"
		role = "host"
	} else if PathExists(filepath.Join(filename, "vz")) {
		system = "openvz"
		role = "guest"
	}

	// not use dmidecode because it requires root
	if PathExists(filepath.Join(filename, "self", "status")) {
		contents, err := ReadLines(filepath.Join(filename, "self", "status"))
		if err == nil {

			if StringsContains(contents, "s_context:") ||
				StringsContains(contents, "VxID:") {
				system = "linux-vserver"
			}
			// TODO: guest or host
		}
	}

	if PathExists(filepath.Join(filename, "1", "environ")) {
		contents, err := ReadFile(filepath.Join(filename, "1", "environ"))

		if err == nil {
			if strings.Contains(contents, "container=lxc") {
				system = "lxc"
				role = "guest"
			}
		}
	}

	if PathExists(filepath.Join(filename, "self", "cgroup")) {
		contents, err := ReadLines(filepath.Join(filename, "self", "cgroup"))
		if err == nil {
			if StringsContains(contents, "lxc") {
				system = "lxc"
				role = "guest"
			} else if StringsContains(contents, "docker") {
				system = "docker"
				role = "guest"
			} else if StringsContains(contents, "machine-rkt") {
				system = "rkt"
				role = "guest"
			} else if PathExists("/usr/bin/lxc-version") {
				system = "lxc"
				role = "host"
			}
		}
	}

	if PathExists(HostEtc("os-release")) {
		p, _, err := GetOSRelease()
		if err == nil && p == "coreos" {
			system = "rkt" // Is it true?
			role = "host"
		}
	}
	return system, role, nil
}

func GetOSRelease() (platform string, version string, err error) {
	contents, err := ReadLines(HostEtc("os-release"))
	if err != nil {
		return "", "", nil // return empty
	}
	for _, line := range contents {
		field := strings.Split(line, "=")
		if len(field) < 2 {
			continue
		}
		switch field[0] {
		case "ID": // use ID for lowercase
			platform = trimQuotes(field[1])
		case "VERSION":
			version = trimQuotes(field[1])
		}
	}
	return platform, version, nil
}

// Remove quotes of the source string
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}
