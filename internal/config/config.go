package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds the configuration for the application
type Config struct {
	Dispatcher   string
	Consolidator string
	FileServer   string
}

// LoadConfig reads the configuration from a file and returns a Config struct
func LoadConfig(filepath string) (*Config, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	config := &Config{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue // Skip empty lines and comments
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "dispatcher":
			config.Dispatcher = value
		case "consolidator":
			config.Consolidator = value
		case "fileserver":
			config.FileServer = value
		default:
			return nil, fmt.Errorf("unknown config key: %s", key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if config.Dispatcher == "" || config.Consolidator == "" || config.FileServer == "" {
		return nil, fmt.Errorf("missing required config values")
	}

	return config, nil
}
