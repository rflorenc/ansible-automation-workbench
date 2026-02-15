package config

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConnectionConfig represents a pre-configured connection in the config file.
type ConnectionConfig struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Role     string `yaml:"role"` // "source" or "destination"
	Scheme   string `yaml:"scheme"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Insecure bool   `yaml:"insecure"`
}

// Config holds all configuration (CLI flags + config file).
type Config struct {
	Listen      string             `yaml:"listen"`
	Dev         bool               `yaml:"-"`
	Connections []ConnectionConfig `yaml:"connections"`

	// internal: path to config file (from CLI flag)
	configFile string
}

// Parse reads CLI flags, then overlays config file values.
// CLI flags take precedence over config file values.
func Parse() *Config {
	c := &Config{}
	flag.StringVar(&c.configFile, "config", "", "Path to config file (YAML)")
	flag.StringVar(&c.Listen, "listen", "", "HTTP listen address")
	flag.BoolVar(&c.Dev, "dev", false, "Dev mode (proxy frontend to Vite dev server)")
	flag.Parse()

	// Load config file if specified
	if c.configFile != "" {
		if err := c.loadFile(c.configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config file: %v\n", err)
			os.Exit(1)
		}
	}

	// Apply defaults for anything still unset
	if c.Listen == "" {
		c.Listen = ":8080"
	}

	return c
}

// loadFile reads a YAML config file. Values from the file are only applied
// if the corresponding CLI flag was not explicitly set.
func (c *Config) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var file Config
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	// Only apply file values if CLI flag wasn't set
	if c.Listen == "" && file.Listen != "" {
		c.Listen = file.Listen
	}

	// Connections always come from config file
	c.Connections = file.Connections

	return nil
}
