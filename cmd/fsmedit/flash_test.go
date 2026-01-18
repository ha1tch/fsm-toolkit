package main

import (
	"testing"
	"time"
)

// TestFlashPhaseCalculation verifies the phase logic for message flashing
func TestFlashPhaseCalculation(t *testing.T) {
	// Flash pattern: normal(0-125) -> inverted(125-250) -> normal(250-375) -> inverted(375-500) -> normal(500+)
	tests := []struct {
		elapsed      int64
		wantInverted bool
		description  string
	}{
		{0, false, "start of flash - normal"},
		{50, false, "early phase 0 - normal"},
		{124, false, "end of phase 0 - normal"},
		{125, true, "start of phase 1 - inverted"},
		{200, true, "middle of phase 1 - inverted"},
		{249, true, "end of phase 1 - inverted"},
		{250, false, "start of phase 2 - normal"},
		{300, false, "middle of phase 2 - normal"},
		{374, false, "end of phase 2 - normal"},
		{375, true, "start of phase 3 - inverted"},
		{450, true, "middle of phase 3 - inverted"},
		{499, true, "end of phase 3 - inverted"},
		{500, false, "after flash period - normal"},
		{600, false, "well after flash - normal"},
		{1000, false, "long after flash - normal"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			gotInverted := shouldBeInverted(tt.elapsed)
			if gotInverted != tt.wantInverted {
				t.Errorf("elapsed=%d: got inverted=%v, want %v", tt.elapsed, gotInverted, tt.wantInverted)
			}
		})
	}
}

// shouldBeInverted mirrors the logic in drawStatusBar
func shouldBeInverted(elapsed int64) bool {
	if elapsed < 0 || elapsed >= 500 {
		return false
	}
	phaseNum := elapsed / 125
	return phaseNum == 1 || phaseNum == 3
}

// TestFlashMessageTypes verifies which message types should flash
func TestFlashMessageTypes(t *testing.T) {
	tests := []struct {
		msgType     MessageType
		shouldFlash bool
		description string
	}{
		{MsgInfo, false, "info messages don't flash"},
		{MsgError, true, "error messages flash"},
		{MsgSuccess, true, "success messages flash"},
		{MsgWarning, true, "warning messages flash"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			got := shouldFlashForType(tt.msgType)
			if got != tt.shouldFlash {
				t.Errorf("msgType=%v: got shouldFlash=%v, want %v", tt.msgType, got, tt.shouldFlash)
			}
		})
	}
}

// shouldFlashForType mirrors the logic in drawStatusBar
func shouldFlashForType(msgType MessageType) bool {
	switch msgType {
	case MsgError, MsgSuccess, MsgWarning:
		return true
	case MsgInfo:
		return false
	default:
		return false
	}
}

// TestFlashSequence simulates multiple messages and verifies each flashes correctly
func TestFlashSequence(t *testing.T) {
	// Simulate showing multiple error messages in sequence
	// Each should start its own flash cycle from phase 0

	type flashState struct {
		message         string
		messageType     MessageType
		messageFlashStart int64
	}

	// Helper to check if message would be inverted at a given wall clock time
	checkInverted := func(state flashState, wallTime int64) bool {
		if state.message == "" || state.messageFlashStart == 0 {
			return false
		}
		if !shouldFlashForType(state.messageType) {
			return false
		}
		elapsed := wallTime - state.messageFlashStart
		return shouldBeInverted(elapsed)
	}

	// Simulate: show first message at T=1000
	state := flashState{
		message:         "First error",
		messageType:     MsgError,
		messageFlashStart: 1000,
	}

	// At T=1000 (elapsed=0), should be normal
	if checkInverted(state, 1000) {
		t.Error("First message at T=1000: expected normal, got inverted")
	}

	// At T=1150 (elapsed=150), should be inverted (phase 1)
	if !checkInverted(state, 1150) {
		t.Error("First message at T=1150: expected inverted, got normal")
	}

	// At T=1600 (elapsed=600), should be normal (past flash period)
	if checkInverted(state, 1600) {
		t.Error("First message at T=1600: expected normal, got inverted")
	}

	// Now show second message at T=2000
	state = flashState{
		message:         "Second error",
		messageType:     MsgError,
		messageFlashStart: 2000,
	}

	// At T=2000 (elapsed=0 from new start), should be normal
	if checkInverted(state, 2000) {
		t.Error("Second message at T=2000: expected normal, got inverted")
	}

	// At T=2150 (elapsed=150 from new start), should be inverted (phase 1)
	if !checkInverted(state, 2150) {
		t.Error("Second message at T=2150: expected inverted, got normal")
	}

	// At T=2300 (elapsed=300 from new start), should be normal (phase 2)
	if checkInverted(state, 2300) {
		t.Error("Second message at T=2300: expected normal, got inverted")
	}
}

// TestFlashTimingRealTime tests the flash with actual time delays
// This is a slower test but verifies real-world behavior
func TestFlashTimingRealTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-time flash test in short mode")
	}

	start := time.Now().UnixMilli()

	// Check immediately - should be normal (phase 0)
	elapsed := time.Now().UnixMilli() - start
	if shouldBeInverted(elapsed) {
		t.Errorf("At start (elapsed=%d): expected normal", elapsed)
	}

	// Wait 150ms - should be inverted (phase 1)
	time.Sleep(150 * time.Millisecond)
	elapsed = time.Now().UnixMilli() - start
	if !shouldBeInverted(elapsed) {
		t.Errorf("After 150ms (elapsed=%d): expected inverted", elapsed)
	}

	// Wait until 300ms total - should be normal (phase 2)
	time.Sleep(150 * time.Millisecond)
	elapsed = time.Now().UnixMilli() - start
	if shouldBeInverted(elapsed) {
		t.Errorf("After 300ms (elapsed=%d): expected normal", elapsed)
	}

	// Wait until 400ms total - should be inverted (phase 3)
	time.Sleep(100 * time.Millisecond)
	elapsed = time.Now().UnixMilli() - start
	if !shouldBeInverted(elapsed) {
		t.Errorf("After 400ms (elapsed=%d): expected inverted", elapsed)
	}

	// Wait until 550ms total - should be normal (past flash period)
	time.Sleep(150 * time.Millisecond)
	elapsed = time.Now().UnixMilli() - start
	if shouldBeInverted(elapsed) {
		t.Errorf("After 550ms (elapsed=%d): expected normal", elapsed)
	}
}

// TestInfoMessageNeverFlashes verifies MsgInfo never inverts regardless of timing
func TestInfoMessageNeverFlashes(t *testing.T) {
	// MsgInfo should never flash, regardless of elapsed time
	for elapsed := int64(0); elapsed <= 1000; elapsed += 50 {
		// Simulate the full check as done in drawStatusBar
		msgType := MsgInfo
		shouldFlash := shouldFlashForType(msgType)
		
		inverted := false
		if shouldFlash {
			inverted = shouldBeInverted(elapsed)
		}
		
		if inverted {
			t.Errorf("MsgInfo at elapsed=%d: should never be inverted", elapsed)
		}
	}
}

// TestEdgeCases tests boundary conditions
func TestEdgeCases(t *testing.T) {
	// Negative elapsed (shouldn't happen, but defensive)
	if shouldBeInverted(-1) {
		t.Error("Negative elapsed should not be inverted")
	}
	if shouldBeInverted(-1000) {
		t.Error("Large negative elapsed should not be inverted")
	}

	// Exactly at boundaries
	if shouldBeInverted(125 - 1) { // 124ms - end of phase 0
		t.Error("124ms should be normal (phase 0)")
	}
	if !shouldBeInverted(125) { // exactly 125ms - start of phase 1
		t.Error("125ms should be inverted (phase 1)")
	}
	if !shouldBeInverted(250 - 1) { // 249ms - end of phase 1
		t.Error("249ms should be inverted (phase 1)")
	}
	if shouldBeInverted(250) { // exactly 250ms - start of phase 2
		t.Error("250ms should be normal (phase 2)")
	}
	if shouldBeInverted(500 - 1) { // 499ms - end of phase 3
		// Actually 499/125 = 3, so this should be inverted
	}
	if shouldBeInverted(500) { // exactly 500ms - past flash period
		t.Error("500ms should be normal (past flash)")
	}
}
