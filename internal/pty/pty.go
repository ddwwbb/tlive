package pty

import "io"

// Process wraps a command running in a pseudo-terminal.
type Process interface {
	io.Reader
	io.Writer
	Resize(rows, cols uint16) error
	Wait() (int, error)
	Close() error
	Pid() int
}
