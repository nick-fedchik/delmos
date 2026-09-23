package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration дозволяє записувати тривалості у YAML у вигляді "15s", "2m" тощо.
type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("тривалість має бути рядком (наприклад \"15s\"): %w", err)
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("некоректна тривалість %q: %w", raw, err)
	}

	*d = Duration(parsed)
	return nil
}
