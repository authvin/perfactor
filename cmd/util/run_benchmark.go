package util

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const p = string(os.PathSeparator)

// RunCode Run external code in _tmp as a separate program - go test
// flags - if any flags need to be passed when running the code
// benchname - the name of the benchmark method in the test file to run
// id - the id of the run, used to name the output
func RunCode(flags string, benchName string, testName string, id string, filename string, folderPath string, doProfile bool, count int) string {
	// Command should be a perf call with appropriate arguments
	//output, err := exec.Command("cmd", "/c", "dir").CombinedOutput()
	// go test %flags% -bench=%benchName% -run=NONE -benchmem -memprofile mem.pprof -cpuprofile cpu.pprof > %id%.bench
	// Potentially replace with: https://cs.opensource.google/go/go/+/refs/tags/go1.19.2:src/testing/benchmark.go;l=511
	if runtime.GOOS == "windows" {
		return runCodeWindows(flags, benchName, id, filename, folderPath, testName, count)
	} else {
		return runCodeLinux(flags, benchName, folderPath, testName, doProfile, count)
	}
}

func runCodeLinux(flags string, benchName string, folderPath string, testName string, doProfile bool, count int) string {
	res, err := exec.Command("pwd").Output()
	if err != nil {
		fmt.Println("Failed to get PWD: " + err.Error())
		return "FAILED"
	}
	pwd := string(res)
	pwd = strings.Trim(pwd, "\n") + p
	err = os.MkdirAll(folderPath, 0777)
	if err != nil {
		fmt.Println("Failed to make folders: " + err.Error())
		return "FAILED"
	}

	// set up all the arguments in an array, to allow for conditional arguments
	args := make([]string, 0)
	args = append(args, "test", flags)       // we use go test, plus any flags that need to be passed to the executing method
	args = append(args, "-bench="+benchName) // the name of the benchmark method in the test file to run
	args = append(args, "-run="+testName)    // We don't run any normal tests. Maybe have this be a default value?
	args = append(args, fmt.Sprintf("-count=%d", count))
	if doProfile {
		args = append(args, "-cpuprofile", "cpu.pprof") // record cpu profile
		args = append(args, "-memprofile", "mem.pprof") // record memory profile
	}
	//args = append(args, ">", outputPath+id+".bench") // put in "%id%.bench" for later use

	cmd := exec.Command("go", args...)
	cmd.Dir = pwd + folderPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println("failed to run code: " + err.Error())
	}
	if output != nil {
		return string(output)
	}
	return "FAILED"
}

func runCodeWindows(flags string, benchName string, id string, inputPath string, outputPath string, testName string, count int) string {
	output, err := exec.Command("powershell", "-nologo", "-noprofile", // opens powershell
		"cd", inputPath, // move into the tmp folder
		"go", "test", flags, // we use go test, plus any flags that need to be passed to the executing method
		"-bench="+benchName,                             // the name of the benchmark method in the test file to run
		"-run="+testName,                                // We don't run any normal tests. Maybe have this be a default value?
		"-cpuprofile", "./"+outputPath+id+p+"cpu.pprof", // record cpu profile
		"-memprofile", outputPath+id+p+"mem.pprof", // record memory profile
		">", outputPath+id+p+id+".bench").CombinedOutput() // put in "%id%.bench" for later use, in the _data directory
	if err != nil {
		fmt.Println("Failed to execute windows command: " + err.Error())
	}
	if output != nil {
		return string(output)
	}
	return "FAILED"
}
