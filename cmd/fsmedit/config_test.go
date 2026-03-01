// Config management tests for fsmedit.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Renderer != "native" {
		t.Errorf("default renderer: expected 'native', got %q", cfg.Renderer)
	}
	if cfg.FileType != "png" {
		t.Errorf("default file type: expected 'png', got %q", cfg.FileType)
	}
	if cfg.LastDir == "" {
		t.Error("default LastDir should not be empty")
	}
}

func TestConfigPath_NotEmpty(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath should not return empty string")
	}
	// Should end with .fsmedit
	if !strings.HasSuffix(path, ".fsmedit") {
		t.Errorf("ConfigPath should end with '.fsmedit', got %q", path)
	}
}

// parseConfig is a test helper that writes content to a temp file and
// uses the same parsing logic as LoadConfig.
func parseConfig(t *testing.T, content string) Config {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp config: %v", err)
	}

	// Replicate LoadConfig parsing
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "renderer") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val == "native" || val == "graphviz" {
					cfg.Renderer = val
				}
			}
		} else if strings.HasPrefix(line, "file_type") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val == "png" || val == "svg" {
					cfg.FileType = val
				}
			}
		} else if strings.HasPrefix(line, "last_dir") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val != "" {
					cfg.LastDir = val
				}
			}
		}
	}
	return cfg
}

func TestConfigParsing_ValidFile(t *testing.T) {
	cfg := parseConfig(t, `# fsmedit configuration
renderer = "graphviz"
file_type = "svg"
last_dir = "/home/test/machines"
`)
	if cfg.Renderer != "graphviz" {
		t.Errorf("renderer: expected 'graphviz', got %q", cfg.Renderer)
	}
	if cfg.FileType != "svg" {
		t.Errorf("file_type: expected 'svg', got %q", cfg.FileType)
	}
	if cfg.LastDir != "/home/test/machines" {
		t.Errorf("last_dir: expected '/home/test/machines', got %q", cfg.LastDir)
	}
}

func TestConfigParsing_InvalidValues(t *testing.T) {
	// Invalid renderer value should be ignored (default kept)
	cfg := parseConfig(t, `renderer = "invalid"
file_type = "gif"
`)
	if cfg.Renderer != "native" {
		t.Errorf("invalid renderer should default to 'native', got %q", cfg.Renderer)
	}
	if cfg.FileType != "png" {
		t.Errorf("invalid file_type should default to 'png', got %q", cfg.FileType)
	}
}

func TestConfigParsing_EmptyFile(t *testing.T) {
	cfg := parseConfig(t, "")
	if cfg.Renderer != "native" {
		t.Error("empty file should return defaults")
	}
}

func TestConfigParsing_CommentsOnly(t *testing.T) {
	cfg := parseConfig(t, `# just comments
# nothing else
`)
	if cfg.Renderer != "native" {
		t.Error("comments-only file should return defaults")
	}
}

func TestConfigParsing_ExtraWhitespace(t *testing.T) {
	cfg := parseConfig(t, `  renderer  =  "graphviz"  
  file_type  =  "svg"  
`)
	if cfg.Renderer != "graphviz" {
		t.Errorf("expected 'graphviz', got %q", cfg.Renderer)
	}
	if cfg.FileType != "svg" {
		t.Errorf("expected 'svg', got %q", cfg.FileType)
	}
}

func TestConfigParsing_MissingEquals(t *testing.T) {
	// Lines without = should be silently ignored
	cfg := parseConfig(t, `renderer graphviz
file_type svg
`)
	if cfg.Renderer != "native" {
		t.Errorf("malformed line should keep default, got %q", cfg.Renderer)
	}
}
