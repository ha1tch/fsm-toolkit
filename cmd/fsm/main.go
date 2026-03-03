// Command fsm is a CLI tool for working with finite state machines.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/codegen"
	"github.com/ha1tch/fsm-toolkit/pkg/export"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
	"github.com/ha1tch/fsm-toolkit/pkg/version"
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
  machines   List machines in a bundle
  analyse    Analyse FSM for potential issues (alias: analyze)
  run        Run FSM interactively
  validate   Validate FSM file
  view       Visualise FSM (generates PNG and opens it)
  edit       Open visual editor (invokes fsmedit)
  bundle     Create bundle from multiple FSM files
  extract    Extract machine from bundle
  netlist    Export structural netlist (text, kicad, json)
  properties Query state class assignments and property values

Examples:
  fsm convert input.json -o output.fsm
  fsm dot input.fsm | dot -Tpng -o output.png
  fsm png input.fsm -o diagram.png
  fsm svg input.fsm -o diagram.svg
  fsm generate input.fsm --lang c -o fsm.h
  fsm generate bundle.fsm --all --lang go
  fsm analyse input.fsm
  fsm analyze bundle.fsm --all
  fsm view input.fsm
  fsm edit input.fsm
  fsm run input.fsm
  fsm machines bundle.fsm
  fsm info bundle.fsm --machine pedestrian
  fsm bundle main.fsm child.fsm -o combined.fsm
  fsm extract bundle.fsm --machine child -o child.fsm
  fsm netlist circuit.json --format kicad -o circuit.net

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
	case "machines":
		cmdMachines(args)
	case "bundle":
		cmdBundle(args)
	case "extract":
		cmdExtract(args)
	case "analyse", "analyze":
		cmdAnalyse(args)
	case "run":
		cmdRun(args)
	case "validate":
		cmdValidate(args)
	case "netlist":
		cmdNetlist(args)
	case "properties":
		cmdProperties(args)
	case "view":
		cmdView(args)
	case "edit":
		cmdEdit(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-v", "--version", "version":
		fmt.Printf("fsm %s\n", version.Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}

func cmdConvert(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm convert <input>... [-o output] [--pretty] [--no-labels]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Supports wildcards: fsm convert *.json -o .fsm")
		fmt.Fprintln(os.Stderr, "When converting multiple files, -o specifies the output extension")
		os.Exit(1)
	}

	var inputs []string
	var outputSpec string
	pretty := false
	noLabels := false

	// Parse arguments
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 < len(args) {
				outputSpec = args[i+1]
				i++
			}
		case "--pretty":
			pretty = true
		case "--no-labels":
			noLabels = true
		default:
			// Expand wildcards
			matches, err := filepath.Glob(args[i])
			if err != nil || len(matches) == 0 {
				// Not a glob or no matches - use as-is
				inputs = append(inputs, args[i])
			} else {
				inputs = append(inputs, matches...)
			}
		}
	}

	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "No input files specified")
		os.Exit(1)
	}

	// Process each input file
	for _, input := range inputs {
		output := outputSpec

		// Determine output filename
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
		} else if strings.HasPrefix(output, ".") {
			// Output is just an extension - apply to input basename
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(input, ext)
			output = base + outputSpec
		}
		// else: output is a full filename (only valid for single input)

		// Load input
		f, err := loadFSM(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
			continue
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
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
			continue
		}

		fmt.Printf("Converted: %s -> %s\n", input, output)
	}
}

func cmdDot(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm dot <input> [-o output] [-t title] [-m machine]")
		os.Exit(1)
	}

	input := args[0]
	var output, title, machineName string

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
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		}
	}

	f, err := loadFSMWithMachine(input, machineName)
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
		fmt.Printf("Usage: fsm %s <input> [-o output] [-t title] [--native] [native options...]\n", format)
		fmt.Println("")
		fmt.Printf("Generates a %s image from the FSM.\n", strings.ToUpper(format))
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -o, --output    Output file (default: input name with new extension)")
		fmt.Println("  -t, --title     Set diagram title (default: FSM name or type)")
		fmt.Println("  -m, --machine   Select machine from bundle")
		fmt.Println("  --all           Render all machines in bundle (tiled output)")
		fmt.Println("  --native        Use built-in renderer (no Graphviz required)")
		fmt.Println("")
		fmt.Println("Native renderer options (only with --native):")
		fmt.Println("  --font-size N   Base font size in pixels (default: 14)")
		fmt.Println("  --spacing N     Node spacing multiplier (default: 1.5)")
		fmt.Println("  --width N       Canvas width in pixels (default: 800)")
		fmt.Println("  --height N      Canvas height in pixels (default: 600)")
		if format == "svg" {
			fmt.Println("  --shape SHAPE   State shape: circle, ellipse, rect, roundrect, diamond")
		}
		fmt.Println("")
		fmt.Println("Without --native, requires Graphviz 'dot' to be installed:")
		fmt.Println("  https://graphviz.org/download/")
		return
	}

	input := args[0]
	var output, title, machineName string
	native := false
	renderAll := false
	fontSize := 0
	shape := ""
	spacing := 0.0
	canvasWidth := 0
	canvasHeight := 0

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
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--all":
			renderAll = true
		case "--native":
			native = true
		case "--font-size":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &fontSize)
				i++
			}
		case "--shape":
			if i+1 < len(args) {
				shape = strings.ToLower(args[i+1])
				i++
			}
		case "--spacing":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%f", &spacing)
				i++
			}
		case "--width":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &canvasWidth)
				i++
			}
		case "--height":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &canvasHeight)
				i++
			}
		}
	}

	// Handle --all flag for bundles
	if renderAll && filepath.Ext(input) == ".fsm" {
		renderAllMachines(input, output, format, native, fontSize, spacing, canvasWidth, canvasHeight, shape)
		return
	}

	// Default output filename
	if output == "" {
		base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		if machineName != "" {
			base = machineName
		}
		output = base + "." + format
	}

	// Load FSM first
	f, err := loadFSMWithMachine(input, machineName)
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

	// Native SVG rendering (no Graphviz needed)
	if native {
		if format == "svg" {
			opts := fsmfile.DefaultSVGOptions()
			opts.Title = title
			
			// Apply custom options
			if fontSize > 0 {
				opts.FontSize = fontSize
			}
			if spacing > 0 {
				opts.NodeSpacing = spacing
			}
			if canvasWidth > 0 {
				opts.Width = canvasWidth
			}
			if canvasHeight > 0 {
				opts.Height = canvasHeight
			}
			
			// Parse shape option
			switch shape {
			case "circle":
				opts.StateShape = fsmfile.ShapeCircle
			case "ellipse":
				opts.StateShape = fsmfile.ShapeEllipse
			case "rect", "rectangle":
				opts.StateShape = fsmfile.ShapeRect
			case "roundrect", "rounded":
				opts.StateShape = fsmfile.ShapeRoundRect
			case "diamond":
				opts.StateShape = fsmfile.ShapeDiamond
			}
			
			svg := fsmfile.GenerateSVGNative(f, opts)

			if err := os.WriteFile(output, []byte(svg), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
				os.Exit(1)
			}
			fmt.Printf("Generated: %s (native)\n", output)
			return
		} else if format == "png" {
			opts := fsmfile.DefaultPNGOptions()
			opts.Title = title
			
			// Apply custom options
			if fontSize > 0 {
				opts.FontSize = fontSize
			}
			if spacing > 0 {
				opts.NodeSpacing = spacing
			}
			if canvasWidth > 0 {
				opts.Width = canvasWidth
			}
			if canvasHeight > 0 {
				opts.Height = canvasHeight
			}
			
			outFile, err := os.Create(output)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", output, err)
				os.Exit(1)
			}
			defer outFile.Close()
			
			if err := fsmfile.RenderPNG(f, outFile, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error rendering PNG: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Generated: %s (native)\n", output)
			return
		}
	}

	// Check if dot is available
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Graphviz 'dot' command not found in PATH.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Tip: Use --native flag for built-in rendering without Graphviz:")
		fmt.Fprintf(os.Stderr, "  fsm %s %s --native\n", format, input)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or install Graphviz from: https://graphviz.org/download/")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Installation:")
		fmt.Fprintln(os.Stderr, "  macOS:   brew install graphviz")
		fmt.Fprintln(os.Stderr, "  Ubuntu:  sudo apt install graphviz")
		fmt.Fprintln(os.Stderr, "  Windows: choco install graphviz")
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "Usage: fsm info <input> [--machine <name>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "For bundles, use --machine to select a specific machine.")
		os.Exit(1)
	}

	var input, machineName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--machine", "-m":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "Error: input file required")
		os.Exit(1)
	}

	// Bundle detection: show summary when no --machine specified
	if machineName == "" && filepath.Ext(input) == ".fsm" {
		if isBundle, _ := fsmfile.IsBundle(input); isBundle {
			machines, err := fsmfile.ListMachines(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", input, err)
				os.Exit(1)
			}
			fmt.Printf("Bundle:      %s (%d machines)\n", filepath.Base(input), len(machines))
			fmt.Printf("Machines:    ")
			for i, m := range machines {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", m.Name)
			}
			fmt.Println()
			fmt.Println()
			fmt.Printf("Showing: %s (use -m to select a machine, or 'fsm machines' to list all)\n\n", machines[0].Name)
		}
	}

	f, err := loadFSMWithMachine(input, machineName)
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
	v := f.Vocab()
	fmt.Printf("%-12s %d\n", v.States+":", len(f.States))
	fmt.Printf("%-12s %d\n", v.Alphabet+":", len(f.Alphabet))
	if len(f.OutputAlphabet) > 0 {
		fmt.Printf("%-12s %d\n", v.Output+"s:", len(f.OutputAlphabet))
	}
	fmt.Printf("%-12s %d\n", v.Transition+"s:", len(f.Transitions))
	fmt.Printf("%-12s %s\n", v.Initial+":", f.Initial)
	if len(f.Accepting) > 0 {
		fmt.Printf("%-12s %v\n", v.Accepting+":", f.Accepting)
	}
	
	// Display linked states
	if f.HasLinkedStates() {
		fmt.Println()
		fmt.Printf("Linked %s:\n", v.States)
		for state, machine := range f.LinkedMachines {
			if machine != "" {
				fmt.Printf("  %s → %s\n", state, machine)
			} else {
				fmt.Printf("  %s → (unspecified)\n", state)
			}
		}
	}

	// Display class information
	userClasses := 0
	for name := range f.Classes {
		if name != "default_state" {
			userClasses++
		}
	}
	if userClasses > 0 || len(f.StateClasses) > 0 {
		fmt.Println()
		fmt.Printf("Classes:     %d\n", userClasses)
		for name, cls := range f.Classes {
			if name == "default_state" {
				continue
			}
			fmt.Printf("  %s (%d properties)\n", name, len(cls.Properties))
		}
		if len(f.StateClasses) > 0 {
			fmt.Println()
			fmt.Printf("Class Assignments (%s → Class):\n", v.State)
			for state, class := range f.StateClasses {
				fmt.Printf("  %s → %s\n", state, class)
			}
		}
	}

	// Display net information
	if f.HasNets() {
		fmt.Println()
		signalNets := f.SignalNets()
		powerNets := len(f.Nets) - len(signalNets)
		fmt.Printf("Nets:        %d", len(f.Nets))
		if powerNets > 0 {
			fmt.Printf(" (%d signal, %d power)", len(signalNets), powerNets)
		}
		fmt.Println()
		for _, n := range f.Nets {
			var eps []string
			for _, ep := range n.Endpoints {
				eps = append(eps, ep.Instance+"."+ep.Port)
			}
			tag := ""
			if f.IsPowerNet(n) {
				tag = " [power]"
			}
			fmt.Printf("  %s: %s%s\n", n.Name, strings.Join(eps, ", "), tag)
		}
	}
	
	fmt.Println()
	fmt.Printf("%-12s %v\n", v.States+":", f.States)
	fmt.Printf("%-12s %v\n", v.Alphabet+":", f.Alphabet)
	if len(f.OutputAlphabet) > 0 {
		fmt.Printf("%-12s %v\n", v.Output+"s:", f.OutputAlphabet)
	}
}

func cmdAnalyse(args []string) {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm analyse <input> [-m machine] [--all]")
		fmt.Println("       fsm analyze <input> [-m machine] [--all]")
		fmt.Println("")
		fmt.Println("Analyse FSM for potential issues:")
		fmt.Println("  - Unreachable states (not reachable from initial)")
		fmt.Println("  - Dead states (no outgoing transitions, not accepting)")
		fmt.Println("  - Non-determinism in DFA (multiple transitions on same input)")
		fmt.Println("  - Incomplete DFA (missing transitions for some inputs)")
		fmt.Println("  - Unused inputs (defined but never used)")
		fmt.Println("  - Unused outputs (defined but never used)")
		fmt.Println("")
		fmt.Println("Bundle analysis (--all) also checks:")
		fmt.Println("  - Cross-machine alphabet conflicts")
		fmt.Println("  - Orphaned machines (not linked from any other)")
		fmt.Println("  - Missing linked machine targets")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -m, --machine   Select machine from bundle")
		fmt.Println("  --all           Analyse all machines in bundle")
		if len(args) < 1 {
			os.Exit(1)
		}
		return
	}

	var input, machineName string
	var analyseAll bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--all":
			analyseAll = true
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	// Handle --all for bundles
	if analyseAll {
		analyseAllMachines(input)
		return
	}

	f, err := loadFSMWithMachine(input, machineName)
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
		if len(w.Symbols) > 0 {
			fmt.Printf("    Symbols: %v\n", w.Symbols)
		}
	}
}

// analyseAllMachines analyses all machines in a bundle plus cross-machine issues
func analyseAllMachines(input string) {
	// Check if it's a bundle
	isBundle, err := fsmfile.IsBundle(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", input, err)
		os.Exit(1)
	}
	if !isBundle {
		fmt.Fprintln(os.Stderr, "Error: --all requires a bundle file with multiple machines")
		os.Exit(1)
	}

	// List all machines
	machines, err := fsmfile.ListMachines(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing machines: %v\n", err)
		os.Exit(1)
	}

	// Load all FSMs
	fsms := make(map[string]*fsm.FSM)
	for _, m := range machines {
		f, _, err := fsmfile.ReadMachineFromBundle(input, m.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading machine %s: %v\n", m.Name, err)
			continue
		}
		fsms[m.Name] = f
	}

	totalIssues := 0

	// Analyse each machine individually
	for _, m := range machines {
		f := fsms[m.Name]
		if f == nil {
			continue
		}

		warnings := f.Analyse()
		if len(warnings) > 0 {
			fmt.Printf("=== %s ===\n", m.Name)
			for _, w := range warnings {
				fmt.Printf("  [%s] %s\n", w.Type, w.Message)
				if len(w.States) > 0 {
					fmt.Printf("    States: %v\n", w.States)
				}
				if len(w.Symbols) > 0 {
					fmt.Printf("    Symbols: %v\n", w.Symbols)
				}
			}
			fmt.Println()
			totalIssues += len(warnings)
		}
	}

	// Cross-machine analysis
	crossIssues := analyseCrossMachine(fsms)
	if len(crossIssues) > 0 {
		fmt.Println("=== Cross-Machine Issues ===")
		for _, issue := range crossIssues {
			fmt.Printf("  %s\n", issue)
		}
		fmt.Println()
		totalIssues += len(crossIssues)
	}

	// Summary
	if totalIssues == 0 {
		fmt.Printf("No issues found in %d machines.\n", len(machines))
	} else {
		fmt.Printf("Total: %d issue(s) across %d machines.\n", totalIssues, len(machines))
	}
}

// analyseCrossMachine checks for issues across machines in a bundle
func analyseCrossMachine(fsms map[string]*fsm.FSM) []string {
	var issues []string

	// Track which machines are linked to
	linkedTargets := make(map[string]bool)
	for name, f := range fsms {
		for state, target := range f.LinkedMachines {
			linkedTargets[target] = true
			// Check target exists
			if _, ok := fsms[target]; !ok {
				issues = append(issues, fmt.Sprintf("[MISSING_TARGET] %s: state '%s' links to non-existent machine '%s'", name, state, target))
			}
		}
	}

	// Check for orphaned machines (not the first/main and not linked to)
	machineNames := make([]string, 0, len(fsms))
	for name := range fsms {
		machineNames = append(machineNames, name)
	}
	sort.Strings(machineNames)

	if len(machineNames) > 1 {
		mainMachine := machineNames[0] // Assume first alphabetically is main (or could be explicitly marked)
		for _, name := range machineNames {
			if name != mainMachine && !linkedTargets[name] {
				issues = append(issues, fmt.Sprintf("[ORPHAN] Machine '%s' is not linked from any other machine", name))
			}
		}
	}

	// Check for alphabet conflicts (same input symbol with different semantics might be intentional,
	// but worth flagging if alphabets are very different)
	// This is informational - different machines can have different alphabets
	
	// Check for accept/reject handling in parent machines
	for name, f := range fsms {
		if len(f.LinkedMachines) > 0 {
			// Check if accept/reject are in the alphabet
			hasAccept := false
			hasReject := false
			for _, inp := range f.Alphabet {
				if inp == "accept" {
					hasAccept = true
				}
				if inp == "reject" {
					hasReject = true
				}
			}
			if !hasAccept {
				issues = append(issues, fmt.Sprintf("[MISSING_ACCEPT] %s: has linked states but no 'accept' input defined", name))
			}
			if !hasReject {
				issues = append(issues, fmt.Sprintf("[MISSING_REJECT] %s: has linked states but no 'reject' input defined", name))
			}
		}
	}

	return issues
}

func cmdValidate(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm validate <input> [-m machine] [--bundle]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -m, --machine   Select machine from bundle")
		fmt.Fprintln(os.Stderr, "  --bundle        Validate linked state references across bundle")
		os.Exit(1)
	}

	var input, machineName string
	var validateBundle bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--bundle":
			validateBundle = true
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	// Bundle validation mode
	if validateBundle {
		result, err := fsmfile.ValidateBundleLinks(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error validating bundle: %v\n", err)
			os.Exit(1)
		}
		
		if len(result.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  ⚠ %s\n", w)
			}
			fmt.Println()
		}
		
		if len(result.Errors) > 0 {
			fmt.Println("Errors:")
			for _, e := range result.Errors {
				fmt.Printf("  ✗ %s\n", e)
			}
			fmt.Println()
		}
		
		if result.Valid {
			fmt.Printf("%s: bundle links valid\n", input)
		} else {
			fmt.Fprintf(os.Stderr, "%s: bundle validation failed\n", input)
			os.Exit(1)
		}
		return
	}

	f, err := loadFSMWithMachine(input, machineName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	err = f.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	v := f.Vocab()
	fmt.Printf("%s: valid %s with %d %s, %d %s\n",
		input, f.Type, len(f.States), strings.ToLower(v.States), len(f.Transitions), strings.ToLower(v.Transition)+"s")
}

func cmdRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm run <input> [-m machine]")
		os.Exit(1)
	}

	var input, machineName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	// Check if this is a bundle with linked states
	isBundle, _ := fsmfile.IsBundle(input)
	if isBundle {
		runBundle(input, machineName)
		return
	}

	f, err := loadFSMWithMachine(input, machineName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Check if single machine has linked states (warn user)
	if f.HasLinkedStates() {
		fmt.Println("Warning: This FSM has linked states but is not in a bundle.")
		fmt.Println("Linked state delegation will not work. Use 'fsm bundle' to create a bundle.")
		fmt.Println()
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

// runBundle runs a bundle with linked state support.
func runBundle(path, mainMachine string) {
	// Load all machines from bundle
	machines, err := fsmfile.ListMachines(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing machines: %v\n", err)
		os.Exit(1)
	}

	if len(machines) == 0 {
		fmt.Fprintf(os.Stderr, "No machines found in bundle\n")
		os.Exit(1)
	}

	// If no main specified, use first machine
	if mainMachine == "" {
		mainMachine = machines[0].Name
	}

	// Load all FSMs
	fsmMap := make(map[string]*fsm.FSM)
	for _, m := range machines {
		f, _, err := fsmfile.ReadMachineFromBundle(path, m.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading machine %s: %v\n", m.Name, err)
			os.Exit(1)
		}
		fsmMap[m.Name] = f
	}

	// Check if any machine has linked states
	hasLinks := false
	for _, f := range fsmMap {
		if f.HasLinkedStates() {
			hasLinks = true
			break
		}
	}

	// Create bundle runner
	bundleRunner, err := fsm.NewBundleRunner(fsmMap, mainMachine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating bundle runner: %v\n", err)
		os.Exit(1)
	}

	mainFSM := fsmMap[mainMachine]
	fmt.Printf("Bundle: %s (%d machines)\n", path, len(machines))
	fmt.Printf("Main: %s (%s)\n", mainMachine, mainFSM.Type)
	if hasLinks {
		fmt.Println("Linked states enabled - delegation prompt shows as >>")
	}
	fmt.Printf("Commands: <input>, reset, status, history, inputs, machines, quit\n")
	fmt.Println()

	fmt.Println(bundleRunner.Status())

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(bundleRunner.Prompt())
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
			bundleRunner.Reset()
			fmt.Println("Reset to initial state")
			fmt.Println(bundleRunner.Status())
		case "status":
			fmt.Println(bundleRunner.Status())
		case "history":
			printBundleHistory(bundleRunner)
		case "inputs":
			inputs := bundleRunner.AvailableInputs()
			if len(inputs) == 0 {
				fmt.Println("No inputs available from current state")
			} else {
				fmt.Printf("Available inputs: %v\n", inputs)
			}
		case "machines":
			fmt.Printf("Active: %s (depth: %d)\n", bundleRunner.CurrentMachine(), bundleRunner.DelegationDepth())
			fmt.Println("All machines:")
			for name := range fsmMap {
				marker := "  "
				if name == bundleRunner.CurrentMachine() {
					marker = "→ "
				}
				fmt.Printf("%s%s\n", marker, name)
			}
		case "help", "?":
			fmt.Println("Commands:")
			fmt.Println("  <input>  - Send input to FSM")
			fmt.Println("  reset    - Reset to initial state")
			fmt.Println("  status   - Show current status")
			fmt.Println("  history  - Show execution history")
			fmt.Println("  inputs   - Show available inputs")
			fmt.Println("  machines - Show active machine info")
			fmt.Println("  quit     - Exit")
		default:
			// Treat as input
			output, err := bundleRunner.Step(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			if output != "" {
				fmt.Printf("Output: %s\n", output)
			}
			fmt.Println(bundleRunner.Status())
		}
	}
}

func printBundleHistory(br *fsm.BundleRunner) {
	history := br.History()
	if len(history) == 0 {
		fmt.Println("No history yet")
		return
	}

	fmt.Println("History:")
	for i, step := range history {
		var line string
		if step.Delegated {
			line = fmt.Sprintf("  %d: [%s] %s → delegated", i+1, step.Machine, step.FromState)
		} else if step.Returned {
			line = fmt.Sprintf("  %d: [%s] ← returned (%s)", i+1, step.Machine, step.Result)
		} else {
			line = fmt.Sprintf("  %d: [%s] %s --%s--> %s",
				i+1, step.Machine, step.FromState, step.Input, step.ToState)
			if step.Output != "" {
				line += fmt.Sprintf(" [%s]", step.Output)
			}
		}
		fmt.Println(line)
	}
}

func printStatus(r *fsm.Runner, f *fsm.FSM) {
	v := f.Vocab()
	status := fmt.Sprintf("%s: %s", v.State, r.CurrentState())
	if r.IsAccepting() {
		status += " [accepting]"
	}
	// Show outputs for Moore machines and NFAs with state outputs
	if f.Type == fsm.TypeMoore || (f.Type == fsm.TypeNFA && len(f.StateOutputs) > 0) {
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
		fmt.Fprintln(os.Stderr, "Usage: fsm generate <input> --lang <c|rust|go|tinygo> [-o output] [--package name] [-m machine] [--all]")
		os.Exit(1)
	}

	// Check for help flag
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm generate <input> --lang <language> [-o output] [--package name] [-m machine] [--all]")
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
		fmt.Println("  -m, --machine   Select machine from bundle")
		fmt.Println("  --all           Generate code for all machines in bundle")
		fmt.Println("                  Output files named: <machine>.<ext>")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  fsm generate machine.fsm --lang c -o machine.h")
		fmt.Println("  fsm generate machine.fsm --lang rust -o machine.rs")
		fmt.Println("  fsm generate machine.fsm --lang go --package myfsm -o myfsm.go")
		fmt.Println("  fsm generate bundle.fsm --machine child --lang c -o child.h")
		fmt.Println("  fsm generate bundle.fsm --all --lang go --package fsms")
		return
	}

	input := args[0]
	var output, lang, packageName, machineName string
	var generateAll bool

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
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--all":
			generateAll = true
		}
	}

	if lang == "" {
		fmt.Fprintln(os.Stderr, "Error: --lang is required")
		fmt.Fprintln(os.Stderr, "Use: fsm generate --help")
		os.Exit(1)
	}

	// Handle --all for bundles
	if generateAll {
		generateAllMachines(input, lang, packageName)
		return
	}

	// Load FSM
	f, err := loadFSMWithMachine(input, machineName)
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

// generateAllMachines generates code for all machines in a bundle
func generateAllMachines(input, lang, packageName string) {
	// Check if it's a bundle
	isBundle, err := fsmfile.IsBundle(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", input, err)
		os.Exit(1)
	}
	if !isBundle {
		fmt.Fprintln(os.Stderr, "Error: --all requires a bundle file with multiple machines")
		os.Exit(1)
	}

	// List all machines
	machines, err := fsmfile.ListMachines(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing machines: %v\n", err)
		os.Exit(1)
	}

	// Determine file extension
	var ext string
	switch lang {
	case "c":
		ext = ".h"
	case "rust":
		ext = ".rs"
	case "go", "tinygo":
		ext = ".go"
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown language: %s\n", lang)
		os.Exit(1)
	}

	// Generate code for each machine
	for _, m := range machines {
		f, _, err := fsmfile.ReadMachineFromBundle(input, m.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading machine %s: %v\n", m.Name, err)
			continue
		}

		var code string
		switch lang {
		case "c":
			code = codegen.GenerateC(f)
		case "rust":
			code = codegen.GenerateRust(f)
		case "go", "tinygo":
			// Use machine name as package if not specified
			pkg := packageName
			if pkg == "" {
				pkg = m.Name
			}
			code = codegen.GenerateGo(f, pkg)
		}

		outputFile := m.Name + ext
		if err := os.WriteFile(outputFile, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputFile, err)
			continue
		}
		fmt.Printf("Generated: %s\n", outputFile)
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

// loadFSMWithMachine loads an FSM, optionally selecting a specific machine from a bundle.
// If machineName is empty and the file is a bundle, loads the first machine.
func loadFSMWithMachine(path string, machineName string) (*fsm.FSM, error) {
	ext := filepath.Ext(path)

	if ext == ".fsm" {
		// Check if it's a bundle
		isBundle, err := fsmfile.IsBundle(path)
		if err != nil {
			return nil, err
		}
		
		if isBundle {
			// It's a bundle - need to select a machine
			if machineName != "" {
				// Load specific machine
				f, _, err := fsmfile.ReadMachineFromBundle(path, machineName)
				return f, err
			}
			
			// No machine specified - load the first one
			machines, err := fsmfile.ListMachines(path)
			if err != nil {
				return nil, err
			}
			if len(machines) == 0 {
				return nil, fmt.Errorf("bundle contains no machines")
			}
			
			f, _, err := fsmfile.ReadMachineFromBundle(path, machines[0].Name)
			return f, err
		}
	}

	return loadFSM(path)
}

func cmdMachines(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm machines <bundle.fsm>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Lists all machines contained in a bundle.")
		os.Exit(1)
	}

	input := args[0]
	
	if filepath.Ext(input) != ".fsm" {
		fmt.Fprintf(os.Stderr, "Error: %s is not a .fsm file\n", input)
		os.Exit(1)
	}

	machines, err := fsmfile.ListMachines(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", input, err)
		os.Exit(1)
	}

	if len(machines) == 1 {
		fmt.Printf("%s contains 1 machine:\n\n", input)
	} else {
		fmt.Printf("%s contains %d machines:\n\n", input, len(machines))
	}

	// Print table header
	fmt.Printf("  %-20s %-8s %6s %6s  %s\n", "NAME", "TYPE", "STATES", "TRANS", "DESCRIPTION")
	fmt.Printf("  %-20s %-8s %6s %6s  %s\n", "----", "----", "------", "-----", "-----------")

	for _, m := range machines {
		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Printf("  %-20s %-8s %6d %6d  %s\n", m.Name, m.Type, m.StateCount, m.TransCount, desc)
	}
}

func cmdBundle(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: fsm bundle <input1.fsm> <input2.fsm> ... -o <output.fsm>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Combines multiple .fsm files into a single bundle.")
		fmt.Fprintln(os.Stderr, "Each input becomes a named machine in the bundle.")
		os.Exit(1)
	}

	var inputs []string
	var output string

	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			output = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			inputs = append(inputs, args[i])
		}
	}

	if output == "" {
		fmt.Fprintln(os.Stderr, "Error: -o output.fsm is required")
		os.Exit(1)
	}

	if len(inputs) < 1 {
		fmt.Fprintln(os.Stderr, "Error: at least one input file is required")
		os.Exit(1)
	}

	// Create bundle
	err := fsmfile.CreateBundle(inputs, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating bundle: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created bundle: %s (%d machines)\n", output, len(inputs))
}

func cmdExtract(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: fsm extract <bundle.fsm> --machine <name> [-o output.fsm]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Extracts a single machine from a bundle.")
		os.Exit(1)
	}

	var input, machineName, output string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--machine", "-m":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "Error: input bundle is required")
		os.Exit(1)
	}

	if machineName == "" {
		fmt.Fprintln(os.Stderr, "Error: --machine name is required")
		os.Exit(1)
	}

	if output == "" {
		output = machineName + ".fsm"
	}

	// Extract machine
	f, layout, err := fsmfile.ReadMachineFromBundle(input, machineName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting %s: %v\n", machineName, err)
		os.Exit(1)
	}

	// Write to output
	var positions map[string][2]int
	var offsetX, offsetY int
	if layout != nil {
		positions = make(map[string][2]int)
		for name, pos := range layout.States {
			positions[name] = [2]int{pos.X, pos.Y}
		}
		offsetX = layout.Editor.CanvasOffsetX
		offsetY = layout.Editor.CanvasOffsetY
	}

	err = fsmfile.WriteFSMFileWithLayout(output, f, true, positions, offsetX, offsetY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
		os.Exit(1)
	}

	fmt.Printf("Extracted %s to %s\n", machineName, output)
}

// renderAllMachines renders all machines in a bundle to separate files or a tiled image
func renderAllMachines(input, outputPattern, format string, native bool, fontSize int, spacing float64, canvasWidth, canvasHeight int, shape string) {
	machines, err := fsmfile.ListMachines(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing machines: %v\n", err)
		os.Exit(1)
	}

	if len(machines) == 0 {
		fmt.Fprintln(os.Stderr, "No machines found in bundle")
		os.Exit(1)
	}

	// Render each machine to a separate file
	for _, m := range machines {
		f, _, err := fsmfile.ReadMachineFromBundle(input, m.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading machine %s: %v\n", m.Name, err)
			continue
		}

		// Generate output filename
		var output string
		if outputPattern != "" {
			// If pattern contains %s, replace with machine name
			if strings.Contains(outputPattern, "%s") {
				output = fmt.Sprintf(outputPattern, m.Name)
			} else {
				// Use pattern as directory/prefix
				dir := filepath.Dir(outputPattern)
				base := strings.TrimSuffix(filepath.Base(outputPattern), filepath.Ext(outputPattern))
				output = filepath.Join(dir, base+"_"+m.Name+"."+format)
			}
		} else {
			output = m.Name + "." + format
		}

		// Ensure output directory exists
		if dir := filepath.Dir(output); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
				continue
			}
		}

		// Generate title
		title := f.Name
		if title == "" {
			title = fmt.Sprintf("%s (%s)", m.Name, strings.ToUpper(string(f.Type)))
		}

		if native {
			if format == "png" {
				opts := fsmfile.DefaultPNGOptions()
				opts.Title = title
				if fontSize > 0 {
					opts.FontSize = fontSize
				}
				if spacing > 0 {
					opts.NodeSpacing = spacing
				}
				if canvasWidth > 0 {
					opts.Width = canvasWidth
				}
				if canvasHeight > 0 {
					opts.Height = canvasHeight
				}

				outFile, err := os.Create(output)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", output, err)
					continue
				}
				if err := fsmfile.RenderPNG(f, outFile, opts); err != nil {
					outFile.Close()
					fmt.Fprintf(os.Stderr, "Error rendering %s: %v\n", m.Name, err)
					continue
				}
				outFile.Close()
			} else if format == "svg" {
				opts := fsmfile.DefaultSVGOptions()
				opts.Title = title
				if fontSize > 0 {
					opts.FontSize = fontSize
				}
				if spacing > 0 {
					opts.NodeSpacing = spacing
				}
				if canvasWidth > 0 {
					opts.Width = canvasWidth
				}
				if canvasHeight > 0 {
					opts.Height = canvasHeight
				}

				// Parse shape option
				switch shape {
				case "circle":
					opts.StateShape = fsmfile.ShapeCircle
				case "ellipse":
					opts.StateShape = fsmfile.ShapeEllipse
				case "rect", "rectangle":
					opts.StateShape = fsmfile.ShapeRect
				case "roundrect", "rounded":
					opts.StateShape = fsmfile.ShapeRoundRect
				case "diamond":
					opts.StateShape = fsmfile.ShapeDiamond
				}

				svg := fsmfile.GenerateSVGNative(f, opts)
				if err := os.WriteFile(output, []byte(svg), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", output, err)
					continue
				}
			}
		} else {
			// Use Graphviz
			dotPath, err := exec.LookPath("dot")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: Graphviz 'dot' not found. Use --native flag.")
				os.Exit(1)
			}

			dot := fsmfile.GenerateDOT(f, title)
			cmd := exec.Command(dotPath, "-T"+format)
			cmd.Stdin = strings.NewReader(dot)

			outFile, err := os.Create(output)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", output, err)
				continue
			}

			cmd.Stdout = outFile
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				outFile.Close()
				fmt.Fprintf(os.Stderr, "Error running dot for %s: %v\n", m.Name, err)
				continue
			}
			outFile.Close()
		}

		fmt.Printf("Generated: %s\n", output)
	}

	fmt.Printf("\nRendered %d machines from %s\n", len(machines), input)
}

func cmdNetlist(args []string) {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: fsm netlist <input> [options]")
		fmt.Println("")
		fmt.Println("Export structural netlist from an FSM with port/net data.")
		fmt.Println("")
		fmt.Println("Formats:")
		fmt.Println("  text     Human-readable netlist (default)")
		fmt.Println("  kicad    KiCad legacy S-expression (.net)")
		fmt.Println("  json     Structured JSON netlist")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -f, --format   Output format: text, kicad, json (default: text)")
		fmt.Println("  -o, --output   Output file (default: stdout)")
		fmt.Println("  -m, --machine  Select machine from bundle")
		fmt.Println("  --bake         Write derived KiCad fields into source file classes")
		fmt.Println("                 (kicad_part, kicad_footprint). Only for JSON files.")
		fmt.Println("                 Does not overwrite existing values.")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  fsm netlist circuit.json")
		fmt.Println("  fsm netlist circuit.fsm --format kicad -o circuit.net")
		fmt.Println("  fsm netlist bundle.fsm -m controller --format json")
		fmt.Println("  fsm netlist circuit.json --bake")
		if len(args) < 1 {
			os.Exit(1)
		}
		return
	}

	var input, output, format, machineName string
	var bake bool
	format = "text"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "-o", "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-m", "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--bake":
			bake = true
		default:
			if !strings.HasPrefix(args[i], "-") && input == "" {
				input = args[i]
			}
		}
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "Error: input file required")
		os.Exit(1)
	}

	// Handle --bake: write KiCad fields into source file, then exit.
	if bake {
		cmdNetlistBake(input)
		return
	}

	// Load FSM.
	f, err := loadFSMWithMachine(input, machineName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
		os.Exit(1)
	}

	// Build the netlist.
	nl := export.Build(f)

	if len(nl.Components) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: no components with ports found. Is this a structural FSM?")
	}

	// Determine output writer.
	var w *os.File
	if output == "" || output == "-" {
		w = os.Stdout
	} else {
		w, err = os.Create(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", output, err)
			os.Exit(1)
		}
		defer w.Close()
	}

	// Export.
	switch format {
	case "text":
		err = export.WriteText(w, nl)
	case "kicad":
		err = export.WriteKiCad(w, nl, f)
	case "json":
		err = export.WriteJSON(w, nl)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s (use text, kicad, or json)\n", format)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing netlist: %v\n", err)
		os.Exit(1)
	}

	// Summary to stderr (so stdout can be piped).
	if output != "" && output != "-" {
		fmt.Fprintf(os.Stderr, "Exported: %d components, %d nets -> %s (%s)\n",
			len(nl.Components), len(nl.Nets), output, format)
	}

	if len(nl.Unresolved) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d components without KiCad part mapping:\n", len(nl.Unresolved))
		for _, u := range nl.Unresolved {
			fmt.Fprintf(os.Stderr, "  %s\n", u)
		}
	}
}

// cmdNetlistBake writes derived KiCad fields into source file classes.
// Supports both FSM JSON files and .classes.json library files.
func cmdNetlistBake(input string) {
	data, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", input, err)
		os.Exit(1)
	}

	if strings.HasSuffix(input, ".classes.json") {
		bakeClassLibrary(input, data)
	} else if strings.HasSuffix(input, ".json") {
		bakeFSMJSON(input, data)
	} else {
		fmt.Fprintln(os.Stderr, "Error: --bake only supports .json and .classes.json files")
		os.Exit(1)
	}
}

// bakeClassLibrary derives KiCad fields in a .classes.json library file.
func bakeClassLibrary(path string, data []byte) {
	var lib map[string]*fsm.Class
	if err := json.Unmarshal(data, &lib); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", path, err)
		os.Exit(1)
	}

	changed := 0
	for name, cls := range lib {
		cls.Name = name // map key is the name; struct field may be empty
		if cls.DeriveKiCadFields() {
			changed++
			fmt.Fprintf(os.Stderr, "  %s -> part=%s footprint=%s\n",
				name, cls.KiCadPart, cls.KiCadFootprint)
		}
	}

	if changed == 0 {
		fmt.Fprintf(os.Stderr, "%s: all classes already have KiCad fields (or none are derivable)\n", path)
		return
	}

	// Clear Name fields so they don't appear in the map-keyed JSON.
	// We use a custom struct to avoid the "name" field in output.
	type libClass struct {
		Parent         string            `json:"parent,omitempty"`
		Properties     []fsm.PropertyDef `json:"properties"`
		Ports          []fsm.Port        `json:"ports,omitempty"`
		KiCadPart      string            `json:"kicad_part,omitempty"`
		KiCadFootprint string            `json:"kicad_footprint,omitempty"`
	}
	outLib := make(map[string]libClass)
	for name, cls := range lib {
		outLib[name] = libClass{
			Parent:         cls.Parent,
			Properties:     cls.Properties,
			Ports:          cls.Ports,
			KiCadPart:      cls.KiCadPart,
			KiCadFootprint: cls.KiCadFootprint,
		}
	}

	out, err := json.MarshalIndent(outLib, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Baked KiCad fields into %d classes in %s\n", changed, path)
}

// bakeFSMJSON derives KiCad fields in an FSM JSON file's classes.
func bakeFSMJSON(path string, data []byte) {
	f, err := fsmfile.ParseJSON(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", path, err)
		os.Exit(1)
	}

	changed := 0
	for name, cls := range f.Classes {
		if name == fsm.DefaultClassName {
			continue
		}
		if cls.DeriveKiCadFields() {
			changed++
			fmt.Fprintf(os.Stderr, "  %s -> part=%s footprint=%s\n",
				name, cls.KiCadPart, cls.KiCadFootprint)
		}
	}

	if changed == 0 {
		fmt.Fprintf(os.Stderr, "%s: all classes already have KiCad fields (or none are derivable)\n", path)
		return
	}

	out, err := fsmfile.ToJSON(f, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Baked KiCad fields into %d classes in %s\n", changed, path)
}
