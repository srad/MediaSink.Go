package utils

import (
	"io"
	"log"
	"os/exec"
	"syscall"
)

// ExecSync See: https://stackoverflow.com/questions/10385551/get-exit-code-go
func ExecSync(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	log.Println("Executing: ", exe, args, "\\n")

	//stdout, _ := cmd.StdoutPipe()
	sterr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Printf("[ExecSync] %s: %v", string(b), err)
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
