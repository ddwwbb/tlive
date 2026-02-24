//go:build !windows

package pty

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

type unixProcess struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

func Start(name string, args []string, rows, cols uint16) (Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, err
	}
	return &unixProcess{ptmx: ptmx, cmd: cmd}, nil
}

func (p *unixProcess) Read(b []byte) (int, error)  { return p.ptmx.Read(b) }
func (p *unixProcess) Write(b []byte) (int, error) { return p.ptmx.Write(b) }
func (p *unixProcess) Resize(rows, cols uint16) error {
	return pty.Setsize(p.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (p *unixProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), nil
		}
		return -1, err
	}
	return 0, nil
}

func (p *unixProcess) Close() error { return p.ptmx.Close() }

func (p *unixProcess) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}
