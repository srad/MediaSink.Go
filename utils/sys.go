package utils

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"
)

type CommandInfo struct {
	Command string
	Pid     int
}

type ExecArgs struct {
	OnStart     func(CommandInfo)
	OnPipeOut   func(string)
	OnPipeErr   func(string)
	Command     string
	CommandArgs []string
}

func (execArgs *ExecArgs) ToString() string {
	return fmt.Sprintf("%s %s", execArgs.Command, strings.Join(execArgs.CommandArgs, " "))
}

// ExecSync See: https://stackoverflow.com/questions/10385551/get-exit-code-go
func ExecSync(execArgs *ExecArgs) error {
	cmd := exec.Command(execArgs.Command, execArgs.CommandArgs...)
	log.Println("Executing: ", execArgs.ToString())

	//stdout, _ := cmd.StdoutPipe()
	sterr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	if execArgs.OnStart != nil {
		execArgs.OnStart(CommandInfo{Pid: cmd.Process.Pid, Command: execArgs.ToString()})
	}

	if execArgs.OnPipeErr != nil {
		scanner := bufio.NewScanner(sterr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			execArgs.OnPipeErr(scanner.Text())
		}
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return err
				//return status.ExitStatus()
			}
		}
		return err
	}

	return nil
}
