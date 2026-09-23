// Package config завантажує та валідує конфігурацію DELMOS.
//
// Секрети (паролі БД) приймаються виключно зі змінних середовища; наявність
// пароля у YAML-файлі є помилкою конфігурації, а не попередженням.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultPath — штатне розташування конфігурації згідно з FHS-розкладкою DELMOS.
const DefaultPath = "/usr/local/etc/delmos/delmos.yaml"

type Config struct {
	Server   Server   `yaml:"server"`
	Database Database `yaml:"database"`
	Logging  Logging  `yaml:"logging"`
}

type Server struct {
	Address         string   `yaml:"address"`
	ReadTimeout     Duration `yaml:"read_timeout"`
	WriteTimeout    Duration `yaml:"write_timeout"`
	IdleTimeout     Duration `yaml:"idle_timeout"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`
	// CookieSecure ввімкає прапорець Secure і префікс __Host- у сесійних cookie.
	// Вимкнений лише для локальної розробки без TLS (ARCHITECTURE.md: Secure під HTTPS).
	CookieSecure bool `yaml:"cookie_secure"`
}

type Database struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Name           string `yaml:"name"`
	User           string `yaml:"user"`
	MigratorUser   string `yaml:"migrator_user"`
	SSLMode        string `yaml:"ssl_mode"`
	MaxConnections int32  `yaml:"max_connections"`

	password         string
	migratorPassword string
}

type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func defaults() Config {
	return Config{
		Server: Server{
			Address:         "127.0.0.1:10020",
			ReadTimeout:     Duration(10 * time.Second),
			WriteTimeout:    Duration(30 * time.Second),
			IdleTimeout:     Duration(60 * time.Second),
			ShutdownTimeout: Duration(15 * time.Second),
			CookieSecure:    true,
		},
		Database: Database{
			Host:           "/var/run/postgresql",
			Port:           5432,
			Name:           "delmos",
			User:           "delmos",
			MigratorUser:   "delmos_migrator",
			SSLMode:        "prefer",
			MaxConnections: 10,
		},
		Logging: Logging{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load читає YAML-файл (якщо він існує), застосовує змінні середовища та валідує результат.
func Load(path string) (Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // шлях задає оператор прапорцем -config, а не недовірений ввід
		switch {
		case err == nil:
			decoder := yaml.NewDecoder(bytes.NewReader(data))
			decoder.KnownFields(true) // невідомий ключ (зокрема password) — помилка, а не мовчазне ігнорування
			if err := decoder.Decode(&cfg); err != nil {
				return Config{}, fmt.Errorf("розбір %s: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist) && path == DefaultPath:
			// Штатний шлях відсутній — працюємо на значеннях за замовчуванням і змінних середовища.
		default:
			return Config{}, fmt.Errorf("читання %s: %w", path, err)
		}
	}

	if err := cfg.applyEnv(); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) applyEnv() error {
	stringEnv := map[string]*string{
		"DELMOS_SERVER_ADDRESS":       &c.Server.Address,
		"DELMOS_DB_HOST":              &c.Database.Host,
		"DELMOS_DB_NAME":              &c.Database.Name,
		"DELMOS_DB_USER":              &c.Database.User,
		"DELMOS_DB_MIGRATOR_USER":     &c.Database.MigratorUser,
		"DELMOS_DB_SSLMODE":           &c.Database.SSLMode,
		"DELMOS_DB_PASSWORD":          &c.Database.password,
		"DELMOS_DB_MIGRATOR_PASSWORD": &c.Database.migratorPassword,
		"DELMOS_LOG_LEVEL":            &c.Logging.Level,
		"DELMOS_LOG_FORMAT":           &c.Logging.Format,
	}
	for key, target := range stringEnv {
		if value, ok := os.LookupEnv(key); ok {
			*target = value
		}
	}

	if value, ok := os.LookupEnv("DELMOS_DB_PORT"); ok {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("DELMOS_DB_PORT: %w", err)
		}
		c.Database.Port = port
	}

	if value, ok := os.LookupEnv("DELMOS_COOKIE_SECURE"); ok {
		secure, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("DELMOS_COOKIE_SECURE: %w", err)
		}
		c.Server.CookieSecure = secure
	}

	if c.Database.migratorPassword == "" {
		c.Database.migratorPassword = c.Database.password
	}

	return nil
}

var allowedSSLModes = map[string]bool{
	"disable": true, "allow": true, "prefer": true,
	"require": true, "verify-ca": true, "verify-full": true,
}

func (c Config) validate() error {
	var problems []string

	host, port, err := net.SplitHostPort(c.Server.Address)
	if err != nil {
		problems = append(problems, fmt.Sprintf("server.address %q має бути у форматі host:port", c.Server.Address))
	} else {
		listenPort, err := strconv.Atoi(port)
		if err != nil || listenPort < 1 || listenPort > 65535 {
			problems = append(problems, fmt.Sprintf("server.address: порт %q поза діапазоном 1..65535", port))
		}
		if host == "" {
			problems = append(problems, "server.address: хост обов'язковий; DELMOS слухає локально за реверс-проксі")
		}
	}

	for name, d := range map[string]Duration{
		"server.read_timeout":     c.Server.ReadTimeout,
		"server.write_timeout":    c.Server.WriteTimeout,
		"server.idle_timeout":     c.Server.IdleTimeout,
		"server.shutdown_timeout": c.Server.ShutdownTimeout,
	} {
		if d <= 0 {
			problems = append(problems, name+" має бути більшим за нуль")
		}
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		problems = append(problems, fmt.Sprintf("database.port %d поза діапазоном 1..65535", c.Database.Port))
	}
	if c.Database.Name == "" {
		problems = append(problems, "database.name обов'язковий")
	}
	if c.Database.User == "" {
		problems = append(problems, "database.user обов'язковий")
	}
	if c.Database.MigratorUser == "" {
		problems = append(problems, "database.migrator_user обов'язковий")
	}
	if !allowedSSLModes[c.Database.SSLMode] {
		problems = append(problems, fmt.Sprintf("database.ssl_mode %q не підтримується", c.Database.SSLMode))
	}
	if c.Database.MaxConnections < 1 || c.Database.MaxConnections > 500 {
		problems = append(problems, fmt.Sprintf("database.max_connections %d поза діапазоном 1..500", c.Database.MaxConnections))
	}

	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf("logging.level %q не підтримується", c.Logging.Level))
	}
	switch c.Logging.Format {
	case "json", "text":
	default:
		problems = append(problems, fmt.Sprintf("logging.format %q не підтримується", c.Logging.Format))
	}

	if len(problems) > 0 {
		return fmt.Errorf("некоректна конфігурація:\n  - %s", strings.Join(problems, "\n  - "))
	}

	return nil
}

// AppDSN повертає рядок підключення робітничої ролі застосунку.
func (d Database) AppDSN() string {
	return d.dsn(d.User, d.password)
}

// MigratorDSN повертає рядок підключення ролі мігратора (DDL).
func (d Database) MigratorDSN() string {
	return d.dsn(d.MigratorUser, d.migratorPassword)
}

func (d Database) dsn(user, password string) string {
	dsn := url.URL{
		Scheme: "postgres",
		Path:   "/" + d.Name,
	}
	if password == "" {
		dsn.User = url.User(user)
	} else {
		dsn.User = url.UserPassword(user, password)
	}

	query := url.Values{}
	query.Set("sslmode", d.SSLMode)

	if strings.HasPrefix(d.Host, "/") {
		query.Set("host", d.Host) // UNIX-сокет передається параметром, а не в частині host URL
		query.Set("port", strconv.Itoa(d.Port))
	} else {
		dsn.Host = net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
	}

	dsn.RawQuery = query.Encode()
	return dsn.String()
}

// LogValue гарантує, що паролі ніколи не потрапляють у журнал разом із параметрами БД.
func (d Database) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("host", d.Host),
		slog.Int("port", d.Port),
		slog.String("name", d.Name),
		slog.String("user", d.User),
		slog.String("migrator_user", d.MigratorUser),
		slog.String("ssl_mode", d.SSLMode),
	)
}
