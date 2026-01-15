// Command fsm is a CLI tool for working with finite state machines.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/codegen"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

const usage = `fsm - Finite State Machine toolkit

Usage:
  fsm <command> [options]

Commands:
  convert    Convert between formats (json, hex, fsm)
  dot        Generate Graphviz DOT output
  png        Generate PNG image (requires Graphviz)
  svg        Generate SVG image (requires Graphviz)
  generate   Generate code (C, Rust, Go/TinyGo)
  info       Show FSM information
  analyse    Analyse FSM for potential issues
  run        Run FSM interactively
  validate   Validate FSM file
  view       Visualise FSM (generates PNG and opens it)
  edit       Open visual editor (invokes fsmedit)

Examples:
  fsm convert input.json -o output.fsm
  fsm dot input.fsm | dot -Tpng -o output.png
  fsm png input.fsm -o diagram.png
  fsm svg input.fsm -o diagram.svg
  fsm generate input.fsm --lang c -o fsm.h
  fsm analyse input.fsm
  fsm view input.fsm
  fsm edit input.fsm
  fsm run input.fsm

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
	case "png":
		cmdImage(args, "png")
	case "svg":
		cmdImage(args, "svg")
	case "generate":
		cmdGenerate(args)
	case "info":
		cmdInfo(args)
	case "analyse", "analyze":
		cmdAnalyse(args)
	case "run":
		cmdRun(args)
	case "validate":
		cmdValidate(args)
	case "view":
		cmdView(args)
	case "edit":
		cmdEdit(args)
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

func cmdImage(args []string, format string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: fsm %s <input> [-o output] [-t title]\n", format)
		os.Exit(1)
	}

	// Check for help flag
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Printf("Usage: fsm %s <input> [-o output] [-t title]\n", format)
		fmt.Println("")
		fmt.Printf("Generates a %s image from the FSM.\n", strings.ToUpper(format))
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -o, --output    Output file (default: input name with new extension)")
		fmt.Println("  -t, --title     Set diagram title (default: FSM name or type)")
		fmt.Println("")
		fmt.Println("Requires Graphviz 'dot' to be installed:")
		fmt.Println("  https://graphviz.org/download/")
		return
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

	// Default output filename
	if output == "" {
		base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		output = base + "." + format
	}

	// Check if dot is available
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Graphviz 'dot' command not found in PATH.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please install Graphviz from: https://graphviz.org/download/")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Installation:")
		fmt.Fprintln(os.Stderr, "  macOS:   brew install graphviz")
		fmt.Fprintln(os.Stderr, "  Ubuntu:  sudo apt install graphviz")
		fmt.Fprintln(os.Stderr, "  Windows: choco install graphviz")
		os.Exit(1)
	}

	// Load FSM
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Generate title
	if title == "" {
		if f.Name != "" {
			title = f.Name
		} else {
			title = fmt.Sprintf("%s: %d states", strings.ToUpper(string(f.Type)), len(f.States))
		}
	}

	// Generate DOT
	dot := fsmfile.GenerateDOT(f, title)

	// Run dot to generate image
	cmd := exec.Command(dotPath, "-T"+format)
	cmd.Stdin = strings.NewReader(dot)

	outFile, err := os.Create(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", output, err)
		os.Exit(1)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running dot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", output)
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

func cmdAnalyse(args []string) {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm analyse <input>")
		fmt.Println("")
		fmt.Println("Analyse FSM for potential issues:")
		fmt.Println("  - Unreachable states (not reachable from initial)")
		fmt.Println("  - Dead states (no outgoing transitions, not accepting)")
		fmt.Println("  - Non-determinism in DFA (multiple transitions on same input)")
		fmt.Println("  - Incomplete DFA (missing transitions for some inputs)")
		fmt.Println("  - Unused inputs (defined but never used)")
		fmt.Println("  - Unused outputs (defined but never used)")
		if len(args) < 1 {
			os.Exit(1)
		}
		return
	}

	input := args[0]
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	warnings := f.Analyse()

	if len(warnings) == 0 {
		fmt.Println("No issues found.")
		return
	}

	fmt.Printf("Found %d issue(s):\n\n", len(warnings))
	for _, w := range warnings {
		fmt.Printf("  [%s] %s\n", w.Type, w.Message)
		if len(w.States) > 0 {
			fmt.Printf("    States: %v\n", w.States)
		}
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

func cmdView(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm view <input> [-t title]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Generates a PNG visualisation and opens it with the system viewer.")
		fmt.Fprintln(os.Stderr, "Requires Graphviz 'dot' to be installed.")
		os.Exit(1)
	}

	// Check for help flag
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm view <input> [-t title]")
		fmt.Println("")
		fmt.Println("Generates a PNG visualisation of the FSM and opens it with the")
		fmt.Println("system's default image viewer.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -t, --title    Set diagram title (default: FSM name or type)")
		fmt.Println("")
		fmt.Println("Requires Graphviz 'dot' to be installed:")
		fmt.Println("  https://graphviz.org/download/")
		return
	}

	input := args[0]
	var title string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-t", "--title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		}
	}

	// Check if dot is available
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Graphviz 'dot' command not found in PATH.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please install Graphviz from: https://graphviz.org/download/")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Installation:")
		fmt.Fprintln(os.Stderr, "  macOS:   brew install graphviz")
		fmt.Fprintln(os.Stderr, "  Ubuntu:  sudo apt install graphviz")
		fmt.Fprintln(os.Stderr, "  Windows: choco install graphviz")
		os.Exit(1)
	}

	// Load FSM
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Generate title
	if title == "" {
		if f.Name != "" {
			title = f.Name
		} else {
			title = fmt.Sprintf("%s: %d states", strings.ToUpper(string(f.Type)), len(f.States))
		}
	}

	// Generate DOT
	dot := fsmfile.GenerateDOT(f, title)

	// Create temp files
	baseName := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	dotFile := filepath.Join(os.TempDir(), baseName+".dot")
	pngFile := filepath.Join(os.TempDir(), baseName+".png")

	// Write DOT file
	if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing DOT file: %v\n", err)
		os.Exit(1)
	}

	// Run dot to generate PNG
	cmd := exec.Command(dotPath, "-Tpng", dotFile, "-o", pngFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running dot: %v\n%s\n", err, output)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", pngFile)

	// Open with system viewer
	if err := openFile(pngFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening viewer: %v\n", err)
		fmt.Fprintf(os.Stderr, "PNG file available at: %s\n", pngFile)
		os.Exit(1)
	}
}

func cmdEdit(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Println("Usage: fsm edit [file]")
		fmt.Println("")
		fmt.Println("Open the visual FSM editor (fsmedit).")
		fmt.Println("")
		fmt.Println("Searches for fsmedit in:")
		fmt.Println("  1. PATH")
		fmt.Println("  2. Current working directory")
		fmt.Println("  3. Same directory as fsm executable")
		return
	}

	// Find fsmedit executable
	editorPath := findEditor()
	if editorPath == "" {
		fmt.Fprintln(os.Stderr, "Error: fsmedit not found.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Build it with: go build -o fsmedit ./cmd/fsmedit/")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Searched in:")
		fmt.Fprintln(os.Stderr, "  - PATH")
		fmt.Fprintln(os.Stderr, "  - Current working directory")
		fmt.Fprintln(os.Stderr, "  - Same directory as fsm executable")
		os.Exit(1)
	}

	// Build command with args
	cmd := exec.Command(editorPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error running fsmedit: %v\n", err)
		os.Exit(1)
	}
}

// findEditor searches for fsmedit in PATH, pwd, and fsm's directory
func findEditor() string {
	editorName := "fsmedit"
	if runtime.GOOS == "windows" {
		editorName = "fsmedit.exe"
	}

	// 1. Check PATH
	if path, err := exec.LookPath(editorName); err == nil {
		return path
	}

	// 2. Check current working directory
	if pwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(pwd, editorName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	// 3. Check same directory as fsm executable
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidate := filepath.Join(exeDir, editorName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}

func cmdGenerate(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm generate <input> --lang <c|rust|go|tinygo> [-o output] [--package name]")
		os.Exit(1)
	}

	// Check for help flag
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm generate <input> --lang <language> [-o output] [--package name]")
		fmt.Println("")
		fmt.Println("Generates code from FSM definition.")
		fmt.Println("")
		fmt.Println("Languages:")
		fmt.Println("  c        C with header-only implementation")
		fmt.Println("  rust     Rust module")
		fmt.Println("  go       Go package (also works with TinyGo)")
		fmt.Println("  tinygo   Alias for go")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --lang, -l      Target language (required)")
		fmt.Println("  -o, --output    Output file (default: stdout)")
		fmt.Println("  --package, -p   Package name (Go only, default: fsm)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  fsm generate machine.fsm --lang c -o machine.h")
		fmt.Println("  fsm generate machine.fsm --lang rust -o machine.rs")
		fmt.Println("  fsm generate machine.fsm --lang go --package myfsm -o myfsm.go")
		return
	}

	input := args[0]
	var output, lang, packageName string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-l", "--lang":
			if i+1 < len(args) {
				lang = strings.ToLower(args[i+1])
				i++
			}
		case "-p", "--package":
			if i+1 < len(args) {
				packageName = args[i+1]
				i++
			}
		}
	}

	if lang == "" {
		fmt.Fprintln(os.Stderr, "Error: --lang is required")
		fmt.Fprintln(os.Stderr, "Use: fsm generate --help")
		os.Exit(1)
	}

	// Load FSM
	f, err := loadFSM(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Generate code
	var code string
	switch lang {
	case "c":
		code = codegen.GenerateC(f)
	case "rust":
		code = codegen.GenerateRust(f)
	case "go", "tinygo":
		code = codegen.GenerateGo(f, packageName)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown language: %s\n", lang)
		fmt.Fprintln(os.Stderr, "Supported: c, rust, go, tinygo")
		os.Exit(1)
	}

	// Output
	if output != "" {
		err := os.WriteFile(output, []byte(code), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
			os.Exit(1)
		}
		fmt.Printf("Generated: %s\n", output)
	} else {
		fmt.Print(code)
	}
}

// openFile opens a file with the system's default application.
func openFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("explorer.exe", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
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
