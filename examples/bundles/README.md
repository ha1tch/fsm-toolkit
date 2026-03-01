# Example Bundles

This directory contains example FSM bundles demonstrating linked states and hierarchical machine composition.

## Bundles

### sensor_monitor.fsm

**Embedded Sensor Monitoring System**

A 4-machine bundle for embedded sensor applications with calibration, sampling, and alarm handling:

| Machine | Description |
|---------|-------------|
| `sensor_monitor` | Main supervisor: init → calibrate → sample → process → alarm check |
| `calibration_seq` | Zero offset, gain adjustment, linearity check, NVRAM storage |
| `adc_sampler` | 8x oversampling with mux selection, settling time, averaging |
| `threshold_checker` | Hysteresis-based alarm detection to prevent oscillation |

Demonstrates embedded patterns:
- Hardware initialisation sequences
- ADC oversampling and averaging
- Calibration with non-volatile storage
- Alarm hysteresis (rising/falling thresholds)
- Fault handling and recovery

```bash
# Generate TinyGo code for microcontroller
fsm generate examples/bundles/sensor_monitor.fsm --all --lang tinygo

# Generate C headers
fsm generate examples/bundles/sensor_monitor.fsm --all --lang c
```

### http_validator.fsm

**HTTP Request Validation Pipeline**

A 4-machine bundle for validating HTTP requests in stages:

| Machine | Description |
|---------|-------------|
| `http_request` | Parent orchestrator - sequences URL, header, and body validation |
| `url_validator` | Validates URL format (scheme://host/path) |
| `header_validator` | Checks required headers (Host, Content-Type) |
| `body_validator` | Validates JSON body structure |

```bash
fsm machines examples/bundles/http_validator.fsm
fsm analyse examples/bundles/http_validator.fsm --all
fsm png examples/bundles/http_validator.fsm --all --native
```

### order_processing.fsm

**E-commerce Order Workflow**

A 3-machine bundle for processing orders with payment and shipping:

| Machine | Description |
|---------|-------------|
| `order_processor` | Main workflow: cart → checkout → processing → shipped → delivered |
| `payment_validator` | Card validation, balance check, charge processing |
| `address_validator` | Format, ZIP code, and deliverability checks |

```bash
fsm run examples/bundles/order_processing.fsm
# > checkout
# >> card_valid
# >> sufficient_funds
# >> charge_ok
# > (returned to order_processor)
```

### auth_mfa.fsm

**Multi-Factor Authentication**

A 3-machine bundle implementing MFA with password and TOTP:

| Machine | Description |
|---------|-------------|
| `auth_flow` | Orchestrates password → TOTP → authenticated flow |
| `password_checker` | Rate-limited password validation (3 attempts before lockout) |
| `totp_checker` | TOTP validation with time window tolerance |

Demonstrates:
- Rate limiting via state counting
- Lockout states
- Time-based validation windows

### doc_approval.fsm

**Document Approval Workflow**

A 3-machine bundle for multi-stage document approval:

| Machine | Description |
|---------|-------------|
| `doc_approval` | Main workflow: draft → review → management → approved |
| `reviewer_workflow` | Peer review with comments and revision cycles |
| `manager_workflow` | Budget and compliance checks before final sign-off |

Useful for:
- Content management systems
- Regulatory compliance workflows
- Multi-level authorization processes

## Usage

### View bundle contents

```bash
fsm machines examples/bundles/auth_mfa.fsm
```

### Analyse all machines

```bash
fsm analyse examples/bundles/order_processing.fsm --all
```

### Generate code for all machines

```bash
fsm generate examples/bundles/http_validator.fsm --all --lang go
```

### Run interactively

```bash
fsm run examples/bundles/auth_mfa.fsm
```

The `>>` prompt indicates you're in a child machine (delegated state).

### Edit with fsmedit

```bash
fsmedit examples/bundles/doc_approval.fsm
```

- Double-click linked states to dive into child machines
- `Ctrl+B` to navigate back
- Breadcrumb bar shows navigation path

### Render all machines

```bash
fsm png examples/bundles/order_processing.fsm --all --native
```

Creates separate PNG files for each machine.

## Creating Your Own Bundles

1. Create individual FSM files (JSON or .fsm format)
2. Add `linked_machines` to the parent:
   ```json
   {
     "linked_machines": {
       "validating": "validator_machine"
     }
   }
   ```
3. Bundle them:
   ```bash
   fsm bundle parent.fsm child1.fsm child2.fsm -o bundle.fsm
   ```
4. Validate links:
   ```bash
   fsm validate bundle.fsm --bundle
   ```

See [MACHINES.md](../../MACHINES.md) for complete documentation on linked states.
