// Render pipeline for fsmedit — generates images and opens system viewer.
package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

func (ed *Editor) renderView() {
	// Generate title
	title := ed.fsm.Name
	if title == "" {
		title = "FSM"
	}

	var tmpPath string
	useNative := ed.config.Renderer == "native"
	useSVG := ed.config.FileType == "svg"

	// Check for dot if graphviz is selected
	if !useNative {
		if _, err := exec.LookPath("dot"); err != nil {
			ed.showMessage("Graphviz not found, using native renderer", MsgInfo)
			useNative = true
		}
	}

	if useNative {
		if useSVG {
			// Native SVG
			tmpFile, err := os.CreateTemp("", "fsm-*.svg")
			if err != nil {
				ed.showMessage("Failed to create temp file", MsgError)
				return
			}
			tmpPath = tmpFile.Name()
			tmpFile.Close()

			opts := fsmfile.DefaultSVGOptions()
			opts.Title = title
			svg := fsmfile.GenerateSVGNative(ed.fsm, opts)

			if err := os.WriteFile(tmpPath, []byte(svg), 0644); err != nil {
				ed.showMessage("Failed to write SVG", MsgError)
				os.Remove(tmpPath)
				return
			}
		} else {
			// Native PNG
			tmpFile, err := os.CreateTemp("", "fsm-*.png")
			if err != nil {
				ed.showMessage("Failed to create temp file", MsgError)
				return
			}
			tmpPath = tmpFile.Name()

			opts := fsmfile.DefaultPNGOptions()
			opts.Title = title
			if err := fsmfile.RenderPNG(ed.fsm, tmpFile, opts); err != nil {
				tmpFile.Close()
				ed.showMessage("Failed to generate PNG: "+err.Error(), MsgError)
				os.Remove(tmpPath)
				return
			}
			tmpFile.Close()
		}
	} else {
		// Graphviz
		ext := ".png"
		format := "png"
		if useSVG {
			ext = ".svg"
			format = "svg"
		}

		tmpFile, err := os.CreateTemp("", "fsm-*"+ext)
		if err != nil {
			ed.showMessage("Failed to create temp file", MsgError)
			return
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()

		dot := fsmfile.GenerateDOT(ed.fsm, title)
		cmd := exec.Command("dot", "-T"+format, "-o", tmpPath)
		cmd.Stdin = strings.NewReader(dot)
		if err := cmd.Run(); err != nil {
			ed.showMessage("dot failed: "+err.Error(), MsgError)
			os.Remove(tmpPath)
			return
		}
	}

	// Open with system viewer
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", tmpPath)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", tmpPath)
	default: // linux, etc
		openCmd = exec.Command("xdg-open", tmpPath)
	}

	if err := openCmd.Start(); err != nil {
		ed.showMessage("Failed to open viewer: "+err.Error(), MsgError)
		os.Remove(tmpPath)
		return
	}

	ed.showMessage("Opened in viewer: "+tmpPath, MsgInfo)
}
