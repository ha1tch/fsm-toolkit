// Config management for fsmedit.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)


// Config holds persistent editor settings
type Config struct {
	Renderer    string // "native" or "graphviz"
	FileType    string // "png" or "svg"
	LastDir     string // last used directory
	Vocabulary  string // "fsm" (default), "circuit", "generic"
	ClassLibDir string // directory for .classes.json library files
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	cwd, _ := os.Getwd()
	return Config{
		Renderer:   "native",
		FileType:   "png",
		LastDir:    cwd,
		Vocabulary: "fsm",
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fsmedit"
	}
	return filepath.Join(home, ".fsmedit")
}

// LoadConfig loads configuration from TOML file
func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	
	// Simple TOML parser for our settings
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "renderer":
			if val == "native" || val == "graphviz" {
				cfg.Renderer = val
			}
		case "file_type":
			if val == "png" || val == "svg" {
				cfg.FileType = val
			}
		case "last_dir":
			if val != "" {
				cfg.LastDir = val
			}
		case "vocabulary":
			if val == "fsm" || val == "circuit" || val == "generic" {
				cfg.Vocabulary = val
			}
		case "class_lib_dir":
			cfg.ClassLibDir = val
		}
	}
	return cfg
}

// SaveConfig saves configuration to TOML file
func SaveConfig(cfg Config) error {
	content := fmt.Sprintf("# fsmedit configuration\nrenderer = \"%s\"\nfile_type = \"%s\"\nlast_dir = \"%s\"\nvocabulary = \"%s\"\nclass_lib_dir = \"%s\"\n",
		cfg.Renderer, cfg.FileType, cfg.LastDir, cfg.Vocabulary, cfg.ClassLibDir)
	return os.WriteFile(ConfigPath(), []byte(content), 0644)
}
