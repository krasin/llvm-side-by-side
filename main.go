package main

import (
	"bytes"
	"exec"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	t1 = flag.String("t1", "", "Path to the first toolchain")
	t2 = flag.String("t2", "", "Path to the second toolchain")
	test = flag.String("test", "", "Path to the test bitcode file")

	stackSpaceRegexp = regexp.MustCompile(`([0-9]+) pei[^N]+Number of bytes used for stack in all functions`)
	asmInstrsRegexp = regexp.MustCompile(`([0-9]+) asm-printer[^N]+Number of machine instrs printed`)
	execTimeRegexp = regexp.MustCompile(`Total Execution Time: ([0-9.]+) seconds \(([0-9.]+) wall clock\)`)
)

type Stats struct {
	AsmInstrs int
	StackSpace int

	Seconds float64
	WallSeconds float64
}

func runTest(toolchain, test string) (stderr string, err os.Error) {
	cmd := exec.Command(path.Join(toolchain, "bin/llc"), "-O0", "-stats", "--time-passes", 
		"-relocation-model=pic", "-O0", "-asm-verbose=false")
	var data []byte
	if data, err = ioutil.ReadFile(test); err != nil {
		return
	}
	cmd.Stdin = bytes.NewBuffer(data)
	var outPipe, errPipe io.Reader

	if errPipe, err = cmd.StderrPipe(); err != nil {
		return
	}
	if outPipe, err = cmd.StdoutPipe(); err != nil {
		return
	}
	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("cmd.Start: %v", err)
	}
	if _, err = ioutil.ReadAll(outPipe); err != nil {
		return "", fmt.Errorf("ioutil.ReadAll(outPipe): %v", err)
	}	
	var stderrData []byte
	if stderrData, err = ioutil.ReadAll(errPipe); err != nil {
		return "", fmt.Errorf("ioutil.ReadAll(errPipe): %v", err)
	}
	if err = cmd.Wait(); err != nil {
		return "", fmt.Errorf("cmd.Wait: %v", err)
	}
	stderr = string(stderrData)
	return
}

func parseTestOutput(stderr string) (res *Stats) {
	res = new(Stats)
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if asmInstrsRegexp.MatchString(line) {
			ss := asmInstrsRegexp.FindStringSubmatch(line)
			if len(ss) != 2 {
				log.Printf("parseTestOutput: could not parse AsmInstrs statistic for line=[%s]", line)
				continue
			}
			var err os.Error
			if res.AsmInstrs, err = strconv.Atoi(ss[1]); err != nil {
				log.Printf("parseTestOutput: could not parse int value of AsmInstrs statistic " +
                                    "for line=[%s], matched substring=[%s], err: %v", line, ss[1], err)
				continue
			}
		}
		if stackSpaceRegexp.MatchString(line) {
			ss := stackSpaceRegexp.FindStringSubmatch(line)
			if len(ss) != 2 {
				log.Printf("parseTestOutput: could not parse StackSpace statistic for line=[%s]", line)
				continue
			}
			var err os.Error
			if res.StackSpace, err = strconv.Atoi(ss[1]); err != nil {
				log.Printf("parseTestOutput: could not parse int value of StackSpace statistic " +
                                    "for line=[%s], matched substring=[%s], err: %v", line, ss[1], err)
				continue
			}
		}
		if execTimeRegexp.MatchString(line) {
			ss := execTimeRegexp.FindStringSubmatch(line)
			if len(ss) != 3 {
				log.Printf("parseTestOutput: could not parse ExecTime statistic for line=[%s]", line)
				continue
			}
			var err os.Error
			if res.Seconds, err = strconv.Atof64(ss[1]); err != nil {
				log.Printf("parseTestOutput: could not parse int value of Seconds statistic " +
					"for line=[%s], matched substring=[%s], err: %v", line, ss[1], err)
				continue
			}
			if res.WallSeconds, err = strconv.Atof64(ss[2]); err != nil {
				log.Printf("parseTestOutput: could not parse int value of WallSeconds statistic " +
					"for line=[%s], matched substring=[%s], err: %v", line, ss[2], err)
				continue
			}
		}
	}
	return
}

func runAndParse(toolchain, test string) (stats *Stats, err os.Error) {
	var stderr string
	if stderr, err = runTest(toolchain, test); err != nil {
		return
	}
	stats = parseTestOutput(stderr)
	return
}

func runBoth(t1, t2, test string) (stats [2]*Stats, err os.Error) {
	if stats[0], err = runAndParse(t1, test); err != nil {
		log.Fatalf("runTest(t1=%s, test=%s): %v", t1, test, err) 
	}

	if stats[1], err = runAndParse(t2, test); err != nil {
		log.Fatalf("runTest(t2=%s, test=%s): %v", t2, test, err) 
	}
	return
}

func printStats(stats [2]*Stats) (err os.Error) {
	fmt.Printf("%s\t%d\t%d\t%v\t%v\t%d\t%d\t%v\t%v\n", path.Base(*test),
		stats[0].AsmInstrs, stats[0].StackSpace, stats[0].Seconds, stats[0].WallSeconds,
		stats[1].AsmInstrs, stats[1].StackSpace, stats[1].Seconds, stats[1].WallSeconds)
	return
}

func checkArg(name string, cond bool) {
	if (!cond) {
		fmt.Fprintf(os.Stderr, "%s is not specified\n", name)
		flag.PrintDefaults()
		os.Exit(1)		
	}
}

func main() {
	flag.Parse()
	checkArg("-test", *test != "")
	checkArg("-t1", *t1 != "")
	checkArg("-t2", *t2 != "")

	fmt.Printf("Running test: %s\n", *test)
	var err os.Error
	var stats [2]*Stats
	if _, err = runBoth(*t1, *t2, *test); err != nil {
		log.Fatalf("runBoth: %v", err)
	}
	if stats, err = runBoth(*t1, *t2, *test); err != nil {
		log.Fatalf("runBoth(2): %v", err)
	}
	if err = printStats(stats); err != nil {
		log.Fatalf("printStats: %v", err)
	}
}