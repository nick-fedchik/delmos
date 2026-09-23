package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "delmos.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("підготовка файлу конфігурації: %v", err)
	}

	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, "server:\n  address: \"127.0.0.1:10020\"\n"))
	if err != nil {
		t.Fatalf("неочікувана помилка: %v", err)
	}

	if cfg.Database.Name != "delmos" || cfg.Database.MigratorUser != "delmos_migrator" {
		t.Errorf("типові значення БД не застосовано: %+v", cfg.Database)
	}
	if cfg.Server.ShutdownTimeout.Duration().Seconds() != 15 {
		t.Errorf("типовий shutdown_timeout має бути 15s, отримано %s", cfg.Server.ShutdownTimeout)
	}
}

func TestLoadRejectsSecretsInFile(t *testing.T) {
	path := writeConfig(t, "database:\n  user: delmos\n  password: s3cret\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("пароль у YAML має відхилятися, щоб секрет не потрапив у репозиторій")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("повідомлення про помилку не повинно містити значення секрету: %v", err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]string{
		"порт поза діапазоном":   "database:\n  port: 70000\n",
		"невідомий sslmode":      "database:\n  ssl_mode: maybe\n",
		"адреса без порту":       "server:\n  address: \"127.0.0.1\"\n",
		"нульовий таймаут":       "server:\n  shutdown_timeout: 0s\n",
		"невідомий рівень логів": "logging:\n  level: trace\n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("очікувалася помилка валідації")
			}
		})
	}
}

func TestEnvOverridesFile(t *testing.T) {
	t.Setenv("DELMOS_DB_NAME", "delmos_env")
	t.Setenv("DELMOS_DB_PORT", "6432")

	cfg, err := Load(writeConfig(t, "database:\n  name: delmos_file\n  port: 5432\n"))
	if err != nil {
		t.Fatalf("неочікувана помилка: %v", err)
	}

	if cfg.Database.Name != "delmos_env" || cfg.Database.Port != 6432 {
		t.Errorf("змінні середовища мають переважати над файлом: %+v", cfg.Database)
	}
}

func TestDSNBuilding(t *testing.T) {
	t.Setenv("DELMOS_DB_PASSWORD", "p@ss word")

	cfg, err := Load(writeConfig(t, "database:\n  host: /var/run/postgresql\n"))
	if err != nil {
		t.Fatalf("неочікувана помилка: %v", err)
	}

	socketDSN := cfg.Database.AppDSN()
	if !strings.Contains(socketDSN, "host=%2Fvar%2Frun%2Fpostgresql") {
		t.Errorf("UNIX-сокет має передаватися параметром host: %s", socketDSN)
	}

	cfg.Database.Host = "127.0.0.1"
	if tcpDSN := cfg.Database.AppDSN(); !strings.Contains(tcpDSN, "@127.0.0.1:5432/") {
		t.Errorf("TCP-підключення має містити host:port: %s", tcpDSN)
	}

	if migratorDSN := cfg.Database.MigratorDSN(); !strings.Contains(migratorDSN, "delmos_migrator") {
		t.Errorf("DSN мігратора має використовувати роль мігратора: %s", migratorDSN)
	}
}

func TestDatabaseLogValueHidesPassword(t *testing.T) {
	t.Setenv("DELMOS_DB_PASSWORD", "topsecret")

	cfg, err := Load(writeConfig(t, "database:\n  name: delmos\n"))
	if err != nil {
		t.Fatalf("неочікувана помилка: %v", err)
	}

	if logged := cfg.Database.LogValue().String(); strings.Contains(logged, "topsecret") {
		t.Errorf("пароль не повинен потрапляти в журнал: %s", logged)
	}
}
