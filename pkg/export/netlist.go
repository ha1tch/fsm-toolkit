// Package export provides netlist export in multiple formats.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// Component represents one instance in the netlist output.
type Component struct {
	Ref       string `json:"ref"`       // reference designator (state name)
	Value     string `json:"value"`     // component type (class name)
	Footprint string `json:"footprint"` // KiCad footprint or ""
	PinCount  int    `json:"pin_count"` // total pins
}

// NetPin represents one pin on a net.
type NetPin struct {
	Ref  string `json:"ref"`  // reference designator
	Pin  string `json:"pin"`  // pin number (as string for KiCad)
	Port string `json:"port"` // port name
}

// NetEntry represents one net in the netlist output.
type NetEntry struct {
	Name string   `json:"name"`
	Pins []NetPin `json:"pins"`
}

// Netlist is the intermediate representation used by all exporters.
type Netlist struct {
	Name       string      `json:"name"`
	Components []Component `json:"components"`
	Nets       []NetEntry  `json:"nets"`
	Unresolved []string    `json:"unresolved,omitempty"` // components without KiCad mapping
}

// Build creates a Netlist from an FSM. It derives KiCad fields for
// 74xx components that lack them. The derivation is non-destructive —
// it does not modify the source FSM.
func Build(f *fsm.FSM) *Netlist {
	nl := &Netlist{
		Name: f.Name,
	}

	// Build component list from states that have classes with ports.
	// We work with copies so derivation doesn't mutate the original.
	classCache := make(map[string]*fsm.Class)
	for name, cls := range f.Classes {
		if name == fsm.DefaultClassName {
			continue
		}
		// Deep-copy the class so DeriveKiCadFields doesn't mutate.
		cp := *cls
		cp.DeriveKiCadFields()
		classCache[name] = &cp
	}

	for _, state := range f.States {
		className := f.GetStateClass(state)
		cls := classCache[className]
		if cls == nil || cls.PinCount() == 0 {
			continue // skip behavioural-only states
		}

		comp := Component{
			Ref:       state,
			Value:     className,
			Footprint: cls.KiCadFootprint,
			PinCount:  cls.PinCount(),
		}
		nl.Components = append(nl.Components, comp)

		if cls.KiCadPart == "" {
			nl.Unresolved = append(nl.Unresolved, state+" ("+className+")")
		}
	}

	// Sort components by ref designator.
	sort.Slice(nl.Components, func(i, j int) bool {
		return nl.Components[i].Ref < nl.Components[j].Ref
	})

	// Build net list.
	for _, n := range f.Nets {
		entry := NetEntry{Name: n.Name}

		for _, ep := range n.Endpoints {
			className := f.GetStateClass(ep.Instance)
			cls := classCache[className]
			pin := "?"
			if cls != nil {
				if p := cls.GetPort(ep.Port); p != nil {
					if p.PinNumber > 0 {
						pin = fmt.Sprintf("%d", p.PinNumber)
					}
				}
			}
			entry.Pins = append(entry.Pins, NetPin{
				Ref:  ep.Instance,
				Pin:  pin,
				Port: ep.Port,
			})
		}

		nl.Nets = append(nl.Nets, entry)
	}

	// Sort nets by name.
	sort.Slice(nl.Nets, func(i, j int) bool {
		return nl.Nets[i].Name < nl.Nets[j].Name
	})

	return nl
}

// --- Text export ---

// WriteText writes a human-readable netlist.
func WriteText(w io.Writer, nl *Netlist) error {
	fmt.Fprintf(w, "# Netlist: %s\n", nl.Name)
	fmt.Fprintf(w, "# Components: %d, Nets: %d\n\n", len(nl.Components), len(nl.Nets))

	fmt.Fprintln(w, "# Components")
	for _, c := range nl.Components {
		fp := c.Footprint
		if fp == "" {
			fp = "(no footprint)"
		}
		fmt.Fprintf(w, "%-12s %-30s %s\n", c.Ref, c.Value, fp)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# Nets")
	for _, n := range nl.Nets {
		var parts []string
		for _, p := range n.Pins {
			parts = append(parts, fmt.Sprintf("%s.%s(pin%s)", p.Ref, p.Port, p.Pin))
		}
		fmt.Fprintf(w, "%-16s %s\n", n.Name+":", strings.Join(parts, ", "))
	}

	if len(nl.Unresolved) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "# Unresolved KiCad mappings")
		for _, u := range nl.Unresolved {
			fmt.Fprintf(w, "#   %s\n", u)
		}
	}

	return nil
}

// --- JSON export ---

// WriteJSON writes the netlist as formatted JSON.
func WriteJSON(w io.Writer, nl *Netlist) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(nl)
}

// --- KiCad .net export ---

// WriteKiCad writes a KiCad legacy S-expression netlist (.net format).
// This format is imported by PCBnew (KiCad 5, 6, and 7+).
func WriteKiCad(w io.Writer, nl *Netlist, f *fsm.FSM) error {
	// Build a class cache with derived fields for KiCad part lookup.
	classCache := make(map[string]*fsm.Class)
	for name, cls := range f.Classes {
		if name == fsm.DefaultClassName {
			continue
		}
		cp := *cls
		cp.DeriveKiCadFields()
		classCache[name] = &cp
	}

	fmt.Fprintln(w, "(export (version D)")

	// Design section.
	designName := strings.ReplaceAll(nl.Name, `"`, `'`)
	fmt.Fprintf(w, "  (design (source \"fsm-toolkit: %s\"))\n", designName)

	// Components section.
	fmt.Fprintln(w, "  (components")
	for _, c := range nl.Components {
		cls := classCache[c.Value]

		kicadPart := ""
		kicadLib := "fsm-toolkit"
		if cls != nil && cls.KiCadPart != "" {
			parts := strings.SplitN(cls.KiCadPart, ":", 2)
			if len(parts) == 2 {
				kicadLib = parts[0]
				kicadPart = parts[1]
			} else {
				kicadPart = cls.KiCadPart
			}
		} else {
			kicadPart = c.Value
		}

		footprint := c.Footprint
		if footprint == "" {
			footprint = "UNASSIGNED"
		}

		fmt.Fprintf(w, "    (comp (ref %s)\n", escapeKiCad(c.Ref))
		fmt.Fprintf(w, "      (value %s)\n", escapeKiCad(kicadPart))
		fmt.Fprintf(w, "      (footprint %s)\n", escapeKiCad(footprint))
		fmt.Fprintf(w, "      (libsource (lib %s) (part %s))\n",
			escapeKiCad(kicadLib), escapeKiCad(kicadPart))
		fmt.Fprintln(w, "    )")
	}
	fmt.Fprintln(w, "  )")

	// Nets section.
	fmt.Fprintln(w, "  (nets")
	for i, n := range nl.Nets {
		fmt.Fprintf(w, "    (net (code %d) (name %s)\n", i+1, quoteKiCad(n.Name))
		for _, p := range n.Pins {
			fmt.Fprintf(w, "      (node (ref %s) (pin %s))\n",
				escapeKiCad(p.Ref), p.Pin)
		}
		fmt.Fprintln(w, "    )")
	}
	fmt.Fprintln(w, "  )")

	fmt.Fprintln(w, ")")
	return nil
}

// escapeKiCad returns a KiCad-safe identifier. If the string contains
// spaces or special characters, it is quoted.
func escapeKiCad(s string) string {
	if s == "" {
		return `""`
	}
	for _, ch := range s {
		if ch == ' ' || ch == '"' || ch == '(' || ch == ')' {
			return quoteKiCad(s)
		}
	}
	return s
}

// quoteKiCad wraps a string in double quotes, escaping inner quotes.
func quoteKiCad(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
