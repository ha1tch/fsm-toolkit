package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func TestVocabularies(t *testing.T) {
	for _, key := range []string{fsm.VocabFSM, fsm.VocabCircuit, fsm.VocabGeneric} {
		v, ok := fsm.Vocabularies[key]
		if !ok {
			t.Errorf("vocabulary %q not found", key)
			continue
		}
		if v.State == "" || v.States == "" || v.Transition == "" || v.Alphabet == "" {
			t.Errorf("vocabulary %q has empty labels", key)
		}
	}
}

func TestVocabDefault(t *testing.T) {
	ed := &Editor{fsm: fsm.New(fsm.TypeDFA)}
	ed.fsm.Vocabulary = "fsm"
	v := ed.Vocab()
	if v.State != "State" {
		t.Errorf("fsm vocab State = %q, want %q", v.State, "State")
	}
	if v.Transition != "Transition" {
		t.Errorf("fsm vocab Transition = %q, want %q", v.Transition, "Transition")
	}
}

func TestVocabCircuit(t *testing.T) {
	ed := &Editor{fsm: fsm.New(fsm.TypeDFA)}
	ed.fsm.Vocabulary = "circuit"
	v := ed.Vocab()
	if v.State != "Component" {
		t.Errorf("circuit vocab State = %q, want %q", v.State, "Component")
	}
	if v.Transition != "Connection" {
		t.Errorf("circuit vocab Transition = %q, want %q", v.Transition, "Connection")
	}
}

func TestVocabUnknownFallback(t *testing.T) {
	ed := &Editor{fsm: fsm.New(fsm.TypeDFA)}
	ed.fsm.Vocabulary = "nonexistent"
	v := ed.Vocab()
	if v.State != "State" {
		t.Errorf("unknown vocab should fall back to fsm, got State=%q", v.State)
	}
}

func TestVocabAuto(t *testing.T) {
	// No structural features → resolves to fsm.
	f := fsm.New(fsm.TypeDFA)
	f.Vocabulary = fsm.VocabAuto
	ed := &Editor{fsm: f}
	v := ed.Vocab()
	if v.State != "State" {
		t.Errorf("auto with no ports: State = %q, want %q", v.State, "State")
	}

	// Add a class with ports → resolves to circuit.
	cls := &fsm.Class{
		Ports: []fsm.Port{{Name: "A", Direction: fsm.PortInput, PinNumber: 1}},
	}
	f.Classes["test_ic"] = cls
	v = ed.Vocab()
	if v.State != "Component" {
		t.Errorf("auto with ports: State = %q, want %q", v.State, "Component")
	}
}

func TestVocabAutoWithNets(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Vocabulary = fsm.VocabAuto
	f.Nets = []fsm.Net{{Name: "CLK", Endpoints: []fsm.NetEndpoint{{Instance: "U1", Port: "Q"}}}}
	if got := fsm.DetectVocabulary(f); got != fsm.VocabCircuit {
		t.Errorf("DetectVocabulary with nets = %q, want %q", got, fsm.VocabCircuit)
	}
}

func TestResolvedVocabulary(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Vocabulary = fsm.VocabAuto
	if got := f.ResolvedVocabulary(); got != fsm.VocabFSM {
		t.Errorf("auto empty = %q, want %q", got, fsm.VocabFSM)
	}
	f.Nets = []fsm.Net{{Name: "X"}}
	if got := f.ResolvedVocabulary(); got != fsm.VocabCircuit {
		t.Errorf("auto with nets = %q, want %q", got, fsm.VocabCircuit)
	}
}

func TestSyncVocabularyFromConfig(t *testing.T) {
	ed := &Editor{fsm: fsm.New(fsm.TypeDFA)}
	ed.config.Vocabulary = "circuit"

	// FSM has no vocabulary → sync should apply config default.
	ed.syncVocabularyFromConfig()
	if ed.fsm.Vocabulary != "circuit" {
		t.Errorf("after sync: Vocabulary = %q, want %q", ed.fsm.Vocabulary, "circuit")
	}

	// FSM has explicit vocabulary → sync should not override.
	ed.fsm.Vocabulary = "generic"
	ed.syncVocabularyFromConfig()
	if ed.fsm.Vocabulary != "generic" {
		t.Errorf("after second sync: Vocabulary = %q, want %q", ed.fsm.Vocabulary, "generic")
	}
}

func TestVocabNames(t *testing.T) {
	names := fsm.VocabNames()
	if len(names) != 4 {
		t.Fatalf("VocabNames: got %d, want 4", len(names))
	}
	expected := []string{"fsm", "circuit", "generic", "auto"}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("VocabNames[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestParseClassLibrary(t *testing.T) {
	data := []byte(`{
		"test_class": {
			"properties": [
				{"name": "count", "type": "int64"},
				{"name": "items", "type": "list"},
				{"name": "label", "type": "[40]string"}
			]
		},
		"other_class": {
			"properties": [
				{"name": "enabled", "type": "bool"}
			]
		}
	}`)

	classes, err := parseClassLibrary(data)
	if err != nil {
		t.Fatalf("parseClassLibrary: %v", err)
	}
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}

	// Sort for deterministic order.
	sort.Slice(classes, func(i, j int) bool {
		return classes[i].Name < classes[j].Name
	})

	if classes[0].Name != "other_class" {
		t.Errorf("class[0].Name = %q, want %q", classes[0].Name, "other_class")
	}
	if len(classes[0].Properties) != 1 {
		t.Errorf("other_class: expected 1 property, got %d", len(classes[0].Properties))
	}

	if classes[1].Name != "test_class" {
		t.Errorf("class[1].Name = %q, want %q", classes[1].Name, "test_class")
	}
	if len(classes[1].Properties) != 3 {
		t.Errorf("test_class: expected 3 properties, got %d", len(classes[1].Properties))
	}
}

func TestParseClassLibraryWithParent(t *testing.T) {
	data := []byte(`{
		"child": {
			"parent": "base_part",
			"properties": [
				{"name": "voltage", "type": "float64"}
			]
		}
	}`)

	classes, err := parseClassLibrary(data)
	if err != nil {
		t.Fatalf("parseClassLibrary: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(classes))
	}
	if classes[0].Parent != "base_part" {
		t.Errorf("parent = %q, want %q", classes[0].Parent, "base_part")
	}
}

func TestParseClassLibraryInvalid(t *testing.T) {
	_, err := parseClassLibrary([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseClassLibraryWithPorts(t *testing.T) {
	data := []byte(`{
		"7474_dual_d_flipflop": {
			"properties": [
				{"name": "trigger", "type": "[40]string"}
			],
			"ports": [
				{"name": "1CLR_N", "direction": "input",  "pin_number": 1,  "group": "FF1"},
				{"name": "1D",     "direction": "input",  "pin_number": 2,  "group": "FF1"},
				{"name": "1CLK",   "direction": "input",  "pin_number": 3,  "group": "FF1"},
				{"name": "GND",    "direction": "power",  "pin_number": 7},
				{"name": "2Q",     "direction": "output", "pin_number": 9,  "group": "FF2"},
				{"name": "VCC",    "direction": "power",  "pin_number": 14}
			]
		}
	}`)

	classes, err := parseClassLibrary(data)
	if err != nil {
		t.Fatalf("parseClassLibrary: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(classes))
	}

	cls := classes[0]
	if !cls.HasPorts() {
		t.Fatal("expected class to have ports")
	}
	if len(cls.Ports) != 6 {
		t.Errorf("expected 6 ports, got %d", len(cls.Ports))
	}

	// Verify port data fidelity.
	p := cls.GetPort("1CLK")
	if p == nil {
		t.Fatal("port 1CLK not found")
	}
	if p.Direction != fsm.PortInput {
		t.Errorf("1CLK direction = %q, want %q", p.Direction, fsm.PortInput)
	}
	if p.PinNumber != 3 {
		t.Errorf("1CLK pin_number = %d, want 3", p.PinNumber)
	}
	if p.Group != "FF1" {
		t.Errorf("1CLK group = %q, want %q", p.Group, "FF1")
	}

	// Verify power ports.
	pwr := cls.PowerPorts()
	if len(pwr) != 2 {
		t.Errorf("expected 2 power ports, got %d", len(pwr))
	}

	// Verify signal ports.
	sig := cls.SignalPorts()
	if len(sig) != 4 {
		t.Errorf("expected 4 signal ports, got %d", len(sig))
	}
}

func TestParseClassLibraryNoPorts(t *testing.T) {
	// Existing format without ports should still work and produce no ports.
	data := []byte(`{
		"simple_class": {
			"properties": [
				{"name": "value", "type": "float64"}
			]
		}
	}`)

	classes, err := parseClassLibrary(data)
	if err != nil {
		t.Fatalf("parseClassLibrary: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(classes))
	}
	if classes[0].HasPorts() {
		t.Error("expected no ports for class without port definitions")
	}
}

func TestCountClassLibFiles(t *testing.T) {
	dir := t.TempDir()

	// No files.
	if got := countClassLibFiles(dir); got != 0 {
		t.Errorf("empty dir: got %d, want 0", got)
	}

	// One valid file.
	os.WriteFile(filepath.Join(dir, "test.classes.json"), []byte("{}"), 0644)
	if got := countClassLibFiles(dir); got != 1 {
		t.Errorf("one file: got %d, want 1", got)
	}

	// Non-matching file ignored.
	os.WriteFile(filepath.Join(dir, "other.json"), []byte("{}"), 0644)
	if got := countClassLibFiles(dir); got != 1 {
		t.Errorf("with non-matching: got %d, want 1", got)
	}
}

func TestBuildSettingsItems(t *testing.T) {
	ed := &Editor{}
	ed.config = DefaultConfig()
	ed.fsm = fsm.New(fsm.TypeDFA)

	items := ed.buildSettingsItems()

	// Should have 5 settings.
	if len(items) != 5 {
		t.Fatalf("expected 5 settings items, got %d", len(items))
	}

	// Check keys.
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	expected := []string{"renderer", "file_type", "fsm_type", "vocabulary", "class_lib_dir"}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("item[%d].Key = %q, want %q", i, keys[i], k)
		}
	}

	// Vocabulary should have 4 values (fsm, circuit, generic, auto).
	vocabItem := items[3]
	if len(vocabItem.Values) != 4 {
		t.Errorf("vocabulary values: got %d, want 4", len(vocabItem.Values))
	}

	// class_lib_dir has no Values (text input).
	if items[4].Values != nil {
		t.Error("class_lib_dir should have nil Values")
	}
}

func TestIntToStr(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
	}
	for _, tc := range cases {
		if got := intToStr(tc.in); got != tc.want {
			t.Errorf("intToStr(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadClassLibrariesIntegration(t *testing.T) {
	// Create a temp dir with two library files.
	dir := t.TempDir()

	lib1 := `{
		"part_a": {
			"properties": [{"name": "voltage", "type": "float64"}]
		},
		"part_b": {
			"properties": [{"name": "pins", "type": "list"}]
		}
	}`
	lib2 := `{
		"part_c": {
			"properties": [{"name": "count", "type": "int64"}]
		},
		"part_a": {
			"properties": [{"name": "should_be_skipped", "type": "bool"}]
		}
	}`

	os.WriteFile(filepath.Join(dir, "lib1.classes.json"), []byte(lib1), 0644)
	os.WriteFile(filepath.Join(dir, "lib2.classes.json"), []byte(lib2), 0644)
	// Non-matching file should be ignored.
	os.WriteFile(filepath.Join(dir, "readme.json"), []byte(`{"ignore": true}`), 0644)

	ed := &Editor{}
	ed.config.ClassLibDir = dir
	ed.fsm = fsm.New(fsm.TypeDFA)
	ed.fsm.EnsureClassMaps()

	ed.loadClassLibraries()

	// Should have default_state + part_a + part_b + part_c = 4.
	if len(ed.fsm.Classes) != 4 {
		t.Errorf("expected 4 classes, got %d", len(ed.fsm.Classes))
		for name := range ed.fsm.Classes {
			t.Logf("  class: %s", name)
		}
	}

	// part_a should have the FIRST definition (voltage), not the duplicate.
	partA := ed.fsm.Classes["part_a"]
	if partA == nil {
		t.Fatal("part_a not found")
	}
	if len(partA.Properties) != 1 || partA.Properties[0].Name != "voltage" {
		t.Errorf("part_a should have voltage property from first file, got: %+v", partA.Properties)
	}

	// part_b should have list property.
	partB := ed.fsm.Classes["part_b"]
	if partB == nil {
		t.Fatal("part_b not found")
	}
	if partB.Properties[0].Type != fsm.PropList {
		t.Errorf("part_b.pins type = %q, want list", partB.Properties[0].Type)
	}
}
