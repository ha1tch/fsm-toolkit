// properties.go — "fsm properties" subcommand.
//
// Queries state class assignments and property values from a .fsm or .json
// file, with filtering by state name and/or class, and output in four formats.
//
// Usage:
//   fsm properties <input> [options]
//
// Options:
//   --machine <name>    Select a machine from a bundle (default: first)
//   --all               Iterate all machines in a bundle
//   --state <name>      Show only the given state
//   --class <name>      Show only states assigned to this class
//   --format <fmt>      Output format: text (default), json, csv,
//                       asciitable, htmltable

package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"sort"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

// propRow is a single row in the output table.
type propRow struct {
	Machine  string
	State    string
	Class    string
	Property string
	Value    string
}

// ---- command entry point -----------------------------------------------------

func cmdProperties(args []string) {
	const usageMsg = `Usage: fsm properties <input> [options]

Options:
  --machine <name>   Select a machine from a bundle (default: first)
  --all              Iterate all machines in a bundle
  --state <name>     Show only the given state
  --class <name>     Show only states assigned to this class
  --format <fmt>     Output format: text (default), json, csv, asciitable, htmltable

Examples:
  fsm properties heladera.fsm
  fsm properties heladera.fsm --state temperatura_elevada
  fsm properties paracaidas.fsm --all --class shelf_terminal --format csv
  fsm properties bundle.fsm --all --format json
`
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usageMsg)
		os.Exit(1)
	}

	var (
		input       string
		machineName string
		allMachines bool
		filterState string
		filterClass string
		format      = "text"
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--machine", "-m":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--all", "-a":
			allMachines = true
		case "--state", "-s":
			if i+1 < len(args) {
				filterState = args[i+1]
				i++
			}
		case "--class", "-c":
			if i+1 < len(args) {
				filterClass = args[i+1]
				i++
			}
		case "--format", "-f":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "-h", "--help":
			fmt.Print(usageMsg)
			os.Exit(0)
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

	validFormats := map[string]bool{
		"text": true, "json": true,
		"csv": true, "asciitable": true, "htmltable": true,
	}
	if !validFormats[format] {
		fmt.Fprintf(os.Stderr, "Error: unknown format %q (valid: text, json, csv, asciitable, htmltable)\n", format)
		os.Exit(1)
	}

	// Collect rows from one or all machines.
	var rows []propRow

	isBundle, _ := fsmfile.IsBundle(input)

	if isBundle && allMachines {
		machines, err := fsmfile.ListMachines(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing machines: %v\n", err)
			os.Exit(1)
		}
		for _, m := range machines {
			f, _, err := fsmfile.ReadMachineFromBundle(input, m.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading machine %q: %v\n", m.Name, err)
				os.Exit(1)
			}
			rows = append(rows, extractRows(f, m.Name, filterState, filterClass)...)
		}
	} else {
		f, err := loadFSMWithMachine(input, machineName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", input, err)
			os.Exit(1)
		}
		name := machineName
		if name == "" {
			name = f.Name
		}
		rows = extractRows(f, name, filterState, filterClass)
	}

	// Render.
	switch format {
	case "text":
		renderText(rows)
	case "json":
		renderJSON(rows)
	case "csv":
		renderCSV(rows)
	case "asciitable":
		renderASCIITable(rows)
	case "htmltable":
		renderHTMLTable(rows)
	}
}

// ---- row extraction ----------------------------------------------------------

// extractRows collects one propRow per (state, property) pair for an FSM,
// respecting optional filters.
func extractRows(f *fsm.FSM, machineName, filterState, filterClass string) []propRow {
	var rows []propRow

	// Iterate states in declaration order for stable output.
	for _, state := range f.States {
		if filterState != "" && state != filterState {
			continue
		}

		class := f.StateClasses[state]
		if class == "" {
			class = fsm.DefaultClassName
		}

		if filterClass != "" && class != filterClass {
			continue
		}

		props := f.StateProperties[state]

		if len(props) == 0 {
			// State has a class assignment but no property values — emit one row
			// so the assignment is visible.
			rows = append(rows, propRow{
				Machine:  machineName,
				State:    state,
				Class:    class,
				Property: "(none)",
				Value:    "",
			})
			continue
		}

		// Emit one row per property, sorted by name.
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			rows = append(rows, propRow{
				Machine:  machineName,
				State:    state,
				Class:    class,
				Property: k,
				Value:    fmt.Sprintf("%v", props[k]),
			})
		}
	}

	return rows
}

// ---- renderers ---------------------------------------------------------------

func renderText(rows []propRow) {
	if len(rows) == 0 {
		fmt.Println("(no properties found)")
		return
	}

	lastMachine := ""
	lastState := ""

	for _, r := range rows {
		if r.Machine != lastMachine {
			if lastMachine != "" {
				fmt.Println()
			}
			fmt.Printf("Machine: %s\n", r.Machine)
			lastMachine = r.Machine
			lastState = ""
		}
		if r.State != lastState {
			fmt.Printf("  State: %s  [class: %s]\n", r.State, r.Class)
			lastState = r.State
		}
		if r.Property == "(none)" {
			fmt.Printf("    (no properties set)\n")
		} else {
			fmt.Printf("    %-22s %s\n", r.Property+":", r.Value)
		}
	}
}

func renderJSON(rows []propRow) {
	// Group into machine → state → {class, properties{}}
	type stateEntry struct {
		Class      string            `json:"class"`
		Properties map[string]string `json:"properties"`
	}
	type machineEntry map[string]stateEntry

	out := make(map[string]machineEntry)

	for _, r := range rows {
		if out[r.Machine] == nil {
			out[r.Machine] = make(machineEntry)
		}
		me := out[r.Machine]

		entry, ok := me[r.State]
		if !ok {
			entry = stateEntry{
				Class:      r.Class,
				Properties: make(map[string]string),
			}
		}
		if r.Property != "(none)" {
			entry.Properties[r.Property] = r.Value
		}
		me[r.State] = entry
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

func renderCSV(rows []propRow) {
	fmt.Println("machine,state,class,property,value")
	for _, r := range rows {
		fmt.Printf("%s,%s,%s,%s,%s\n",
			csvEscape(r.Machine),
			csvEscape(r.State),
			csvEscape(r.Class),
			csvEscape(r.Property),
			csvEscape(r.Value),
		)
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func renderASCIITable(rows []propRow) {
	if len(rows) == 0 {
		fmt.Println("(no properties found)")
		return
	}

	headers := []string{"Machine", "State", "Class", "Property", "Value"}
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		cells := []string{r.Machine, r.State, r.Class, r.Property, r.Value}
		for i, c := range cells {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	sep := "+"
	for _, w := range widths {
		sep += strings.Repeat("-", w+2) + "+"
	}

	printRow := func(cells []string) {
		fmt.Print("|")
		for i, c := range cells {
			fmt.Printf(" %-*s |", widths[i], c)
		}
		fmt.Println()
	}

	fmt.Println(sep)
	printRow(headers)
	fmt.Println(sep)
	for _, r := range rows {
		printRow([]string{r.Machine, r.State, r.Class, r.Property, r.Value})
	}
	fmt.Println(sep)
}

func renderHTMLTable(rows []propRow) {
	fmt.Println(`<!DOCTYPE html>`)
	fmt.Println(`<html><head><meta charset="utf-8">`)
	fmt.Println(`<style>`)
	fmt.Println(`  body { font-family: monospace; padding: 1em; }`)
	fmt.Println(`  table { border-collapse: collapse; width: 100%; }`)
	fmt.Println(`  th, td { border: 1px solid #ccc; padding: 6px 10px; text-align: left; }`)
	fmt.Println(`  th { background: #f0f0f0; }`)
	fmt.Println(`  tr:nth-child(even) { background: #fafafa; }`)
	fmt.Println(`</style></head><body>`)
	fmt.Printf("<p>%d row(s)</p>\n", len(rows))
	fmt.Println(`<table>`)
	fmt.Println(`  <thead><tr>`)
	for _, h := range []string{"Machine", "State", "Class", "Property", "Value"} {
		fmt.Printf("    <th>%s</th>\n", h)
	}
	fmt.Println(`  </tr></thead>`)
	fmt.Println(`  <tbody>`)
	for _, r := range rows {
		fmt.Println(`    <tr>`)
		for _, v := range []string{r.Machine, r.State, r.Class, r.Property, r.Value} {
			fmt.Printf("      <td>%s</td>\n", html.EscapeString(v))
		}
		fmt.Println(`    </tr>`)
	}
	fmt.Println(`  </tbody>`)
	fmt.Println(`</table>`)
	fmt.Println(`</body></html>`)
}
