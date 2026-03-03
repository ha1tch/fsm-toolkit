package fsmfile

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// Labels represents the labels.toml content.
type Labels struct {
	FSM      FSMMeta           `toml:"fsm"`
	States   map[int]string    `toml:"states"`
	Inputs   map[int]string    `toml:"inputs"`
	Outputs  map[int]string    `toml:"outputs"`
	Machines map[string]string `toml:"machines"` // state name -> linked machine name
	Nets     map[string]string `toml:"nets"`     // net name -> "U3.3Y, U7.2D"
}

// FSMMeta contains FSM metadata.
type FSMMeta struct {
	Version     int    `toml:"version"`
	Type        string `toml:"type"`
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Vocabulary  string `toml:"vocabulary"`
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
	if f.Vocabulary != "" {
		sb.WriteString(fmt.Sprintf("vocabulary = %q\n", f.Vocabulary))
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
	
	// Write linked machines section if any
	if f.HasLinkedStates() {
		sb.WriteString("[machines]\n")
		for state, machine := range f.LinkedMachines {
			if machine != "" {
				sb.WriteString(fmt.Sprintf("%q = %q\n", state, machine))
			}
		}
		sb.WriteString("\n")
	}

	// Write nets section if any
	if len(f.Nets) > 0 {
		sb.WriteString("[nets]\n")
		for _, n := range f.Nets {
			var eps []string
			for _, ep := range n.Endpoints {
				eps = append(eps, ep.Instance+"."+ep.Port)
			}
			sb.WriteString(fmt.Sprintf("%q = %q\n", n.Name, strings.Join(eps, ", ")))
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// ParseLabels parses labels.toml content.
// Simple parser that doesn't require external TOML library.
func ParseLabels(text string) (*Labels, error) {
	labels := &Labels{
		States:   make(map[int]string),
		Inputs:   make(map[int]string),
		Outputs:  make(map[int]string),
		Machines: make(map[string]string),
		Nets:     make(map[string]string),
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
		
		// Remove quotes from key if present (for machines section)
		if len(key) >= 2 && key[0] == '"' && key[len(key)-1] == '"' {
			key = key[1 : len(key)-1]
		}
		
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
			case "vocabulary":
				labels.FSM.Vocabulary = value
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
		case "machines":
			// key is state name (string), value is machine name
			labels.Machines[key] = value
		case "nets":
			// key is net name (string), value is endpoint list string
			labels.Nets[key] = value
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
	return WriteFSMFileWithLayout(path, f, includeLabels, nil, 0, 0)
}

// WriteFSMFileWithLayout writes an FSM with layout metadata.
func WriteFSMFileWithLayout(path string, f *fsm.FSM, includeLabels bool, positions map[string][2]int, offsetX, offsetY int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return WriteFSMWithLayout(file, f, includeLabels, positions, offsetX, offsetY)
}

// WriteFSM writes an FSM to a writer in .fsm format.
func WriteFSM(w io.Writer, f *fsm.FSM, includeLabels bool) error {
	return WriteFSMWithLayout(w, f, includeLabels, nil, 0, 0)
}

// WriteFSMWithLayout writes an FSM with layout to a writer.
func WriteFSMWithLayout(w io.Writer, f *fsm.FSM, includeLabels bool, positions map[string][2]int, offsetX, offsetY int) error {
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
	
	// Write layout.toml if positions provided
	if len(positions) > 0 {
		layoutContent := GenerateLayout(positions, offsetX, offsetY)
		lw, err := zw.Create("layout.toml")
		if err != nil {
			return err
		}
		if _, err := lw.Write([]byte(layoutContent)); err != nil {
			return err
		}
	}

	// Write classes.json if class data present
	if classData, cerr := generateClassesJSON(f); cerr != nil {
		return cerr
	} else if classData != nil {
		cw, err := zw.Create("classes.json")
		if err != nil {
			return err
		}
		if _, err := cw.Write(classData); err != nil {
			return err
		}
	}
	
	return nil
}

// ReadFSMFile reads an FSM from a .fsm file.
func ReadFSMFile(path string) (*fsm.FSM, error) {
	f, _, err := ReadFSMFileWithLayout(path)
	return f, err
}

// ReadFSMFileWithLayout reads an FSM and layout from a .fsm file.
func ReadFSMFileWithLayout(path string) (*fsm.FSM, *Layout, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	
	return ReadFSMWithLayout(file, info.Size())
}

// ReadFSM reads an FSM from a reader containing .fsm format.
func ReadFSM(r io.ReaderAt, size int64) (*fsm.FSM, error) {
	f, _, err := ReadFSMWithLayout(r, size)
	return f, err
}

// ReadFSMWithLayout reads an FSM and layout from a reader.
func ReadFSMWithLayout(r io.ReaderAt, size int64) (*fsm.FSM, *Layout, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, err
	}
	
	var hexContent, labelsContent, layoutContent string
	var classesData []byte
	
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}
		
		switch f.Name {
		case "machine.hex":
			hexContent = string(data)
		case "labels.toml":
			labelsContent = string(data)
		case "layout.toml":
			layoutContent = string(data)
		case "classes.json":
			classesData = data
		}
	}
	
	if hexContent == "" {
		return nil, nil, fmt.Errorf("machine.hex not found in archive")
	}
	
	records, err := ParseHex(hexContent)
	if err != nil {
		return nil, nil, err
	}
	
	var labels *Labels
	if labelsContent != "" {
		labels, err = ParseLabels(labelsContent)
		if err != nil {
			return nil, nil, err
		}
	}
	
	var layout *Layout
	if layoutContent != "" {
		layout, err = ParseLayout(layoutContent)
		if err != nil {
			return nil, nil, err
		}
	}
	
	fsmResult, err := RecordsToFSM(records, labels)
	if err != nil {
		return nil, nil, err
	}

	// Apply class data if present
	if classesData != nil {
		if err := applyClassesJSON(fsmResult, classesData); err != nil {
			return nil, nil, err
		}
	}

	return fsmResult, layout, nil
}

// ReadFSMBytes reads an FSM from bytes in .fsm format.
func ReadFSMBytes(data []byte) (*fsm.FSM, error) {
	r := bytes.NewReader(data)
	return ReadFSM(r, int64(len(data)))
}

// ReadFSMBytesWithLayout reads an FSM and layout from bytes.
func ReadFSMBytesWithLayout(data []byte) (*fsm.FSM, *Layout, error) {
	r := bytes.NewReader(data)
	return ReadFSMWithLayout(r, int64(len(data)))
}

// MachineInfo contains basic information about a machine in a bundle.
type MachineInfo struct {
	Name        string // filename without .hex extension
	HexFile     string // full filename (e.g., "main.hex" or "pedestrian.hex")
	Type        string // FSM type if labels available
	Description string // description if labels available
	StateCount  int    // number of states
	TransCount  int    // number of transitions
}

// ListMachines returns information about all machines in a .fsm bundle.
// A bundle can contain multiple .hex files. The traditional single-machine
// format uses "machine.hex"; bundles use named files like "main.hex", "child.hex".
func ListMachines(path string) ([]MachineInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return ListMachinesFromReader(file, info.Size())
}

// ListMachinesFromReader returns machine info from a reader.
func ListMachinesFromReader(r io.ReaderAt, size int64) ([]MachineInfo, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	// Collect all .hex files and labels
	hexFiles := make(map[string]string)       // filename -> content
	labelsContent := make(map[string]string)  // machine name -> labels content

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".hex") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			hexFiles[f.Name] = string(data)
		} else if strings.HasSuffix(f.Name, ".toml") && strings.Contains(f.Name, "labels") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			// Determine which machine this labels file belongs to
			// "labels.toml" -> "machine", "pedestrian.labels.toml" -> "pedestrian"
			name := f.Name
			if name == "labels.toml" {
				labelsContent["machine"] = string(data)
			} else {
				// Extract machine name from "name.labels.toml" or "[name]" sections
				parts := strings.Split(name, ".")
				if len(parts) >= 2 {
					labelsContent[parts[0]] = string(data)
				}
			}
		}
	}

	if len(hexFiles) == 0 {
		return nil, fmt.Errorf("no .hex files found in archive")
	}

	// Build machine info list
	var machines []MachineInfo
	for hexFile, content := range hexFiles {
		// Derive machine name from filename
		name := strings.TrimSuffix(hexFile, ".hex")

		// Parse hex to count states and transitions
		records, err := ParseHex(content)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", hexFile, err)
		}

		stateCount := 0
		transCount := 0
		fsmType := ""
		hasMealy := false
		hasNFAMulti := false
		hasMooreOutput := false
		
		for _, rec := range records {
			switch rec.Type {
			case 0x0002: // TypeStateDecl
				stateCount++
				// Check if state has Moore output (Field3 != 0)
				if rec.Field3 != 0 {
					hasMooreOutput = true
				}
			case 0x0000: // TypeDFATransition
				transCount++
			case 0x0001: // TypeMealyTransition
				transCount++
				hasMealy = true
			case 0x0003: // TypeNFAMulti
				transCount++
				hasNFAMulti = true
			}
		}
		
		// Infer FSM type from record types
		if hasMealy {
			fsmType = "MEALY"
		} else if hasMooreOutput {
			fsmType = "MOORE"
		} else if hasNFAMulti {
			fsmType = "NFA"
		} else {
			fsmType = "DFA"
		}

		info := MachineInfo{
			Name:       name,
			HexFile:    hexFile,
			Type:       fsmType,
			StateCount: stateCount,
			TransCount: transCount,
		}

		// Try to get description from labels
		if labels, ok := labelsContent[name]; ok {
			parsed, err := ParseLabels(labels)
			if err == nil {
				info.Description = parsed.FSM.Description
				if info.Type == "" {
					info.Type = strings.ToUpper(parsed.FSM.Type)
				}
			}
		}

		machines = append(machines, info)
	}

	// Sort by name for consistent ordering
	sort.Slice(machines, func(i, j int) bool {
		// "machine" (default) comes first, then alphabetical
		if machines[i].Name == "machine" {
			return true
		}
		if machines[j].Name == "machine" {
			return false
		}
		return machines[i].Name < machines[j].Name
	})

	return machines, nil
}

// IsBundle returns true if the .fsm file contains multiple machines.
func IsBundle(path string) (bool, error) {
	machines, err := ListMachines(path)
	if err != nil {
		return false, err
	}
	return len(machines) > 1, nil
}

// ReadMachineFromBundle reads a specific machine from a bundle by name.
// If name is empty, reads the default "machine.hex" or the first available.
func ReadMachineFromBundle(path string, machineName string) (*fsm.FSM, *Layout, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}

	return ReadMachineFromBundleReader(file, info.Size(), machineName)
}

// ReadMachineFromBundleReader reads a specific machine from a bundle.
func ReadMachineFromBundleReader(r io.ReaderAt, size int64, machineName string) (*fsm.FSM, *Layout, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, err
	}

	// Determine which hex file to read
	targetHex := "machine.hex"
	if machineName != "" && machineName != "machine" {
		targetHex = machineName + ".hex"
	}

	var hexContent, labelsContent, layoutContent string
	var classesData []byte
	var foundHex bool

	// First pass: look for exact match
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}

		switch {
		case f.Name == targetHex:
			hexContent = string(data)
			foundHex = true
		case f.Name == "labels.toml" && (machineName == "" || machineName == "machine"):
			labelsContent = string(data)
		case f.Name == machineName+".labels.toml":
			labelsContent = string(data)
		case f.Name == "layout.toml" && (machineName == "" || machineName == "machine"):
			layoutContent = string(data)
		case f.Name == machineName+".layout.toml":
			layoutContent = string(data)
		case f.Name == "classes.json" && (machineName == "" || machineName == "machine"):
			classesData = data
		case f.Name == machineName+".classes.json":
			classesData = data
		}
	}

	// If not found and no specific name requested, try first .hex file
	if !foundHex && machineName == "" {
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".hex") {
				rc, err := f.Open()
				if err != nil {
					return nil, nil, err
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, nil, err
				}
				hexContent = string(data)
				foundHex = true
				break
			}
		}
	}

	if !foundHex {
		return nil, nil, fmt.Errorf("machine %q not found in bundle", machineName)
	}

	records, err := ParseHex(hexContent)
	if err != nil {
		return nil, nil, err
	}

	var labels *Labels
	if labelsContent != "" {
		labels, err = ParseLabels(labelsContent)
		if err != nil {
			return nil, nil, err
		}
	}

	var layout *Layout
	if layoutContent != "" {
		layout, err = ParseLayout(layoutContent)
		if err != nil {
			return nil, nil, err
		}
	}

	fsmResult, err := RecordsToFSM(records, labels)
	if err != nil {
		return nil, nil, err
	}

	// Apply class data if present
	if classesData != nil {
		if err := applyClassesJSON(fsmResult, classesData); err != nil {
			return nil, nil, err
		}
	}

	return fsmResult, layout, nil
}

// CreateBundle combines multiple .fsm files into a single bundle.
// Each input file becomes a named machine (derived from filename).
func CreateBundle(inputs []string, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	for _, inputPath := range inputs {
		// Derive machine name from filename
		baseName := filepath.Base(inputPath)
		machineName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		// Read the input .fsm file
		inputFile, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("opening %s: %w", inputPath, err)
		}

		info, err := inputFile.Stat()
		if err != nil {
			inputFile.Close()
			return fmt.Errorf("stat %s: %w", inputPath, err)
		}

		zr, err := zip.NewReader(inputFile, info.Size())
		if err != nil {
			inputFile.Close()
			return fmt.Errorf("reading %s: %w", inputPath, err)
		}

		// Copy files with namespaced names
		for _, f := range zr.File {
			var outName string
			switch f.Name {
			case "machine.hex":
				outName = machineName + ".hex"
			case "labels.toml":
				outName = machineName + ".labels.toml"
			case "layout.toml":
				outName = machineName + ".layout.toml"
			case "classes.json":
				outName = machineName + ".classes.json"
			default:
				// Skip unknown files or already-namespaced files
				continue
			}

			rc, err := f.Open()
			if err != nil {
				inputFile.Close()
				return fmt.Errorf("reading %s from %s: %w", f.Name, inputPath, err)
			}

			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				inputFile.Close()
				return fmt.Errorf("reading %s from %s: %w", f.Name, inputPath, err)
			}

			w, err := zw.Create(outName)
			if err != nil {
				inputFile.Close()
				return fmt.Errorf("creating %s: %w", outName, err)
			}

			if _, err := w.Write(data); err != nil {
				inputFile.Close()
				return fmt.Errorf("writing %s: %w", outName, err)
			}
		}

		inputFile.Close()
	}

	return nil
}

// LinkValidationResult contains the result of bundle link validation.
type LinkValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// ValidateBundleLinks validates all linked state references in a bundle.
// It checks that:
// - All linked machines exist in the bundle
// - Linked machines are DFAs (required for delegation)
// - No circular links exist
// - Linked states have accept/reject transitions defined
func ValidateBundleLinks(path string) (*LinkValidationResult, error) {
	result := &LinkValidationResult{Valid: true}
	
	// List all machines in bundle
	machines, err := ListMachines(path)
	if err != nil {
		return nil, fmt.Errorf("listing machines: %w", err)
	}
	
	if len(machines) < 2 {
		// Not a bundle or single machine - nothing to validate
		return result, nil
	}
	
	// Build machine name set and load all FSMs
	machineSet := make(map[string]bool)
	machineTypes := make(map[string]string)
	machineFSMs := make(map[string]*fsm.FSM)
	
	for _, m := range machines {
		machineSet[m.Name] = true
		machineTypes[m.Name] = m.Type
		
		// Load each machine
		f, _, err := ReadMachineFromBundle(path, m.Name)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to load %s: %v", m.Name, err))
			result.Valid = false
			continue
		}
		machineFSMs[m.Name] = f
	}
	
	// Check each machine's linked states
	for machineName, f := range machineFSMs {
		if !f.HasLinkedStates() {
			continue
		}
		
		for state, targetMachine := range f.LinkedMachines {
			if targetMachine == "" {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: state %q is marked as linked but has no target machine", machineName, state))
				continue
			}
			
			// Check target exists
			if !machineSet[targetMachine] {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: state %q links to non-existent machine %q", machineName, state, targetMachine))
				result.Valid = false
				continue
			}
			
			// Check target is DFA
			if machineTypes[targetMachine] != "dfa" && machineTypes[targetMachine] != "DFA" {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: state %q links to %s which is %s (must be DFA)",
						machineName, state, targetMachine, machineTypes[targetMachine]))
				result.Valid = false
			}
			
			// Check for self-link
			if targetMachine == machineName {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: state %q links to itself", machineName, state))
				result.Valid = false
			}
		}
	}
	
	// Check for circular links
	circularErrors := detectCircularLinks(machineFSMs)
	if len(circularErrors) > 0 {
		result.Errors = append(result.Errors, circularErrors...)
		result.Valid = false
	}
	
	return result, nil
}

// detectCircularLinks finds circular link chains between machines.
func detectCircularLinks(machines map[string]*fsm.FSM) []string {
	var errors []string
	
	// Build link graph: machine -> set of machines it links to
	linkGraph := make(map[string]map[string]bool)
	for name, f := range machines {
		if !f.HasLinkedStates() {
			continue
		}
		linkGraph[name] = make(map[string]bool)
		for _, target := range f.LinkedMachines {
			if target != "" {
				linkGraph[name][target] = true
			}
		}
	}
	
	// DFS to detect cycles
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	
	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		if inStack[node] {
			// Found cycle - find where it starts
			for i, p := range path {
				if p == node {
					cycle := append(path[i:], node)
					errors = append(errors, fmt.Sprintf("circular link detected: %s", 
						strings.Join(cycle, " → ")))
					return true
				}
			}
			return true
		}
		
		if visited[node] {
			return false
		}
		
		visited[node] = true
		inStack[node] = true
		
		for target := range linkGraph[node] {
			if dfs(target, append(path, node)) {
				return true
			}
		}
		
		inStack[node] = false
		return false
	}
	
	for name := range linkGraph {
		if !visited[name] {
			dfs(name, nil)
		}
	}
	
	return errors
}

// BundleMachineData holds FSM and layout data for a machine in a bundle
type BundleMachineData struct {
	FSM       *fsm.FSM
	Positions map[string][2]int
	OffsetX   int
	OffsetY   int
}

// UpdateBundleMachines updates specific machines in a bundle file.
// Only the machines in the 'updates' map are modified; others are preserved.
func UpdateBundleMachines(bundlePath string, updates map[string]BundleMachineData) error {
	// Read existing bundle
	existingFile, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("opening bundle: %w", err)
	}
	
	info, err := existingFile.Stat()
	if err != nil {
		existingFile.Close()
		return fmt.Errorf("stat bundle: %w", err)
	}
	
	zr, err := zip.NewReader(existingFile, info.Size())
	if err != nil {
		existingFile.Close()
		return fmt.Errorf("reading bundle: %w", err)
	}
	
	// Collect all existing files
	existingFiles := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			existingFile.Close()
			return fmt.Errorf("reading %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			existingFile.Close()
			return fmt.Errorf("reading %s: %w", f.Name, err)
		}
		existingFiles[f.Name] = data
	}
	existingFile.Close()
	
	// Generate updated files for each machine in updates
	for machineName, data := range updates {
		// Generate hex records
		records, _, _, _ := FSMToRecords(data.FSM)
		hexContent := FormatHex(records, 1)
		existingFiles[machineName+".hex"] = []byte(hexContent)
		
		// Generate labels.toml
		labelsContent := generateLabelsToml(data.FSM)
		existingFiles[machineName+".labels.toml"] = []byte(labelsContent)
		
		// Generate layout.toml
		if len(data.Positions) > 0 {
			layoutContent := generateLayoutToml(data.Positions, data.OffsetX, data.OffsetY)
			existingFiles[machineName+".layout.toml"] = []byte(layoutContent)
		}

		// Generate classes.json
		if classData, cerr := generateClassesJSON(data.FSM); cerr == nil && classData != nil {
			existingFiles[machineName+".classes.json"] = classData
		} else if !hasClassData(data.FSM) {
			// Remove stale classes.json if class data was cleared
			delete(existingFiles, machineName+".classes.json")
		}
	}
	
	// Write new bundle to temp file
	tempPath := bundlePath + ".tmp"
	outFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	
	zw := zip.NewWriter(outFile)
	
	for name, data := range existingFiles {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(tempPath)
			return fmt.Errorf("creating %s: %w", name, err)
		}
		if _, err := w.Write(data); err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(tempPath)
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	
	if err := zw.Close(); err != nil {
		outFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("closing zip: %w", err)
	}
	outFile.Close()
	
	// Replace original with temp
	if err := os.Rename(tempPath, bundlePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("replacing bundle: %w", err)
	}
	
	return nil
}

// generateLabelsToml creates labels.toml content for an FSM
func generateLabelsToml(f *fsm.FSM) string {
	var buf bytes.Buffer
	
	buf.WriteString("[fsm]\n")
	buf.WriteString("version = 1\n")
	buf.WriteString(fmt.Sprintf("type = %q\n", f.Type))
	if f.Name != "" {
		buf.WriteString(fmt.Sprintf("name = %q\n", f.Name))
	}
	if f.Description != "" {
		buf.WriteString(fmt.Sprintf("description = %q\n", f.Description))
	}
	buf.WriteString("\n")
	
	buf.WriteString("[states]\n")
	for i, s := range f.States {
		buf.WriteString(fmt.Sprintf("0x%04X = %q\n", i, s))
	}
	buf.WriteString("\n")
	
	buf.WriteString("[inputs]\n")
	for i, inp := range f.Alphabet {
		buf.WriteString(fmt.Sprintf("0x%04X = %q\n", i, inp))
	}
	
	if len(f.OutputAlphabet) > 0 {
		buf.WriteString("\n[outputs]\n")
		for i, out := range f.OutputAlphabet {
			buf.WriteString(fmt.Sprintf("0x%04X = %q\n", i, out))
		}
	}
	
	if len(f.LinkedMachines) > 0 {
		buf.WriteString("\n[machines]\n")
		for state, machine := range f.LinkedMachines {
			buf.WriteString(fmt.Sprintf("%q = %q\n", state, machine))
		}
	}

	if len(f.Nets) > 0 {
		buf.WriteString("\n[nets]\n")
		for _, n := range f.Nets {
			var eps []string
			for _, ep := range n.Endpoints {
				eps = append(eps, ep.Instance+"."+ep.Port)
			}
			buf.WriteString(fmt.Sprintf("%q = %q\n", n.Name, strings.Join(eps, ", ")))
		}
	}
	
	return buf.String()
}

// generateLayoutToml creates layout.toml content
func generateLayoutToml(positions map[string][2]int, offsetX, offsetY int) string {
	var buf bytes.Buffer
	
	buf.WriteString("[layout]\n")
	buf.WriteString("version = 1\n\n")
	
	buf.WriteString("[editor]\n")
	buf.WriteString(fmt.Sprintf("canvas_offset_x = %d\n", offsetX))
	buf.WriteString(fmt.Sprintf("canvas_offset_y = %d\n", offsetY))
	buf.WriteString("\n")
	
	buf.WriteString("[states]\n")
	for name, pos := range positions {
		buf.WriteString(fmt.Sprintf("[states.%q]\n", name))
		buf.WriteString(fmt.Sprintf("x = %d\n", pos[0]))
		buf.WriteString(fmt.Sprintf("y = %d\n", pos[1]))
	}
	
	return buf.String()
}

// classesJSON is the JSON representation of class data within a .fsm zip.
type classesJSON struct {
	Classes         map[string]*fsm.Class            `json:"classes,omitempty"`
	StateClasses    map[string]string                 `json:"state_classes,omitempty"`
	StateProperties map[string]map[string]interface{} `json:"state_properties,omitempty"`
	Nets            []fsm.Net                         `json:"nets,omitempty"`
}

// hasClassData reports whether the FSM carries user-defined class information
// beyond the built-in default_state class, or structural net data.
func hasClassData(f *fsm.FSM) bool {
	if len(f.StateClasses) > 0 || len(f.StateProperties) > 0 || len(f.Nets) > 0 {
		return true
	}
	// Check for user-defined classes (anything beyond default_state)
	for name := range f.Classes {
		if name != fsm.DefaultClassName {
			return true
		}
	}
	return false
}

// generateClassesJSON serializes the FSM's class data to JSON.
// Returns nil, nil when there is nothing to serialize.
func generateClassesJSON(f *fsm.FSM) ([]byte, error) {
	if !hasClassData(f) {
		return nil, nil
	}
	j := classesJSON{
		Classes:         f.Classes,
		StateClasses:    f.StateClasses,
		StateProperties: f.StateProperties,
		Nets:            f.Nets,
	}
	return json.MarshalIndent(j, "", "  ")
}

// applyClassesJSON deserializes class data and applies it to the FSM.
func applyClassesJSON(f *fsm.FSM, data []byte) error {
	var j classesJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return fmt.Errorf("parsing classes.json: %w", err)
	}
	if j.Classes != nil {
		for name, cls := range j.Classes {
			f.Classes[name] = cls
		}
	}
	if j.StateClasses != nil {
		f.StateClasses = j.StateClasses
	}
	if j.StateProperties != nil {
		f.StateProperties = j.StateProperties
	}
	if len(j.Nets) > 0 {
		f.Nets = j.Nets
	}
	return nil
}

// WriteBundleFromData creates a new bundle file from in-memory machine data.
// Unlike UpdateBundleMachines, this does not require an existing file on disk.
// All machines in the 'machines' map are written to the new bundle.
func WriteBundleFromData(bundlePath string, machines map[string]BundleMachineData) error {
	if len(machines) == 0 {
		return fmt.Errorf("no machines to write")
	}

	outFile, err := os.Create(bundlePath)
	if err != nil {
		return fmt.Errorf("creating bundle: %w", err)
	}

	zw := zip.NewWriter(outFile)

	for machineName, data := range machines {
		// Generate hex records
		records, _, _, _ := FSMToRecords(data.FSM)
		hexContent := FormatHex(records, 1)
		w, err := zw.Create(machineName + ".hex")
		if err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(bundlePath)
			return fmt.Errorf("creating %s.hex: %w", machineName, err)
		}
		if _, err := w.Write([]byte(hexContent)); err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(bundlePath)
			return fmt.Errorf("writing %s.hex: %w", machineName, err)
		}

		// Generate labels.toml
		labelsContent := generateLabelsToml(data.FSM)
		w, err = zw.Create(machineName + ".labels.toml")
		if err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(bundlePath)
			return fmt.Errorf("creating %s.labels.toml: %w", machineName, err)
		}
		if _, err := w.Write([]byte(labelsContent)); err != nil {
			zw.Close()
			outFile.Close()
			os.Remove(bundlePath)
			return fmt.Errorf("writing %s.labels.toml: %w", machineName, err)
		}

		// Generate layout.toml if positions available
		if len(data.Positions) > 0 {
			layoutContent := generateLayoutToml(data.Positions, data.OffsetX, data.OffsetY)
			w, err = zw.Create(machineName + ".layout.toml")
			if err != nil {
				zw.Close()
				outFile.Close()
				os.Remove(bundlePath)
				return fmt.Errorf("creating %s.layout.toml: %w", machineName, err)
			}
			if _, err := w.Write([]byte(layoutContent)); err != nil {
				zw.Close()
				outFile.Close()
				os.Remove(bundlePath)
				return fmt.Errorf("writing %s.layout.toml: %w", machineName, err)
			}
		}

		// Generate classes.json if class data present
		if classData, cerr := generateClassesJSON(data.FSM); cerr != nil {
			zw.Close()
			outFile.Close()
			os.Remove(bundlePath)
			return fmt.Errorf("generating %s.classes.json: %w", machineName, cerr)
		} else if classData != nil {
			w, err = zw.Create(machineName + ".classes.json")
			if err != nil {
				zw.Close()
				outFile.Close()
				os.Remove(bundlePath)
				return fmt.Errorf("creating %s.classes.json: %w", machineName, err)
			}
			if _, err := w.Write(classData); err != nil {
				zw.Close()
				outFile.Close()
				os.Remove(bundlePath)
				return fmt.Errorf("writing %s.classes.json: %w", machineName, err)
			}
		}
	}

	if err := zw.Close(); err != nil {
		outFile.Close()
		os.Remove(bundlePath)
		return fmt.Errorf("closing zip: %w", err)
	}
	outFile.Close()

	return nil
}
