package helpers

import (
	"os/exec"
	"strings"
)

func GetCommand(cmd *exec.Cmd) string {
	if cmd.Args == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(cmd.Args, " "))
}
