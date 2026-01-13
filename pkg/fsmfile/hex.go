// Package fsmfile handles the .fsm file format (zip with machine.hex and labels.toml).
package fsmfile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// Record types
const (
	TypeDFATransition   uint16 = 0x0000
	TypeMealyTransition uint16 = 0x0001
	TypeStateDecl       uint16 = 0x0002
	TypeNFAMulti        uint16 = 0x0003
)

// Special values
const (
	EpsilonInput uint16 = 0xFFFF
)

// Record represents a single hex record.
type Record struct {
	Type   uint16
	Field1 uint16 // Source state or state ID
	Field2 uint16 // Input or flags
	Field3 uint16 // Target state or output
	Field4 uint16 // Output/flags or continuation
}

// FormatRecord formats a record as "TYPE SSSS:IIII TTTT:OOOO".
func FormatRecord(r Record) string {
	return fmt.Sprintf("%04X %04X:%04X %04X:%04X",
		r.Type, r.Field1, r.Field2, r.Field3, r.Field4)
}

// ParseRecord parses a record from "TYPE SSSS:IIII TTTT:OOOO" format.
func ParseRecord(s string) (Record, error) {
	// Remove separators
	clean := strings.ReplaceAll(s, " ", "")
	clean = strings.ReplaceAll(clean, ":", "")
	
	if len(clean) != 20 {
		return Record{}, fmt.Errorf("invalid record length: %d", len(clean))
	}
	
	var r Record
	var err error
	
	r.Type, err = parseHex16(clean[0:4])
	if err != nil {
		return r, err
	}
	r.Field1, err = parseHex16(clean[4:8])
	if err != nil {
		return r, err
	}
	r.Field2, err = parseHex16(clean[8:12])
	if err != nil {
		return r, err
	}
	r.Field3, err = parseHex16(clean[12:16])
	if err != nil {
		return r, err
	}
	r.Field4, err = parseHex16(clean[16:20])
	if err != nil {
		return r, err
	}
	
	return r, nil
}

func parseHex16(s string) (uint16, error) {
	v, err := strconv.ParseUint(s, 16, 16)
	return uint16(v), err
}

// ParseHex parses hex records from text.
func ParseHex(text string) ([]Record, error) {
	// Remove comments
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	text = strings.Join(cleanLines, " ")
	
	// Match pattern: TYPE SSSS:IIII TTTT:OOOO
	pattern := regexp.MustCompile(`([0-9A-Fa-f]{4})\s*([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})\s*([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	
	var records []Record
	for _, m := range matches {
		rec := fmt.Sprintf("%s %s:%s %s:%s", m[1], m[2], m[3], m[4], m[5])
		r, err := ParseRecord(rec)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	
	return records, nil
}

// FormatHex formats records as text.
func FormatHex(records []Record, width int) string {
	var lines []string
	
	for i := 0; i < len(records); i += width {
		end := i + width
		if end > len(records) {
			end = len(records)
		}
		
		var row []string
		for _, r := range records[i:end] {
			row = append(row, FormatRecord(r))
		}
		lines = append(lines, strings.Join(row, "   "))
	}
	
	return strings.Join(lines, "\n")
}

// FSMToRecords converts an FSM to hex records.
func FSMToRecords(f *fsm.FSM) ([]Record, map[int]string, map[int]string, map[int]string) {
	var records []Record
	
	// Build index maps
	stateIdx := make(map[string]int)
	for i, s := range f.States {
		stateIdx[s] = i
	}
	
	inputIdx := make(map[string]int)
	for i, a := range f.Alphabet {
		inputIdx[a] = i
	}
	
	outputIdx := make(map[string]int)
	for i, o := range f.OutputAlphabet {
		outputIdx[o] = i
	}
	
	// Reverse maps for labels
	stateNames := make(map[int]string)
	for s, i := range stateIdx {
		stateNames[i] = s
	}
	inputNames := make(map[int]string)
	for s, i := range inputIdx {
		inputNames[i] = s
	}
	outputNames := make(map[int]string)
	for s, i := range outputIdx {
		outputNames[i] = s
	}
	
	// State declarations
	if f.Type == fsm.TypeMoore {
		for _, state := range f.States {
			sid := uint16(stateIdx[state])
			var flags uint16
			if state == f.Initial {
				flags |= 0x1
			}
			if f.IsAccepting(state) {
				flags |= 0x2
			}
			var out uint16
			if o, ok := f.StateOutputs[state]; ok {
				out = uint16(outputIdx[o] + 1) // +1 so 0 means no output
			}
			records = append(records, Record{
				Type:   TypeStateDecl,
				Field1: sid,
				Field2: flags,
				Field3: out,
				Field4: 0,
			})
		}
	} else {
		// Only emit state decls if flags needed
		for _, state := range f.States {
			var flags uint16
			if state == f.Initial {
				flags |= 0x1
			}
			if f.IsAccepting(state) {
				flags |= 0x2
			}
			if flags != 0 {
				records = append(records, Record{
					Type:   TypeStateDecl,
					Field1: uint16(stateIdx[state]),
					Field2: flags,
					Field3: 0,
					Field4: 0,
				})
			}
		}
	}
	
	// Transitions
	for _, t := range f.Transitions {
		src := uint16(stateIdx[t.From])
		
		var inp uint16 = EpsilonInput
		if t.Input != nil {
			inp = uint16(inputIdx[*t.Input])
		}
		
		if f.Type == fsm.TypeMealy {
			tgt := uint16(stateIdx[t.To[0]])
			var out uint16
			if t.Output != nil {
				out = uint16(outputIdx[*t.Output])
			}
			records = append(records, Record{
				Type:   TypeMealyTransition,
				Field1: src,
				Field2: inp,
				Field3: tgt,
				Field4: out,
			})
		} else if len(t.To) == 1 {
			tgt := uint16(stateIdx[t.To[0]])
			records = append(records, Record{
				Type:   TypeDFATransition,
				Field1: src,
				Field2: inp,
				Field3: tgt,
				Field4: 0,
			})
		} else {
			// NFA multi-target
			for i, to := range t.To {
				tgt := uint16(stateIdx[to])
				var cont uint16
				if i < len(t.To)-1 {
					cont = 1
				}
				records = append(records, Record{
					Type:   TypeNFAMulti,
					Field1: src,
					Field2: inp,
					Field3: tgt,
					Field4: cont,
				})
			}
		}
	}
	
	return records, stateNames, inputNames, outputNames
}

// RecordsToFSM converts hex records to an FSM.
func RecordsToFSM(records []Record, labels *Labels) (*fsm.FSM, error) {
	// Extract label mappings
	stateLabels := make(map[int]string)
	inputLabels := make(map[int]string)
	outputLabels := make(map[int]string)
	
	var fsmType fsm.Type
	var fsmName, fsmDesc string
	
	if labels != nil {
		fsmType = fsm.Type(labels.FSM.Type)
		fsmName = labels.FSM.Name
		fsmDesc = labels.FSM.Description
		
		for k, v := range labels.States {
			stateLabels[k] = v
		}
		for k, v := range labels.Inputs {
			inputLabels[k] = v
		}
		for k, v := range labels.Outputs {
			outputLabels[k] = v
		}
	}
	
	// Parse records
	stateIDs := make(map[int]bool)
	inputIDs := make(map[int]bool)
	outputIDs := make(map[int]bool)
	
	var initialState int = -1
	acceptingStates := make(map[int]bool)
	stateOutputs := make(map[int]int)
	
	type transition struct {
		from   int
		input  *int // nil for epsilon
		to     []int
		output *int
	}
	var transitions []transition
	
	var hasMealy, hasNFAMulti, hasMooreOutputs bool
	var nfaPending *transition
	
	for _, r := range records {
		switch r.Type {
		case TypeStateDecl:
			stateID := int(r.Field1)
			flags := r.Field2
			outputVal := int(r.Field3)
			
			stateIDs[stateID] = true
			if flags&0x1 != 0 {
				initialState = stateID
			}
			if flags&0x2 != 0 {
				acceptingStates[stateID] = true
			}
			if outputVal != 0 {
				hasMooreOutputs = true
				stateOutputs[stateID] = outputVal - 1
				outputIDs[outputVal-1] = true
			}
			
		case TypeDFATransition:
			src, inp, tgt := int(r.Field1), int(r.Field2), int(r.Field3)
			stateIDs[src] = true
			stateIDs[tgt] = true
			
			var inputPtr *int
			if uint16(inp) != EpsilonInput {
				inputIDs[inp] = true
				inputPtr = &inp
			}
			
			transitions = append(transitions, transition{
				from:  src,
				input: inputPtr,
				to:    []int{tgt},
			})
			
		case TypeMealyTransition:
			hasMealy = true
			src, inp, tgt, out := int(r.Field1), int(r.Field2), int(r.Field3), int(r.Field4)
			stateIDs[src] = true
			stateIDs[tgt] = true
			outputIDs[out] = true
			
			var inputPtr *int
			if uint16(inp) != EpsilonInput {
				inputIDs[inp] = true
				inputPtr = &inp
			}
			
			transitions = append(transitions, transition{
				from:   src,
				input:  inputPtr,
				to:     []int{tgt},
				output: &out,
			})
			
		case TypeNFAMulti:
			hasNFAMulti = true
			src, inp, tgt, cont := int(r.Field1), int(r.Field2), int(r.Field3), r.Field4
			stateIDs[src] = true
			stateIDs[tgt] = true
			
			var inputPtr *int
			if uint16(inp) != EpsilonInput {
				inputIDs[inp] = true
				inputPtr = &inp
			}
			
			// Group multi-target transitions
			if nfaPending == nil || nfaPending.from != src || !intPtrEqual(nfaPending.input, inputPtr) {
				if nfaPending != nil {
					transitions = append(transitions, *nfaPending)
				}
				nfaPending = &transition{
					from:  src,
					input: inputPtr,
					to:    []int{tgt},
				}
			} else {
				nfaPending.to = append(nfaPending.to, tgt)
			}
			
			if cont == 0 {
				transitions = append(transitions, *nfaPending)
				nfaPending = nil
			}
		}
	}
	
	if nfaPending != nil {
		transitions = append(transitions, *nfaPending)
	}
	
	// Determine FSM type
	if fsmType == "" {
		if hasMealy {
			fsmType = fsm.TypeMealy
		} else if hasMooreOutputs {
			fsmType = fsm.TypeMoore
		} else if hasNFAMulti {
			fsmType = fsm.TypeNFA
		} else {
			// Check for epsilon transitions
			for _, t := range transitions {
				if t.input == nil {
					fsmType = fsm.TypeNFA
					break
				}
			}
			if fsmType == "" {
				fsmType = fsm.TypeDFA
			}
		}
	}
	
	// Generate names
	stateName := func(i int) string {
		if n, ok := stateLabels[i]; ok {
			return n
		}
		return fmt.Sprintf("S%d", i)
	}
	inputName := func(i int) string {
		if n, ok := inputLabels[i]; ok {
			return n
		}
		return fmt.Sprintf("i%d", i)
	}
	outputName := func(i int) string {
		if n, ok := outputLabels[i]; ok {
			return n
		}
		return fmt.Sprintf("o%d", i)
	}
	
	// Build FSM
	f := fsm.New(fsmType)
	f.Name = fsmName
	f.Description = fsmDesc
	
	// Add states in order
	for i := 0; i <= maxKey(stateIDs); i++ {
		if stateIDs[i] {
			f.AddState(stateName(i))
		}
	}
	
	// Add inputs in order
	for i := 0; i <= maxKey(inputIDs); i++ {
		if inputIDs[i] {
			f.AddInput(inputName(i))
		}
	}
	
	// Add outputs in order
	for i := 0; i <= maxKey(outputIDs); i++ {
		if outputIDs[i] {
			f.AddOutput(outputName(i))
		}
	}
	
	// Set initial and accepting
	if initialState >= 0 {
		f.SetInitial(stateName(initialState))
	}
	var accepting []string
	for s := range acceptingStates {
		accepting = append(accepting, stateName(s))
	}
	f.SetAccepting(accepting)
	
	// Set Moore outputs
	for s, o := range stateOutputs {
		f.SetStateOutput(stateName(s), outputName(o))
	}
	
	// Add transitions
	for _, t := range transitions {
		var inputPtr *string
		if t.input != nil {
			name := inputName(*t.input)
			inputPtr = &name
		}
		
		var toNames []string
		for _, to := range t.to {
			toNames = append(toNames, stateName(to))
		}
		
		var outputPtr *string
		if t.output != nil {
			name := outputName(*t.output)
			outputPtr = &name
		}
		
		f.AddTransition(stateName(t.from), inputPtr, toNames, outputPtr)
	}
	
	return f, nil
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func maxKey(m map[int]bool) int {
	max := -1
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}
