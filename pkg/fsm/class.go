package fsm

import "fmt"

// PropertyType represents the data type of a class property.
// Each type is coupled to a specific widget in the TUI editor:
//
//	Float64     → numeric editbox
//	Int64       → numeric editbox
//	Uint64      → numeric editbox
//	ShortString → single-line editbox (max 40 characters)
//	String      → multi-line textarea
//	Bool        → checkbox/toggle
//	List        → list editor (add/remove string items)
type PropertyType string

const (
	PropFloat64     PropertyType = "float64"
	PropInt64       PropertyType = "int64"
	PropUint64      PropertyType = "uint64"
	PropShortString PropertyType = "[40]string"
	PropString      PropertyType = "string"
	PropBool        PropertyType = "bool"
	PropList        PropertyType = "list"
)

// ValidPropertyTypes returns all valid property type strings.
func ValidPropertyTypes() []PropertyType {
	return []PropertyType{
		PropFloat64,
		PropInt64,
		PropUint64,
		PropShortString,
		PropString,
		PropBool,
		PropList,
	}
}

// IsValidPropertyType checks whether a string is a valid property type.
func IsValidPropertyType(s string) bool {
	for _, t := range ValidPropertyTypes() {
		if string(t) == s {
			return true
		}
	}
	return false
}

// PropertyDef defines a single property within a class.
type PropertyDef struct {
	Name string       `json:"name"`
	Type PropertyType `json:"type"`
}

// Class defines a collection of property definitions.
// Every class has a unique name scoped to the .fsm file.
//
// The "default_state" class is always present and provides properties
// common to all states. Currently inheritance is not implemented, but
// the Parent field is reserved for future use: when set, the class
// inherits all properties from its parent class, with its own
// properties taking precedence on name conflicts.
type Class struct {
	Name       string        `json:"name"`
	Parent     string        `json:"parent,omitempty"` // reserved for future inheritance
	Properties []PropertyDef `json:"properties"`
	Ports      []Port        `json:"ports,omitempty"` // empty for purely behavioural classes

	// EDA interop — optional, populated by derivation or user override.
	KiCadPart      string `json:"kicad_part,omitempty"`      // e.g. "74xx:7400"
	KiCadFootprint string `json:"kicad_footprint,omitempty"` // e.g. "Package_DIP:DIP-14_W7.62mm"
}

// HasProperty checks whether the class defines a property with the given name.
func (c *Class) HasProperty(name string) bool {
	for _, p := range c.Properties {
		if p.Name == name {
			return true
		}
	}
	return false
}

// GetProperty returns the property definition with the given name, or nil.
func (c *Class) GetProperty(name string) *PropertyDef {
	for i := range c.Properties {
		if c.Properties[i].Name == name {
			return &c.Properties[i]
		}
	}
	return nil
}

// AddProperty adds a property to the class. Returns an error if a
// property with the same name already exists.
func (c *Class) AddProperty(name string, typ PropertyType) error {
	if c.HasProperty(name) {
		return fmt.Errorf("property %q already exists in class %q", name, c.Name)
	}
	c.Properties = append(c.Properties, PropertyDef{Name: name, Type: typ})
	return nil
}

// RemoveProperty removes a property by name. Returns false if not found.
func (c *Class) RemoveProperty(name string) bool {
	for i, p := range c.Properties {
		if p.Name == name {
			c.Properties = append(c.Properties[:i], c.Properties[i+1:]...)
			return true
		}
	}
	return false
}

// PinCount returns the number of ports (including power).
func (c *Class) PinCount() int {
	return len(c.Ports)
}

// DeriveKiCadFields populates KiCadPart and KiCadFootprint from the
// class name and pin count when they are empty. Returns true if any
// field was set.
//
// Derivation rules:
//   - KiCadPart: extract leading numeric prefix from class name
//     (e.g. "7400_quad_nand" → "74xx:7400").
//   - KiCadFootprint: standard DIP package from pin count
//     (e.g. 14 pins → "Package_DIP:DIP-14_W7.62mm").
func (c *Class) DeriveKiCadFields() bool {
	changed := false

	if c.KiCadPart == "" {
		if part := derive74xxPart(c.Name); part != "" {
			c.KiCadPart = part
			changed = true
		}
	}

	if c.KiCadFootprint == "" && c.PinCount() > 0 {
		c.KiCadFootprint = deriveDIPFootprint(c.PinCount())
		changed = true
	}

	return changed
}

// derive74xxPart extracts the leading numeric prefix from a class name
// and returns a KiCad library reference like "74xx:7400".
func derive74xxPart(name string) string {
	// Extract leading digits.
	i := 0
	for i < len(name) && (name[i] >= '0' && name[i] <= '9') {
		i++
	}
	if i == 0 {
		return ""
	}
	num := name[:i]

	// Must start with "74" to belong to the 74xx family.
	if len(num) < 4 || num[:2] != "74" {
		return ""
	}

	return "74xx:" + num
}

// deriveDIPFootprint returns the KiCad DIP footprint string for a
// given pin count.
func deriveDIPFootprint(pins int) string {
	if pins <= 0 {
		return ""
	}
	// Standard DIP packages: W7.62mm for ≤28 pins, W15.24mm for wider.
	width := "W7.62mm"
	if pins > 28 {
		width = "W15.24mm"
	}
	return fmt.Sprintf("Package_DIP:DIP-%d_%s", pins, width)
}

// DefaultClassName is the name of the built-in class that provides
// properties common to all states.
const DefaultClassName = "default_state"

// NewDefaultClass creates the built-in default_state class with
// standard properties.
func NewDefaultClass() *Class {
	return &Class{
		Name: DefaultClassName,
		Properties: []PropertyDef{
			{Name: "description", Type: PropString},
			{Name: "color", Type: PropShortString},
			{Name: "weight", Type: PropFloat64},
			{Name: "timeout", Type: PropFloat64},
			{Name: "is_error", Type: PropBool},
		},
	}
}

// DefaultValue returns the zero value for a property type, suitable
// for initialising a new state's properties.
func DefaultValue(typ PropertyType) interface{} {
	switch typ {
	case PropFloat64:
		return float64(0)
	case PropInt64:
		return int64(0)
	case PropUint64:
		return uint64(0)
	case PropShortString:
		return ""
	case PropString:
		return ""
	case PropBool:
		return false
	case PropList:
		return []string{}
	default:
		return nil
	}
}

// --- FSM-level class operations ---

// EnsureClassMaps initialises the class system maps if they are nil.
// Safe to call multiple times. Used after deserialisation of older files
// that predate the class system.
func (f *FSM) EnsureClassMaps() {
	if f.Classes == nil {
		f.Classes = make(map[string]*Class)
	}
	if _, ok := f.Classes[DefaultClassName]; !ok {
		f.Classes[DefaultClassName] = NewDefaultClass()
	}
	if f.StateClasses == nil {
		f.StateClasses = make(map[string]string)
	}
	if f.StateProperties == nil {
		f.StateProperties = make(map[string]map[string]interface{})
	}
}

// AddClass adds a new class definition. Returns an error if the name
// is already taken.
func (f *FSM) AddClass(c *Class) error {
	f.EnsureClassMaps()
	if _, exists := f.Classes[c.Name]; exists {
		return fmt.Errorf("class %q already exists", c.Name)
	}
	f.Classes[c.Name] = c
	return nil
}

// RemoveClass removes a class definition and unassigns any states that
// belonged to it (they revert to default_state). Returns false if the
// class does not exist. The default_state class cannot be removed.
func (f *FSM) RemoveClass(name string) bool {
	if name == DefaultClassName {
		return false
	}
	f.EnsureClassMaps()
	if _, exists := f.Classes[name]; !exists {
		return false
	}
	delete(f.Classes, name)

	// Revert states that were assigned to this class.
	for state, cls := range f.StateClasses {
		if cls == name {
			f.StateClasses[state] = DefaultClassName
			// Clear property values that don't belong to default_state.
			f.resetStateProperties(state)
		}
	}
	return true
}

// GetStateClass returns the class name assigned to a state.
// Returns DefaultClassName if the state has no explicit assignment.
func (f *FSM) GetStateClass(state string) string {
	f.EnsureClassMaps()
	if cls, ok := f.StateClasses[state]; ok && cls != "" {
		return cls
	}
	return DefaultClassName
}

// SetStateClass assigns a class to a state and initialises missing
// property values with defaults. Returns an error if the class doesn't exist.
func (f *FSM) SetStateClass(state, className string) error {
	f.EnsureClassMaps()
	cls, ok := f.Classes[className]
	if !ok {
		return fmt.Errorf("class %q does not exist", className)
	}
	f.StateClasses[state] = className

	// Ensure property map exists.
	if f.StateProperties[state] == nil {
		f.StateProperties[state] = make(map[string]interface{})
	}

	// Initialise missing properties with defaults.
	for _, prop := range cls.Properties {
		if _, exists := f.StateProperties[state][prop.Name]; !exists {
			f.StateProperties[state][prop.Name] = DefaultValue(prop.Type)
		}
	}

	// Also initialise default_state properties (always present).
	if className != DefaultClassName {
		if defClass, ok := f.Classes[DefaultClassName]; ok {
			for _, prop := range defClass.Properties {
				if _, exists := f.StateProperties[state][prop.Name]; !exists {
					f.StateProperties[state][prop.Name] = DefaultValue(prop.Type)
				}
			}
		}
	}

	return nil
}

// GetStatePropertyValue returns the value of a property for a state.
// Returns nil if the property is not set.
func (f *FSM) GetStatePropertyValue(state, propName string) interface{} {
	f.EnsureClassMaps()
	if props, ok := f.StateProperties[state]; ok {
		return props[propName]
	}
	return nil
}

// SetStatePropertyValue sets a property value for a state.
func (f *FSM) SetStatePropertyValue(state, propName string, value interface{}) {
	f.EnsureClassMaps()
	if f.StateProperties[state] == nil {
		f.StateProperties[state] = make(map[string]interface{})
	}
	f.StateProperties[state][propName] = value
}

// EffectiveProperties returns the full list of property definitions
// applicable to a state: the default_state properties followed by the
// state's own class properties (if different from default_state).
// This is the hook for future inheritance — when parent chains are
// supported, this method walks the chain.
func (f *FSM) EffectiveProperties(state string) []PropertyDef {
	f.EnsureClassMaps()
	var result []PropertyDef
	seen := make(map[string]bool)

	// Default class properties first.
	if defClass, ok := f.Classes[DefaultClassName]; ok {
		for _, p := range defClass.Properties {
			result = append(result, p)
			seen[p.Name] = true
		}
	}

	// State's own class properties (if not default_state).
	className := f.GetStateClass(state)
	if className != DefaultClassName {
		if cls, ok := f.Classes[className]; ok {
			for _, p := range cls.Properties {
				if !seen[p.Name] {
					result = append(result, p)
					seen[p.Name] = true
				}
			}
		}
	}

	return result
}

// resetStateProperties removes property values that don't belong to
// the state's current class or default_state.
func (f *FSM) resetStateProperties(state string) {
	props := f.EffectiveProperties(state)
	valid := make(map[string]bool, len(props))
	for _, p := range props {
		valid[p.Name] = true
	}
	if f.StateProperties[state] != nil {
		for k := range f.StateProperties[state] {
			if !valid[k] {
				delete(f.StateProperties[state], k)
			}
		}
	}
}

// ClassNames returns sorted class names, with default_state first.
func (f *FSM) ClassNames() []string {
	f.EnsureClassMaps()
	names := make([]string, 0, len(f.Classes))
	for name := range f.Classes {
		if name != DefaultClassName {
			names = append(names, name)
		}
	}
	// Sort for deterministic ordering.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	// default_state always first.
	result := make([]string, 0, len(names)+1)
	result = append(result, DefaultClassName)
	result = append(result, names...)
	return result
}
