package main

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// --- Value formatting tests ---

func TestFormatPropertyValue_Float64(t *testing.T) {
	cases := []struct {
		val  interface{}
		want string
	}{
		{float64(0), "0"},
		{float64(3.14), "3.14"},
		{float64(-1.5), "-1.5"},
		{int64(42), "42"},  // int64 coerced to float display
		{nil, ""},
	}
	for _, tc := range cases {
		got := formatPropertyValue(fsm.PropFloat64, tc.val)
		if got != tc.want {
			t.Errorf("formatPropertyValue(float64, %v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestFormatPropertyValue_Int64(t *testing.T) {
	cases := []struct {
		val  interface{}
		want string
	}{
		{int64(0), "0"},
		{int64(-99), "-99"},
		{float64(7), "7"},  // JSON round-trip produces float64
		{nil, ""},
	}
	for _, tc := range cases {
		got := formatPropertyValue(fsm.PropInt64, tc.val)
		if got != tc.want {
			t.Errorf("formatPropertyValue(int64, %v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestFormatPropertyValue_Uint64(t *testing.T) {
	cases := []struct {
		val  interface{}
		want string
	}{
		{uint64(0), "0"},
		{uint64(255), "255"},
		{float64(10), "10"}, // JSON round-trip
		{nil, ""},
	}
	for _, tc := range cases {
		got := formatPropertyValue(fsm.PropUint64, tc.val)
		if got != tc.want {
			t.Errorf("formatPropertyValue(uint64, %v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestFormatPropertyValue_Bool(t *testing.T) {
	if got := formatPropertyValue(fsm.PropBool, true); got != "true" {
		t.Errorf("got %q, want %q", got, "true")
	}
	if got := formatPropertyValue(fsm.PropBool, false); got != "false" {
		t.Errorf("got %q, want %q", got, "false")
	}
	if got := formatPropertyValue(fsm.PropBool, nil); got != "" {
		t.Errorf("got %q for nil, want empty", got)
	}
}

func TestFormatPropertyValue_String(t *testing.T) {
	if got := formatPropertyValue(fsm.PropString, "hello"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := formatPropertyValue(fsm.PropShortString, "short"); got != "short" {
		t.Errorf("got %q, want %q", got, "short")
	}
}

// --- Value parsing tests ---

func TestParsePropertyValue_Float64(t *testing.T) {
	val, err := parsePropertyValue(fsm.PropFloat64, "3.14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(float64); !ok || v != 3.14 {
		t.Errorf("got %v (%T), want 3.14 (float64)", val, val)
	}

	_, err = parsePropertyValue(fsm.PropFloat64, "abc")
	if err == nil {
		t.Error("expected error for non-numeric input")
	}
}

func TestParsePropertyValue_Int64(t *testing.T) {
	val, err := parsePropertyValue(fsm.PropInt64, "-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(int64); !ok || v != -42 {
		t.Errorf("got %v (%T), want -42 (int64)", val, val)
	}

	_, err = parsePropertyValue(fsm.PropInt64, "3.14")
	if err == nil {
		t.Error("expected error for float input to int64")
	}
}

func TestParsePropertyValue_Uint64(t *testing.T) {
	val, err := parsePropertyValue(fsm.PropUint64, "255")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(uint64); !ok || v != 255 {
		t.Errorf("got %v (%T), want 255 (uint64)", val, val)
	}

	_, err = parsePropertyValue(fsm.PropUint64, "-1")
	if err == nil {
		t.Error("expected error for negative uint64")
	}
}

func TestParsePropertyValue_Bool(t *testing.T) {
	for _, input := range []string{"true", "1", "True", "TRUE"} {
		val, err := parsePropertyValue(fsm.PropBool, input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", input, err)
			continue
		}
		if v, ok := val.(bool); !ok || !v {
			t.Errorf("parsePropertyValue(bool, %q) = %v, want true", input, val)
		}
	}
	for _, input := range []string{"false", "0", "False"} {
		val, err := parsePropertyValue(fsm.PropBool, input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", input, err)
			continue
		}
		if v, ok := val.(bool); !ok || v {
			t.Errorf("parsePropertyValue(bool, %q) = %v, want false", input, val)
		}
	}
}

func TestParsePropertyValue_ShortString_LengthLimit(t *testing.T) {
	// Exactly 40 characters should be fine.
	s40 := "1234567890123456789012345678901234567890"
	val, err := parsePropertyValue(fsm.PropShortString, s40)
	if err != nil {
		t.Fatalf("unexpected error for 40-char string: %v", err)
	}
	if val != s40 {
		t.Errorf("value mismatch")
	}

	// 41 characters should fail.
	s41 := s40 + "x"
	_, err = parsePropertyValue(fsm.PropShortString, s41)
	if err == nil {
		t.Error("expected error for 41-char [40]string")
	}
}

func TestParsePropertyValue_String_NoLimit(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "a"
	}
	val, err := parsePropertyValue(fsm.PropString, long)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != long {
		t.Error("string value mismatch")
	}
}

// --- Row building tests ---

func TestBuildClassAssignRows_SingleFSM(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "test_machine"
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.Initial = "s0"

	ed := &Editor{
		fsm: f,
	}

	rows := ed.buildClassAssignRows()

	// Expect 1 header + 3 state rows.
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	if !rows[0].IsHeader {
		t.Error("first row should be a header")
	}
	if rows[0].Machine != "test_machine" {
		t.Errorf("header machine = %q, want %q", rows[0].Machine, "test_machine")
	}
	for i := 1; i <= 3; i++ {
		if rows[i].IsHeader {
			t.Errorf("row %d should not be a header", i)
		}
		if rows[i].Class != fsm.DefaultClassName {
			t.Errorf("row %d class = %q, want %q", i, rows[i].Class, fsm.DefaultClassName)
		}
	}
}

func TestBuildClassAssignRows_WithClassAssignment(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "m"
	f.AddState("idle")
	f.AddState("error")
	f.Initial = "idle"

	enemy := &fsm.Class{
		Name:       "error_class",
		Properties: []fsm.PropertyDef{{Name: "code", Type: fsm.PropInt64}},
	}
	f.AddClass(enemy)
	f.SetStateClass("error", "error_class")

	ed := &Editor{fsm: f}
	rows := ed.buildClassAssignRows()

	// Find the error row.
	found := false
	for _, row := range rows {
		if row.State == "error" {
			if row.Class != "error_class" {
				t.Errorf("error state class = %q, want %q", row.Class, "error_class")
			}
			found = true
		}
	}
	if !found {
		t.Error("error state not found in rows")
	}
}

func TestBuildClassAssignRows_Bundle(t *testing.T) {
	f1 := fsm.New(fsm.TypeDFA)
	f1.Name = "machine_a"
	f1.AddState("a1")
	f1.AddState("a2")
	f1.Initial = "a1"

	f2 := fsm.New(fsm.TypeNFA)
	f2.Name = "machine_b"
	f2.AddState("b1")
	f2.Initial = "b1"

	ed := &Editor{
		fsm:            f1,
		isBundle:       true,
		bundleMachines: []string{"machine_a", "machine_b"},
		bundleFSMs: map[string]*fsm.FSM{
			"machine_a": f1,
			"machine_b": f2,
		},
	}

	rows := ed.buildClassAssignRows()

	// Expect: header_a, a1, a2, header_b, b1 = 5 rows.
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}

	headers := 0
	for _, r := range rows {
		if r.IsHeader {
			headers++
		}
	}
	if headers != 2 {
		t.Errorf("expected 2 headers, got %d", headers)
	}

	// Check machine grouping.
	if rows[0].Machine != "machine_a" || !rows[0].IsHeader {
		t.Error("first header should be machine_a")
	}
	if rows[3].Machine != "machine_b" || !rows[3].IsHeader {
		t.Error("second header should be machine_b")
	}
}

// --- Property editor row building ---

func TestOpenPropertyEditor_RowStructure(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "m"
	f.AddState("s0")
	f.Initial = "s0"

	custom := &fsm.Class{
		Name: "custom",
		Properties: []fsm.PropertyDef{
			{Name: "hp", Type: fsm.PropInt64},
			{Name: "name", Type: fsm.PropShortString},
		},
	}
	f.AddClass(custom)
	f.SetStateClass("s0", "custom")

	ed := &Editor{fsm: f}

	// Simulate openPropertyEditor.
	ed.propEditorMachine = "(current)"
	ed.propEditorState = "s0"
	f.EnsureClassMaps()

	className := f.GetStateClass("s0")

	var rows []propEditorRow

	// Default properties header + rows.
	if defClass, ok := f.Classes[fsm.DefaultClassName]; ok && len(defClass.Properties) > 0 {
		rows = append(rows, propEditorRow{IsHeader: true, Label: "default_state properties"})
		for _, prop := range defClass.Properties {
			val := f.GetStatePropertyValue("s0", prop.Name)
			if val == nil {
				val = fsm.DefaultValue(prop.Type)
			}
			rows = append(rows, propEditorRow{PropDef: prop, Value: val})
		}
	}

	// Class-specific.
	if className != fsm.DefaultClassName {
		if cls, ok := f.Classes[className]; ok && len(cls.Properties) > 0 {
			rows = append(rows, propEditorRow{IsHeader: true, Label: className + " properties"})
			for _, prop := range cls.Properties {
				val := f.GetStatePropertyValue("s0", prop.Name)
				if val == nil {
					val = fsm.DefaultValue(prop.Type)
				}
				rows = append(rows, propEditorRow{PropDef: prop, Value: val})
			}
		}
	}

	// Default class has 5 props, custom has 2.
	// Expected: header + 5 + header + 2 = 9 rows.
	if len(rows) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(rows))
	}

	// First header.
	if !rows[0].IsHeader || rows[0].Label != "default_state properties" {
		t.Errorf("row 0: expected default_state header, got %+v", rows[0])
	}
	// Second header at index 6.
	if !rows[6].IsHeader || rows[6].Label != "custom properties" {
		t.Errorf("row 6: expected custom header, got %+v", rows[6])
	}
	// hp should be at index 7.
	if rows[7].PropDef.Name != "hp" || rows[7].PropDef.Type != fsm.PropInt64 {
		t.Errorf("row 7: expected hp/int64, got %s/%s", rows[7].PropDef.Name, rows[7].PropDef.Type)
	}
}

// --- fsmForMachine ---

func TestFsmForMachine_SingleFile(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	ed := &Editor{fsm: f}

	got := ed.fsmForMachine("anything")
	if got != f {
		t.Error("fsmForMachine should return ed.fsm for non-bundle")
	}
}

func TestFsmForMachine_Bundle(t *testing.T) {
	f1 := fsm.New(fsm.TypeDFA)
	f2 := fsm.New(fsm.TypeNFA)

	ed := &Editor{
		fsm:      f1,
		isBundle: true,
		bundleFSMs: map[string]*fsm.FSM{
			"a": f1,
			"b": f2,
		},
	}

	if got := ed.fsmForMachine("b"); got != f2 {
		t.Error("fsmForMachine('b') should return f2")
	}
	if got := ed.fsmForMachine("nonexistent"); got != f1 {
		t.Error("fsmForMachine for missing machine should fall back to ed.fsm")
	}
}

// --- List property type tests ---

func TestFormatPropertyValue_List(t *testing.T) {
	// Empty list.
	if got := formatPropertyValue(fsm.PropList, []string{}); got != "(empty)" {
		t.Errorf("empty list: got %q, want %q", got, "(empty)")
	}
	// Non-empty.
	if got := formatPropertyValue(fsm.PropList, []string{"a", "b", "c"}); got != "[3 items]" {
		t.Errorf("3 items: got %q, want %q", got, "[3 items]")
	}
	// []interface{} (from JSON).
	if got := formatPropertyValue(fsm.PropList, []interface{}{"x"}); got != "[1 items]" {
		t.Errorf("interface slice: got %q, want %q", got, "[1 items]")
	}
	// nil.
	if got := formatPropertyValue(fsm.PropList, nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
}
