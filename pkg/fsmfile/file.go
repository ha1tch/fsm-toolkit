package fsmfile

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// Labels represents the labels.toml content.
type Labels struct {
	FSM     FSMMeta        `toml:"fsm"`
	States  map[int]string `toml:"states"`
	Inputs  map[int]string `toml:"inputs"`
	Outputs map[int]string `toml:"outputs"`
}

// FSMMeta contains FSM metadata.
type FSMMeta struct {
	Version     int    `toml:"version"`
	Type        string `toml:"type"`
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// GenerateLabels creates labels.toml content.
func GenerateLabels(f *fsm.FSM, states, inputs, outputs map[int]string) string {
	var sb strings.Builder
	
	sb.WriteString("[fsm]\n")
	sb.WriteString("version = 1\n")
	sb.WriteString(fmt.Sprintf("type = %q\n", f.Type))
	if f.Name != "" {
		sb.WriteString(fmt.Sprintf("name = %q\n", f.Name))
	}
	if f.Description != "" {
		sb.WriteString(fmt.Sprintf("description = %q\n", f.Description))
	}
	sb.WriteString("\n")
	
	if len(states) > 0 {
		sb.WriteString("[states]\n")
		keys := sortedKeys(states)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("0x%04X = %q\n", k, states[k]))
		}
		sb.WriteString("\n")
	}
	
	if len(inputs) > 0 {
		sb.WriteString("[inputs]\n")
		keys := sortedKeys(inputs)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("0x%04X = %q\n", k, inputs[k]))
		}
		sb.WriteString("\n")
	}
	
	if len(outputs) > 0 {
		sb.WriteString("[outputs]\n")
		keys := sortedKeys(outputs)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("0x%04X = %q\n", k, outputs[k]))
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// ParseLabels parses labels.toml content.
// Simple parser that doesn't require external TOML library.
func ParseLabels(text string) (*Labels, error) {
	labels := &Labels{
		States:  make(map[int]string),
		Inputs:  make(map[int]string),
		Outputs: make(map[int]string),
	}
	
	var currentSection string
	
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}
		
		// Key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes from value
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		switch currentSection {
		case "fsm":
			switch key {
			case "version":
				labels.FSM.Version, _ = strconv.Atoi(value)
			case "type":
				labels.FSM.Type = value
			case "name":
				labels.FSM.Name = value
			case "description":
				labels.FSM.Description = value
			}
		case "states":
			idx := parseHexKey(key)
			if idx >= 0 {
				labels.States[idx] = value
			}
		case "inputs":
			idx := parseHexKey(key)
			if idx >= 0 {
				labels.Inputs[idx] = value
			}
		case "outputs":
			idx := parseHexKey(key)
			if idx >= 0 {
				labels.Outputs[idx] = value
			}
		}
	}
	
	return labels, nil
}

func parseHexKey(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseInt(s[2:], 16, 32)
		if err == nil {
			return int(v)
		}
	}
	v, err := strconv.Atoi(s)
	if err == nil {
		return v
	}
	return -1
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// WriteFSMFile writes an FSM to a .fsm file.
func WriteFSMFile(path string, f *fsm.FSM, includeLabels bool) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return WriteFSM(file, f, includeLabels)
}

// WriteFSM writes an FSM to a writer in .fsm format.
func WriteFSM(w io.Writer, f *fsm.FSM, includeLabels bool) error {
	zw := zip.NewWriter(w)
	defer zw.Close()
	
	// Convert to records
	records, states, inputs, outputs := FSMToRecords(f)
	
	// Write machine.hex
	hexContent := FormatHex(records, 4) + "\n"
	hw, err := zw.Create("machine.hex")
	if err != nil {
		return err
	}
	if _, err := hw.Write([]byte(hexContent)); err != nil {
		return err
	}
	
	// Write labels.toml if requested
	if includeLabels {
		labelsContent := GenerateLabels(f, states, inputs, outputs)
		lw, err := zw.Create("labels.toml")
		if err != nil {
			return err
		}
		if _, err := lw.Write([]byte(labelsContent)); err != nil {
			return err
		}
	}
	
	return nil
}

// ReadFSMFile reads an FSM from a .fsm file.
func ReadFSMFile(path string) (*fsm.FSM, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	
	return ReadFSM(file, info.Size())
}

// ReadFSM reads an FSM from a reader containing .fsm format.
func ReadFSM(r io.ReaderAt, size int64) (*fsm.FSM, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	
	var hexContent, labelsContent string
	
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		
		switch f.Name {
		case "machine.hex":
			hexContent = string(data)
		case "labels.toml":
			labelsContent = string(data)
		}
	}
	
	if hexContent == "" {
		return nil, fmt.Errorf("machine.hex not found in archive")
	}
	
	records, err := ParseHex(hexContent)
	if err != nil {
		return nil, err
	}
	
	var labels *Labels
	if labelsContent != "" {
		labels, err = ParseLabels(labelsContent)
		if err != nil {
			return nil, err
		}
	}
	
	return RecordsToFSM(records, labels)
}

// ReadFSMBytes reads an FSM from bytes in .fsm format.
func ReadFSMBytes(data []byte) (*fsm.FSM, error) {
	r := bytes.NewReader(data)
	return ReadFSM(r, int64(len(data)))
}
