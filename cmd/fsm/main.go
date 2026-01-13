// Command fsm is a CLI tool for working with finite state machines.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

const usage = `fsm - Finite State Machine toolkit

Usage:
  fsm <command> [options]

Commands:
  convert    Convert between formats (json, hex, fsm)
  dot        Generate Graphviz DOT output
  info       Show FSM information
  run        Run FSM interactively
  validate   Validate FSM file

Examples:
  fsm convert input.json -o output.fsm
  fsm convert input.fsm -o output.json --pretty
  fsm dot input.fsm | dot -Tpng -o output.png
  fsm run input.fsm
  fsm info input.fsm

Use "fsm <command> -h" for more information about a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "convert":
		cmdConvert(args)
	case "dot":
		cmdDot(args)
	case "info":
		cmdInfo(args)
	case "run":
		cmdRun(args)
	case "validate":
		cmdValidate(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}

func cmdConvert(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm convert <input> [-o output] [--pretty] [--no-labels]")
		os.Exit(1)
	}

	input := args[0]
	var output string
	pretty := false
	noLabels := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "--pretty":
			pretty = true
		case "--no-labels":
			noLabels = true
		}
	}

	// Load input
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Determine output format
	if output == "" {
		// Default: change extension
		ext := filepath.Ext(input)
		base := strings.TrimSuffix(input, ext)
		switch ext {
		case ".json":
			output = base + ".fsm"
		case ".fsm", ".hex":
			output = base + ".json"
		default:
			output = base + ".fsm"
		}
	}

	// Write output
	outExt := filepath.Ext(output)
	switch outExt {
	case ".fsm":
		err = fsmfile.WriteFSMFile(output, f, !noLabels)
	case ".json":
		data, jerr := fsmfile.ToJSON(f, pretty)
		if jerr != nil {
			err = jerr
		} else {
			err = os.WriteFile(output, data, 0644)
		}
	case ".hex":
		records, _, _, _ := fsmfile.FSMToRecords(f)
		hex := fsmfile.FormatHex(records, 4)
		err = os.WriteFile(output, []byte(hex+"\n"), 0644)
	default:
		fmt.Fprintf(os.Stderr, "Unknown output format: %s\n", outExt)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
		os.Exit(1)
	}

	fmt.Printf("Written: %s\n", output)
}

func cmdDot(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm dot <input> [-o output] [-t title]")
		os.Exit(1)
	}

	input := args[0]
	var output, title string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-t", "--title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		}
	}

	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	if title == "" {
		if f.Name != "" {
			title = f.Name
		} else {
			title = fmt.Sprintf("%s: %d states", strings.ToUpper(string(f.Type)), len(f.States))
		}
	}

	dot := fsmfile.GenerateDOT(f, title)

	if output != "" {
		err = os.WriteFile(output, []byte(dot), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
			os.Exit(1)
		}
	} else {
		fmt.Print(dot)
	}
}

func cmdInfo(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm info <input>")
		os.Exit(1)
	}

	input := args[0]
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	fmt.Printf("Type:        %s\n", f.Type)
	if f.Name != "" {
		fmt.Printf("Name:        %s\n", f.Name)
	}
	if f.Description != "" {
		fmt.Printf("Description: %s\n", f.Description)
	}
	fmt.Printf("States:      %d\n", len(f.States))
	fmt.Printf("Inputs:      %d\n", len(f.Alphabet))
	if len(f.OutputAlphabet) > 0 {
		fmt.Printf("Outputs:     %d\n", len(f.OutputAlphabet))
	}
	fmt.Printf("Transitions: %d\n", len(f.Transitions))
	fmt.Printf("Initial:     %s\n", f.Initial)
	if len(f.Accepting) > 0 {
		fmt.Printf("Accepting:   %v\n", f.Accepting)
	}
	fmt.Println()
	fmt.Printf("States:      %v\n", f.States)
	fmt.Printf("Alphabet:    %v\n", f.Alphabet)
	if len(f.OutputAlphabet) > 0 {
		fmt.Printf("Outputs:     %v\n", f.OutputAlphabet)
	}
}

func cmdValidate(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm validate <input>")
		os.Exit(1)
	}

	input := args[0]
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	err = f.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: valid %s with %d states, %d transitions\n",
		input, f.Type, len(f.States), len(f.Transitions))
}

func cmdRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm run <input>")
		os.Exit(1)
	}

	input := args[0]
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	runner, err := fsm.NewRunner(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating runner: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("FSM: %s (%s)\n", f.Name, f.Type)
	fmt.Printf("Commands: <input>, reset, status, history, inputs, quit\n")
	fmt.Println()

	printStatus(runner, f)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "" {
			continue
		}

		switch cmd {
		case "quit", "exit", "q":
			return
		case "reset":
			runner.Reset()
			fmt.Println("Reset to initial state")
			printStatus(runner, f)
		case "status":
			printStatus(runner, f)
		case "history":
			printHistory(runner)
		case "inputs":
			inputs := runner.AvailableInputs()
			if len(inputs) == 0 {
				fmt.Println("No inputs available from current state")
			} else {
				fmt.Printf("Available inputs: %v\n", inputs)
			}
		case "help", "?":
			fmt.Println("Commands:")
			fmt.Println("  <input>  - Send input to FSM")
			fmt.Println("  reset    - Reset to initial state")
			fmt.Println("  status   - Show current status")
			fmt.Println("  history  - Show execution history")
			fmt.Println("  inputs   - Show available inputs")
			fmt.Println("  quit     - Exit")
		default:
			// Treat as input
			output, err := runner.Step(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			if output != "" {
				fmt.Printf("Output: %s\n", output)
			}
			printStatus(runner, f)
		}
	}
}

func printStatus(r *fsm.Runner, f *fsm.FSM) {
	status := fmt.Sprintf("State: %s", r.CurrentState())
	if r.IsAccepting() {
		status += " [accepting]"
	}
	if f.Type == fsm.TypeMoore {
		if out := r.CurrentOutput(); out != "" {
			status += fmt.Sprintf(" -> %s", out)
		}
	}
	fmt.Println(status)
}

func printHistory(r *fsm.Runner) {
	history := r.History()
	if len(history) == 0 {
		fmt.Println("No history yet")
		return
	}

	fmt.Println("History:")
	for i, step := range history {
		line := fmt.Sprintf("  %d: %s --%s--> %s",
			i+1, step.FromState, step.Input, step.ToState)
		if step.Output != "" {
			line += fmt.Sprintf(" [%s]", step.Output)
		}
		fmt.Println(line)
	}
}

func loadFSM(path string) (*fsm.FSM, error) {
	ext := filepath.Ext(path)

	switch ext {
	case ".fsm":
		return fsmfile.ReadFSMFile(path)
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return fsmfile.ParseJSON(data)
	case ".hex":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		records, err := fsmfile.ParseHex(string(data))
		if err != nil {
			return nil, err
		}
		return fsmfile.RecordsToFSM(records, nil)
	default:
		return nil, fmt.Errorf("unknown file format: %s", ext)
	}
}
