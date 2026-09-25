package pidfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndRemove(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "sub", "delmos.pid")

	pf, err := Write(pidPath)
	if err != nil {
		t.Fatalf("неочікувана помилка запису pid-файлу: %v", err)
	}
	if pf == nil {
		t.Fatal("очікувався непустий об'єкт File")
	}

	pid, err := Read(pidPath)
	if err != nil {
		t.Fatalf("читання pid-файлу: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("очікувався PID %d, отримано %d", os.Getpid(), pid)
	}

	if err := pf.Remove(); err != nil {
		t.Fatalf("видалення pid-файлу: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("файл мав бути видалений, але os.Stat повернув: %v", err)
	}
}

func TestWriteEmptyPathIsNoop(t *testing.T) {
	pf, err := Write("")
	if err != nil {
		t.Fatalf("порожній шлях не має повертати помилку: %v", err)
	}
	if pf != nil {
		t.Errorf("порожній шлях мав повернути nil File, отримано %+v", pf)
	}
}

func TestWriteRejectsAlreadyRunningProcess(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "delmos.pid")

	pf, err := Write(pidPath)
	if err != nil {
		t.Fatalf("перший запис pid-файлу: %v", err)
	}
	defer func() { _ = pf.Remove() }()

	// Друга спроба для того ж шляху з живим процесом
	_, err = Write(pidPath)
	if err == nil {
		t.Fatal("очікувалася помилка блокування запущеного процесу, але отримано nil")
	}
}

func TestWriteOverwritesStalePID(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "stale.pid")

	// Записуємо неіснуючий PID (наприклад, 4194300 у Linux PID_MAX)
	stalePID := 4194300
	if err := os.WriteFile(pidPath, []byte("4194300\n"), 0o644); err != nil {
		t.Fatalf("підготовка застарілого pid-файлу: %v", err)
	}

	pf, err := Write(pidPath)
	if err != nil {
		t.Fatalf("застарілий pid-файл мав бути перезаписаний: %v", err)
	}
	defer func() { _ = pf.Remove() }()

	pid, err := Read(pidPath)
	if err != nil {
		t.Fatalf("читання оновленого pid-файлу: %v", err)
	}
	if pid == stalePID || pid != os.Getpid() {
		t.Errorf("pid мав бути оновлений до %d, отримано %d", os.Getpid(), pid)
	}
}

func TestReadInvalidContent(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "invalid.pid")

	if err := os.WriteFile(pidPath, []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatalf("підготовка файлу: %v", err)
	}

	if _, err := Read(pidPath); err == nil {
		t.Fatal("очікувалася помилка читання нечислового PID")
	}

	if err := os.WriteFile(pidPath, []byte("-12\n"), 0o644); err != nil {
		t.Fatalf("підготовка файлу: %v", err)
	}

	if _, err := Read(pidPath); err == nil {
		t.Fatal("очікувалася помилка читання від'ємного PID")
	}
}
