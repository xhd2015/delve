package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	err := handle(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func handle(args []string) error {
	var projectDir string
	n := len(args)

	var remainArgs []string
	var verbose bool
	for i := 0; i < n; i++ {
		arg := args[i]
		if arg == "--" {
			remainArgs = append(remainArgs, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			remainArgs = append(remainArgs, arg)
			continue
		}
		switch arg {
		case "--project-dir":
			if i+1 >= n {
				return fmt.Errorf("project-dir is not set")
			}
			projectDir = args[i+1]
			i++
		case "-v", "--verbose":
			verbose = true
		default:
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}

	tmpDir, err := os.MkdirTemp("", "xgo")
	if err != nil {
		return err
	}

	var binaryName string
	if len(args) > 0 {
		baseName := filepath.Base(args[0])
		binaryName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	} else {
		binaryName = "__debug_bin"
	}

	binary := filepath.Join(tmpDir, binaryName)

	buildArgs := []string{"build", "-o", binary, "-gcflags=all=-N -l"}
	buildArgs = append(buildArgs, remainArgs...)

	if verbose {
		if projectDir != "" {
			fmt.Fprintf(os.Stderr, "cd %s\n", projectDir)
		}
		fmt.Fprintf(os.Stderr, "go %s\n", strings.Join(toShArgs(buildArgs), " "))
	}
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Dir = projectDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err = buildCmd.Run()
	if err != nil {
		return err
	}

	runCmd := exec.Command(binary)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func toShArgs(args []string) []string {
	var shArgs []string
	for _, arg := range args {
		if !strings.Contains(arg, " ") {
			shArgs = append(shArgs, arg)
			continue
		}
		shArgs = append(shArgs, fmt.Sprintf("'%s'", arg))
	}
	return shArgs
}
