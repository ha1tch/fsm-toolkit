// Package fuzz provides fuzz testing for FSM parsers.
// Run with: go test -fuzz=FuzzParseHex -fuzztime=30s ./tests/fuzz/
package fuzz

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

// FuzzParseHex tests the hex record parser with arbitrary input.
// Looking for panics, infinite loops, or memory issues.
func FuzzParseHex(f *testing.F) {
	// Seed with valid records
	f.Add("0000 0000:0000 0001:0000")
	f.Add("0001 0000:0000 0001:0002")
	f.Add("0002 0000:0001 0000:0000")
	f.Add("0003 0000:0000 0001:0002")
	
	// Seed with edge cases
	f.Add("")
	f.Add("0000")
	f.Add("0000 0000:0000")
	f.Add("FFFF FFFF:FFFF FFFF:FFFF")
	f.Add("0000 0000:0000 0001:0000\n0000 0001:0000 0000:0000")
	
	// Seed with potentially malicious input
	f.Add("0000 0000:0000 0001:0000 extra garbage")
	f.Add("    0000 0000:0000 0001:0000    ")
	f.Add("0000\t0000:0000\t0001:0000")
	f.Add("zzzz zzzz:zzzz zzzz:zzzz")
	
	f.Fuzz(func(t *testing.T, data string) {
		// Should not panic
		records, err := fsmfile.ParseHex(data)
		
		if err == nil && len(records) > 0 {
			// If parsing succeeded, formatting should not panic
			_ = fsmfile.FormatHex(records, 4)
		}
	})
}

// FuzzParseJSON tests the JSON parser with arbitrary input.
func FuzzParseJSON(f *testing.F) {
	// Seed with valid JSON
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[]}`))
	f.Add([]byte(`{"type":"nfa","states":["s0","s1"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s1","input":"a"}]}`))
	f.Add([]byte(`{"type":"moore","states":["s0"],"alphabet":["a"],"initial":"s0","state_outputs":{"s0":"x"}}`))
	f.Add([]byte(`{"type":"mealy","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a","output":"x"}]}`))
	
	// Seed with edge cases
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"type":"dfa"}`))
	f.Add([]byte(`{"type":"unknown","states":["s0"],"initial":"s0"}`))
	
	// Seed with large values
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":[],"initial":"s0","accepting":[],"transitions":[]}`))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		fsm, err := fsmfile.ParseJSON(data)
		
		if err == nil && fsm != nil {
			// If parsing succeeded, serialization should not panic
			_, _ = fsmfile.ToJSON(fsm, false)
			_, _ = fsmfile.ToJSON(fsm, true)
		}
	})
}

// FuzzParseRecord tests individual record parsing.
func FuzzParseRecord(f *testing.F) {
	f.Add("0000 0000:0000 0001:0000")
	f.Add("0001 0000:0000 0001:0002")
	f.Add("0002 0000:0001 0000:0000")
	f.Add("0003 0000:0000 0001:0002")
	f.Add("")
	f.Add("0")
	f.Add("00000000000000000000")
	f.Add("FFFFFFFFFFFFFFFFFFFFFFFF")
	
	f.Fuzz(func(t *testing.T, data string) {
		// Should not panic
		record, err := fsmfile.ParseRecord(data)
		
		if err == nil {
			// Round-trip should work
			formatted := fsmfile.FormatRecord(record)
			record2, err2 := fsmfile.ParseRecord(formatted)
			if err2 != nil {
				t.Errorf("Round-trip failed: %v -> %v -> error: %v", data, formatted, err2)
			}
			if record != record2 {
				t.Errorf("Round-trip mismatch: %v != %v", record, record2)
			}
		}
	})
}

// FuzzRecordsToFSM tests converting records to FSM.
func FuzzRecordsToFSM(f *testing.F) {
	// Generate some seed records
	seeds := [][]fsmfile.Record{
		{}, // empty
		{{Type: 0x0002, Field1: 0, Field2: 1, Field3: 0, Field4: 0}}, // single state
		{
			{Type: 0x0002, Field1: 0, Field2: 1, Field3: 0, Field4: 0},
			{Type: 0x0002, Field1: 1, Field2: 0, Field3: 0, Field4: 0},
			{Type: 0x0000, Field1: 0, Field2: 0, Field3: 1, Field4: 0},
		},
	}
	
	for _, records := range seeds {
		data, _ := json.Marshal(records)
		f.Add(data)
	}
	
	f.Fuzz(func(t *testing.T, data []byte) {
		var records []fsmfile.Record
		if err := json.Unmarshal(data, &records); err != nil {
			return // invalid JSON, skip
		}
		
		// Should not panic
		fsm, err := fsmfile.RecordsToFSM(records, nil)
		
		if err == nil && fsm != nil {
			// If conversion succeeded, reverse should not panic
			_, _, _, _ = fsmfile.FSMToRecords(fsm)
		}
	})
}

// FuzzFSMArchive tests reading malformed .fsm archives.
func FuzzFSMArchive(f *testing.F) {
	// Create a minimal valid archive
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("machine.hex")
	w.Write([]byte("0002 0000:0001 0000:0000"))
	zw.Close()
	f.Add(buf.Bytes())
	
	// Empty archive
	buf.Reset()
	zw = zip.NewWriter(&buf)
	zw.Close()
	f.Add(buf.Bytes())
	
	// Random bytes
	f.Add([]byte{})
	f.Add([]byte{0x50, 0x4B, 0x03, 0x04}) // ZIP magic
	f.Add([]byte("not a zip"))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		// Create a reader from the data
		reader := bytes.NewReader(data)
		zr, err := zip.NewReader(reader, int64(len(data)))
		if err != nil {
			return // not a valid zip, skip
		}
		
		// Try to find and parse machine.hex
		for _, file := range zr.File {
			if file.Name == "machine.hex" {
				rc, err := file.Open()
				if err != nil {
					return
				}
				defer rc.Close()
				
				var hexData bytes.Buffer
				hexData.ReadFrom(rc)
				
				// Should not panic
				_, _ = fsmfile.ParseHex(hexData.String())
			}
		}
	})
}

// FuzzValidateAndAnalyse tests that validation and analysis don't panic.
func FuzzValidateAndAnalyse(f *testing.F) {
	// Seed with various FSM structures via JSON
	seeds := []string{
		`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`,
		`{"type":"nfa","states":["s0","s1"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":["s0","s1"],"input":"a"}]}`,
		`{"type":"moore","states":["s0"],"alphabet":[],"initial":"s0","state_outputs":{"s0":"out"}}`,
		`{"type":"mealy","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a","output":"x"}]}`,
		// Edge cases
		`{"type":"dfa","states":[],"alphabet":[],"initial":""}`,
		`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"missing"}`,
	}
	
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	
	f.Fuzz(func(t *testing.T, data []byte) {
		fsm, err := fsmfile.ParseJSON(data)
		if err != nil || fsm == nil {
			return
		}
		
		// These should never panic
		_ = fsm.Validate()
		_ = fsm.Analyse()
	})
}

// FuzzRunner tests the FSM runner with arbitrary inputs.
func FuzzRunner(f *testing.F) {
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a","b"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"},{"from":"s1","to":"s0","input":"b"}]}`), "a")
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a","b"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"},{"from":"s1","to":"s0","input":"b"}]}`), "b")
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a","b"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"},{"from":"s1","to":"s0","input":"b"}]}`), "ababab")
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a","b"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"},{"from":"s1","to":"s0","input":"b"}]}`), "")
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a","b"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"},{"from":"s1","to":"s0","input":"b"}]}`), "xyz")
	
	f.Fuzz(func(t *testing.T, fsmData []byte, inputs string) {
		f, err := fsmfile.ParseJSON(fsmData)
		if err != nil || f == nil {
			return
		}
		
		if f.Validate() != nil {
			return
		}
		
		runner, err := fsm.NewRunner(f)
		if err != nil {
			return
		}
		
		// Run through inputs - should not panic
		for _, r := range inputs {
			input := string(r)
			_, _ = runner.Step(input)
		}
		
		// These should not panic
		_ = runner.CurrentState()
		_ = runner.CurrentStates()
		_ = runner.IsAccepting()
		_ = runner.AvailableInputs()
		_ = runner.History()
		_ = runner.CurrentOutput()
		
		// Reset should not panic
		runner.Reset()
	})
}

// FuzzLayout tests the layout algorithms.
func FuzzLayout(f *testing.F) {
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), 100, 100)
	f.Add([]byte(`{"type":"dfa","states":["s0","s1","s2","s3","s4"],"alphabet":["a"],"initial":"s0"}`), 200, 150)
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), 0, 0)
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), -1, -1)
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), 10000, 10000)
	
	f.Fuzz(func(t *testing.T, fsmData []byte, width, height int) {
		f, err := fsmfile.ParseJSON(fsmData)
		if err != nil || f == nil {
			return
		}
		
		// Should not panic even with weird dimensions
		_ = fsmfile.SmartLayout(f, width, height)
		_ = fsmfile.AutoLayout(f, fsmfile.LayoutGrid, width, height)
		_ = fsmfile.AutoLayout(f, fsmfile.LayoutCircular, width, height)
		_ = fsmfile.AutoLayout(f, fsmfile.LayoutHierarchical, width, height)
		_ = fsmfile.AutoLayout(f, fsmfile.LayoutForceDirected, width, height)
	})
}

// FuzzSVGNative tests native SVG generation.
func FuzzSVGNative(f *testing.F) {
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`))
	f.Add([]byte(`{"type":"moore","states":["s0","s1"],"alphabet":["a"],"initial":"s0","state_outputs":{"s0":"x","s1":"y"}}`))
	f.Add([]byte(`{"type":"mealy","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a","output":"<script>alert(1)</script>"}]}`)) // XSS attempt
	
	f.Fuzz(func(t *testing.T, fsmData []byte) {
		f, err := fsmfile.ParseJSON(fsmData)
		if err != nil || f == nil {
			return
		}
		
		opts := fsmfile.DefaultSVGOptions()
		
		// Should not panic
		svg := fsmfile.GenerateSVGNative(f, opts)
		
		// Basic sanity check
		if len(svg) > 0 && !bytes.Contains([]byte(svg), []byte("<svg")) {
			t.Error("Generated SVG doesn't contain <svg tag")
		}
	})
}

// FuzzDOT tests DOT generation.
func FuzzDOT(f *testing.F) {
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), "Test")
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), "")
	f.Add([]byte(`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0"}`), "Title with \"quotes\" and \\ backslash")
	f.Add([]byte(`{"type":"dfa","states":["state with spaces"],"alphabet":["a"],"initial":"state with spaces"}`), "Test")
	
	f.Fuzz(func(t *testing.T, fsmData []byte, title string) {
		f, err := fsmfile.ParseJSON(fsmData)
		if err != nil || f == nil {
			return
		}
		
		// Should not panic
		dot := fsmfile.GenerateDOT(f, title)
		
		// Basic sanity check
		if len(dot) > 0 && !bytes.Contains([]byte(dot), []byte("digraph")) {
			t.Error("Generated DOT doesn't contain digraph")
		}
	})
}

// FuzzCodegen tests code generation.
func FuzzCodegen(f *testing.F) {
	f.Add([]byte(`{"type":"dfa","states":["s0","s1"],"alphabet":["a"],"initial":"s0","accepting":["s1"],"transitions":[{"from":"s0","to":"s1","input":"a"}]}`))
	f.Add([]byte(`{"type":"moore","states":["s0"],"alphabet":["a"],"initial":"s0","state_outputs":{"s0":"x"},"output_alphabet":["x"]}`))
	f.Add([]byte(`{"type":"mealy","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a","output":"y"}],"output_alphabet":["y"]}`))
	
	f.Fuzz(func(t *testing.T, fsmData []byte) {
		// Note: codegen is imported from a separate package
		// This test ensures the FSM structures passed to codegen don't cause panics
		f, err := fsmfile.ParseJSON(fsmData)
		if err != nil || f == nil {
			return
		}
		
		// Basic validation - codegen needs valid FSMs
		if f.Validate() != nil {
			return
		}
		
		// Codegen functions would be tested here if imported
		// For now, just verify the FSM is in a valid state for codegen
		_ = f.States
		_ = f.Alphabet
		_ = f.Transitions
	})
}
