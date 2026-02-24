//go:build windows

package pty

import "fmt"

func Start(name string, args []string, rows, cols uint16) (Process, error) {
	return nil, fmt.Errorf("PTY not yet supported on Windows; use WSL or Unix")
}
