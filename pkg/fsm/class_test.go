package fsm

import (
	"testing"
)

// --- PropertyType tests ---

func TestValidPropertyTypes(t *testing.T) {
	types := ValidPropertyTypes()
	if len(types) != 7 {
		t.Fatalf("expected 7 property types, got %d", len(types))
	}
	for _, typ := range types {
		if !IsValidPropertyType(string(typ)) {
			t.Errorf("IsValidPropertyType(%q) = false, want true", typ)
		}
	}
	if IsValidPropertyType("invalid") {
		t.Error("IsValidPropertyType(\"invalid\") = true, want false")
	}
}

func TestDefaultValue(t *testing.T) {
	cases := []struct {
		typ  PropertyType
		want interface{}
	}{
		{PropFloat64, float64(0)},
		{PropInt64, int64(0)},
		{PropUint64, uint64(0)},
		{PropShortString, ""},
		{PropString, ""},
		{PropBool, false},
	}
	for _, tc := range cases {
		got := DefaultValue(tc.typ)
		if got != tc.want {
			t.Errorf("DefaultValue(%q) = %v (%T), want %v (%T)", tc.typ, got, got, tc.want, tc.want)
		}
	}
	// List default is []string{} (slices not comparable with !=).
	listDefault := DefaultValue(PropList)
	if items, ok := listDefault.([]string); !ok || len(items) != 0 {
		t.Errorf("DefaultValue(list) = %v (%T), want empty []string", listDefault, listDefault)
	}
}

// --- Class definition tests ---

func TestClass_AddRemoveProperty(t *testing.T) {
	c := &Class{Name: "test", Properties: []PropertyDef{}}

	if err := c.AddProperty("health", PropFloat64); err != nil {
		t.Fatalf("AddProperty: %v", err)
	}
	if err := c.AddProperty("name", PropShortString); err != nil {
		t.Fatalf("AddProperty: %v", err)
	}
	if len(c.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(c.Properties))
	}

	// Duplicate.
	if err := c.AddProperty("health", PropInt64); err == nil {
		t.Error("expected error on duplicate property, got nil")
	}

	// Lookup.
	p := c.GetProperty("health")
	if p == nil || p.Type != PropFloat64 {
		t.Errorf("GetProperty(\"health\") = %v, want float64 def", p)
	}
	if c.GetProperty("nonexistent") != nil {
		t.Error("GetProperty(\"nonexistent\") should be nil")
	}

	// Remove.
	if !c.RemoveProperty("health") {
		t.Error("RemoveProperty(\"health\") = false, want true")
	}
	if c.RemoveProperty("health") {
		t.Error("RemoveProperty(\"health\") second call should return false")
	}
	if len(c.Properties) != 1 {
		t.Fatalf("expected 1 property after removal, got %d", len(c.Properties))
	}
}

func TestNewDefaultClass(t *testing.T) {
	c := NewDefaultClass()
	if c.Name != DefaultClassName {
		t.Errorf("name = %q, want %q", c.Name, DefaultClassName)
	}
	if len(c.Properties) != 5 {
		t.Fatalf("expected 5 default properties, got %d", len(c.Properties))
	}

	// Verify expected properties.
	expected := map[string]PropertyType{
		"description": PropString,
		"color":       PropShortString,
		"weight":      PropFloat64,
		"timeout":     PropFloat64,
		"is_error":    PropBool,
	}
	for _, p := range c.Properties {
		want, ok := expected[p.Name]
		if !ok {
			t.Errorf("unexpected default property %q", p.Name)
			continue
		}
		if p.Type != want {
			t.Errorf("property %q: type = %q, want %q", p.Name, p.Type, want)
		}
	}
}

// --- FSM-level class operations ---

func TestFSM_NewHasDefaultClass(t *testing.T) {
	f := New(TypeNFA)
	if _, ok := f.Classes[DefaultClassName]; !ok {
		t.Fatal("new FSM should have default_state class")
	}
	if len(f.Classes) != 1 {
		t.Errorf("new FSM should have exactly 1 class, got %d", len(f.Classes))
	}
}

func TestFSM_AddRemoveClass(t *testing.T) {
	f := New(TypeDFA)

	enemy := &Class{
		Name: "enemy",
		Properties: []PropertyDef{
			{Name: "health", Type: PropFloat64},
			{Name: "aggro", Type: PropBool},
		},
	}
	if err := f.AddClass(enemy); err != nil {
		t.Fatalf("AddClass: %v", err)
	}
	if _, ok := f.Classes["enemy"]; !ok {
		t.Fatal("class 'enemy' should exist after AddClass")
	}

	// Duplicate.
	if err := f.AddClass(enemy); err == nil {
		t.Error("expected error on duplicate class")
	}

	// Cannot remove default.
	if f.RemoveClass(DefaultClassName) {
		t.Error("should not be able to remove default_state")
	}

	// Assign a state to enemy, then remove class.
	f.AddState("goblin")
	if err := f.SetStateClass("goblin", "enemy"); err != nil {
		t.Fatalf("SetStateClass: %v", err)
	}

	if !f.RemoveClass("enemy") {
		t.Fatal("RemoveClass(\"enemy\") = false")
	}

	// State should revert to default_state.
	if cls := f.GetStateClass("goblin"); cls != DefaultClassName {
		t.Errorf("state class after removal = %q, want %q", cls, DefaultClassName)
	}
}

func TestFSM_SetStateClass_InitialisesProperties(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("s0")

	npc := &Class{
		Name: "npc",
		Properties: []PropertyDef{
			{Name: "dialogue", Type: PropString},
			{Name: "level", Type: PropInt64},
		},
	}
	f.AddClass(npc)
	f.SetStateClass("s0", "npc")

	// Should have default_state properties + npc properties.
	props := f.StateProperties["s0"]
	if props == nil {
		t.Fatal("state properties should be initialised")
	}

	// Check default_state props exist.
	if _, ok := props["description"]; !ok {
		t.Error("missing default_state property 'description'")
	}
	if _, ok := props["is_error"]; !ok {
		t.Error("missing default_state property 'is_error'")
	}

	// Check npc props exist with correct zero values.
	if v, ok := props["dialogue"]; !ok {
		t.Error("missing npc property 'dialogue'")
	} else if v != "" {
		t.Errorf("dialogue = %v, want empty string", v)
	}
	if v, ok := props["level"]; !ok {
		t.Error("missing npc property 'level'")
	} else if v != int64(0) {
		t.Errorf("level = %v (%T), want int64(0)", v, v)
	}
}

func TestFSM_GetSetPropertyValue(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("s0")
	f.SetStateClass("s0", DefaultClassName)

	f.SetStatePropertyValue("s0", "description", "test state")
	f.SetStatePropertyValue("s0", "weight", 3.14)
	f.SetStatePropertyValue("s0", "is_error", true)

	if v := f.GetStatePropertyValue("s0", "description"); v != "test state" {
		t.Errorf("description = %v, want \"test state\"", v)
	}
	if v := f.GetStatePropertyValue("s0", "weight"); v != 3.14 {
		t.Errorf("weight = %v, want 3.14", v)
	}
	if v := f.GetStatePropertyValue("s0", "is_error"); v != true {
		t.Errorf("is_error = %v, want true", v)
	}
	if v := f.GetStatePropertyValue("s0", "nonexistent"); v != nil {
		t.Errorf("nonexistent = %v, want nil", v)
	}
}

func TestFSM_EffectiveProperties(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("s0")
	f.AddState("s1")

	enemy := &Class{
		Name: "enemy",
		Properties: []PropertyDef{
			{Name: "health", Type: PropFloat64},
			{Name: "description", Type: PropString}, // overlaps with default_state
		},
	}
	f.AddClass(enemy)
	f.SetStateClass("s1", "enemy")

	// s0 (default_state): should only have default properties.
	props0 := f.EffectiveProperties("s0")
	if len(props0) != 5 {
		t.Errorf("s0: expected 5 effective props, got %d", len(props0))
	}

	// s1 (enemy): default props + enemy's unique prop (health).
	// 'description' overlaps, so it shouldn't be duplicated.
	props1 := f.EffectiveProperties("s1")
	// default: description, color, weight, timeout, is_error (5)
	// enemy adds: health (1) — description is already in default
	if len(props1) != 6 {
		t.Errorf("s1: expected 6 effective props, got %d", len(props1))
		for _, p := range props1 {
			t.Logf("  %s: %s", p.Name, p.Type)
		}
	}

	// Verify 'health' is present.
	found := false
	for _, p := range props1 {
		if p.Name == "health" {
			found = true
		}
	}
	if !found {
		t.Error("s1: 'health' property not in effective properties")
	}
}

func TestFSM_ClassNames(t *testing.T) {
	f := New(TypeDFA)
	f.AddClass(&Class{Name: "beta"})
	f.AddClass(&Class{Name: "alpha"})

	names := f.ClassNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 class names, got %d", len(names))
	}
	if names[0] != DefaultClassName {
		t.Errorf("first class name should be %q, got %q", DefaultClassName, names[0])
	}
}

func TestFSM_EnsureClassMaps_NilSafe(t *testing.T) {
	// Simulates loading an older file that has nil class maps.
	f := &FSM{
		Type:   TypeDFA,
		States: []string{"s0"},
	}
	// Should not panic.
	f.EnsureClassMaps()

	if f.Classes == nil {
		t.Error("Classes should be non-nil after EnsureClassMaps")
	}
	if _, ok := f.Classes[DefaultClassName]; !ok {
		t.Error("default_state should exist after EnsureClassMaps")
	}
	if f.StateClasses == nil {
		t.Error("StateClasses should be non-nil")
	}
	if f.StateProperties == nil {
		t.Error("StateProperties should be non-nil")
	}
}

func TestClassNames_SortedDeterministic(t *testing.T) {
	f := New(TypeDFA)
	f.EnsureClassMaps()

	// Add classes in arbitrary order.
	f.AddClass(&Class{Name: "7432_quad_or"})
	f.AddClass(&Class{Name: "7400_quad_nand"})
	f.AddClass(&Class{Name: "74181_4bit_alu"})
	f.AddClass(&Class{Name: "7408_quad_and"})

	// Call multiple times — order must be identical each time.
	for attempt := 0; attempt < 10; attempt++ {
		names := f.ClassNames()
		if names[0] != DefaultClassName {
			t.Fatalf("attempt %d: first entry should be default_state, got %q", attempt, names[0])
		}
		// Remaining entries should be sorted.
		for i := 2; i < len(names); i++ {
			if names[i] < names[i-1] {
				t.Fatalf("attempt %d: not sorted at index %d: %q > %q", attempt, i, names[i-1], names[i])
			}
		}
	}

	names := f.ClassNames()
	expected := []string{DefaultClassName, "7400_quad_nand", "7408_quad_and", "74181_4bit_alu", "7432_quad_or"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(names))
	}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want)
		}
	}
}
