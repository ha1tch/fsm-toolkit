package codegen

import (
	"fmt"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// GenerateGo generates Go code for the FSM.
// The generated code is compatible with both standard Go and TinyGo.
// If the FSM is an NFA, it is first converted to a DFA.
func GenerateGo(f *fsm.FSM, packageName string) string {
	// Convert NFA to DFA for code generation
	if f.Type == fsm.TypeNFA {
		f = f.ToDFA()
	}

	var sb strings.Builder
	typeName := toPascalCase(sanitizeName(f.Name))
	if typeName == "" {
		typeName = "FSM"
	}
	if packageName == "" {
		packageName = "fsm"
	}

	// Header
	sb.WriteString(fmt.Sprintf(`// Code generated from FSM definition. DO NOT EDIT.
// FSM: %s
// Type: %s

package %s

`, f.Name, f.Type, packageName))

	// State type
	sb.WriteString(fmt.Sprintf("// %sState represents FSM states\n", typeName))
	sb.WriteString(fmt.Sprintf("type %sState uint16\n\n", typeName))

	// State constants
	sb.WriteString("const (\n")
	for i, state := range f.States {
		constName := fmt.Sprintf("%sState%s", typeName, toPascalCase(state))
		if i == 0 {
			sb.WriteString(fmt.Sprintf("\t%s %sState = iota\n", constName, typeName))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s\n", constName))
		}
	}
	sb.WriteString(")\n\n")

	// State names for debugging
	sb.WriteString(fmt.Sprintf("var %sStateNames = [...]string{\n", strings.ToLower(typeName)))
	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("\t%q,\n", state))
	}
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (s %sState) String() string {\n", typeName))
	sb.WriteString(fmt.Sprintf("\tif int(s) < len(%sStateNames) {\n", strings.ToLower(typeName)))
	sb.WriteString(fmt.Sprintf("\t\treturn %sStateNames[s]\n", strings.ToLower(typeName)))
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn \"unknown\"\n")
	sb.WriteString("}\n\n")

	// Input type
	sb.WriteString(fmt.Sprintf("// %sInput represents FSM inputs\n", typeName))
	sb.WriteString(fmt.Sprintf("type %sInput uint16\n\n", typeName))

	// Input constants
	sb.WriteString("const (\n")
	for i, input := range f.Alphabet {
		constName := fmt.Sprintf("%sInput%s", typeName, toPascalCase(input))
		if i == 0 {
			sb.WriteString(fmt.Sprintf("\t%s %sInput = iota\n", constName, typeName))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s\n", constName))
		}
	}
	sb.WriteString(")\n\n")

	// Input names
	sb.WriteString(fmt.Sprintf("var %sInputNames = [...]string{\n", strings.ToLower(typeName)))
	for _, input := range f.Alphabet {
		sb.WriteString(fmt.Sprintf("\t%q,\n", input))
	}
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func (i %sInput) String() string {\n", typeName))
	sb.WriteString(fmt.Sprintf("\tif int(i) < len(%sInputNames) {\n", strings.ToLower(typeName)))
	sb.WriteString(fmt.Sprintf("\t\treturn %sInputNames[i]\n", strings.ToLower(typeName)))
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn \"unknown\"\n")
	sb.WriteString("}\n\n")

	// Output type (if applicable)
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString(fmt.Sprintf("// %sOutput represents FSM outputs\n", typeName))
		sb.WriteString(fmt.Sprintf("type %sOutput uint16\n\n", typeName))

		// Output constants
		sb.WriteString("const (\n")
		for i, output := range f.OutputAlphabet {
			constName := fmt.Sprintf("%sOutput%s", typeName, toPascalCase(output))
			if i == 0 {
				sb.WriteString(fmt.Sprintf("\t%s %sOutput = iota\n", constName, typeName))
			} else {
				sb.WriteString(fmt.Sprintf("\t%s\n", constName))
			}
		}
		sb.WriteString(")\n\n")

		// Output names
		sb.WriteString(fmt.Sprintf("var %sOutputNames = [...]string{\n", strings.ToLower(typeName)))
		for _, output := range f.OutputAlphabet {
			sb.WriteString(fmt.Sprintf("\t%q,\n", output))
		}
		sb.WriteString("}\n\n")

		sb.WriteString(fmt.Sprintf("func (o %sOutput) String() string {\n", typeName))
		sb.WriteString(fmt.Sprintf("\tif int(o) < len(%sOutputNames) {\n", strings.ToLower(typeName)))
		sb.WriteString(fmt.Sprintf("\t\treturn %sOutputNames[o]\n", strings.ToLower(typeName)))
		sb.WriteString("\t}\n")
		sb.WriteString("\treturn \"unknown\"\n")
		sb.WriteString("}\n\n")
	}

	// FSM struct
	sb.WriteString(fmt.Sprintf("// %s is the finite state machine\n", typeName))
	sb.WriteString(fmt.Sprintf("type %s struct {\n", typeName))
	sb.WriteString(fmt.Sprintf("\tstate %sState\n", typeName))
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString(fmt.Sprintf("\toutput %sOutput\n", typeName))
		sb.WriteString("\thasOutput bool\n")
	}
	sb.WriteString("}\n\n")

	// New function
	sb.WriteString(fmt.Sprintf("// New%s creates a new FSM in its initial state\n", typeName))
	sb.WriteString(fmt.Sprintf("func New%s() *%s {\n", typeName, typeName))
	sb.WriteString(fmt.Sprintf("\tf := &%s{\n", typeName))
	sb.WriteString(fmt.Sprintf("\t\tstate: %sState%s,\n", typeName, toPascalCase(f.Initial)))
	if f.Type == fsm.TypeMoore {
		if out, ok := f.StateOutputs[f.Initial]; ok {
			sb.WriteString(fmt.Sprintf("\t\toutput: %sOutput%s,\n", typeName, toPascalCase(out)))
			sb.WriteString("\t\thasOutput: true,\n")
		}
	}
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn f\n")
	sb.WriteString("}\n\n")

	// State getter
	sb.WriteString(fmt.Sprintf("// State returns the current state\n"))
	sb.WriteString(fmt.Sprintf("func (f *%s) State() %sState {\n", typeName, typeName))
	sb.WriteString("\treturn f.state\n")
	sb.WriteString("}\n\n")

	// Step function
	sb.WriteString(fmt.Sprintf("// Step processes an input and transitions to the next state.\n"))
	sb.WriteString("// Returns true if a valid transition occurred.\n")
	sb.WriteString(fmt.Sprintf("func (f *%s) Step(input %sInput) bool {\n", typeName, typeName))
	sb.WriteString("\tswitch f.state {\n")

	// Group transitions by from state
	transByState := make(map[string][]fsm.Transition)
	for _, t := range f.Transitions {
		transByState[t.From] = append(transByState[t.From], t)
	}

	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("\tcase %sState%s:\n", typeName, toPascalCase(state)))
		sb.WriteString("\t\tswitch input {\n")

		if trans, ok := transByState[state]; ok {
			for _, t := range trans {
				if t.Input == nil || len(t.To) == 0 {
					continue
				}

				sb.WriteString(fmt.Sprintf("\t\tcase %sInput%s:\n", typeName, toPascalCase(*t.Input)))
				sb.WriteString(fmt.Sprintf("\t\t\tf.state = %sState%s\n", typeName, toPascalCase(t.To[0])))

				if f.Type == fsm.TypeMoore {
					if out, ok := f.StateOutputs[t.To[0]]; ok {
						sb.WriteString(fmt.Sprintf("\t\t\tf.output = %sOutput%s\n", typeName, toPascalCase(out)))
						sb.WriteString("\t\t\tf.hasOutput = true\n")
					}
				} else if f.Type == fsm.TypeMealy && t.Output != nil {
					sb.WriteString(fmt.Sprintf("\t\t\tf.output = %sOutput%s\n", typeName, toPascalCase(*t.Output)))
					sb.WriteString("\t\t\tf.hasOutput = true\n")
				}

				sb.WriteString("\t\t\treturn true\n")
			}
		}

		sb.WriteString("\t\t}\n")
	}

	sb.WriteString("\t}\n")
	sb.WriteString("\treturn false\n")
	sb.WriteString("}\n\n")

	// CanStep function
	sb.WriteString(fmt.Sprintf("// CanStep returns true if the input is valid from current state (without transitioning)\n"))
	sb.WriteString(fmt.Sprintf("func (f *%s) CanStep(input %sInput) bool {\n", typeName, typeName))
	sb.WriteString("\tswitch f.state {\n")

	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("\tcase %sState%s:\n", typeName, toPascalCase(state)))
		sb.WriteString("\t\tswitch input {\n")

		if trans, ok := transByState[state]; ok {
			for _, t := range trans {
				if t.Input == nil || len(t.To) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("\t\tcase %sInput%s:\n", typeName, toPascalCase(*t.Input)))
				sb.WriteString("\t\t\treturn true\n")
			}
		}

		sb.WriteString("\t\t}\n")
	}

	sb.WriteString("\t}\n")
	sb.WriteString("\treturn false\n")
	sb.WriteString("}\n\n")

	// IsAccepting function
	sb.WriteString("// IsAccepting returns true if the current state is an accepting state\n")
	sb.WriteString(fmt.Sprintf("func (f *%s) IsAccepting() bool {\n", typeName))
	if len(f.Accepting) > 0 {
		sb.WriteString("\tswitch f.state {\n")
		sb.WriteString("\tcase ")
		for i, acc := range f.Accepting {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%sState%s", typeName, toPascalCase(acc)))
		}
		sb.WriteString(":\n")
		sb.WriteString("\t\treturn true\n")
		sb.WriteString("\t}\n")
	}
	sb.WriteString("\treturn false\n")
	sb.WriteString("}\n\n")

	// Output function (if applicable)
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString(fmt.Sprintf("// Output returns the current output and whether it's valid\n"))
		sb.WriteString(fmt.Sprintf("func (f *%s) Output() (%sOutput, bool) {\n", typeName, typeName))
		sb.WriteString("\treturn f.output, f.hasOutput\n")
		sb.WriteString("}\n\n")
	}

	// Reset function
	sb.WriteString("// Reset returns the FSM to its initial state\n")
	sb.WriteString(fmt.Sprintf("func (f *%s) Reset() {\n", typeName))
	sb.WriteString(fmt.Sprintf("\tf.state = %sState%s\n", typeName, toPascalCase(f.Initial)))
	if f.Type == fsm.TypeMoore {
		if out, ok := f.StateOutputs[f.Initial]; ok {
			sb.WriteString(fmt.Sprintf("\tf.output = %sOutput%s\n", typeName, toPascalCase(out)))
			sb.WriteString("\tf.hasOutput = true\n")
		} else {
			sb.WriteString("\tf.hasOutput = false\n")
		}
	} else if f.Type == fsm.TypeMealy {
		sb.WriteString("\tf.hasOutput = false\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// GenerateTinyGo is an alias for GenerateGo as the output is compatible.
// TinyGo-specific optimizations:
// - Uses uint16 for state/input/output types (matches FSM format capacity)
// - No heap allocations in step function
// - No reflection or interface{} usage
func GenerateTinyGo(f *fsm.FSM, packageName string) string {
	return GenerateGo(f, packageName)
}
