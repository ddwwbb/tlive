package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LockInfo is persisted to disk so that subsequent tlive run processes
// can discover and reuse an already-running daemon.
type LockInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	Pid   int    `json:"pid"`
}

// DefaultLockPath returns ~/.termlive/daemon.json.
func DefaultLockPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".termlive", "daemon.json")
}

// WriteLockFile atomically writes lock info, creating parent dirs as needed.
func WriteLockFile(path string, info LockInfo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadLockFile reads and parses the lock file.
func ReadLockFile(path string) (LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LockInfo{}, err
	}
	var info LockInfo
	err = json.Unmarshal(data, &info)
	return info, err
}

// RemoveLockFile deletes the lock file (idempotent).
func RemoveLockFile(path string) {
	os.Remove(path)
}
