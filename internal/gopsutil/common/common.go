package common

//
// gopsutil is a port of psutil(http://pythonhosted.org/psutil/).
// This covers these architectures.
//  - linux (amd64, arm)
//  - freebsd (amd64)
//  - windows (amd64)
import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	Timeout    = 3 * time.Second
	ErrTimeout = errors.New("command timed out")
)

type Invoker interface {
	Command(string, ...string) ([]byte, error)
	CommandWithContext(context.Context, string, ...string) ([]byte, error)
}

type Invoke struct{}

func (i Invoke) Command(name string, arg ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	return i.CommandWithContext(ctx, name, arg...)
}

func (i Invoke) CommandWithContext(ctx context.Context, name string, arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, arg...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return buf.Bytes(), err
	}

	if err := cmd.Wait(); err != nil {
		return buf.Bytes(), err
	}

	return buf.Bytes(), nil
}

type FakeInvoke struct {
	Suffix string // Suffix species expected file name suffix such as "fail"
	Error  error  // If Error specfied, return the error.
}

// Command in FakeInvoke returns from expected file if exists.
func (i FakeInvoke) Command(name string, arg ...string) ([]byte, error) {
	if i.Error != nil {
		return []byte{}, i.Error
	}

	arch := runtime.GOOS

	commandName := filepath.Base(name)

	fname := strings.Join(append([]string{commandName}, arg...), "")
	fname = url.QueryEscape(fname)
	fpath := path.Join("testdata", arch, fname)
	if i.Suffix != "" {
		fpath += "_" + i.Suffix
	}
	if PathExists(fpath) {
		return os.ReadFile(fpath)
	}
	return []byte{}, fmt.Errorf("could not find testdata: %s", fpath)
}

func (i FakeInvoke) CommandWithContext(ctx context.Context, name string, arg ...string) ([]byte, error) {
	return i.Command(name, arg...)
}

var ErrNotImplementedError = errors.New("not implemented yet")

// ReadFile reads contents from a file
func ReadFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)

	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ReadLines reads contents from a file and splits them by new lines.
// A convenience wrapper to ReadLinesOffsetN(filename, 0, -1).
func ReadLines(filename string) ([]string, error) {
	return ReadLinesOffsetN(filename, 0, -1)
}

// ReadLines reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
//
//	n >= 0: at most n lines
//	n < 0: whole file
func ReadLinesOffsetN(filename string, offset uint, n int) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(f)

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}

func IntToString(orig []int8) string {
	ret := make([]byte, len(orig))
	size := -1
	for i, o := range orig {
		if o == 0 {
			size = i
			break
		}
		ret[i] = byte(o)
	}
	if size == -1 {
		size = len(orig)
	}

	return string(ret[0:size])
}

func UintToString(orig []uint8) string {
	ret := make([]byte, len(orig))
	size := -1
	for i, o := range orig {
		if o == 0 {
			size = i
			break
		}
		ret[i] = byte(o)
	}
	if size == -1 {
		size = len(orig)
	}

	return string(ret[0:size])
}

func ByteToString(orig []byte) string {
	n := -1
	l := -1
	for i, b := range orig {
		// skip left side null
		if l == -1 && b == 0 {
			continue
		}
		if l == -1 {
			l = i
		}

		if b == 0 {
			break
		}
		n = i + 1
	}
	if n == -1 {
		return string(orig)
	}
	return string(orig[l:n])
}

// ReadInts reads contents from single line file and returns them as []int32.
func ReadInts(filename string) ([]int64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []int64{}, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(f)

	var ret []int64

	r := bufio.NewReader(f)

	// The int files that this is concerned with should only be one liners.
	line, err := r.ReadString('\n')
	if err != nil {
		return []int64{}, err
	}

	i, err := strconv.ParseInt(strings.Trim(line, "\n"), 10, 32)
	if err != nil {
		return []int64{}, err
	}
	ret = append(ret, i)

	return ret, nil
}

// Parse Hex to uint32 without error
func HexToUint32(hex string) uint32 {
	vv, _ := strconv.ParseUint(hex, 16, 32)
	return uint32(vv)
}

// Parse to int32 without error
func mustParseInt32(val string) int32 {
	vv, _ := strconv.ParseInt(val, 10, 32)
	return int32(vv)
}

// Parse to uint64 without error
func mustParseUint64(val string) uint64 {
	vv, _ := strconv.ParseInt(val, 10, 64)
	return uint64(vv)
}

// Parse to Float64 without error
func mustParseFloat64(val string) float64 {
	vv, _ := strconv.ParseFloat(val, 64)
	return vv
}

// StringsHas checks the target string slice contains src or not
func StringsHas(target []string, src string) bool {
	for _, t := range target {
		if strings.TrimSpace(t) == src {
			return true
		}
	}
	return false
}

// StringsContains checks the src in any string of the target string slice
func StringsContains(target []string, src string) bool {
	for _, t := range target {
		if strings.Contains(t, src) {
			return true
		}
	}
	return false
}

// IntContains checks the src in any int of the target int slice.
func IntContains(target []int, src int) bool {
	for _, t := range target {
		if src == t {
			return true
		}
	}
	return false
}

// get struct attributes.
// This method is used only for debugging platform dependent code.
func attributes(m interface{}) map[string]reflect.Type {
	typ := reflect.TypeOf(m)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	attrs := make(map[string]reflect.Type)
	if typ.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < typ.NumField(); i++ {
		p := typ.Field(i)
		if !p.Anonymous {
			attrs[p.Name] = p.Type
		}
	}

	return attrs
}

func PathExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

// GetEnv retrieves the environment variable key. If it does not exist it returns the default.
func GetEnv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}

	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}

func HostProc(combineWith ...string) string {
	return GetEnv("HOST_PROC", "/proc", combineWith...)
}

func HostSys(combineWith ...string) string {
	return GetEnv("HOST_SYS", "/sys", combineWith...)
}

func HostEtc(combineWith ...string) string {
	return GetEnv("HOST_ETC", "/etc", combineWith...)
}

func HostVar(combineWith ...string) string {
	return GetEnv("HOST_VAR", "/var", combineWith...)
}

func HostRun(combineWith ...string) string {
	return GetEnv("HOST_RUN", "/run", combineWith...)
}

func HostDev(combineWith ...string) string {
	return GetEnv("HOST_DEV", "/dev", combineWith...)
}

// getSysctrlEnv sets LC_ALL=C in a list of env vars for use when running
// sysctl commands (see DoSysctrl).
func getSysctrlEnv(env []string) []string {
	foundLC := false
	for i, line := range env {
		if strings.HasPrefix(line, "LC_ALL") {
			env[i] = "LC_ALL=C"
			foundLC = true
		}
	}
	if !foundLC {
		env = append(env, "LC_ALL=C")
	}
	return env
}
