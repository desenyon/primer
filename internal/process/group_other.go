//go:build !darwin && !linux

package process

import (
	"errors"
	"os/exec"
)

func configureGroup(cmd *exec.Cmd) error {
	return errors.New("supervised launch currently requires macOS or Linux")
}
func terminateGroup(cmd *exec.Cmd) error { return cmd.Process.Kill() }
func killGroup(cmd *exec.Cmd) error      { return cmd.Process.Kill() }
