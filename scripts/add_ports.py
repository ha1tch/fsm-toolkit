#!/usr/bin/env python3
"""Add port definitions to 74xx class library JSON files.

Uses standard 74xx DIP pinouts. Each port has:
  - name: pin function name
  - direction: input / output / bidir / power
  - pin_number: physical DIP pin number
  - group: sub-unit identifier (e.g., "GATE_A", "FF1")

Run from fsm-toolkit root: python3 scripts/add_ports.py
"""

import json
import os
import sys

# ─── Pinout database ─────────────────────────────────────────────

def P(name, direction, pin, group=""):
    """Shorthand for port dict."""
    p = {"name": name, "direction": direction, "pin_number": pin}
    if group:
        p["group"] = group
    return p

# Helper for standard quad 2-input gate pinout (7400, 7402, 7408, 7432, 7486)
# Pin layout: 1A,1B,1Y,2A,2B,2Y,GND,3Y,3A,3B,4Y,4A,4B,VCC
def quad_2input_gate(logic_fn):
    return [
        P("1A", "input",  1, "GATE_A"), P("1B", "input",  2, "GATE_A"),
        P("1Y", "output", 3, "GATE_A"),
        P("2A", "input",  4, "GATE_B"), P("2B", "input",  5, "GATE_B"),
        P("2Y", "output", 6, "GATE_B"),
        P("GND", "power", 7),
        P("3Y", "output", 8, "GATE_C"),
        P("3A", "input",  9, "GATE_C"), P("3B", "input", 10, "GATE_C"),
        P("4Y", "output", 11, "GATE_D"),
        P("4A", "input",  12, "GATE_D"), P("4B", "input", 13, "GATE_D"),
        P("VCC", "power", 14),
    ]

# 7402 quad NOR has *inverted* pin arrangement: Y,A,B instead of A,B,Y
def quad_2input_gate_7402():
    return [
        P("1Y", "output", 1, "GATE_A"),
        P("1A", "input",  2, "GATE_A"), P("1B", "input",  3, "GATE_A"),
        P("2Y", "output", 4, "GATE_B"),
        P("2A", "input",  5, "GATE_B"), P("2B", "input",  6, "GATE_B"),
        P("GND", "power", 7),
        P("3A", "input",  8, "GATE_C"), P("3B", "input",  9, "GATE_C"),
        P("3Y", "output", 10, "GATE_C"),
        P("4A", "input",  11, "GATE_D"), P("4B", "input", 12, "GATE_D"),
        P("4Y", "output", 13, "GATE_D"),
        P("VCC", "power", 14),
    ]

# Standard hex inverter pinout (7404, 7414)
def hex_inverter():
    return [
        P("1A", "input",  1, "INV_1"), P("1Y", "output", 2, "INV_1"),
        P("2A", "input",  3, "INV_2"), P("2Y", "output", 4, "INV_2"),
        P("3A", "input",  5, "INV_3"), P("3Y", "output", 6, "INV_3"),
        P("GND", "power", 7),
        P("4Y", "output", 8, "INV_4"), P("4A", "input",  9, "INV_4"),
        P("5Y", "output", 10, "INV_5"), P("5A", "input", 11, "INV_5"),
        P("6Y", "output", 12, "INV_6"), P("6A", "input", 13, "INV_6"),
        P("VCC", "power", 14),
    ]

# Hex buffer open-collector (7407, 7417) — same pinout as hex inverter
def hex_buffer_oc():
    return hex_inverter()  # same physical pinout, different logic function


PINOUTS = {
    # ─── Gates ────────────────────────────────────────────────
    "7400_quad_nand": quad_2input_gate("NAND"),
    "7402_quad_nor": quad_2input_gate_7402(),
    "7404_hex_inverter": hex_inverter(),
    "7408_quad_and": quad_2input_gate("AND"),
    "7432_quad_or": quad_2input_gate("OR"),
    "7486_quad_xor": quad_2input_gate("XOR"),

    # ─── Buffers ──────────────────────────────────────────────
    "7407_hex_buffer_oc": hex_buffer_oc(),
    "7417_hex_buffer_oc_hv": hex_buffer_oc(),

    # 74125: quad tristate, active-low enable
    "74125_quad_tristate_buffer": [
        P("1OE_N", "input", 1, "BUF_1"), P("1A", "input", 2, "BUF_1"),
        P("1Y", "output", 3, "BUF_1"),
        P("2OE_N", "input", 4, "BUF_2"), P("2A", "input", 5, "BUF_2"),
        P("2Y", "output", 6, "BUF_2"),
        P("GND", "power", 7),
        P("3Y", "output", 8, "BUF_3"),
        P("3A", "input", 9, "BUF_3"), P("3OE_N", "input", 10, "BUF_3"),
        P("4Y", "output", 11, "BUF_4"),
        P("4A", "input", 12, "BUF_4"), P("4OE_N", "input", 13, "BUF_4"),
        P("VCC", "power", 14),
    ],

    # 74126: quad tristate, active-high enable
    "74126_quad_tristate_buffer": [
        P("1OE", "input", 1, "BUF_1"), P("1A", "input", 2, "BUF_1"),
        P("1Y", "output", 3, "BUF_1"),
        P("2OE", "input", 4, "BUF_2"), P("2A", "input", 5, "BUF_2"),
        P("2Y", "output", 6, "BUF_2"),
        P("GND", "power", 7),
        P("3Y", "output", 8, "BUF_3"),
        P("3A", "input", 9, "BUF_3"), P("3OE", "input", 10, "BUF_3"),
        P("4Y", "output", 11, "BUF_4"),
        P("4A", "input", 12, "BUF_4"), P("4OE", "input", 13, "BUF_4"),
        P("VCC", "power", 14),
    ],

    # 74244: octal buffer, DIP-20
    "74244_octal_buffer": [
        P("1OE_N", "input", 1),
        P("1A0", "input", 2, "BUF_A"), P("2Y0", "output", 3, "BUF_B"),
        P("1A1", "input", 4, "BUF_A"), P("2Y1", "output", 5, "BUF_B"),
        P("1A2", "input", 6, "BUF_A"), P("2Y2", "output", 7, "BUF_B"),
        P("1A3", "input", 8, "BUF_A"), P("2Y3", "output", 9, "BUF_B"),
        P("GND", "power", 10),
        P("2A0", "input", 11, "BUF_B"), P("1Y0", "output", 12, "BUF_A"),
        P("2A1", "input", 13, "BUF_B"), P("1Y1", "output", 14, "BUF_A"),
        P("2A2", "input", 15, "BUF_B"), P("1Y2", "output", 16, "BUF_A"),
        P("2A3", "input", 17, "BUF_B"), P("1Y3", "output", 18, "BUF_A"),
        P("2OE_N", "input", 19),
        P("VCC", "power", 20),
    ],

    # 74245: octal bus transceiver, DIP-20
    "74245_octal_bus_transceiver": [
        P("DIR", "input", 1),
        P("A0", "bidir", 2, "SIDE_A"), P("B0", "bidir", 18, "SIDE_B"),
        P("A1", "bidir", 3, "SIDE_A"), P("B1", "bidir", 17, "SIDE_B"),
        P("A2", "bidir", 4, "SIDE_A"), P("B2", "bidir", 16, "SIDE_B"),
        P("A3", "bidir", 5, "SIDE_A"), P("B3", "bidir", 15, "SIDE_B"),
        P("A4", "bidir", 6, "SIDE_A"), P("B4", "bidir", 14, "SIDE_B"),
        P("A5", "bidir", 7, "SIDE_A"), P("B5", "bidir", 13, "SIDE_B"),
        P("A6", "bidir", 8, "SIDE_A"), P("B6", "bidir", 12, "SIDE_B"),
        P("A7", "bidir", 9, "SIDE_A"), P("B7", "bidir", 11, "SIDE_B"),
        P("GND", "power", 10),
        P("OE_N", "input", 19),
        P("VCC", "power", 20),
    ],

    # ─── Sequential ───────────────────────────────────────────

    # 7474: dual D flip-flop, DIP-14
    "7474_dual_d_flipflop": [
        P("1CLR_N", "input", 1, "FF1"), P("1D", "input", 2, "FF1"),
        P("1CLK", "input", 3, "FF1"), P("1PRE_N", "input", 4, "FF1"),
        P("1Q", "output", 5, "FF1"), P("1Q_N", "output", 6, "FF1"),
        P("GND", "power", 7),
        P("2Q_N", "output", 8, "FF2"), P("2Q", "output", 9, "FF2"),
        P("2PRE_N", "input", 10, "FF2"), P("2CLK", "input", 11, "FF2"),
        P("2D", "input", 12, "FF2"), P("2CLR_N", "input", 13, "FF2"),
        P("VCC", "power", 14),
    ],

    # 7476: dual JK flip-flop, DIP-16
    "7476_dual_jk_flipflop": [
        P("1CLK", "input", 1, "FF1"), P("1PRE_N", "input", 2, "FF1"),
        P("1CLR_N", "input", 3, "FF1"), P("1J", "input", 4, "FF1"),
        P("VCC", "power", 5),
        P("2CLK", "input", 6, "FF2"), P("2PRE_N", "input", 7, "FF2"),
        P("2CLR_N", "input", 8, "FF2"),
        P("2J", "input", 9, "FF2"), P("2Q_N", "output", 10, "FF2"),
        P("2Q", "output", 11, "FF2"), P("2K", "input", 12, "FF2"),
        P("GND", "power", 13),
        P("1Q_N", "output", 14, "FF1"), P("1Q", "output", 15, "FF1"),
        P("1K", "input", 16, "FF1"),
    ],

    # 7490: decade counter, DIP-14
    "7490_decade_counter": [
        P("CKB", "input", 1),
        P("R01", "input", 2), P("R02", "input", 3),
        P("NC1", "input", 4),
        P("VCC", "power", 5),
        P("R91", "input", 6), P("R92", "input", 7),
        P("QC", "output", 8), P("QB", "output", 9),
        P("GND", "power", 10),
        P("QD", "output", 11), P("QA", "output", 12),
        P("NC2", "input", 13),
        P("CKA", "input", 14),
    ],

    # 7493: 4-bit binary counter, DIP-14
    "7493_4bit_counter": [
        P("CKB", "input", 1),
        P("R01", "input", 2), P("R02", "input", 3),
        P("NC1", "input", 4),
        P("VCC", "power", 5),
        P("NC2", "input", 6), P("NC3", "input", 7),
        P("QC", "output", 8), P("QB", "output", 9),
        P("GND", "power", 10),
        P("QD", "output", 11), P("QA", "output", 12),
        P("NC4", "input", 13),
        P("CKA", "input", 14),
    ],

    # 74161: synchronous 4-bit counter, DIP-16
    "74161_sync_4bit_counter": [
        P("CLR_N", "input", 1),
        P("CLK", "input", 2),
        P("A", "input", 3), P("B", "input", 4),
        P("C", "input", 5), P("D", "input", 6),
        P("ENP", "input", 7),
        P("GND", "power", 8),
        P("LOAD_N", "input", 9),
        P("ENT", "input", 10),
        P("QD", "output", 11), P("QC", "output", 12),
        P("QB", "output", 13), P("QA", "output", 14),
        P("RCO", "output", 15),
        P("VCC", "power", 16),
    ],

    # 74164: 8-bit serial-in parallel-out shift register, DIP-14
    "74164_8bit_shift_register": [
        P("A", "input", 1), P("B", "input", 2),
        P("QA", "output", 3), P("QB", "output", 4),
        P("QC", "output", 5), P("QD", "output", 6),
        P("GND", "power", 7),
        P("CLK", "input", 8),
        P("CLR_N", "input", 9),
        P("QE", "output", 10), P("QF", "output", 11),
        P("QG", "output", 12), P("QH", "output", 13),
        P("VCC", "power", 14),
    ],

    # ─── Arithmetic ───────────────────────────────────────────

    # 7483: 4-bit full adder, DIP-16
    "7483_4bit_adder": [
        P("A4", "input", 1),
        P("S3", "output", 2), P("A3", "input", 3),
        P("B3", "input", 4), P("VCC", "power", 5),
        P("S2", "output", 6), P("B2", "input", 7),
        P("A2", "input", 8),
        P("S1", "output", 9), P("A1", "input", 10),
        P("B1", "input", 11), P("GND", "power", 12),
        P("C0", "input", 13),
        P("C4", "output", 14),
        P("S4", "output", 15), P("B4", "input", 16),
    ],

    # 74283: 4-bit fast adder, DIP-16 (same pinout as 7483)
    "74283_4bit_adder_fast": [
        P("S2", "output", 1), P("B2", "input", 2),
        P("A2", "input", 3), P("S1", "output", 4),
        P("A1", "input", 5), P("B1", "input", 6),
        P("C0", "input", 7),
        P("GND", "power", 8),
        P("C4", "output", 9),
        P("S4", "output", 10), P("B4", "input", 11),
        P("A4", "input", 12), P("S3", "output", 13),
        P("A3", "input", 14), P("B3", "input", 15),
        P("VCC", "power", 16),
    ],

    # 7485: 4-bit magnitude comparator, DIP-16
    "7485_4bit_comparator": [
        P("B3", "input", 1), P("LT_IN", "input", 2),
        P("EQ_IN", "input", 3), P("GT_IN", "input", 4),
        P("GT_OUT", "output", 5), P("EQ_OUT", "output", 6),
        P("LT_OUT", "output", 7),
        P("GND", "power", 8),
        P("B0", "input", 9), P("A0", "input", 10),
        P("B1", "input", 11), P("A1", "input", 12),
        P("A2", "input", 13), P("B2", "input", 14),
        P("A3", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74181: 4-bit ALU, DIP-24
    "74181_4bit_alu": [
        P("B0", "input", 1), P("A0", "input", 2),
        P("S3", "input", 3), P("S2", "input", 4),
        P("S1", "input", 5), P("S0", "input", 6),
        P("CN", "input", 7),
        P("M", "input", 8),
        P("F0", "output", 9), P("F1", "output", 10),
        P("F2", "output", 11),
        P("GND", "power", 12),
        P("F3", "output", 13),
        P("A_EQ_B", "output", 14),
        P("P_N", "output", 15), P("CN4", "output", 16),
        P("G_N", "output", 17),
        P("B3", "input", 18), P("A3", "input", 19),
        P("B2", "input", 20), P("A2", "input", 21),
        P("B1", "input", 22), P("A1", "input", 23),
        P("VCC", "power", 24),
    ],

    # 74182: carry lookahead generator, DIP-16
    "74182_carry_lookahead": [
        P("G1_N", "input", 1), P("P1_N", "input", 2),
        P("G0_N", "input", 3), P("P0_N", "input", 4),
        P("P3_N", "input", 5), P("G3_N", "input", 6),
        P("P2_N", "input", 7),
        P("GND", "power", 8),
        P("G2_N", "input", 9),
        P("CN_Z", "output", 10), P("G_N", "output", 11),
        P("CN_Y", "output", 12), P("CN_X", "output", 13),
        P("P_N", "output", 14),
        P("CN", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74180: 9-bit parity generator/checker, DIP-14
    "74180_parity_generator": [
        P("I0", "input", 1), P("I1", "input", 2),
        P("I2", "input", 3), P("I3", "input", 4),
        P("EVEN", "output", 5), P("ODD", "output", 6),
        P("GND", "power", 7),
        P("I7", "input", 8),
        P("I6", "input", 9), P("I5", "input", 10),
        P("I4", "input", 11),
        P("EVEN_IN", "input", 12), P("ODD_IN", "input", 13),
        P("VCC", "power", 14),
    ],

    # ─── Mux/Demux ────────────────────────────────────────────

    # 74150: 16-to-1 mux, DIP-24
    "74150_16to1_mux": [
        P("D7", "input", 1), P("D6", "input", 2),
        P("D5", "input", 3), P("D4", "input", 4),
        P("D3", "input", 5), P("D2", "input", 6),
        P("D1", "input", 7), P("D0", "input", 8),
        P("STROBE_N", "input", 9),
        P("W_N", "output", 10),
        P("D", "input", 11), P("GND", "power", 12),
        P("C", "input", 13), P("B", "input", 14),
        P("A", "input", 15),
        P("D15", "input", 16), P("D14", "input", 17),
        P("D13", "input", 18), P("D12", "input", 19),
        P("D11", "input", 20), P("D10", "input", 21),
        P("D9", "input", 22), P("D8", "input", 23),
        P("VCC", "power", 24),
    ],

    # 74151: 8-to-1 mux, DIP-16
    "74151_8to1_mux": [
        P("D3", "input", 1), P("D2", "input", 2),
        P("D1", "input", 3), P("D0", "input", 4),
        P("Y", "output", 5), P("W_N", "output", 6),
        P("STROBE_N", "input", 7),
        P("GND", "power", 8),
        P("C", "input", 9), P("B", "input", 10),
        P("A", "input", 11),
        P("D7", "input", 12), P("D6", "input", 13),
        P("D5", "input", 14), P("D4", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74153: dual 4-to-1 mux, DIP-16
    "74153_dual_4to1_mux": [
        P("1STROBE_N", "input", 1, "MUX_1"),
        P("B", "input", 2), P("1C3", "input", 3, "MUX_1"),
        P("1C2", "input", 4, "MUX_1"), P("1C1", "input", 5, "MUX_1"),
        P("1C0", "input", 6, "MUX_1"), P("1Y", "output", 7, "MUX_1"),
        P("GND", "power", 8),
        P("2Y", "output", 9, "MUX_2"), P("2C0", "input", 10, "MUX_2"),
        P("2C1", "input", 11, "MUX_2"), P("2C2", "input", 12, "MUX_2"),
        P("2C3", "input", 13, "MUX_2"),
        P("A", "input", 14),
        P("2STROBE_N", "input", 15, "MUX_2"),
        P("VCC", "power", 16),
    ],

    # 74157: quad 2-to-1 mux, DIP-16
    "74157_quad_2to1_mux": [
        P("SEL", "input", 1),
        P("1A", "input", 2, "MUX_1"), P("1B", "input", 3, "MUX_1"),
        P("1Y", "output", 4, "MUX_1"),
        P("2A", "input", 5, "MUX_2"), P("2B", "input", 6, "MUX_2"),
        P("2Y", "output", 7, "MUX_2"),
        P("GND", "power", 8),
        P("3Y", "output", 9, "MUX_3"),
        P("3B", "input", 10, "MUX_3"), P("3A", "input", 11, "MUX_3"),
        P("4Y", "output", 12, "MUX_4"),
        P("4B", "input", 13, "MUX_4"), P("4A", "input", 14, "MUX_4"),
        P("STROBE_N", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74138: 3-to-8 decoder, DIP-16
    "74138_3to8_decoder": [
        P("A", "input", 1), P("B", "input", 2), P("C", "input", 3),
        P("G2A_N", "input", 4), P("G2B_N", "input", 5),
        P("G1", "input", 6),
        P("Y7_N", "output", 7),
        P("GND", "power", 8),
        P("Y6_N", "output", 9), P("Y5_N", "output", 10),
        P("Y4_N", "output", 11), P("Y3_N", "output", 12),
        P("Y2_N", "output", 13), P("Y1_N", "output", 14),
        P("Y0_N", "output", 15),
        P("VCC", "power", 16),
    ],

    # 74139: dual 2-to-4 decoder, DIP-16
    "74139_dual_2to4_decoder": [
        P("1EN_N", "input", 1, "DEC_1"),
        P("1A0", "input", 2, "DEC_1"), P("1A1", "input", 3, "DEC_1"),
        P("1Y0_N", "output", 4, "DEC_1"), P("1Y1_N", "output", 5, "DEC_1"),
        P("1Y2_N", "output", 6, "DEC_1"), P("1Y3_N", "output", 7, "DEC_1"),
        P("GND", "power", 8),
        P("2Y3_N", "output", 9, "DEC_2"), P("2Y2_N", "output", 10, "DEC_2"),
        P("2Y1_N", "output", 11, "DEC_2"), P("2Y0_N", "output", 12, "DEC_2"),
        P("2A1", "input", 13, "DEC_2"), P("2A0", "input", 14, "DEC_2"),
        P("2EN_N", "input", 15, "DEC_2"),
        P("VCC", "power", 16),
    ],

    # 74154: 4-to-16 decoder, DIP-24
    "74154_4to16_decoder": [
        P("Y0_N", "output", 1), P("Y1_N", "output", 2),
        P("Y2_N", "output", 3), P("Y3_N", "output", 4),
        P("Y4_N", "output", 5), P("Y5_N", "output", 6),
        P("Y6_N", "output", 7), P("Y7_N", "output", 8),
        P("Y8_N", "output", 9), P("Y9_N", "output", 10),
        P("Y10_N", "output", 11),
        P("GND", "power", 12),
        P("Y11_N", "output", 13), P("Y12_N", "output", 14),
        P("Y13_N", "output", 15), P("Y14_N", "output", 16),
        P("Y15_N", "output", 17),
        P("G1_N", "input", 18), P("G2_N", "input", 19),
        P("D", "input", 20), P("C", "input", 21),
        P("B", "input", 22), P("A", "input", 23),
        P("VCC", "power", 24),
    ],

    # ─── Registers / Memory ───────────────────────────────────

    # 7475: quad bistable latch, DIP-16
    "7475_quad_latch": [
        P("1Q_N", "output", 1, "LATCH_12"),
        P("1D", "input", 2, "LATCH_12"), P("2D", "input", 3, "LATCH_12"),
        P("E12", "input", 4, "LATCH_12"),
        P("VCC", "power", 5),
        P("3D", "input", 6, "LATCH_34"), P("4D", "input", 7, "LATCH_34"),
        P("GND", "power", 8),
        P("4Q_N", "output", 9, "LATCH_34"), P("4Q", "output", 10, "LATCH_34"),
        P("3Q_N", "output", 11, "LATCH_34"), P("3Q", "output", 12, "LATCH_34"),
        P("E34", "input", 13, "LATCH_34"),
        P("2Q_N", "output", 14, "LATCH_12"), P("2Q", "output", 15, "LATCH_12"),
        P("1Q", "output", 16, "LATCH_12"),
    ],

    # 74373: octal transparent latch, DIP-20
    "74373_octal_latch": [
        P("OE_N", "input", 1),
        P("Q0", "output", 2), P("D0", "input", 3),
        P("D1", "input", 4), P("Q1", "output", 5),
        P("Q2", "output", 6), P("D2", "input", 7),
        P("D3", "input", 8), P("Q3", "output", 9),
        P("GND", "power", 10),
        P("LE", "input", 11),
        P("Q4", "output", 12), P("D4", "input", 13),
        P("D5", "input", 14), P("Q5", "output", 15),
        P("Q6", "output", 16), P("D6", "input", 17),
        P("D7", "input", 18), P("Q7", "output", 19),
        P("VCC", "power", 20),
    ],

    # 74374: octal D-type flip-flop, DIP-20
    "74374_octal_d_register": [
        P("OE_N", "input", 1),
        P("Q0", "output", 2), P("D0", "input", 3),
        P("D1", "input", 4), P("Q1", "output", 5),
        P("Q2", "output", 6), P("D2", "input", 7),
        P("D3", "input", 8), P("Q3", "output", 9),
        P("GND", "power", 10),
        P("CLK", "input", 11),
        P("Q4", "output", 12), P("D4", "input", 13),
        P("D5", "input", 14), P("Q5", "output", 15),
        P("Q6", "output", 16), P("D6", "input", 17),
        P("D7", "input", 18), P("Q7", "output", 19),
        P("VCC", "power", 20),
    ],

    # 74173: quad D-type register with tristate, DIP-16
    "74173_quad_d_register": [
        P("M", "input", 1), P("N", "input", 2),
        P("Q1", "output", 3), P("Q2", "output", 4),
        P("Q3", "output", 5), P("Q4", "output", 6),
        P("CLK", "input", 7),
        P("GND", "power", 8),
        P("G1_N", "input", 9), P("G2_N", "input", 10),
        P("CLR", "input", 11),
        P("D4", "input", 12), P("D3", "input", 13),
        P("D2", "input", 14), P("D1", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74194: 4-bit bidirectional shift register, DIP-16
    "74194_4bit_shift_register": [
        P("CLR_N", "input", 1),
        P("SR_SER", "input", 2),
        P("A", "input", 3), P("B", "input", 4),
        P("C", "input", 5), P("D", "input", 6),
        P("SL_SER", "input", 7),
        P("GND", "power", 8),
        P("S0", "input", 9), P("S1", "input", 10),
        P("CLK", "input", 11),
        P("QD", "output", 12), P("QC", "output", 13),
        P("QB", "output", 14), P("QA", "output", 15),
        P("VCC", "power", 16),
    ],

    # 74195: 4-bit parallel-access shift register, DIP-16
    "74195_4bit_shift_register_parallel": [
        P("CLR_N", "input", 1), P("J", "input", 2),
        P("K_N", "input", 3), P("A", "input", 4),
        P("B", "input", 5), P("C", "input", 6),
        P("D", "input", 7),
        P("GND", "power", 8),
        P("SH_LD_N", "input", 9), P("CLK", "input", 10),
        P("QD_N", "output", 11), P("QD", "output", 12),
        P("QC", "output", 13), P("QB", "output", 14),
        P("QA", "output", 15),
        P("VCC", "power", 16),
    ],

    # 74189: 64-bit RAM (16x4), DIP-16
    "74189_64bit_ram": [
        P("A0", "input", 1),
        P("CS_N", "input", 2), P("WE_N", "input", 3),
        P("D1", "input", 4), P("O1_N", "output", 5),
        P("D2", "input", 6), P("O2_N", "output", 7),
        P("GND", "power", 8),
        P("O3_N", "output", 9), P("D3", "input", 10),
        P("O4_N", "output", 11), P("D4", "input", 12),
        P("A3", "input", 13), P("A2", "input", 14),
        P("A1", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74170: 4x4 register file, DIP-16
    "74170_4x4_register_file": [
        P("D2", "input", 1), P("D3", "input", 2),
        P("D4", "input", 3), P("RB", "input", 4),
        P("RA", "input", 5),
        P("Q4", "output", 6), P("Q3", "output", 7),
        P("GND", "power", 8),
        P("Q2", "output", 9), P("Q1", "output", 10),
        P("GR_N", "input", 11), P("GW_N", "input", 12),
        P("WB", "input", 13), P("WA", "input", 14),
        P("D1", "input", 15),
        P("VCC", "power", 16),
    ],

    # ─── Specialty ────────────────────────────────────────────

    # 74147: 10-to-4 priority encoder, DIP-16
    "74147_10to4_priority_encoder": [
        P("D4_N", "input", 1), P("D5_N", "input", 2),
        P("D6_N", "input", 3), P("D7_N", "input", 4),
        P("D8_N", "input", 5),
        P("A2_N", "output", 6), P("A1_N", "output", 7),
        P("GND", "power", 8),
        P("A0_N", "output", 9), P("D9_N", "input", 10),
        P("D1_N", "input", 11), P("D2_N", "input", 12),
        P("D3_N", "input", 13), P("A3_N", "output", 14),
        P("NC", "input", 15),
        P("VCC", "power", 16),
    ],

    # 74148: 8-to-3 priority encoder, DIP-16
    "74148_8to3_priority_encoder": [
        P("D4_N", "input", 1), P("D5_N", "input", 2),
        P("D6_N", "input", 3), P("D7_N", "input", 4),
        P("EI_N", "input", 5),
        P("A2_N", "output", 6), P("A1_N", "output", 7),
        P("GND", "power", 8),
        P("A0_N", "output", 9),
        P("D0_N", "input", 10), P("D1_N", "input", 11),
        P("D2_N", "input", 12), P("D3_N", "input", 13),
        P("GS_N", "output", 14), P("EO_N", "output", 15),
        P("VCC", "power", 16),
    ],

    # 7447: BCD to 7-segment decoder (active low, open collector), DIP-16
    "7447_bcd_to_7seg_decoder": [
        P("B", "input", 1), P("C", "input", 2),
        P("LT_N", "input", 3), P("BI_N", "input", 4),
        P("RBI_N", "input", 5),
        P("D", "input", 6), P("A", "input", 7),
        P("GND", "power", 8),
        P("E_N", "output", 9), P("D_SEG_N", "output", 10),
        P("C_SEG_N", "output", 11), P("B_SEG_N", "output", 12),
        P("A_SEG_N", "output", 13),
        P("G_N", "output", 14), P("F_N", "output", 15),
        P("VCC", "power", 16),
    ],

    # 7448: BCD to 7-segment decoder (active high, internal pullup), DIP-16
    "7448_bcd_to_7seg_decoder_internal_pullup": [
        P("B", "input", 1), P("C", "input", 2),
        P("LT_N", "input", 3), P("BI_N", "input", 4),
        P("RBI_N", "input", 5),
        P("D", "input", 6), P("A", "input", 7),
        P("GND", "power", 8),
        P("E", "output", 9), P("D_SEG", "output", 10),
        P("C_SEG", "output", 11), P("B_SEG", "output", 12),
        P("A_SEG", "output", 13),
        P("G", "output", 14), P("F", "output", 15),
        P("VCC", "power", 16),
    ],

    # 7414: hex Schmitt trigger inverter, DIP-14
    "7414_hex_schmitt_inverter": hex_inverter(),

    # 74132: quad 2-input Schmitt trigger NAND, DIP-14
    "74132_quad_schmitt_nand": quad_2input_gate("NAND"),

    # 74121: monostable multivibrator, DIP-14
    "74121_monostable": [
        P("Q_N", "output", 1),
        P("NC1", "input", 2),
        P("A1_N", "input", 3), P("A2_N", "input", 4),
        P("B", "input", 5),
        P("Q", "output", 6),
        P("GND", "power", 7),
        P("NC2", "input", 8),
        P("RINT", "input", 9),
        P("CEXT", "input", 10), P("REXT_CEXT", "input", 11),
        P("NC3", "input", 12), P("NC4", "input", 13),
        P("VCC", "power", 14),
    ],

    # 74123: dual retriggerable monostable, DIP-16
    "74123_dual_monostable": [
        P("1A_N", "input", 1, "MONO_1"), P("1B", "input", 2, "MONO_1"),
        P("1CLR_N", "input", 3, "MONO_1"), P("1Q_N", "output", 4, "MONO_1"),
        P("2Q", "output", 5, "MONO_2"),
        P("2CEXT", "input", 6, "MONO_2"), P("2REXT_CEXT", "input", 7, "MONO_2"),
        P("GND", "power", 8),
        P("2A_N", "input", 9, "MONO_2"), P("2B", "input", 10, "MONO_2"),
        P("2CLR_N", "input", 11, "MONO_2"), P("2Q_N", "output", 12, "MONO_2"),
        P("1Q", "output", 13, "MONO_1"),
        P("1CEXT", "input", 14, "MONO_1"), P("1REXT_CEXT", "input", 15, "MONO_1"),
        P("VCC", "power", 16),
    ],

    # 7401: quad 2-input NAND, open collector, DIP-14 (same pinout as 7402 layout)
    "7401_quad_nand_oc": quad_2input_gate_7402(),

    # 7403: quad 2-input NAND, open collector, DIP-14
    "7403_quad_nand_oc": quad_2input_gate("NAND"),
}


# ─── Merge logic ──────────────────────────────────────────────

def add_ports_to_library(filepath):
    """Read a .classes.json file, add ports from the database, write back."""
    with open(filepath, 'r') as f:
        library = json.load(f)

    modified = False
    missing = []

    for class_name, class_def in library.items():
        if class_name in PINOUTS:
            class_def["ports"] = PINOUTS[class_name]
            modified = True
        else:
            missing.append(class_name)

    if missing:
        print(f"  WARNING: no pinout data for: {', '.join(missing)}")

    if modified:
        with open(filepath, 'w') as f:
            json.dump(library, f, indent=2)
            f.write('\n')
        return True
    return False


def main():
    lib_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                           "class-libraries")
    if not os.path.isdir(lib_dir):
        # Try relative path
        lib_dir = "class-libraries"
    if not os.path.isdir(lib_dir):
        print(f"ERROR: class-libraries directory not found", file=sys.stderr)
        sys.exit(1)

    total_components = 0
    total_ported = 0

    for filename in sorted(os.listdir(lib_dir)):
        if not filename.endswith('.classes.json'):
            continue
        filepath = os.path.join(lib_dir, filename)
        print(f"Processing {filename}...")

        with open(filepath, 'r') as f:
            library = json.load(f)

        count = len(library)
        ported = sum(1 for name in library if name in PINOUTS)
        total_components += count
        total_ported += ported

        if add_ports_to_library(filepath):
            print(f"  Updated: {ported}/{count} components now have ports")
        else:
            print(f"  No changes")

    print(f"\nDone: {total_ported}/{total_components} components have port definitions")

    # Validate: check for duplicate pin names within each component
    errors = 0
    for name, ports in PINOUTS.items():
        seen = set()
        for p in ports:
            if p["name"] in seen:
                print(f"  ERROR: duplicate port name {p['name']!r} in {name}")
                errors += 1
            seen.add(p["name"])
    if errors:
        print(f"\n{errors} validation errors found!")
        sys.exit(1)
    else:
        print("All port names validated (no duplicates)")


if __name__ == "__main__":
    main()
