package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// ====================================================================
// Vocabulary system — delegates to pkg/fsm
// ====================================================================

// Vocab returns the current vocabulary labels. Delegates to the FSM's
// own Vocab() method, which handles "auto" detection and fallback.
func (ed *Editor) Vocab() fsm.VocabLabels {
	if ed.fsm != nil {
		return ed.fsm.Vocab()
	}
	return fsm.Vocabularies[fsm.VocabFSM]
}

// syncVocabularyFromConfig applies the config's vocabulary setting to
// the FSM when the FSM's vocabulary is empty (not set by the file).
// Called after loading a file or creating a new machine.
func (ed *Editor) syncVocabularyFromConfig() {
	if ed.fsm != nil && ed.fsm.Vocabulary == "" && ed.config.Vocabulary != "" {
		ed.fsm.Vocabulary = ed.config.Vocabulary
	}
}

// ====================================================================
// Settings screen
// ====================================================================

// settingsItem describes one row in the settings screen.
type settingsItem struct {
	Label       string   // display label
	Key         string   // internal key for identification
	Values      []string // possible values
	CurrentIdx  int      // index of current value
}

func (ed *Editor) buildSettingsItems() []settingsItem {
	items := []settingsItem{
		{
			Label:  "Renderer",
			Key:    "renderer",
			Values: []string{"native", "graphviz"},
		},
		{
			Label:  "File Type",
			Key:    "file_type",
			Values: []string{"png", "svg"},
		},
		{
			Label:  "FSM Type",
			Key:    "fsm_type",
			Values: []string{"DFA", "NFA", "Mealy", "Moore"},
		},
		{
			Label:  "Vocabulary",
			Key:    "vocabulary",
			Values: fsm.VocabNames(), // fsm, circuit, generic, auto
		},
		{
			Label:  "Class Library Dir",
			Key:    "class_lib_dir",
			Values: nil, // text input, not a cycle
		},
	}

	// Set current indices.
	for i := range items {
		switch items[i].Key {
		case "renderer":
			for j, v := range items[i].Values {
				if v == ed.config.Renderer {
					items[i].CurrentIdx = j
				}
			}
		case "file_type":
			for j, v := range items[i].Values {
				if v == ed.config.FileType {
					items[i].CurrentIdx = j
				}
			}
		case "fsm_type":
			typeName := fsmTypeDisplayName(ed.fsm.Type)
			for j, v := range items[i].Values {
				if v == typeName {
					items[i].CurrentIdx = j
				}
			}
		case "vocabulary":
			vocabVal := ""
			if ed.fsm != nil {
				vocabVal = ed.fsm.Vocabulary
			}
			if vocabVal == "" {
				vocabVal = ed.config.Vocabulary
			}
			for j, v := range items[i].Values {
				if v == vocabVal {
					items[i].CurrentIdx = j
				}
			}
		}
	}

	return items
}

func (ed *Editor) openSettings() {
	ed.settingsCursor = 0
	ed.mode = ModeSettings
}

func (ed *Editor) drawSettings(w, h int) {
	items := ed.buildSettingsItems()

	boxW := 60
	boxH := len(items)*2 + 6
	if boxH > h-4 {
		boxH = h - 4
	}
	if boxH < 12 {
		boxH = 12
	}

	cx, cy, cw, ch := ed.drawOverlayBox("SETTINGS", boxW, boxH, w, h)
	_ = ch

	y := cy + 1

	for i, item := range items {
		isCurrent := (i == ed.settingsCursor)

		// Label.
		labelStyle := styleOverlayDim
		if isCurrent {
			labelStyle = styleOverlayHdr
		}
		ed.drawString(cx, y, item.Label+":", labelStyle)

		// Value.
		valX := cx + 22
		if item.Values != nil {
			// Cycle-style value.
			valStr := item.Values[item.CurrentIdx]
			s := styleOverlay
			if isCurrent {
				s = styleOverlayHl
			}
			// Show arrows for cycleable items.
			display := "< " + valStr + " >"
			ed.drawString(valX, y, display, s)
		} else {
			// Text value (class_lib_dir).
			val := ed.config.ClassLibDir
			if val == "" {
				val = "(not set)"
			}
			maxW := cw - 24
			if len(val) > maxW && maxW > 3 {
				val = "..." + val[len(val)-(maxW-3):]
			}
			s := styleOverlay
			if isCurrent {
				s = styleOverlayHl
			}
			ed.drawString(valX, y, val, s)
		}
		y += 2
	}

	// Vocabulary preview.
	if ed.settingsCursor == 3 { // vocabulary row
		vocab := ed.Vocab()
		y++
		if y < cy+ch-2 {
			preview := vocab.States + " / " + vocab.Transition + " / " + vocab.Alphabet
			if ed.fsm != nil && ed.fsm.Vocabulary == fsm.VocabAuto {
				resolved := ed.fsm.ResolvedVocabulary()
				preview = "(" + resolved + ") " + preview
			}
			ed.drawString(cx+2, y, "Preview: "+preview, styleOverlayDim)
		}
	}

	// Class lib status.
	if ed.settingsCursor == 4 { // class_lib_dir row
		if ed.config.ClassLibDir != "" {
			count := countClassLibFiles(ed.config.ClassLibDir)
			if y+1 < cy+ch-2 {
				y++
				label := "No .classes.json files found"
				if count > 0 {
					label = strings.Replace(
						strings.Replace("N files found", "N", string(rune('0'+count)), 1),
						string(rune('0'+count)), intToStr(count), 1)
				}
				ed.drawString(cx+2, y, label, styleOverlayDim)
			}
		}
	}

	// Help text.
	helpY := cy + ch - 1
	ed.drawString(cx, helpY, "[</>] Change  [Enter] Browse dir  [L] Load libs  [C] Classes  [Esc] Done", styleOverlayDim)
}

func (ed *Editor) handleSettingsKey(ev *tcell.EventKey) bool {
	items := ed.buildSettingsItems()

	switch ev.Key() {
	case tcell.KeyEscape:
		SaveConfig(ed.config)
		ed.updateMenuItems()
		ed.mode = ModeMenu
		return false
	case tcell.KeyUp:
		if ed.settingsCursor > 0 {
			ed.settingsCursor--
		}
	case tcell.KeyDown:
		if ed.settingsCursor < len(items)-1 {
			ed.settingsCursor++
		}
	case tcell.KeyLeft:
		ed.cycleSettingValue(items, -1)
	case tcell.KeyRight:
		ed.cycleSettingValue(items, 1)
	case tcell.KeyEnter:
		if ed.settingsCursor < len(items) {
			item := items[ed.settingsCursor]
			if item.Values != nil {
				// Cycle forward on Enter for cycle items.
				ed.cycleSettingValue(items, 1)
			} else if item.Key == "class_lib_dir" {
				// Prompt for path.
				ed.promptClassLibDir()
			}
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'l', 'L':
			// Load class libraries from configured directory.
			if ed.config.ClassLibDir != "" {
				ed.loadClassLibraries()
			} else {
				ed.showMessage("Set Class Library Dir first", MsgError)
			}
		case 'c', 'C':
			// Open Class Editor.
			ed.openClassEditor()
		}
	}
	return false
}

// cycleSettingValue cycles the current setting value forward or backward.
func (ed *Editor) cycleSettingValue(items []settingsItem, dir int) {
	if ed.settingsCursor >= len(items) {
		return
	}
	item := items[ed.settingsCursor]
	if item.Values == nil {
		return
	}

	newIdx := item.CurrentIdx + dir
	if newIdx < 0 {
		newIdx = len(item.Values) - 1
	}
	if newIdx >= len(item.Values) {
		newIdx = 0
	}

	newVal := item.Values[newIdx]

	switch item.Key {
	case "renderer":
		ed.config.Renderer = newVal
	case "file_type":
		ed.config.FileType = newVal
	case "fsm_type":
		switch newVal {
		case "DFA":
			ed.fsm.Type = fsm.TypeDFA
		case "NFA":
			ed.fsm.Type = fsm.TypeNFA
		case "Mealy":
			ed.fsm.Type = fsm.TypeMealy
		case "Moore":
			ed.fsm.Type = fsm.TypeMoore
		}
		ed.modified = true
	case "vocabulary":
		if ed.fsm != nil {
			ed.fsm.Vocabulary = newVal
			ed.modified = true
		}
		ed.config.Vocabulary = newVal
	}
}

// promptClassLibDir opens the file picker in directory-only mode.
func (ed *Editor) promptClassLibDir() {
	// Start from current class lib dir, or working directory.
	startDir := ed.config.ClassLibDir
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	ed.currentDir = startDir

	// Set up directory picker mode.
	ed.dirPickerMode = true
	ed.dirPickerAction = func(selectedDir string) {
		ed.config.ClassLibDir = selectedDir
		SaveConfig(ed.config)
		ed.showMessage("Class library dir: "+selectedDir, MsgSuccess)
	}
	ed.filePickerFocus = 0 // Start with directories focused.
	ed.refreshFilePicker()
	ed.mode = ModeFilePicker
}

// ====================================================================
// Feature 3: Class library loading
// ====================================================================

// countClassLibFiles counts .classes.json files in a directory.
func countClassLibFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".classes.json") {
			count++
		}
	}
	return count
}

// loadClassLibraries scans the configured class library directory for
// .classes.json files and imports their class definitions into the
// current FSM and populates the component catalog for the drawer.
func (ed *Editor) loadClassLibraries() {
	dir := ed.config.ClassLibDir
	if dir == "" {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		ed.showMessage("Cannot read dir: "+err.Error(), MsgError)
		return
	}

	ed.fsm.EnsureClassMaps()
	ed.catalog = nil
	loaded := 0
	skipped := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".classes.json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		classes, err := parseClassLibrary(data)
		if err != nil {
			ed.showMessage("Error in "+e.Name()+": "+err.Error(), MsgError)
			continue
		}

		// Build catalog category from this file.
		catName := catalogNameFromFile(e.Name())
		cat := CatalogCategory{
			Name:   catName,
			Source: e.Name(),
		}

		for _, cls := range classes {
			cat.Classes = append(cat.Classes, cls)
			if _, exists := ed.fsm.Classes[cls.Name]; exists {
				skipped++
				continue
			}
			ed.fsm.Classes[cls.Name] = cls
			loaded++
		}

		if len(cat.Classes) > 0 {
			// Sort classes by name within category.
			sortCatalogClasses(cat.Classes)
			ed.catalog = append(ed.catalog, cat)
		}
	}

	// Sort categories by name.
	sortCatalog(ed.catalog)

	if loaded > 0 {
		ed.modified = true
	}
	ed.showMessage(intToStr(loaded)+" classes loaded, "+intToStr(skipped)+" skipped (exist)", MsgSuccess)
}

// catalogNameFromFile derives a display name from a .classes.json filename.
// "74xx_gates.classes.json" → "74xx Gates"
func catalogNameFromFile(filename string) string {
	name := strings.TrimSuffix(filename, ".classes.json")
	name = strings.ReplaceAll(name, "_", " ")
	// Title-case each word.
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			// Keep all-numeric or known abbreviations as-is.
			if w[0] >= '0' && w[0] <= '9' {
				continue
			}
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// sortCatalogClasses sorts classes by name.
func sortCatalogClasses(classes []*fsm.Class) {
	for i := 1; i < len(classes); i++ {
		for j := i; j > 0 && classes[j].Name < classes[j-1].Name; j-- {
			classes[j], classes[j-1] = classes[j-1], classes[j]
		}
	}
}

// sortCatalog sorts categories by name.
func sortCatalog(cats []CatalogCategory) {
	for i := 1; i < len(cats); i++ {
		for j := i; j > 0 && cats[j].Name < cats[j-1].Name; j-- {
			cats[j], cats[j-1] = cats[j-1], cats[j]
		}
	}
}

// parseClassLibrary parses a .classes.json file which is a JSON object
// mapping class names to their property definitions and optional port lists.
//
// Format:
//
//	{
//	  "class_name": {
//	    "properties": [
//	      {"name": "prop_name", "type": "float64"},
//	      {"name": "items", "type": "list"}
//	    ],
//	    "ports": [
//	      {"name": "1A", "direction": "input", "pin_number": 1, "group": "GATE_A"}
//	    ]
//	  }
//	}
func parseClassLibrary(data []byte) ([]*fsm.Class, error) {
	// Use encoding/json to parse the structure.
	var raw map[string]struct {
		Parent     string            `json:"parent,omitempty"`
		Properties []fsm.PropertyDef `json:"properties"`
		Ports      []fsm.Port        `json:"ports,omitempty"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	classes := make([]*fsm.Class, 0, len(raw))
	for name, def := range raw {
		cls := &fsm.Class{
			Name:       name,
			Parent:     def.Parent,
			Properties: def.Properties,
			Ports:      def.Ports,
		}
		if cls.Properties == nil {
			cls.Properties = []fsm.PropertyDef{}
		}
		classes = append(classes, cls)
	}
	return classes, nil
}

// intToStr converts an int to string (avoiding strconv import).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
