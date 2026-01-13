#!/usr/bin/env python3
"""
FSM Visualiser: Generate Graphviz DOT from JSON, hex, or .fsm format.

Supports:
  - DFA (Deterministic Finite Automaton)
  - NFA (Non-deterministic Finite Automaton)  
  - Moore (outputs on states)
  - Mealy (outputs on transitions)

Usage:
  fsm_visualise.py input.json              # JSON input
  fsm_visualise.py input.hex               # Hex input
  fsm_visualise.py input.fsm               # FSM file format
  fsm_visualise.py input.fsm -o out.dot    # Write to file
  fsm_visualise.py input.fsm | dot -Tpng -o fsm.png  # Pipe to Graphviz
"""

import json
import sys
import argparse
import re
import zipfile
from typing import Optional
from dataclasses import dataclass, field
from pathlib import Path

# Check for toml support
try:
    import tomllib  # Python 3.11+
except ImportError:
    tomllib = None


# Record type markers (uint16)
TYPE_DFA_TRANSITION = 0x0000
TYPE_MEALY_TRANSITION = 0x0001
TYPE_STATE_DECL = 0x0002
TYPE_NFA_MULTI = 0x0003

EPSILON_INPUT = 0xFFFF


@dataclass
class FSM:
    """Internal FSM representation."""
    fsm_type: str
    name: str = ""
    description: str = ""
    states: list[str] = field(default_factory=list)
    alphabet: list[str] = field(default_factory=list)
    initial: Optional[str] = None
    accepting: list[str] = field(default_factory=list)
    transitions: list[dict] = field(default_factory=list)
    state_outputs: dict[str, str] = field(default_factory=dict)
    output_alphabet: list[str] = field(default_factory=list)


def parse_json(data: dict) -> FSM:
    """Parse JSON into FSM structure."""
    return FSM(
        fsm_type=data.get("type", "dfa").lower(),
        name=data.get("name", ""),
        description=data.get("description", ""),
        states=data.get("states", []),
        alphabet=data.get("alphabet", []),
        initial=data.get("initial"),
        accepting=data.get("accepting", []),
        transitions=data.get("transitions", []),
        state_outputs=data.get("state_outputs", {}),
        output_alphabet=data.get("output_alphabet", [])
    )


def parse_labels_toml(text: str) -> dict:
    """Parse labels.toml content."""
    if tomllib is not None:
        return tomllib.loads(text)
    # Fallback parser
    result = {}
    current_section = None
    
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        
        if line.startswith("[") and line.endswith("]"):
            current_section = line[1:-1]
            result[current_section] = {}
        elif "=" in line and current_section:
            key, value = line.split("=", 1)
            key = key.strip()
            value = value.strip().strip('"')
            
            if key.startswith("0x"):
                key = int(key, 16)
            if value.isdigit():
                value = int(value)
            
            result[current_section][key] = value
    
    return result


def parse_hex_input(text: str) -> list[str]:
    """Parse hex records from text."""
    lines = [line for line in text.split("\n") if not line.strip().startswith("#")]
    text = "\n".join(lines)
    
    pattern = r'([0-9A-Fa-f]{4})\s*([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})\s*([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})'
    
    records = []
    for match in re.finditer(pattern, text):
        rec_type, f1, f2, f3, f4 = match.groups()
        record = f"{rec_type.upper()} {f1.upper()}:{f2.upper()} {f3.upper()}:{f4.upper()}"
        records.append(record)
    
    return records


def parse_record(record: str) -> tuple[int, int, int, int, int]:
    """Parse a record from 'TYPE SSSS:IIII TTTT:OOOO' format."""
    clean = record.replace(" ", "").replace(":", "")
    return (
        int(clean[0:4], 16),
        int(clean[4:8], 16),
        int(clean[8:12], 16),
        int(clean[12:16], 16),
        int(clean[16:20], 16)
    )


def hex_to_fsm(records: list[str], labels: Optional[dict] = None) -> FSM:
    """Parse hex uint16 records into FSM."""
    # Extract label mappings if provided
    state_labels = {}
    input_labels = {}
    output_labels = {}
    fsm_meta = {}
    
    if labels:
        fsm_meta = labels.get("fsm", {})
        
        for k, v in labels.get("states", {}).items():
            idx = int(k, 16) if isinstance(k, str) else k
            state_labels[idx] = v
        
        for k, v in labels.get("inputs", {}).items():
            idx = int(k, 16) if isinstance(k, str) else k
            input_labels[idx] = v
        
        for k, v in labels.get("outputs", {}).items():
            idx = int(k, 16) if isinstance(k, str) else k
            output_labels[idx] = v
    
    state_ids = set()
    input_ids = set()
    output_ids = set()
    has_mealy = False
    has_nfa_multi = False
    has_moore_outputs = False
    
    initial_state = None
    accepting_states = set()
    state_outputs = {}
    transitions = []
    nfa_pending = None
    
    for record in records:
        rec_type, f1, f2, f3, f4 = parse_record(record)
        
        if rec_type == TYPE_STATE_DECL:
            state_id, flags, output_val = f1, f2, f3
            state_ids.add(state_id)
            if flags & 0x1:
                initial_state = state_id
            if flags & 0x2:
                accepting_states.add(state_id)
            if output_val != 0:
                has_moore_outputs = True
                state_outputs[state_id] = output_val - 1
                output_ids.add(output_val - 1)
        
        elif rec_type == TYPE_DFA_TRANSITION:
            src, inp, tgt = f1, f2, f3
            state_ids.add(src)
            state_ids.add(tgt)
            if inp != EPSILON_INPUT:
                input_ids.add(inp)
            transitions.append({
                "from": src, 
                "input": inp if inp != EPSILON_INPUT else None, 
                "to": tgt
            })
        
        elif rec_type == TYPE_MEALY_TRANSITION:
            has_mealy = True
            src, inp, tgt, out = f1, f2, f3, f4
            state_ids.add(src)
            state_ids.add(tgt)
            if inp != EPSILON_INPUT:
                input_ids.add(inp)
            output_ids.add(out)
            transitions.append({
                "from": src, 
                "input": inp if inp != EPSILON_INPUT else None, 
                "to": tgt, 
                "output": out
            })
        
        elif rec_type == TYPE_NFA_MULTI:
            has_nfa_multi = True
            src, inp, tgt, cont = f1, f2, f3, f4
            state_ids.add(src)
            state_ids.add(tgt)
            if inp != EPSILON_INPUT:
                input_ids.add(inp)
            
            if nfa_pending is None or nfa_pending[0] != src or nfa_pending[1] != inp:
                if nfa_pending is not None:
                    transitions.append({
                        "from": nfa_pending[0],
                        "input": nfa_pending[1] if nfa_pending[1] != EPSILON_INPUT else None,
                        "to": nfa_pending[2]
                    })
                nfa_pending = (src, inp, [tgt])
            else:
                nfa_pending[2].append(tgt)
            
            if cont == 0:
                transitions.append({
                    "from": nfa_pending[0],
                    "input": nfa_pending[1] if nfa_pending[1] != EPSILON_INPUT else None,
                    "to": nfa_pending[2]
                })
                nfa_pending = None
    
    if nfa_pending is not None:
        transitions.append({
            "from": nfa_pending[0],
            "input": nfa_pending[1] if nfa_pending[1] != EPSILON_INPUT else None,
            "to": nfa_pending[2]
        })
    
    # Determine FSM type
    if has_mealy:
        fsm_type = "mealy"
    elif has_moore_outputs:
        fsm_type = "moore"
    elif has_nfa_multi or any(t.get("input") is None for t in transitions):
        fsm_type = "nfa"
    else:
        fsm_type = "dfa"
    
    if fsm_meta.get("type"):
        fsm_type = fsm_meta["type"]
    
    # Generate symbol names (use labels if available)
    def state_name(i):
        return state_labels.get(i, f"S{i}")
    
    def input_name(i):
        return input_labels.get(i, f"i{i}")
    
    def output_name(i):
        return output_labels.get(i, f"o{i}")
    
    states = [state_name(i) for i in sorted(state_ids)]
    state_map = {i: state_name(i) for i in state_ids}
    
    alphabet = [input_name(i) for i in sorted(input_ids)]
    input_map = {i: input_name(i) for i in input_ids}
    
    output_alphabet = [output_name(i) for i in sorted(output_ids)] if output_ids else []
    output_map = {i: output_name(i) for i in output_ids}
    
    # Convert transitions
    named_transitions = []
    for t in transitions:
        nt = {
            "from": state_map[t["from"]],
            "input": input_map.get(t["input"]) if t["input"] is not None else None,
        }
        if isinstance(t["to"], list):
            nt["to"] = [state_map[x] for x in t["to"]]
        else:
            nt["to"] = state_map[t["to"]]
        if "output" in t:
            nt["output"] = output_map[t["output"]]
        named_transitions.append(nt)
    
    fsm = FSM(
        fsm_type=fsm_type,
        name=fsm_meta.get("name", ""),
        description=fsm_meta.get("description", ""),
        states=states,
        alphabet=alphabet,
        initial=state_map.get(initial_state),
        accepting=[state_map[s] for s in accepting_states],
        transitions=named_transitions,
        output_alphabet=output_alphabet
    )
    
    if fsm_type == "moore":
        fsm.state_outputs = {state_map[k]: output_map[v] for k, v in state_outputs.items()}
    
    return fsm


def read_fsm_file(path: str) -> tuple[list[str], Optional[dict]]:
    """Read .fsm zip file. Returns (records, labels)."""
    with zipfile.ZipFile(path, 'r') as zf:
        hex_content = zf.read("machine.hex").decode("utf-8")
        records = parse_hex_input(hex_content)
        
        labels = None
        if "labels.toml" in zf.namelist():
            labels_content = zf.read("labels.toml").decode("utf-8")
            labels = parse_labels_toml(labels_content)
        
        return records, labels


def escape_dot(s: str) -> str:
    """Escape string for DOT labels."""
    return s.replace('"', '\\"').replace('<', '\\<').replace('>', '\\>')


def fsm_to_dot(fsm: FSM, title: Optional[str] = None) -> str:
    """Convert FSM to Graphviz DOT format."""
    lines = []
    
    # Header
    lines.append("digraph FSM {")
    lines.append("    rankdir=LR;")
    lines.append("    node [fontname=\"Helvetica\", fontsize=11];")
    lines.append("    edge [fontname=\"Helvetica\", fontsize=10];")
    lines.append("")
    
    # Title
    if title:
        lines.append(f"    labelloc=\"t\";")
        lines.append(f"    label=\"{escape_dot(title)}\";")
        lines.append("")
    
    # Invisible start node for initial state arrow
    if fsm.initial:
        lines.append("    __start [shape=none, label=\"\", width=0, height=0];")
        lines.append(f"    __start -> \"{escape_dot(fsm.initial)}\";")
        lines.append("")
    
    # State nodes
    for state in fsm.states:
        attrs = []
        
        if state in fsm.accepting:
            attrs.append("shape=doublecircle")
        else:
            attrs.append("shape=circle")
        
        if fsm.fsm_type == "moore" and state in fsm.state_outputs:
            label = f"{state}\\n/{fsm.state_outputs[state]}"
            attrs.append(f"label=\"{escape_dot(label)}\"")
        
        if attrs:
            lines.append(f"    \"{escape_dot(state)}\" [{', '.join(attrs)}];")
    
    lines.append("")
    
    # Group transitions by (from, to) to combine labels
    edge_labels = {}
    
    for t in fsm.transitions:
        src = t["from"]
        targets = t["to"] if isinstance(t["to"], list) else [t["to"]]
        
        inp = t.get("input")
        if inp is None:
            label = "ε"
        else:
            label = inp
        
        if fsm.fsm_type == "mealy" and "output" in t:
            label = f"{label}/{t['output']}"
        
        for tgt in targets:
            key = (src, tgt)
            if key not in edge_labels:
                edge_labels[key] = []
            edge_labels[key].append(label)
    
    # Write edges
    for (src, tgt), labels in edge_labels.items():
        combined = ", ".join(labels)
        lines.append(f"    \"{escape_dot(src)}\" -> \"{escape_dot(tgt)}\" [label=\"{escape_dot(combined)}\"];")
    
    lines.append("}")
    
    return "\n".join(lines)


def detect_format(path: Path, text: Optional[str] = None) -> str:
    """Detect input format from extension or content."""
    if path.suffix == ".fsm":
        return "fsm"
    if path.suffix == ".json":
        return "json"
    if path.suffix == ".hex":
        return "hex"
    
    # Detect from content
    if text:
        text = text.strip()
        if text.startswith("{") or text.startswith("["):
            return "json"
        if re.search(r'[0-9A-Fa-f]{4}\s*[0-9A-Fa-f]{4}:[0-9A-Fa-f]{4}', text):
            return "hex"
    
    return "json"


def main():
    parser = argparse.ArgumentParser(
        description="Generate Graphviz DOT from FSM (JSON, hex, or .fsm format)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s fsm.json                        # Output DOT to stdout
  %(prog)s fsm.fsm -o fsm.dot              # Write DOT to file
  %(prog)s fsm.json | dot -Tpng -o fsm.png # Generate PNG
  %(prog)s fsm.fsm | dot -Tsvg -o fsm.svg  # Generate SVG
  %(prog)s - < fsm.hex                     # Read from stdin

Input format is auto-detected from extension or content.
        """
    )
    
    parser.add_argument("input", metavar="FILE", help="Input file (JSON, hex, or .fsm), or '-' for stdin")
    parser.add_argument("-o", "--output", metavar="FILE", help="Output file (default: stdout)")
    parser.add_argument("-t", "--title", metavar="TITLE", help="Graph title")
    parser.add_argument("--format", choices=["json", "hex", "fsm"], help="Force input format")
    
    args = parser.parse_args()
    
    try:
        # Read input
        if args.input == "-":
            text = sys.stdin.read()
            input_path = Path("stdin")
            fmt = args.format or detect_format(input_path, text)
        else:
            input_path = Path(args.input)
            fmt = args.format or detect_format(input_path)
            
            if fmt != "fsm":
                with open(args.input, "r") as f:
                    text = f.read()
        
        # Parse
        if fmt == "json":
            data = json.loads(text)
            fsm = parse_json(data)
        elif fmt == "hex":
            records = parse_hex_input(text)
            fsm = hex_to_fsm(records)
        elif fmt == "fsm":
            records, labels = read_fsm_file(args.input)
            fsm = hex_to_fsm(records, labels)
        
        # Generate title
        title = args.title
        if not title:
            if fsm.name:
                title = fsm.name
            else:
                title = f"{fsm.fsm_type.upper()}: {len(fsm.states)} states"
        
        # Generate DOT
        dot = fsm_to_dot(fsm, title)
        
        # Write output
        if args.output:
            with open(args.output, "w") as f:
                f.write(dot)
                f.write("\n")
        else:
            print(dot)
    
    except json.JSONDecodeError as e:
        print(f"JSON parse error: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
