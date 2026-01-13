#!/usr/bin/env python3
"""
Generate a 24-floor elevator FSM in hex uint16 format.

Format: TYPE SSSS:IIII TTTT:OOOO (20 hex chars + separators)

State encoding:
  index = (floor - 1) * 6 + direction * 2 + door
  floor: 1-24
  direction: 0=Idle, 1=Up, 2=Down
  door: 0=Closed, 1=Open

Inputs:
  0 = Timer/arrival
  1 = Door button
  2 = Go up command
  3 = Go down command
  4 = Emergency stop

Outputs:
  0 = None
  1 = Ding (arrived)
  2 = Door opening
  3 = Door closing
  4 = Motor up
  5 = Motor down
  6 = Alarm
"""

FLOORS = 24

# Direction constants
IDLE, UP, DOWN = 0, 1, 2

# Door constants
CLOSED, OPEN = 0, 1

# Input symbols
IN_TIMER = 0
IN_DOOR = 1
IN_GO_UP = 2
IN_GO_DOWN = 3
IN_EMERGENCY = 4

# Output symbols
OUT_NONE = 0
OUT_DING = 1
OUT_DOOR_OPEN = 2
OUT_DOOR_CLOSE = 3
OUT_MOTOR_UP = 4
OUT_MOTOR_DOWN = 5
OUT_ALARM = 6

# Record types
TYPE_MEALY = 0x0001
TYPE_STATE_DECL = 0x0002


def state_index(floor, direction, door):
    """Convert floor/direction/door to state index."""
    return (floor - 1) * 6 + direction * 2 + door


def format_record(rec_type, src, inp, tgt, out):
    """Format as 'TYPE SSSS:IIII TTTT:OOOO'."""
    return f"{rec_type:04X} {src:04X}:{inp:04X} {tgt:04X}:{out:04X}"


def generate_fsm():
    """Generate all elevator records."""
    records = []
    
    # State declaration for initial state (Floor 1, Idle, Closed)
    initial = state_index(1, IDLE, CLOSED)
    records.append(format_record(TYPE_STATE_DECL, initial, 0x0001, 0, 0))  # flag 0x0001 = initial
    
    # Generate transitions
    for floor in range(1, FLOORS + 1):
        # === IDLE/CLOSED state ===
        src = state_index(floor, IDLE, CLOSED)
        
        # Door button → open door
        tgt = state_index(floor, IDLE, OPEN)
        records.append(format_record(TYPE_MEALY, src, IN_DOOR, tgt, OUT_DOOR_OPEN))
        
        # Go up → start moving up (if not top floor)
        if floor < FLOORS:
            tgt = state_index(floor + 1, IDLE, CLOSED)  # Simplified: jump directly to next floor
            records.append(format_record(TYPE_MEALY, src, IN_GO_UP, tgt, OUT_MOTOR_UP))
        else:
            # At top, ignore
            records.append(format_record(TYPE_MEALY, src, IN_GO_UP, src, OUT_NONE))
        
        # Go down → start moving down (if not bottom floor)
        if floor > 1:
            tgt = state_index(floor - 1, IDLE, CLOSED)  # Simplified: jump directly to prev floor
            records.append(format_record(TYPE_MEALY, src, IN_GO_DOWN, tgt, OUT_MOTOR_DOWN))
        else:
            # At bottom, ignore
            records.append(format_record(TYPE_MEALY, src, IN_GO_DOWN, src, OUT_NONE))
        
        # Timer while idle/closed → stay (no-op)
        records.append(format_record(TYPE_MEALY, src, IN_TIMER, src, OUT_NONE))
        
        # Emergency while idle → just alarm (already stopped)
        records.append(format_record(TYPE_MEALY, src, IN_EMERGENCY, src, OUT_ALARM))
        
        # === IDLE/OPEN state ===
        src = state_index(floor, IDLE, OPEN)
        
        # Timer → auto-close door
        tgt = state_index(floor, IDLE, CLOSED)
        records.append(format_record(TYPE_MEALY, src, IN_TIMER, tgt, OUT_DOOR_CLOSE))
        
        # Door button → close door
        records.append(format_record(TYPE_MEALY, src, IN_DOOR, tgt, OUT_DOOR_CLOSE))
        
        # Go up/down while door open → ignore
        records.append(format_record(TYPE_MEALY, src, IN_GO_UP, src, OUT_NONE))
        records.append(format_record(TYPE_MEALY, src, IN_GO_DOWN, src, OUT_NONE))
        
        # Emergency while door open → close and alarm
        records.append(format_record(TYPE_MEALY, src, IN_EMERGENCY, tgt, OUT_ALARM))
    
    return records


def main():
    records = generate_fsm()
    
    print(f"# 24-Floor Elevator FSM (Simplified)")
    print(f"# States: {FLOORS * 2} (24 floors × 2 door states, idle only)")
    print(f"# Transitions: {len(records) - 1}")  # -1 for state decl
    print(f"# Format: TYPE SSSS:IIII TTTT:OOOO")
    print()
    
    # Print in rows of 4
    for i in range(0, len(records), 4):
        row = records[i:i+4]
        print("   ".join(row))
    
    print()
    print(f"# Total records: {len(records)}")
    print(f"# Size: {sum(len(r.replace(' ', '').replace(':', '')) for r in records)} hex chars")


if __name__ == "__main__":
    main()
