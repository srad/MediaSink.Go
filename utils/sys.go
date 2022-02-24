package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"
)

var (
	cmd = make(map[int]*exec.Cmd)
)

type CommandInfo struct {
	Command string
	Pid     int
}

type ExecArgs struct {
	cancel      context.CancelFunc
	OnStart     func(CommandInfo)
	OnPipeOut   func(string)
	OnPipeErr   func(PipeMessage)
	Command     string
	CommandArgs []string
}

type PipeMessage struct {
	Message string
	Pid     int
}

func (execArgs *ExecArgs) ToString() string {
	return fmt.Sprintf("%s %s", execArgs.Command, strings.Join(execArgs.CommandArgs, " "))
}

// ExecSync See: https://stackoverflow.com/questions/10385551/get-exit-code-go
func ExecSync(execArgs *ExecArgs) error {
	c := exec.Command(execArgs.Command, execArgs.CommandArgs...)
	log.Println("Executing: ", execArgs.ToString())

	//stdout, _ := cmd.StdoutPipe()
	sterr, _ := c.StderrPipe()

	if err := c.Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	pid := c.Process.Pid
	cmd[pid] = c
	defer delete(cmd, pid)

	if execArgs.OnStart != nil {
		execArgs.OnStart(CommandInfo{Pid: pid, Command: execArgs.ToString()})
	}

	if execArgs.OnPipeErr != nil {
		scanner := bufio.NewScanner(sterr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			execArgs.OnPipeErr(PipeMessage{Message: scanner.Text(), Pid: pid})
		}
	}

	if err := cmd[pid].Wait(); err != nil {
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

func Terminate(pid int) error {
	if c, ok := cmd[pid]; ok {
		err := c.Process.Signal(syscall.SIGINT)
		delete(cmd, pid)
		return err
	}
	return errors.New(fmt.Sprintf("pid %d not found", pid))
}
