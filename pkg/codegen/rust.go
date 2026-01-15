package codegen

import (
	"fmt"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// GenerateRust generates Rust code for the FSM.
// If the FSM is an NFA, it is first converted to a DFA.
func GenerateRust(f *fsm.FSM) string {
	// Convert NFA to DFA for code generation
	if f.Type == fsm.TypeNFA {
		f = f.ToDFA()
	}

	var sb strings.Builder
	name := toSnakeCase(sanitizeName(f.Name))
	typeName := toPascalCase(sanitizeName(f.Name))
	if name == "" {
		name = "fsm"
		typeName = "Fsm"
	}

	// Header
	sb.WriteString(fmt.Sprintf(`//! Generated FSM: %s
//! Type: %s

`, f.Name, f.Type))

	// State enum
	sb.WriteString("#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]\n")
	sb.WriteString("#[repr(u16)]\n")
	sb.WriteString(fmt.Sprintf("pub enum %sState {\n", typeName))
	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("    %s,\n", toPascalCase(state)))
	}
	sb.WriteString("}\n\n")

	// Input enum
	sb.WriteString("#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]\n")
	sb.WriteString("#[repr(u16)]\n")
	sb.WriteString(fmt.Sprintf("pub enum %sInput {\n", typeName))
	for _, input := range f.Alphabet {
		sb.WriteString(fmt.Sprintf("    %s,\n", toPascalCase(input)))
	}
	sb.WriteString("}\n\n")

	// Output enum (if applicable)
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString("#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]\n")
		sb.WriteString("#[repr(u16)]\n")
		sb.WriteString(fmt.Sprintf("pub enum %sOutput {\n", typeName))
		for _, output := range f.OutputAlphabet {
			sb.WriteString(fmt.Sprintf("    %s,\n", toPascalCase(output)))
		}
		sb.WriteString("}\n\n")
	}

	// FSM struct
	sb.WriteString("#[derive(Debug, Clone)]\n")
	sb.WriteString(fmt.Sprintf("pub struct %s {\n", typeName))
	sb.WriteString(fmt.Sprintf("    state: %sState,\n", typeName))
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString(fmt.Sprintf("    output: Option<%sOutput>,\n", typeName))
	}
	sb.WriteString("}\n\n")

	// Implementation
	sb.WriteString(fmt.Sprintf("impl %s {\n", typeName))

	// new()
	sb.WriteString("    /// Create new FSM in initial state\n")
	sb.WriteString(fmt.Sprintf("    pub fn new() -> Self {\n"))
	sb.WriteString(fmt.Sprintf("        Self {\n"))
	sb.WriteString(fmt.Sprintf("            state: %sState::%s,\n", typeName, toPascalCase(f.Initial)))
	if f.Type == fsm.TypeMoore {
		if out, ok := f.StateOutputs[f.Initial]; ok {
			sb.WriteString(fmt.Sprintf("            output: Some(%sOutput::%s),\n", typeName, toPascalCase(out)))
		} else {
			sb.WriteString("            output: None,\n")
		}
	} else if f.Type == fsm.TypeMealy {
		sb.WriteString("            output: None,\n")
	}
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// state()
	sb.WriteString("    /// Get current state\n")
	sb.WriteString(fmt.Sprintf("    pub fn state(&self) -> %sState {\n", typeName))
	sb.WriteString("        self.state\n")
	sb.WriteString("    }\n\n")

	// step()
	sb.WriteString("    /// Process input, returns true if transition occurred\n")
	sb.WriteString(fmt.Sprintf("    pub fn step(&mut self, input: %sInput) -> bool {\n", typeName))
	sb.WriteString("        match (self.state, input) {\n")

	// Generate match arms
	for _, t := range f.Transitions {
		if t.Input == nil {
			continue // skip epsilon
		}
		if len(t.To) == 0 {
			continue
		}

		fromPascal := toPascalCase(t.From)
		inputPascal := toPascalCase(*t.Input)
		toPascal := toPascalCase(t.To[0])

		sb.WriteString(fmt.Sprintf("            (%sState::%s, %sInput::%s) => {\n",
			typeName, fromPascal, typeName, inputPascal))
		sb.WriteString(fmt.Sprintf("                self.state = %sState::%s;\n", typeName, toPascal))

		if f.Type == fsm.TypeMoore {
			if out, ok := f.StateOutputs[t.To[0]]; ok {
				sb.WriteString(fmt.Sprintf("                self.output = Some(%sOutput::%s);\n", typeName, toPascalCase(out)))
			}
		} else if f.Type == fsm.TypeMealy && t.Output != nil {
			sb.WriteString(fmt.Sprintf("                self.output = Some(%sOutput::%s);\n", typeName, toPascalCase(*t.Output)))
		}

		sb.WriteString("                true\n")
		sb.WriteString("            }\n")
	}

	sb.WriteString("            _ => false,\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// can_step()
	sb.WriteString("    /// Check if input is valid from current state (without transitioning)\n")
	sb.WriteString(fmt.Sprintf("    pub fn can_step(&self, input: %sInput) -> bool {\n", typeName))
	sb.WriteString("        match (self.state, input) {\n")

	for _, t := range f.Transitions {
		if t.Input == nil || len(t.To) == 0 {
			continue
		}
		fromPascal := toPascalCase(t.From)
		inputPascal := toPascalCase(*t.Input)
		sb.WriteString(fmt.Sprintf("            (%sState::%s, %sInput::%s) => true,\n",
			typeName, fromPascal, typeName, inputPascal))
	}

	sb.WriteString("            _ => false,\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// is_accepting()
	sb.WriteString("    /// Check if current state is accepting\n")
	sb.WriteString("    pub fn is_accepting(&self) -> bool {\n")
	if len(f.Accepting) > 0 {
		sb.WriteString("        matches!(self.state, ")
		for i, acc := range f.Accepting {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%sState::%s", typeName, toPascalCase(acc)))
		}
		sb.WriteString(")\n")
	} else {
		sb.WriteString("        false\n")
	}
	sb.WriteString("    }\n\n")

	// output() (if applicable)
	if f.Type == fsm.TypeMoore || f.Type == fsm.TypeMealy {
		sb.WriteString("    /// Get current output\n")
		sb.WriteString(fmt.Sprintf("    pub fn output(&self) -> Option<%sOutput> {\n", typeName))
		sb.WriteString("        self.output\n")
		sb.WriteString("    }\n\n")
	}

	// reset()
	sb.WriteString("    /// Reset to initial state\n")
	sb.WriteString("    pub fn reset(&mut self) {\n")
	sb.WriteString(fmt.Sprintf("        self.state = %sState::%s;\n", typeName, toPascalCase(f.Initial)))
	if f.Type == fsm.TypeMoore {
		if out, ok := f.StateOutputs[f.Initial]; ok {
			sb.WriteString(fmt.Sprintf("        self.output = Some(%sOutput::%s);\n", typeName, toPascalCase(out)))
		} else {
			sb.WriteString("        self.output = None;\n")
		}
	} else if f.Type == fsm.TypeMealy {
		sb.WriteString("        self.output = None;\n")
	}
	sb.WriteString("    }\n")

	sb.WriteString("}\n\n")

	// Default impl
	sb.WriteString(fmt.Sprintf("impl Default for %s {\n", typeName))
	sb.WriteString("    fn default() -> Self {\n")
	sb.WriteString("        Self::new()\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n\n")

	// Display impl for State
	sb.WriteString(fmt.Sprintf("impl std::fmt::Display for %sState {\n", typeName))
	sb.WriteString("    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {\n")
	sb.WriteString("        match self {\n")
	for _, state := range f.States {
		sb.WriteString(fmt.Sprintf("            %sState::%s => write!(f, \"%s\"),\n",
			typeName, toPascalCase(state), state))
	}
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n\n")

	// Display impl for Input
	sb.WriteString(fmt.Sprintf("impl std::fmt::Display for %sInput {\n", typeName))
	sb.WriteString("    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {\n")
	sb.WriteString("        match self {\n")
	for _, input := range f.Alphabet {
		sb.WriteString(fmt.Sprintf("            %sInput::%s => write!(f, \"%s\"),\n",
			typeName, toPascalCase(input), input))
	}
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	// Display impl for Output
	if len(f.OutputAlphabet) > 0 {
		sb.WriteString(fmt.Sprintf("\nimpl std::fmt::Display for %sOutput {\n", typeName))
		sb.WriteString("    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {\n")
		sb.WriteString("        match self {\n")
		for _, output := range f.OutputAlphabet {
			sb.WriteString(fmt.Sprintf("            %sOutput::%s => write!(f, \"%s\"),\n",
				typeName, toPascalCase(output), output))
		}
		sb.WriteString("        }\n")
		sb.WriteString("    }\n")
		sb.WriteString("}\n")
	}

	return sb.String()
}

// Helper functions

func toPascalCase(s string) string {
	if s == "" {
		return "Unknown"
	}
	words := splitWords(s)
	var result strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(string(word[0])))
			if len(word) > 1 {
				result.WriteString(strings.ToLower(word[1:]))
			}
		}
	}
	name := result.String()
	if name == "" {
		return "Unknown"
	}
	return name
}

func toSnakeCase(s string) string {
	words := splitWords(s)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "_")
}

func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}
