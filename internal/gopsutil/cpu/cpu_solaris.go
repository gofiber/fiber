package cpu

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var ClocksPerSec = float64(128)

func init() {
	getconf, err := exec.LookPath("getconf")
	if err != nil {
		return
	}
	out, err := invoke.Command(getconf, "CLK_TCK")
	// ignore errors
	if err == nil {
		i, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err == nil {
			ClocksPerSec = float64(i)
		}
	}
}

// sum all values in a float64 map with float64 keys
func msum(x map[float64]float64) float64 {
	total := 0.0
	for _, y := range x {
		total += y
	}
	return total
}

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	kstatSys, err := exec.LookPath("kstat")
	if err != nil {
		return nil, fmt.Errorf("cannot find kstat: %s", err)
	}
	cpu := make(map[float64]float64)
	idle := make(map[float64]float64)
	user := make(map[float64]float64)
	kern := make(map[float64]float64)
	iowt := make(map[float64]float64)
	//swap := make(map[float64]float64)
	kstatSysOut, err := invoke.CommandWithContext(ctx, kstatSys, "-p", "cpu_stat:*:*:/^idle$|^user$|^kernel$|^iowait$|^swap$/")
	if err != nil {
		return nil, fmt.Errorf("cannot execute kstat: %s", err)
	}
	re := regexp.MustCompile(`[:\s]+`)
	for _, line := range strings.Split(string(kstatSysOut), "\n") {
		fields := re.Split(line, -1)
		if fields[0] != "cpu_stat" {
			continue
		}
		cpuNumber, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse cpu number: %s", err)
		}
		cpu[cpuNumber] = cpuNumber
		switch fields[3] {
		case "idle":
			idle[cpuNumber], err = strconv.ParseFloat(fields[4], 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse idle: %s", err)
			}
		case "user":
			user[cpuNumber], err = strconv.ParseFloat(fields[4], 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse user: %s", err)
			}
		case "kernel":
			kern[cpuNumber], err = strconv.ParseFloat(fields[4], 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse kernel: %s", err)
			}
		case "iowait":
			iowt[cpuNumber], err = strconv.ParseFloat(fields[4], 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse iowait: %s", err)
			}
			//not sure how this translates, don't report, add to kernel, something else?
			/*case "swap":
			swap[cpuNumber], err = strconv.ParseFloat(fields[4], 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse swap: %s", err)
			} */
		}
	}
	ret := make([]TimesStat, 0, len(cpu))
	if percpu {
		for _, c := range cpu {
			ct := &TimesStat{
				CPU:    fmt.Sprintf("cpu%d", int(cpu[c])),
				Idle:   idle[c] / ClocksPerSec,
				User:   user[c] / ClocksPerSec,
				System: kern[c] / ClocksPerSec,
				Iowait: iowt[c] / ClocksPerSec,
			}
			ret = append(ret, *ct)
		}
	} else {
		ct := &TimesStat{
			CPU:    "cpu-total",
			Idle:   msum(idle) / ClocksPerSec,
			User:   msum(user) / ClocksPerSec,
			System: msum(kern) / ClocksPerSec,
			Iowait: msum(iowt) / ClocksPerSec,
		}
		ret = append(ret, *ct)
	}
	return ret, nil
}

func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	psrInfo, err := exec.LookPath("psrinfo")
	if err != nil {
		return nil, fmt.Errorf("cannot find psrinfo: %s", err)
	}
	psrInfoOut, err := invoke.CommandWithContext(ctx, psrInfo, "-p", "-v")
	if err != nil {
		return nil, fmt.Errorf("cannot execute psrinfo: %s", err)
	}

	isaInfo, err := exec.LookPath("isainfo")
	if err != nil {
		return nil, fmt.Errorf("cannot find isainfo: %s", err)
	}
	isaInfoOut, err := invoke.CommandWithContext(ctx, isaInfo, "-b", "-v")
	if err != nil {
		return nil, fmt.Errorf("cannot execute isainfo: %s", err)
	}

	procs, err := parseProcessorInfo(string(psrInfoOut))
	if err != nil {
		return nil, fmt.Errorf("error parsing psrinfo output: %s", err)
	}

	flags, err := parseISAInfo(string(isaInfoOut))
	if err != nil {
		return nil, fmt.Errorf("error parsing isainfo output: %s", err)
	}

	result := make([]InfoStat, 0, len(flags))
	for _, proc := range procs {
		procWithFlags := proc
		procWithFlags.Flags = flags
		result = append(result, procWithFlags)
	}

	return result, nil
}

var flagsMatch = regexp.MustCompile(`[\w\.]+`)

func parseISAInfo(cmdOutput string) ([]string, error) {
	words := flagsMatch.FindAllString(cmdOutput, -1)

	// Sanity check the output
	if len(words) < 4 || words[1] != "bit" || words[3] != "applications" {
		return nil, errors.New("attempted to parse invalid isainfo output")
	}

	flags := make([]string, len(words)-4)
	for i, val := range words[4:] {
		flags[i] = val
	}
	sort.Strings(flags)

	return flags, nil
}

var psrInfoMatch = regexp.MustCompile(`The physical processor has (?:([\d]+) virtual processor \(([\d]+)\)|([\d]+) cores and ([\d]+) virtual processors[^\n]+)\n(?:\s+ The core has.+\n)*\s+.+ \((\w+) ([\S]+) family (.+) model (.+) step (.+) clock (.+) MHz\)\n[\s]*(.*)`)

const (
	psrNumCoresOffset   = 1
	psrNumCoresHTOffset = 3
	psrNumHTOffset      = 4
	psrVendorIDOffset   = 5
	psrFamilyOffset     = 7
	psrModelOffset      = 8
	psrStepOffset       = 9
	psrClockOffset      = 10
	psrModelNameOffset  = 11
)

func parseProcessorInfo(cmdOutput string) ([]InfoStat, error) {
	matches := psrInfoMatch.FindAllStringSubmatch(cmdOutput, -1)

	var infoStatCount int32
	result := make([]InfoStat, 0, len(matches))
	for physicalIndex, physicalCPU := range matches {
		var step int32
		var clock float64

		if physicalCPU[psrStepOffset] != "" {
			stepParsed, err := strconv.ParseInt(physicalCPU[psrStepOffset], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value %q for step as 32-bit integer: %s", physicalCPU[9], err)
			}
			step = int32(stepParsed)
		}

		if physicalCPU[psrClockOffset] != "" {
			clockParsed, err := strconv.ParseInt(physicalCPU[psrClockOffset], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value %q for clock as 32-bit integer: %s", physicalCPU[10], err)
			}
			clock = float64(clockParsed)
		}

		var err error
		var numCores int64
		var numHT int64
		switch {
		case physicalCPU[psrNumCoresOffset] != "":
			numCores, err = strconv.ParseInt(physicalCPU[psrNumCoresOffset], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value %q for core count as 32-bit integer: %s", physicalCPU[1], err)
			}

			for i := 0; i < int(numCores); i++ {
				result = append(result, InfoStat{
					CPU:        infoStatCount,
					PhysicalID: strconv.Itoa(physicalIndex),
					CoreID:     strconv.Itoa(i),
					Cores:      1,
					VendorID:   physicalCPU[psrVendorIDOffset],
					ModelName:  physicalCPU[psrModelNameOffset],
					Family:     physicalCPU[psrFamilyOffset],
					Model:      physicalCPU[psrModelOffset],
					Stepping:   step,
					Mhz:        clock,
				})
				infoStatCount++
			}
		case physicalCPU[psrNumCoresHTOffset] != "":
			numCores, err = strconv.ParseInt(physicalCPU[psrNumCoresHTOffset], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value %q for core count as 32-bit integer: %s", physicalCPU[3], err)
			}

			numHT, err = strconv.ParseInt(physicalCPU[psrNumHTOffset], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value %q for hyperthread count as 32-bit integer: %s", physicalCPU[4], err)
			}

			for i := 0; i < int(numCores); i++ {
				result = append(result, InfoStat{
					CPU:        infoStatCount,
					PhysicalID: strconv.Itoa(physicalIndex),
					CoreID:     strconv.Itoa(i),
					Cores:      int32(numHT) / int32(numCores),
					VendorID:   physicalCPU[psrVendorIDOffset],
					ModelName:  physicalCPU[psrModelNameOffset],
					Family:     physicalCPU[psrFamilyOffset],
					Model:      physicalCPU[psrModelOffset],
					Stepping:   step,
					Mhz:        clock,
				})
				infoStatCount++
			}
		default:
			return nil, errors.New("values for cores with and without hyperthreading are both set")
		}
	}
	return result, nil
}

func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	return runtime.NumCPU(), nil
}
