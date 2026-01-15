// Package codegen generates code from FSM definitions.
package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// GenerateC generates C code for the FSM.
// If the FSM is an NFA, it is first converted to a DFA.
func GenerateC(f *fsm.FSM) string {
	// Convert NFA to DFA for code generation
	if f.Type == fsm.TypeNFA {
		f = f.ToDFA()
	}

	var sb strings.Builder
	name := sanitizeName(f.Name)
	if name == "" {
		name = "fsm"
	}
	NAME := strings.ToUpper(name)

	// Header
	sb.WriteString(fmt.Sprintf(`// Generated FSM: %s
// Type: %s

#ifndef %s_H
#define %s_H

#include <stdint.h>
#include <stdbool.h>

`, f.Name, f.Type, NAME, NAME))

	// Types - simple uint16_t
	sb.WriteString(fmt.Sprintf("typedef uint16_t %s_state_t;\n", name))
	sb.WriteString(fmt.Sprintf("typedef uint16_t %s_input_t;\n", name))
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString(fmt.Sprintf("typedef uint16_t %s_output_t;\n", name))
	}
	sb.WriteString("\n")

	// State constants
	sb.WriteString("// States\n")
	for i, state := range f.States {
		sb.WriteString(fmt.Sprintf("#define %s_STATE_%s %d\n", NAME, strings.ToUpper(sanitizeName(state)), i))
	}
	sb.WriteString("\n")

	// Input constants
	sb.WriteString("// Inputs\n")
	for i, input := range f.Alphabet {
		sb.WriteString(fmt.Sprintf("#define %s_INPUT_%s %d\n", NAME, strings.ToUpper(sanitizeName(input)), i))
	}
	sb.WriteString("\n")

	// Output constants (if applicable)
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString("// Outputs\n")
		for i, output := range f.OutputAlphabet {
			sb.WriteString(fmt.Sprintf("#define %s_OUTPUT_%s %d\n", NAME, strings.ToUpper(sanitizeName(output)), i))
		}
		sb.WriteString("\n")
	}

	// FSM struct
	sb.WriteString("// FSM instance\n")
	sb.WriteString(fmt.Sprintf("typedef struct {\n"))
	sb.WriteString(fmt.Sprintf("    %s_state_t state;\n", name))
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString(fmt.Sprintf("    %s_output_t output;\n", name))
	}
	sb.WriteString(fmt.Sprintf("} %s_t;\n\n", name))

	// Counts
	sb.WriteString("// Counts\n")
	sb.WriteString(fmt.Sprintf("#define %s_STATE_COUNT %d\n", NAME, len(f.States)))
	sb.WriteString(fmt.Sprintf("#define %s_INPUT_COUNT %d\n", NAME, len(f.Alphabet)))
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString(fmt.Sprintf("#define %s_OUTPUT_COUNT %d\n", NAME, len(f.OutputAlphabet)))
	}
	sb.WriteString("\n")

	// Function declarations
	sb.WriteString("// Initialize FSM\n")
	sb.WriteString(fmt.Sprintf("void %s_init(%s_t *fsm);\n\n", name, name))

	sb.WriteString("// Reset FSM to initial state\n")
	sb.WriteString(fmt.Sprintf("void %s_reset(%s_t *fsm);\n\n", name, name))

	sb.WriteString("// Process input, returns true if transition occurred\n")
	sb.WriteString(fmt.Sprintf("bool %s_step(%s_t *fsm, %s_input_t input);\n\n", name, name, name))

	sb.WriteString("// Check if input is valid from current state (without transitioning)\n")
	sb.WriteString(fmt.Sprintf("bool %s_can_step(%s_t *fsm, %s_input_t input);\n\n", name, name, name))

	sb.WriteString("// Check if current state is accepting\n")
	sb.WriteString(fmt.Sprintf("bool %s_is_accepting(%s_t *fsm);\n\n", name, name))

	sb.WriteString("// Get current state\n")
	sb.WriteString(fmt.Sprintf("%s_state_t %s_get_state(%s_t *fsm);\n\n", name, name, name))

	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString("// Get current output\n")
		sb.WriteString(fmt.Sprintf("%s_output_t %s_get_output(%s_t *fsm);\n\n", name, name, name))
	}

	sb.WriteString("// Get state name (for debugging)\n")
	sb.WriteString(fmt.Sprintf("const char* %s_state_name(%s_state_t state);\n\n", name, name))

	sb.WriteString("// Get input name (for debugging)\n")
	sb.WriteString(fmt.Sprintf("const char* %s_input_name(%s_input_t input);\n\n", name, name))

	if len(f.OutputAlphabet) > 0 {
		sb.WriteString("// Get output name (for debugging)\n")
		sb.WriteString(fmt.Sprintf("const char* %s_output_name(%s_output_t output);\n\n", name, name))
	}

	sb.WriteString("#endif // " + NAME + "_H\n\n")

	// Implementation
	sb.WriteString("// ---- Implementation ----\n")
	sb.WriteString("#ifdef " + NAME + "_IMPLEMENTATION\n\n")

	// Init function
	sb.WriteString(fmt.Sprintf("void %s_init(%s_t *fsm) {\n", name, name))
	initialIdx := stateIndex(f, f.Initial)
	sb.WriteString(fmt.Sprintf("    fsm->state = %d;\n", initialIdx))
	if f.Type == fsm.TypeMoore {
		if out, ok := f.StateOutputs[f.Initial]; ok {
			outIdx := outputIndex(f, out)
			sb.WriteString(fmt.Sprintf("    fsm->output = %d;\n", outIdx))
		} else {
			sb.WriteString("    fsm->output = 0;\n")
		}
	} else if f.Type == fsm.TypeMealy {
		sb.WriteString("    fsm->output = 0;\n")
	}
	sb.WriteString("}\n\n")

	// Step function
	sb.WriteString(fmt.Sprintf("bool %s_step(%s_t *fsm, %s_input_t input) {\n", name, name, name))
	sb.WriteString("    switch (fsm->state) {\n")

	// Group transitions by from state
	transByState := make(map[string][]fsm.Transition)
	for _, t := range f.Transitions {
		transByState[t.From] = append(transByState[t.From], t)
	}

	for _, state := range f.States {
		stateIdx := stateIndex(f, state)
		sb.WriteString(fmt.Sprintf("    case %d: // %s\n", stateIdx, state))
		sb.WriteString("        switch (input) {\n")

		if trans, ok := transByState[state]; ok {
			for _, t := range trans {
				if t.Input == nil {
					continue // skip epsilon transitions
				}
				if len(t.To) > 0 {
					inputIdx := inputIndex(f, *t.Input)
					toIdx := stateIndex(f, t.To[0])
					sb.WriteString(fmt.Sprintf("        case %d: // %s\n", inputIdx, *t.Input))
					sb.WriteString(fmt.Sprintf("            fsm->state = %d;\n", toIdx))
					if f.Type == fsm.TypeMoore {
						if out, ok := f.StateOutputs[t.To[0]]; ok {
							outIdx := outputIndex(f, out)
							sb.WriteString(fmt.Sprintf("            fsm->output = %d;\n", outIdx))
						}
					} else if f.Type == fsm.TypeMealy && t.Output != nil {
						outIdx := outputIndex(f, *t.Output)
						sb.WriteString(fmt.Sprintf("            fsm->output = %d;\n", outIdx))
					}
					sb.WriteString("            return true;\n")
				}
			}
		}

		sb.WriteString("        default:\n")
		sb.WriteString("            return false;\n")
		sb.WriteString("        }\n")
	}

	sb.WriteString("    default:\n")
	sb.WriteString("        return false;\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n\n")

	// Can step function (same logic as step but without side effects)
	sb.WriteString(fmt.Sprintf("bool %s_can_step(%s_t *fsm, %s_input_t input) {\n", name, name, name))
	sb.WriteString("    switch (fsm->state) {\n")

	for _, state := range f.States {
		stateIdx := stateIndex(f, state)
		sb.WriteString(fmt.Sprintf("    case %d:\n", stateIdx))
		sb.WriteString("        switch (input) {\n")

		if trans, ok := transByState[state]; ok {
			for _, t := range trans {
				if t.Input == nil || len(t.To) == 0 {
					continue
				}
				inputIdx := inputIndex(f, *t.Input)
				sb.WriteString(fmt.Sprintf("        case %d: return true;\n", inputIdx))
			}
		}

		sb.WriteString("        default: return false;\n")
		sb.WriteString("        }\n")
	}

	sb.WriteString("    default:\n")
	sb.WriteString("        return false;\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n\n")

	// Is accepting function
	sb.WriteString(fmt.Sprintf("bool %s_is_accepting(%s_t *fsm) {\n", name, name))
	if len(f.Accepting) > 0 {
		sb.WriteString("    switch (fsm->state) {\n")
		for _, acc := range f.Accepting {
			accIdx := stateIndex(f, acc)
			sb.WriteString(fmt.Sprintf("    case %d: // %s\n", accIdx, acc))
		}
		sb.WriteString("        return true;\n")
		sb.WriteString("    default:\n")
		sb.WriteString("        return false;\n")
		sb.WriteString("    }\n")
	} else {
		sb.WriteString("    return false;\n")
	}
	sb.WriteString("}\n\n")

	// Get state function
	sb.WriteString(fmt.Sprintf("%s_state_t %s_get_state(%s_t *fsm) {\n", name, name, name))
	sb.WriteString("    return fsm->state;\n")
	sb.WriteString("}\n\n")

	// Get output function
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString(fmt.Sprintf("%s_output_t %s_get_output(%s_t *fsm) {\n", name, name, name))
		sb.WriteString("    return fsm->output;\n")
		sb.WriteString("}\n\n")
	}

	// Reset function
	sb.WriteString(fmt.Sprintf("void %s_reset(%s_t *fsm) {\n", name, name))
	sb.WriteString(fmt.Sprintf("    %s_init(fsm);\n", name))
	sb.WriteString("}\n\n")

	// State name lookup
	sb.WriteString(fmt.Sprintf("static const char* %s_state_names[] = {\n", name))
	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("    \"%s\",\n", state))
	}
	sb.WriteString("};\n\n")

	sb.WriteString(fmt.Sprintf("const char* %s_state_name(%s_state_t state) {\n", name, name))
	sb.WriteString(fmt.Sprintf("    if (state < %s_STATE_COUNT) return %s_state_names[state];\n", NAME, name))
	sb.WriteString("    return \"unknown\";\n")
	sb.WriteString("}\n\n")

	// Input name lookup
	sb.WriteString(fmt.Sprintf("static const char* %s_input_names[] = {\n", name))
	for _, input := range f.Alphabet {
		sb.WriteString(fmt.Sprintf("    \"%s\",\n", input))
	}
	sb.WriteString("};\n\n")

	sb.WriteString(fmt.Sprintf("const char* %s_input_name(%s_input_t input) {\n", name, name))
	sb.WriteString(fmt.Sprintf("    if (input < %s_INPUT_COUNT) return %s_input_names[input];\n", NAME, name))
	sb.WriteString("    return \"unknown\";\n")
	sb.WriteString("}\n\n")

	// Output name lookup
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString(fmt.Sprintf("static const char* %s_output_names[] = {\n", name))
		for _, output := range f.OutputAlphabet {
			sb.WriteString(fmt.Sprintf("    \"%s\",\n", output))
		}
		sb.WriteString("};\n\n")

		sb.WriteString(fmt.Sprintf("const char* %s_output_name(%s_output_t output) {\n", name, name))
		sb.WriteString(fmt.Sprintf("    if (output < %s_OUTPUT_COUNT) return %s_output_names[output];\n", NAME, name))
		sb.WriteString("    return \"unknown\";\n")
		sb.WriteString("}\n\n")
	}

	sb.WriteString("#endif // " + NAME + "_IMPLEMENTATION\n")

	return sb.String()
}

// Helper functions

func sanitizeName(s string) string {
	if s == "" {
		return "unnamed"
	}
	var result strings.Builder
	for i, r := range s {
		if unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) || r == '_' {
			result.WriteRune(r)
		} else if r == ' ' || r == '-' {
			result.WriteRune('_')
		}
	}
	name := result.String()
	if name == "" {
		return "unnamed"
	}
	// Ensure starts with letter
	if unicode.IsDigit(rune(name[0])) {
		name = "_" + name
	}
	return name
}

func stateIndex(f *fsm.FSM, state string) int {
	for i, s := range f.States {
		if s == state {
			return i
		}
	}
	return 0
}

func inputIndex(f *fsm.FSM, input string) int {
	for i, inp := range f.Alphabet {
		if inp == input {
			return i
		}
	}
	return 0
}

func outputIndex(f *fsm.FSM, output string) int {
	for i, out := range f.OutputAlphabet {
		if out == output {
			return i
		}
	}
	return 0
}
