#!/usr/bin/env python3
"""
FSM Converter: JSON <-> FSM File Format

The FSM file format (.fsm) is a zip containing:
  - machine.hex    (required) The state machine in hex uint16 format
  - labels.toml    (optional) Human-readable names for states, inputs, outputs

Hex uint16 Format (20 hex characters per record, grouped as "TYPE SSSS:IIII TTTT:OOOO"):

  All fields are 16-bit unsigned integers (0000-FFFF).
  
  Record types:
    0000 = DFA/NFA transition (single target)
    0001 = Mealy transition (with output)
    0002 = State declaration
    0003 = NFA multi-target transition

  Special values:
    Input FFFF = epsilon transition (NFA)

Usage:
  fsm_converter.py --to-fsm input.json -o output.fsm
  fsm_converter.py --to-json input.fsm -o output.json --pretty
  fsm_converter.py --to-hex input.json                    # hex only, no labels
  fsm_converter.py --to-fsm input.json --no-labels        # .fsm without labels.toml
"""

import json
import sys
import argparse
import re
import zipfile
import io
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

# Special input symbol for epsilon transitions
EPSILON_INPUT = 0xFFFF


@dataclass
class FSM:
    """Internal FSM representation."""
    fsm_type: str  # "dfa", "nfa", "moore", "mealy"
    name: str = ""
    description: str = ""
    states: list[str] = field(default_factory=list)
    alphabet: list[str] = field(default_factory=list)
    initial: Optional[str] = None
    accepting: list[str] = field(default_factory=list)
    transitions: list[dict] = field(default_factory=list)
    state_outputs: dict[str, str] = field(default_factory=dict)  # Moore
    output_alphabet: list[str] = field(default_factory=list)  # Moore/Mealy


def json_to_fsm(data: dict) -> FSM:
    """Parse JSON into FSM structure."""
    fsm = FSM(
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
    
    # Validate
    if len(fsm.states) > 65535:
        raise ValueError(f"Too many states: {len(fsm.states)} (max 65535)")
    if len(fsm.alphabet) > 65534:  # Reserve FFFF for epsilon
        raise ValueError(f"Too many input symbols: {len(fsm.alphabet)} (max 65534)")
    if len(fsm.output_alphabet) > 65535:
        raise ValueError(f"Too many output symbols: {len(fsm.output_alphabet)} (max 65535)")
    
    return fsm


def fsm_to_json(fsm: FSM) -> dict:
    """Convert FSM structure to JSON-serialisable dict."""
    data = {
        "type": fsm.fsm_type,
        "states": fsm.states,
        "alphabet": fsm.alphabet,
        "initial": fsm.initial,
        "accepting": fsm.accepting,
        "transitions": fsm.transitions
    }
    
    if fsm.name:
        data["name"] = fsm.name
    if fsm.description:
        data["description"] = fsm.description
    
    if fsm.fsm_type == "moore":
        data["state_outputs"] = fsm.state_outputs
        data["output_alphabet"] = fsm.output_alphabet
    elif fsm.fsm_type == "mealy":
        data["output_alphabet"] = fsm.output_alphabet
    
    return data


def format_record(rec_type: int, field1: int, field2: int, field3: int, field4: int) -> str:
    """Format a record as 'TYPE SSSS:IIII TTTT:OOOO'."""
    return f"{rec_type:04X} {field1:04X}:{field2:04X} {field3:04X}:{field4:04X}"


def parse_record(record: str) -> tuple[int, int, int, int, int]:
    """Parse a record from 'TYPE SSSS:IIII TTTT:OOOO' format."""
    clean = record.replace(" ", "").replace(":", "")
    if len(clean) != 20:
        raise ValueError(f"Invalid record length: {record} -> {clean} ({len(clean)} chars)")
    
    rec_type = int(clean[0:4], 16)
    field1 = int(clean[4:8], 16)
    field2 = int(clean[8:12], 16)
    field3 = int(clean[12:16], 16)
    field4 = int(clean[16:20], 16)
    
    return rec_type, field1, field2, field3, field4


def fsm_to_hex(fsm: FSM) -> tuple[list[str], dict[int, str], dict[int, str], dict[int, str]]:
    """Convert FSM to hex uint16 records. Returns (records, state_map, input_map, output_map)."""
    records = []
    
    # Build lookup tables
    state_idx = {s: i for i, s in enumerate(fsm.states)}
    input_idx = {s: i for i, s in enumerate(fsm.alphabet)}
    output_idx = {s: i for i, s in enumerate(fsm.output_alphabet)} if fsm.output_alphabet else {}
    
    # Reverse maps for labels
    state_names = {i: s for s, i in state_idx.items()}
    input_names = {i: s for s, i in input_idx.items()}
    output_names = {i: s for s, i in output_idx.items()}
    
    # Determine flags for each state
    state_flags = {}
    for s in fsm.states:
        flags = 0
        if s == fsm.initial:
            flags |= 0x1  # bit 0 = initial
        if s in fsm.accepting:
            flags |= 0x2  # bit 1 = accepting
        state_flags[s] = flags
    
    # Emit state declarations for Moore machines (always) or when flags needed
    if fsm.fsm_type == "moore":
        for state in fsm.states:
            sid = state_idx[state]
            flags = state_flags[state]
            state_out = fsm.state_outputs.get(state)
            if state_out is not None and state_out in output_idx:
                out = output_idx[state_out] + 1  # +1 so 0 means "no output"
            else:
                out = 0
            record = format_record(TYPE_STATE_DECL, sid, flags, out, 0)
            records.append(record)
    else:
        # For non-Moore, only emit state decls if there are flags
        for state in fsm.states:
            flags = state_flags[state]
            if flags != 0:
                sid = state_idx[state]
                record = format_record(TYPE_STATE_DECL, sid, flags, 0, 0)
                records.append(record)
    
    # Emit transitions
    for t in fsm.transitions:
        src = state_idx[t["from"]]
        
        # Handle epsilon transitions
        inp_sym = t.get("input")
        if inp_sym is None or inp_sym == "epsilon" or inp_sym == "ε":
            inp = EPSILON_INPUT
        else:
            inp = input_idx[inp_sym]
        
        targets = t["to"]
        if isinstance(targets, str):
            targets = [targets]
        
        if fsm.fsm_type == "mealy":
            tgt = state_idx[targets[0]]
            out = output_idx.get(t.get("output", ""), 0)
            record = format_record(TYPE_MEALY_TRANSITION, src, inp, tgt, out)
            records.append(record)
        
        elif len(targets) == 1:
            tgt = state_idx[targets[0]]
            record = format_record(TYPE_DFA_TRANSITION, src, inp, tgt, 0)
            records.append(record)
        
        else:
            for i, target in enumerate(targets):
                tgt = state_idx[target]
                cont = 1 if i < len(targets) - 1 else 0
                record = format_record(TYPE_NFA_MULTI, src, inp, tgt, cont)
                records.append(record)
    
    return records, state_names, input_names, output_names


def hex_to_fsm(records: list[str], labels: Optional[dict] = None) -> FSM:
    """Convert hex uint16 records to FSM, optionally using labels."""
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
    
    # Parse records
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
    
    # Override type from labels if present
    if fsm_meta.get("type"):
        fsm_type = fsm_meta["type"]
    
    # Generate symbol names (use labels if available, else default)
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
    
    # Convert transitions to use names
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


def format_hex_output(records: list[str], width: int = 4) -> str:
    """Format hex records into rows."""
    lines = []
    for i in range(0, len(records), width):
        row = records[i:i+width]
        lines.append("   ".join(row))
    return "\n".join(lines)


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


def generate_labels_toml(fsm: FSM, state_map: dict, input_map: dict, output_map: dict) -> str:
    """Generate labels.toml content."""
    lines = []
    
    lines.append("[fsm]")
    lines.append("version = 1")
    lines.append(f'type = "{fsm.fsm_type}"')
    if fsm.name:
        lines.append(f'name = "{fsm.name}"')
    if fsm.description:
        lines.append(f'description = "{fsm.description}"')
    lines.append("")
    
    if state_map:
        lines.append("[states]")
        for idx in sorted(state_map.keys()):
            name = state_map[idx]
            lines.append(f'0x{idx:04X} = "{name}"')
        lines.append("")
    
    if input_map:
        lines.append("[inputs]")
        for idx in sorted(input_map.keys()):
            name = input_map[idx]
            lines.append(f'0x{idx:04X} = "{name}"')
        lines.append("")
    
    if output_map:
        lines.append("[outputs]")
        for idx in sorted(output_map.keys()):
            name = output_map[idx]
            lines.append(f'0x{idx:04X} = "{name}"')
        lines.append("")
    
    return "\n".join(lines)


def parse_labels_toml(text: str) -> dict:
    """Parse labels.toml content."""
    if tomllib is not None:
        return tomllib.loads(text)
    # Fallback parser for basic TOML
    return parse_labels_toml_fallback(text)


def parse_labels_toml_fallback(text: str) -> dict:
    """Simple fallback TOML parser for labels.toml format."""
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
            
            # Handle hex keys
            if key.startswith("0x"):
                key = int(key, 16)
            
            # Handle numeric values
            if value.isdigit():
                value = int(value)
            
            result[current_section][key] = value
    
    return result


def write_fsm_file(path: str, records: list[str], labels_toml: Optional[str] = None):
    """Write .fsm zip file."""
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as zf:
        hex_content = format_hex_output(records) + "\n"
        zf.writestr("machine.hex", hex_content)
        
        if labels_toml:
            zf.writestr("labels.toml", labels_toml)


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


def main():
    parser = argparse.ArgumentParser(
        description="Convert FSM between JSON, hex, and .fsm formats",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
FSM File Format (.fsm):
  A zip file containing:
    - machine.hex    (required) Hex uint16 records
    - labels.toml    (optional) Human-readable names

Hex Format: TYPE SSSS:IIII TTTT:OOOO (20 hex chars + separators)

Examples:
  %(prog)s --to-fsm input.json -o output.fsm      # JSON to .fsm
  %(prog)s --to-json input.fsm -o output.json     # .fsm to JSON
  %(prog)s --to-hex input.json                     # JSON to raw hex
  %(prog)s --to-fsm input.json --no-labels        # .fsm without labels
        """
    )
    
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--to-fsm", metavar="FILE", help="Convert to .fsm format")
    group.add_argument("--to-json", metavar="FILE", help="Convert to JSON format")
    group.add_argument("--to-hex", metavar="FILE", help="Convert to raw hex format")
    
    parser.add_argument("-o", "--output", metavar="FILE", help="Output file (default: stdout for hex/json)")
    parser.add_argument("--pretty", action="store_true", help="Pretty-print JSON output")
    parser.add_argument("--no-labels", action="store_true", help="Omit labels.toml from .fsm output")
    parser.add_argument("--width", type=int, default=4, help="Records per line for hex output")
    
    args = parser.parse_args()
    
    try:
        if args.to_fsm:
            # Read input (JSON or existing .fsm)
            input_path = Path(args.to_fsm)
            
            if input_path.suffix == ".fsm":
                records, labels = read_fsm_file(args.to_fsm)
                fsm = hex_to_fsm(records, labels)
                records, state_map, input_map, output_map = fsm_to_hex(fsm)
            else:
                with open(args.to_fsm, "r") as f:
                    data = json.load(f)
                fsm = json_to_fsm(data)
                records, state_map, input_map, output_map = fsm_to_hex(fsm)
            
            # Generate labels
            labels_toml = None
            if not args.no_labels:
                labels_toml = generate_labels_toml(fsm, state_map, input_map, output_map)
            
            # Write output
            output_path = args.output or (input_path.stem + ".fsm")
            write_fsm_file(output_path, records, labels_toml)
            print(f"Written: {output_path}")
        
        elif args.to_json:
            input_path = Path(args.to_json)
            
            if input_path.suffix == ".fsm":
                records, labels = read_fsm_file(args.to_json)
                fsm = hex_to_fsm(records, labels)
            elif input_path.suffix == ".hex":
                with open(args.to_json, "r") as f:
                    text = f.read()
                records = parse_hex_input(text)
                fsm = hex_to_fsm(records)
            else:
                raise ValueError(f"Unknown input format: {input_path.suffix}")
            
            data = fsm_to_json(fsm)
            
            if args.pretty:
                output = json.dumps(data, indent=2)
            else:
                output = json.dumps(data)
            
            if args.output:
                with open(args.output, "w") as f:
                    f.write(output)
                    f.write("\n")
            else:
                print(output)
        
        elif args.to_hex:
            input_path = Path(args.to_hex)
            
            if input_path.suffix == ".fsm":
                records, labels = read_fsm_file(args.to_hex)
            elif input_path.suffix == ".json":
                with open(args.to_hex, "r") as f:
                    data = json.load(f)
                fsm = json_to_fsm(data)
                records, _, _, _ = fsm_to_hex(fsm)
            else:
                raise ValueError(f"Unknown input format: {input_path.suffix}")
            
            output = format_hex_output(records, args.width) + "\n"
            
            if args.output:
                with open(args.output, "w") as f:
                    f.write(output)
            else:
                print(output)
    
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
