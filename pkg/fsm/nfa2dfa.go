package fsm

import (
	"sort"
	"strings"
)

// ToDFA converts an NFA to an equivalent DFA using the powerset construction.
// The resulting DFA accepts the same language as the original NFA.
// State names in the DFA are comma-separated combinations of NFA states.
func (f *FSM) ToDFA() *FSM {
	if f.Type != TypeNFA {
		// Already deterministic, return a copy
		return f.Copy()
	}

	dfa := &FSM{
		Type:         TypeDFA,
		Name:         f.Name,
		Description:  f.Description,
		Alphabet:     make([]string, len(f.Alphabet)),
		States:       make([]string, 0),
		Transitions:  make([]Transition, 0),
		Accepting:    make([]string, 0),
		StateOutputs: make(map[string]string),
	}
	copy(dfa.Alphabet, f.Alphabet)

	// Helper to compute epsilon closure
	epsilonClosure := func(states map[string]bool) map[string]bool {
		closure := make(map[string]bool)
		for s := range states {
			closure[s] = true
		}

		changed := true
		for changed {
			changed = false
			for state := range closure {
				for _, t := range f.GetEpsilonTransitions(state) {
					for _, to := range t.To {
						if !closure[to] {
							closure[to] = true
							changed = true
						}
					}
				}
			}
		}
		return closure
	}

	// Helper to convert state set to canonical name
	stateSetName := func(states map[string]bool) string {
		var list []string
		for s := range states {
			list = append(list, s)
		}
		sort.Strings(list)
		return strings.Join(list, ",")
	}

	// Helper to check if state set contains an accepting state
	isAccepting := func(states map[string]bool) bool {
		for s := range states {
			if f.IsAccepting(s) {
				return true
			}
		}
		return false
	}

	// Start with epsilon closure of initial state
	initialSet := make(map[string]bool)
	initialSet[f.Initial] = true
	initialSet = epsilonClosure(initialSet)
	dfa.Initial = stateSetName(initialSet)

	// Track which DFA states we've processed
	processed := make(map[string]bool)
	// Queue of state sets to process
	queue := []map[string]bool{initialSet}
	// Map from DFA state name to NFA state set
	dfaStates := make(map[string]map[string]bool)
	dfaStates[dfa.Initial] = initialSet

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentName := stateSetName(current)

		if processed[currentName] {
			continue
		}
		processed[currentName] = true

		// Add to DFA states
		dfa.States = append(dfa.States, currentName)
		if isAccepting(current) {
			dfa.Accepting = append(dfa.Accepting, currentName)
		}

		// For each input symbol
		for _, input := range f.Alphabet {
			// Compute target state set
			targetSet := make(map[string]bool)
			for state := range current {
				transitions := f.GetTransitions(state, &input)
				for _, t := range transitions {
					for _, to := range t.To {
						targetSet[to] = true
					}
				}
			}

			if len(targetSet) == 0 {
				continue // No transition for this input
			}

			// Apply epsilon closure
			targetSet = epsilonClosure(targetSet)
			targetName := stateSetName(targetSet)

			// Add transition
			inp := input // copy for pointer
			dfa.Transitions = append(dfa.Transitions, Transition{
				From:  currentName,
				Input: &inp,
				To:    []string{targetName},
			})

			// Queue target if not seen
			if !processed[targetName] && dfaStates[targetName] == nil {
				dfaStates[targetName] = targetSet
				queue = append(queue, targetSet)
			}
		}
	}

	return dfa
}

// Copy creates a deep copy of the FSM.
func (f *FSM) Copy() *FSM {
	copy := &FSM{
		Type:           f.Type,
		Name:           f.Name,
		Description:    f.Description,
		States:         make([]string, len(f.States)),
		Alphabet:       make([]string, len(f.Alphabet)),
		OutputAlphabet: make([]string, len(f.OutputAlphabet)),
		Transitions:    make([]Transition, len(f.Transitions)),
		Initial:        f.Initial,
		Accepting:      make([]string, len(f.Accepting)),
		StateOutputs:   make(map[string]string),
	}

	copy1(copy.States, f.States)
	copy1(copy.Alphabet, f.Alphabet)
	copy1(copy.OutputAlphabet, f.OutputAlphabet)
	copy1(copy.Accepting, f.Accepting)

	for i, t := range f.Transitions {
		copy.Transitions[i] = Transition{
			From: t.From,
			To:   make([]string, len(t.To)),
		}
		copy1(copy.Transitions[i].To, t.To)
		if t.Input != nil {
			inp := *t.Input
			copy.Transitions[i].Input = &inp
		}
		if t.Output != nil {
			out := *t.Output
			copy.Transitions[i].Output = &out
		}
	}

	for k, v := range f.StateOutputs {
		copy.StateOutputs[k] = v
	}

	return copy
}

// Helper to avoid name collision with builtin copy
func copy1[T any](dst, src []T) {
	copy(dst, src)
}
