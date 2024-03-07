package helpers

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// For windows, process kill, see: https://github.com/mattn/goreman/blob/e9150e84f13c37dff0a79b8faed5b86522f3eb8e/proc_windows.go#L16-L51
func TerminateProc(channelName string) error {
	dll, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer dll.Release()

	pid := 0 // recorded[channelName].Process.Pid

	f, err := dll.FindProc("AttachConsole")
	if err != nil {
		return err
	}
	r1, _, err := f.Call(uintptr(pid))
	if r1 == 0 && !errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		return err
	}

	f, err = dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(0, 1)
	if r1 == 0 {
		return err
	}
	f, err = dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(windows.CTRL_C_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	return nil
}
