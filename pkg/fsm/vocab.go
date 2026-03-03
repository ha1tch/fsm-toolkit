package fsm

// Vocabulary system — domain-neutral labelling for FSM concepts.
//
// The same data model can represent behavioural state machines,
// structural circuits, or generic directed graphs. The vocabulary
// controls how concepts are named in user-facing output: "state" vs
// "component" vs "node", "transition" vs "connection" vs "edge", etc.
//
// Vocabulary is per-FSM and persisted in the file format. Setting it
// to "auto" enables automatic detection based on structural features:
// if the FSM has classes with ports or nets, it resolves to "circuit";
// otherwise, it resolves to "fsm".

// VocabLabels holds the display labels for a given vocabulary mode.
type VocabLabels struct {
	State      string // "State", "Component", "Node"
	States     string // "States", "Components", "Nodes"
	Transition string // "Transition", "Connection", "Edge"
	Alphabet   string // "Alphabet", "Signals", "Labels"
	Initial    string // "Initial", "Entry", "Start"
	Accepting  string // "Accepting", "Terminal", "End"
	Input      string // "Input", "Signal", "Label"
	Output     string // "Output", "Output", "Output"
}

// Vocabulary constants. These are the valid values for FSM.Vocabulary.
const (
	// VocabFSM is the default automata vocabulary.
	VocabFSM = "fsm"

	// VocabCircuit is the structural circuit vocabulary.
	VocabCircuit = "circuit"

	// VocabGeneric is the generic directed graph vocabulary.
	VocabGeneric = "generic"

	// VocabAuto enables automatic detection based on structural
	// features. Resolves to "circuit" if the FSM has classes with
	// ports or nets; otherwise resolves to "fsm".
	VocabAuto = "auto"
)

// Vocabularies maps vocabulary names to their label sets.
// VocabAuto is not in this map because it resolves dynamically.
var Vocabularies = map[string]VocabLabels{
	VocabFSM: {
		State:      "State",
		States:     "States",
		Transition: "Transition",
		Alphabet:   "Alphabet",
		Initial:    "Initial",
		Accepting:  "Accepting",
		Input:      "Input",
		Output:     "Output",
	},
	VocabCircuit: {
		State:      "Component",
		States:     "Components",
		Transition: "Connection",
		Alphabet:   "Signals",
		Initial:    "Entry",
		Accepting:  "Output",
		Input:      "Signal",
		Output:     "Output",
	},
	VocabGeneric: {
		State:      "Node",
		States:     "Nodes",
		Transition: "Edge",
		Alphabet:   "Labels",
		Initial:    "Start",
		Accepting:  "End",
		Input:      "Label",
		Output:     "Output",
	},
}

// VocabNames returns the list of valid vocabulary values in cycle order.
// This is the order used by settings panels and configuration tools.
func VocabNames() []string {
	return []string{VocabFSM, VocabCircuit, VocabGeneric, VocabAuto}
}

// Vocab returns the vocabulary labels for this FSM.
//
// Resolution order:
//   - "fsm", "circuit", "generic": direct lookup
//   - "auto": calls DetectVocabulary and resolves dynamically
//   - "" (empty) or unrecognised: defaults to "fsm"
func (f *FSM) Vocab() VocabLabels {
	name := f.Vocabulary
	if name == VocabAuto {
		name = DetectVocabulary(f)
	}
	if v, ok := Vocabularies[name]; ok {
		return v
	}
	return Vocabularies[VocabFSM]
}

// ResolvedVocabulary returns the concrete vocabulary name after
// resolving "auto". Useful for display: "auto (circuit)" vs "auto (fsm)".
func (f *FSM) ResolvedVocabulary() string {
	if f.Vocabulary == VocabAuto {
		return DetectVocabulary(f)
	}
	if _, ok := Vocabularies[f.Vocabulary]; ok {
		return f.Vocabulary
	}
	return VocabFSM
}

// DetectVocabulary examines the FSM's structural features and returns
// the most appropriate vocabulary name:
//
//   - If any class has ports, or if any nets are defined: "circuit"
//   - Otherwise: "fsm"
//
// This function does not modify the FSM. It is called automatically
// when the vocabulary is set to "auto", or can be called directly by
// callers who want to inspect the result without applying it.
func DetectVocabulary(f *FSM) string {
	// Check for nets
	if len(f.Nets) > 0 {
		return VocabCircuit
	}

	// Check for classes with ports
	for name, cls := range f.Classes {
		if name == DefaultClassName {
			continue
		}
		if len(cls.Ports) > 0 {
			return VocabCircuit
		}
	}

	return VocabFSM
}
