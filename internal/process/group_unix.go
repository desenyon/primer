//go:build darwin || linux

package process

import (
	"os/exec"
	"syscall"
)

func configureGroup(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}
func terminateGroup(cmd *exec.Cmd) error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
func killGroup(cmd *exec.Cmd) error      { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
