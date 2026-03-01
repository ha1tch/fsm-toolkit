// FSM analysis and validation for fsmedit.
package main

import (
	"fmt"
	"strings"
)

func (ed *Editor) runAnalysis() {
	warnings := ed.fsm.Analyse()

	if len(warnings) == 0 {
		ed.showMessage("✓ No issues found", MsgInfo)
		return
	}

	// Build a summary message
	var issues []string
	for _, w := range warnings {
		switch w.Type {
		case "unreachable":
			issues = append(issues, fmt.Sprintf("%d unreachable", len(w.States)))
		case "dead":
			issues = append(issues, fmt.Sprintf("%d dead", len(w.States)))
		case "nondeterministic":
			issues = append(issues, fmt.Sprintf("%d nondet", len(w.States)))
		case "incomplete":
			issues = append(issues, fmt.Sprintf("%d incomplete", len(w.States)))
		case "unused_input":
			issues = append(issues, "unused inputs")
		case "unused_output":
			issues = append(issues, "unused outputs")
		}
	}

	msg := fmt.Sprintf("✗ %d issue(s): %s", len(warnings), strings.Join(issues, ", "))
	ed.showMessage(msg, MsgWarning)
}

func (ed *Editor) runValidate() {
	err := ed.fsm.Validate()
	if err == nil {
		ed.showMessage("✓ FSM is valid", MsgInfo)
	} else {
		ed.showMessage("✗ "+err.Error(), MsgError)
	}
}
