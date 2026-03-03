# Design Philosophy

This document explains what the FSM Toolkit is, what it is not, and how
to think about its architecture when using it for real work.


## The Core Idea

The toolkit treats finite state machines as **authoritative executable
specifications**, not as diagrams that get reimplemented in code.

In most projects, an FSM starts as a whiteboard sketch or a state
diagram in a document. Then someone translates it into a switch
statement or a state table in C, or Rust, or Go. The diagram and the
code coexist, and over time they drift apart. The diagram becomes
aspirational; the code becomes the truth. When a bug is found, nobody
knows whether the diagram was wrong or the translation was wrong.

The toolkit eliminates that gap. The `.fsm` file is the machine. You
validate it, simulate it, analyse it, render it, and generate code from
it. The generated code is a compiled artefact — a rendering of the
specification in a target language. If the specification is correct, the
code is correct by construction.

This is the shift from *descriptive model* to *authoritative source of
truth*, and everything else in the toolkit follows from it.


## Two Planes, Not a Ladder

The toolkit models two aspects of the same system:

**Behaviour.** States, transitions, inputs, outputs. This is the
classical FSM: what the system does in response to events, and what
sequence of states it passes through.

**Structure.** Components, ports, nets, pin connections. This is the
physical realisation: what chips are used, how they are wired, what
signals flow between them.

These are not layers in a hierarchy. They are parallel planes — two
different projections of the same underlying design. Neither is "above"
or "below" the other. A traffic light controller is pure behaviour
with no structural content. A 74xx gate array might be pure structure
with trivial behaviour. A security code lock is both: meaningful state
logic AND a physical circuit with real wiring.

The data model reflects this. Behaviour (states, transitions) and
structure (classes, ports, nets) coexist in the same file without
either depending on the other. You can remove all the nets and still
have a valid FSM. You can remove all the transitions and still have a
valid netlist. Both views are always available; neither is mandatory.


## The Vocabulary System

Because behaviour and structure are parallel projections of the same
model, the toolkit does not force you to think in one vocabulary. The
same data can present itself as:

| Vocabulary | "State" | "Transition" | "Input" |
|------------|---------|-------------|---------|
| `fsm` | State | Transition | Input |
| `circuit` | Component | Connection | Signal |
| `generic` | Node | Edge | Label |

This is not cosmetic relabelling. It reflects a genuine architectural
fact: a state machine and a circuit network are not two different things
the toolkit can model. They are two ways of looking at the same thing.
A "state" and a "component" are the same abstraction — an entity with
identity, properties, and connections. The vocabulary controls which
lens you see it through.

Set `"vocabulary": "auto"` in your file and the toolkit will choose
based on what the file contains. If it has classes with ports or nets,
you see circuit terminology. If not, you see FSM terminology. The data
doesn't change; the presentation does.


## Composition and Parallelism

A single FSM is useful but limited. Real systems are built from
multiple interacting controllers, not one monolithic state machine.

The toolkit addresses this through **bundles**: a single `.fsm` file
containing multiple machines that can operate independently or
interact through **linked-state delegation**.

This is worth understanding carefully, because it has implications
beyond simple modularity.

When two machines in a bundle have no transition dependencies — they
consume different inputs, they evolve through their own states, they
don't delegate to each other — they are **semantically parallel**.
Nothing in the toolkit forces them into a single execution thread or
a global ordering. They are independent transition systems that happen
to be packaged together.

This is how hardware thinks about parallelism. On a circuit board,
every chip runs simultaneously. There is no scheduler. There is no
thread pool. Modules simply evolve according to their own logic, and
coordination happens through physical connections (nets), not through
software synchronisation primitives.

The toolkit follows the same model. Parallelism is not a feature you
opt into with a keyword. It is an emergent property of structural
independence. Two machines that don't interact are parallel by
default. Two machines connected by a linked state have a defined
synchronisation point. The structure of the bundle determines the
concurrency model.

This means the toolkit can express systems that are richer than a
single FSM without introducing the complexity of explicit concurrency
(threads, actors, channels, schedulers). If you need two controllers
that operate independently, you put two machines in a bundle. If you
need them to synchronise at a specific point, you use a linked state.
The result is a network of deterministic controllers with well-defined
interaction boundaries.


## What It Optimises For

The toolkit is deliberately optimised for a specific class of problems:

**Deterministic finite control logic.**

This includes protocol handlers, embedded controllers, hardware state
machines, test harness orchestrators, input parsers, and anything else
where the system's behaviour can be described as a finite set of states
with explicit transitions between them.

The key constraints are intentional:

**Finite states.** Up to 65,535 states, but always bounded. No dynamic
state creation, no unbounded growth. This guarantees that every
reachable configuration can be enumerated and analysed.

**Finite inputs and outputs.** The alphabet is fixed at design time.
New events don't appear at runtime. This makes the transition function
total and inspectable.

**Deterministic execution.** For DFAs, Moore, and Mealy machines, every
(state, input) pair has exactly one successor. The machine's behaviour
is fully predictable from its specification.

**No general computation.** The toolkit does not model arbitrary
algorithms, unbounded data structures, or dynamic memory. Those belong
in the host language. The FSM handles control flow; the host code
handles everything else.

These constraints are not limitations to work around. They are the
source of every useful property the toolkit provides: the ability to
validate structure before execution, to prove reachability, to detect
dead states, to generate correct code in multiple languages, to export
pin-accurate netlists. None of that would be possible if the model
allowed unbounded behaviour.


## The File Format as Architecture

The `.fsm` file format is a ZIP archive with optional members:

```
machine.fsm
 +-- machine.hex       (required: state/transition data)
 +-- labels.toml       (optional: human-readable names)
 +-- layout.toml       (optional: editor positions)
 +-- classes.json      (optional: class, port, net data)
```

This structure implements a **zero-cost abstraction** at the file
format level. A simple 3-state traffic light contains only `machine.hex`
and `labels.toml`. It pays nothing for the class system, the port
model, the net model, or the circuit features. Those capabilities
exist in the toolkit but are not present in the file.

When you add a class with ports, `classes.json` appears. When you
add nets, they go into `classes.json`. The file grows only when you
use structural features. This is why the dual model (behaviour +
structure) works in practice: users who don't need circuits never
encounter circuit complexity.

Bundles extend the same pattern. A bundle is a ZIP containing
multiple machines, each with its own set of optional members. The
format scales from a single 3-state DFA to a multi-machine system
with structural connectivity, without changing the container structure.


## What It Is Not

The toolkit is not a schematic editor. It does not do wire routing,
design rule checking, electrical simulation, or PCB layout. It
maintains a pin-accurate structural description and exports it in
formats that dedicated EDA tools can import.

It is not a programming language. It does not support arbitrary
computation, dynamic allocation, or unbounded loops. It generates
code in real languages; it does not replace them.

It is not a formal verification environment. It provides structural
validation (reachability, completeness, determinism) but does not
prove temporal properties, liveness, or fairness. It tells you
whether your machine is well-formed, not whether it satisfies an
arbitrary specification.

It is not a real-time system modeller. Transitions are logically
instantaneous. There is no built-in notion of clock cycles, timing
constraints, or propagation delay. If you need those, they belong in
the simulation or synthesis tool downstream.

These boundaries are deliberate. The toolkit does one thing well:
it is the authoritative representation of deterministic finite control
logic, bridging the gap between abstract behaviour and concrete
implementation.


## Where It Sits

If you think of system modelling as a space with two axes —
behavioural expressiveness and structural detail — the toolkit
occupies a specific and deliberate region:

**Behavioural axis.** It covers single deterministic machines
completely, and extends into networks of composed machines with
implicit parallelism through structural independence. It does not
attempt to formalise arbitrary concurrency, real-time constraints,
or unbounded computation.

**Structural axis.** It covers pin-accurate component descriptions
with typed ports and named nets, sufficient to produce importable
netlists for real EDA tools. It does not attempt schematic capture,
placement, routing, or physical simulation.

The intersection of these two axes — where behaviour meets structure
in the same model — is where the toolkit is unique. You can simulate
a state machine's logic and then export the physical wiring of the
circuit that implements it, from the same file, without any
translation step.

That intersection is narrow, but for the domains it covers — retro
computing, digital logic education, hobby electronics, embedded
control, protocol design — it is exactly the right place to be.


## See Also

- [Circuits](circuits.md) — The structural connectivity model in detail
- [Workflows](workflows.md) — How the tools fit together in practice
- [Machines](machines.md) — Bundles, linked states, and composition
- [Specification](specification.md) — File format and formal semantics
