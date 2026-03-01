package fsmfile

import (
	"encoding/json"
	"fmt"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// jsonFSM is the JSON representation of an FSM.
type jsonFSM struct {
	Type           string            `json:"type"`
	Name           string            `json:"name,omitempty"`
	Description    string            `json:"description,omitempty"`
	States         []string          `json:"states"`
	Alphabet       []string          `json:"alphabet"`
	Initial        string            `json:"initial"`
	Accepting      []string          `json:"accepting"`
	Transitions    []jsonTransition  `json:"transitions"`
	StateOutputs   map[string]string `json:"state_outputs,omitempty"`
	OutputAlphabet []string          `json:"output_alphabet,omitempty"`
	LinkedMachines map[string]string `json:"linked_machines,omitempty"`

	// Class system
	Classes         map[string]*fsm.Class                `json:"classes,omitempty"`
	StateClasses    map[string]string                     `json:"state_classes,omitempty"`
	StateProperties map[string]map[string]interface{}     `json:"state_properties,omitempty"`
}

type jsonTransition struct {
	From   string      `json:"from"`
	Input  *string     `json:"input"`
	To     interface{} `json:"to"` // string or []string
	Output *string     `json:"output,omitempty"`
}

// ParseJSON parses an FSM from JSON.
func ParseJSON(data []byte) (*fsm.FSM, error) {
	var j jsonFSM
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	
	f := fsm.New(fsm.Type(j.Type))
	f.Name = j.Name
	f.Description = j.Description
	f.States = j.States
	f.Alphabet = j.Alphabet
	f.Initial = j.Initial
	f.Accepting = j.Accepting
	f.OutputAlphabet = j.OutputAlphabet
	
	if j.StateOutputs != nil {
		f.StateOutputs = j.StateOutputs
	}
	
	if j.LinkedMachines != nil {
		f.LinkedMachines = j.LinkedMachines
	}
	
	for _, jt := range j.Transitions {
		var to []string
		switch v := jt.To.(type) {
		case string:
			to = []string{v}
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					to = append(to, str)
				}
			}
		}
		
		f.AddTransition(jt.From, jt.Input, to, jt.Output)
	}

	// Load class system (older files may not have these fields).
	f.EnsureClassMaps()
	if j.Classes != nil {
		for name, cls := range j.Classes {
			f.Classes[name] = cls
		}
	}
	if j.StateClasses != nil {
		f.StateClasses = j.StateClasses
	}
	if j.StateProperties != nil {
		// JSON deserialises numbers as float64. Coerce values to their
		// declared types based on the class property definitions.
		for state, props := range j.StateProperties {
			coerced := make(map[string]interface{}, len(props))
			for k, v := range props {
				coerced[k] = coercePropertyValue(f, state, k, v)
			}
			f.StateProperties[state] = coerced
		}
	}
	
	return f, nil
}

// coercePropertyValue converts a JSON-deserialised value to the correct
// Go type based on the property's declared type in the class definition.
func coercePropertyValue(f *fsm.FSM, state, propName string, raw interface{}) interface{} {
	// Find the property definition.
	var propType fsm.PropertyType
	found := false

	className := f.GetStateClass(state)
	// Check the state's own class first, then default_state.
	for _, cn := range []string{className, fsm.DefaultClassName} {
		if cls, ok := f.Classes[cn]; ok {
			if p := cls.GetProperty(propName); p != nil {
				propType = p.Type
				found = true
				break
			}
		}
	}
	if !found {
		return raw // Unknown property, keep as-is.
	}

	switch propType {
	case fsm.PropFloat64:
		if v, ok := raw.(float64); ok {
			return v
		}
	case fsm.PropInt64:
		if v, ok := raw.(float64); ok {
			return int64(v)
		}
	case fsm.PropUint64:
		if v, ok := raw.(float64); ok {
			return uint64(v)
		}
	case fsm.PropBool:
		if v, ok := raw.(bool); ok {
			return v
		}
	case fsm.PropShortString, fsm.PropString:
		if v, ok := raw.(string); ok {
			if propType == fsm.PropShortString && len(v) > 40 {
				return v[:40]
			}
			return v
		}
	case fsm.PropList:
		// JSON deserialises arrays as []interface{}.
		if arr, ok := raw.([]interface{}); ok {
			items := make([]string, 0, len(arr))
			for _, elem := range arr {
				if s, ok := elem.(string); ok {
					items = append(items, s)
				} else {
					items = append(items, fmt.Sprintf("%v", elem))
				}
			}
			return items
		}
		// Already coerced (e.g. from in-memory set).
		if items, ok := raw.([]string); ok {
			return items
		}
	}
	return raw
}

// ToJSON converts an FSM to JSON.
func ToJSON(f *fsm.FSM, pretty bool) ([]byte, error) {
	j := jsonFSM{
		Type:           string(f.Type),
		Name:           f.Name,
		Description:    f.Description,
		States:         f.States,
		Alphabet:       f.Alphabet,
		Initial:        f.Initial,
		Accepting:      f.Accepting,
		OutputAlphabet: f.OutputAlphabet,
	}
	
	if len(f.StateOutputs) > 0 {
		j.StateOutputs = f.StateOutputs
	}
	
	if len(f.LinkedMachines) > 0 {
		j.LinkedMachines = f.LinkedMachines
	}
	
	for _, t := range f.Transitions {
		jt := jsonTransition{
			From:   t.From,
			Input:  t.Input,
			Output: t.Output,
		}
		
		if len(t.To) == 1 {
			jt.To = t.To[0]
		} else {
			jt.To = t.To
		}
		
		j.Transitions = append(j.Transitions, jt)
	}

	// Serialise class system. Only emit non-default data to keep
	// files clean for FSMs that don't use classes.
	if len(f.Classes) > 0 {
		// Always include classes if any exist (including default_state,
		// since user may have customised its properties).
		j.Classes = f.Classes
	}
	if len(f.StateClasses) > 0 {
		j.StateClasses = f.StateClasses
	}
	if len(f.StateProperties) > 0 {
		// Only include states that have at least one non-zero property.
		filtered := make(map[string]map[string]interface{})
		for state, props := range f.StateProperties {
			if len(props) > 0 {
				hasNonDefault := false
				for _, v := range props {
					if !isZeroValue(v) {
						hasNonDefault = true
						break
					}
				}
				if hasNonDefault {
					filtered[state] = props
				}
			}
		}
		if len(filtered) > 0 {
			j.StateProperties = filtered
		}
	}
	
	if pretty {
		return json.MarshalIndent(j, "", "  ")
	}
	return json.Marshal(j)
}

// isZeroValue checks whether a property value is the zero/default value.
func isZeroValue(v interface{}) bool {
	switch val := v.(type) {
	case float64:
		return val == 0
	case int64:
		return val == 0
	case uint64:
		return val == 0
	case string:
		return val == ""
	case bool:
		return !val
	case nil:
		return true
	case []string:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}
