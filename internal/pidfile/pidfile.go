// Package pidfile забезпечує створення, перевірку та видалення PID-файлу процесу DELMOS.
package pidfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// File представляє створений PID-файл поточного процесу.
type File struct {
	path string
	pid  int
}

// Write створює PID-файл із поточним PID процесу.
// Якщо файл уже існує й процес із таким PID досі живий, повертається помилка.
// Якщо попередній процес завершився (застарілий PID-файл), він безпечно перезаписується.
func Write(path string) (*File, error) {
	if path == "" {
		return nil, nil
	}

	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o750); err != nil {
		return nil, fmt.Errorf("створення каталогу для pid-файлу: %w", err)
	}

	if existingPID, err := Read(cleanPath); err == nil && existingPID > 0 {
		if isProcessAlive(existingPID) {
			return nil, fmt.Errorf("процес DELMOS вже працює з PID %d (файл %s)", existingPID, cleanPath)
		}
	}

	currentPID := os.Getpid()
	content := fmt.Sprintf("%d\n", currentPID)
	//nolint:gosec // G306: права 0600 для захисту PID-файлу
	if err := os.WriteFile(cleanPath, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("запис pid-файлу %s: %w", cleanPath, err)
	}

	return &File{
		path: cleanPath,
		pid:  currentPID,
	}, nil
}

// Read зчитує та валідує PID із файлу.
func Read(path string) (int, error) {
	//nolint:gosec // G304: шлях до pid-файлу задається оператором/конфігурацією
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return 0, err
	}
	raw := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("некоректний вміст pid-файлу %q: %w", raw, err)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("некоректний PID %d у %s", pid, path)
	}
	return pid, nil
}

// Path повертає шлях до pid-файлу.
func (f *File) Path() string {
	if f == nil {
		return ""
	}
	return f.path
}

// PID повертає PID, записаний у файл.
func (f *File) PID() int {
	if f == nil {
		return 0
	}
	return f.pid
}

// Remove видаляє pid-файл, якщо його вміст відповідає поточному процесу.
func (f *File) Remove() error {
	if f == nil || f.path == "" {
		return nil
	}
	existingPID, err := Read(f.path)
	if err == nil && existingPID == f.pid {
		return os.Remove(f.path)
	}
	return nil
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// У Unix Signal(0) не надсилає сигнал, а лише перевіряє існування процесу в таблиці ОС.
	err = process.Signal(syscall.Signal(0))
	if err == nil || errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
