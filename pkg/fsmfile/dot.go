package fsmfile

import (
	"fmt"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// GenerateDOT converts an FSM to Graphviz DOT format.
func GenerateDOT(f *fsm.FSM, title string) string {
	var sb strings.Builder
	
	sb.WriteString("digraph FSM {\n")
	sb.WriteString("    rankdir=LR;\n")
	sb.WriteString("    node [fontname=\"Helvetica\", fontsize=11];\n")
	sb.WriteString("    edge [fontname=\"Helvetica\", fontsize=10];\n")
	sb.WriteString("\n")
	
	// Title
	if title != "" {
		sb.WriteString(fmt.Sprintf("    labelloc=\"t\";\n"))
		sb.WriteString(fmt.Sprintf("    label=\"%s\";\n", escapeDOT(title)))
		sb.WriteString("\n")
	}
	
	// Invisible start node
	if f.Initial != "" {
		sb.WriteString("    __start [shape=none, label=\"\", width=0, height=0];\n")
		sb.WriteString(fmt.Sprintf("    __start -> \"%s\";\n", escapeDOT(f.Initial)))
		sb.WriteString("\n")
	}
	
	// State nodes
	for _, state := range f.States {
		var attrs []string
		
		if f.IsAccepting(state) {
			attrs = append(attrs, "shape=doublecircle")
		} else {
			attrs = append(attrs, "shape=circle")
		}
		
		// Moore output in label
		if f.Type == fsm.TypeMoore {
			if out, ok := f.StateOutputs[state]; ok {
				label := fmt.Sprintf("%s\\n/%s", state, out)
				attrs = append(attrs, fmt.Sprintf("label=\"%s\"", escapeDOT(label)))
			}
		}
		
		sb.WriteString(fmt.Sprintf("    \"%s\" [%s];\n", escapeDOT(state), strings.Join(attrs, ", ")))
	}
	sb.WriteString("\n")
	
	// Group transitions by (from, to)
	edgeLabels := make(map[[2]string][]string)
	
	for _, t := range f.Transitions {
		var label string
		if t.Input == nil {
			label = "ε"
		} else {
			label = *t.Input
		}
		
		if f.Type == fsm.TypeMealy && t.Output != nil {
			label = fmt.Sprintf("%s/%s", label, *t.Output)
		}
		
		for _, to := range t.To {
			key := [2]string{t.From, to}
			edgeLabels[key] = append(edgeLabels[key], label)
		}
	}
	
	// Write edges
	for key, labels := range edgeLabels {
		combined := strings.Join(labels, ", ")
		sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [label=\"%s\"];\n",
			escapeDOT(key[0]), escapeDOT(key[1]), escapeDOT(combined)))
	}
	
	sb.WriteString("}\n")
	
	return sb.String()
}

func escapeDOT(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "<", "\\<")
	s = strings.ReplaceAll(s, ">", "\\>")
	return s
}
