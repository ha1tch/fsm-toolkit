package fsmfile

import (
	"encoding/json"

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
	
	return f, nil
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
	
	if pretty {
		return json.MarshalIndent(j, "", "  ")
	}
	return json.Marshal(j)
}
