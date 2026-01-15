package fsmfile

import (
	"fmt"
	"strconv"
	"strings"
)

// Layout represents visual layout metadata for the editor.
type Layout struct {
	Version  int                  `toml:"version"`
	Editor   EditorMeta           `toml:"editor"`
	States   map[string]StateLayout `toml:"states"`
}

// EditorMeta contains editor-specific settings.
type EditorMeta struct {
	CanvasOffsetX int `toml:"canvas_offset_x"`
	CanvasOffsetY int `toml:"canvas_offset_y"`
}

// StateLayout contains position for a single state.
type StateLayout struct {
	X int `toml:"x"`
	Y int `toml:"y"`
}

// GenerateLayout creates layout.toml content from state positions.
func GenerateLayout(positions map[string][2]int, offsetX, offsetY int) string {
	var sb strings.Builder

	sb.WriteString("[layout]\n")
	sb.WriteString("version = 1\n")
	sb.WriteString("\n")

	sb.WriteString("[editor]\n")
	sb.WriteString(fmt.Sprintf("canvas_offset_x = %d\n", offsetX))
	sb.WriteString(fmt.Sprintf("canvas_offset_y = %d\n", offsetY))
	sb.WriteString("\n")

	if len(positions) > 0 {
		sb.WriteString("[states]\n")
		for name, pos := range positions {
			// Use dotted key syntax for nested tables
			sb.WriteString(fmt.Sprintf("[states.%q]\n", name))
			sb.WriteString(fmt.Sprintf("x = %d\n", pos[0]))
			sb.WriteString(fmt.Sprintf("y = %d\n", pos[1]))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ParseLayout parses layout.toml content.
func ParseLayout(text string) (*Layout, error) {
	layout := &Layout{
		States: make(map[string]StateLayout),
	}

	var currentSection string
	var currentState string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]
			
			// Check for states subsection like [states."green"]
			if strings.HasPrefix(section, "states.") {
				currentSection = "states"
				// Extract state name, handling quoted names
				statePart := section[7:] // after "states."
				currentState = unquoteKey(statePart)
				if _, exists := layout.States[currentState]; !exists {
					layout.States[currentState] = StateLayout{}
				}
			} else {
				currentSection = section
				currentState = ""
			}
			continue
		}

		// Key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "layout":
			if key == "version" {
				layout.Version, _ = strconv.Atoi(value)
			}
		case "editor":
			switch key {
			case "canvas_offset_x":
				layout.Editor.CanvasOffsetX, _ = strconv.Atoi(value)
			case "canvas_offset_y":
				layout.Editor.CanvasOffsetY, _ = strconv.Atoi(value)
			}
		case "states":
			if currentState != "" {
				sl := layout.States[currentState]
				switch key {
				case "x":
					sl.X, _ = strconv.Atoi(value)
				case "y":
					sl.Y, _ = strconv.Atoi(value)
				}
				layout.States[currentState] = sl
			}
		}
	}

	return layout, nil
}

// unquoteKey removes surrounding quotes from a TOML key.
func unquoteKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
