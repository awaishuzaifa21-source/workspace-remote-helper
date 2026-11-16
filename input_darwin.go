//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
)

// macOS: Quartz events through CoreGraphics. macOS only delivers them after the person allows
// the app that runs the helper (Terminal) in System Settings, Privacy & Security, Accessibility.
type cgPoint struct{ X, Y float64 }

var q struct {
	once sync.Once
	ok   bool
	w, h float64

	mouse   func(uintptr, uint32, cgPoint, uint32) uintptr
	keyb    func(uintptr, uint16, bool) uintptr
	scroll  func(uintptr, uint32, uint32, int32, int32, int32) uintptr
	flags   func(uintptr, uint64)
	field   func(uintptr, uint32, int64)
	post    func(uint32, uintptr)
	release func(uintptr)
	trusted func() bool

	mu       sync.Mutex
	pos      cgPoint
	held     [3]bool
	mods     uint64
	lastDown time.Time
	lastPos  cgPoint
	clicks   int64
}

func qInit() bool {
	q.once.Do(func() {
		cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		as, err := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		var mainID func() uint32
		var pw, ph func(uint32) uintptr
		purego.RegisterLibFunc(&mainID, cg, "CGMainDisplayID")
		purego.RegisterLibFunc(&pw, cg, "CGDisplayPixelsWide")
		purego.RegisterLibFunc(&ph, cg, "CGDisplayPixelsHigh")
		purego.RegisterLibFunc(&q.mouse, cg, "CGEventCreateMouseEvent")
		purego.RegisterLibFunc(&q.keyb, cg, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&q.scroll, cg, "CGEventCreateScrollWheelEvent2")
		purego.RegisterLibFunc(&q.flags, cg, "CGEventSetFlags")
		purego.RegisterLibFunc(&q.field, cg, "CGEventSetIntegerValueField")
		purego.RegisterLibFunc(&q.post, cg, "CGEventPost")
		purego.RegisterLibFunc(&q.release, cf, "CFRelease")
		purego.RegisterLibFunc(&q.trusted, as, "AXIsProcessTrusted")
		id := mainID()
		q.w, q.h = float64(pw(id)), float64(ph(id))
		q.ok = q.w > 0 && q.h > 0
	})
	return q.ok
}

func canInject() bool { return testMode || qInit() }

func platformNote() string {
	if testMode || !qInit() || q.trusted() {
		return ""
	}
	return "macOS needs your permission first: open System Settings, Privacy & Security, Accessibility, and turn on Terminal. Then press Allow control again."
}

func confirm(tech, title string) bool {
	if testMode {
		return testConfirm()
	}
	safe := func(s string) string { return strings.NewReplacer(`"`, "", `\`, "").Replace(s) }
	msg := safe(tech) + " asks to control your mouse and keyboard"
	if title != "" {
		msg += " for: " + safe(title)
	}
	msg += ". Allow it for this session?"
	script := fmt.Sprintf(`display dialog "%s" with title "Workspace Remote Helper" buttons {"No", "Yes"} default button "No" with icon caution`, msg)
	out, err := exec.Command("osascript", "-e", script).Output()
	return err == nil && strings.Contains(string(out), "Yes")
}

const (
	evLeftDown, evLeftUp, evRightDown, evRightUp, evMoved, evLeftDrag, evRightDrag = 1, 2, 3, 4, 5, 6, 7
	evOtherDown, evOtherUp, evOtherDrag                                          = 25, 26, 27
	fieldClickState                                                              = 1
)

func (p cgPoint) near(o cgPoint) bool { return abs64(p.X-o.X) < 5 && abs64(p.Y-o.Y) < 5 }

func inject(e event) {
	if testMode {
		testLog(e)
		return
	}
	if !qInit() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if e.T == "move" || ((e.T == "down" || e.T == "up") && e.X >= 0) {
		q.pos = cgPoint{clean01(e.X) * (q.w - 1), clean01(e.Y) * (q.h - 1)}
	}
	// Browser buttons 0 left, 1 middle, 2 right; Quartz buttons 0 left, 1 right, 2 other.
	qb := [3]uint32{0, 2, 1}
	switch e.T {
	case "move":
		typ, btn := uint32(evMoved), uint32(0)
		switch {
		case q.held[0]:
			typ, btn = evLeftDrag, 0
		case q.held[2]:
			typ, btn = evRightDrag, 1
		case q.held[1]:
			typ, btn = evOtherDrag, 2
		}
		send(q.mouse(0, typ, q.pos, btn))
	case "down", "up":
		down := e.T == "down"
		q.held[e.B] = down
		types := [3][2]uint32{{evLeftDown, evLeftUp}, {evOtherDown, evOtherUp}, {evRightDown, evRightUp}}[e.B]
		typ := types[1]
		if down {
			typ = types[0]
			if time.Since(q.lastDown) < 400*time.Millisecond && q.pos.near(q.lastPos) {
				q.clicks++
			} else {
				q.clicks = 1
			}
			q.lastDown, q.lastPos = time.Now(), q.pos
		}
		ev := q.mouse(0, typ, q.pos, qb[e.B])
		q.field(ev, fieldClickState, q.clicks) // double clicks need the click count
		send(ev)
	case "wheel":
		send(q.scroll(0, 0, 1, int32(-e.DY), 0, 0)) // pixel units; negative scrolls down
	case "key":
		kc, ok := macKeys[e.Code]
		if !ok {
			return
		}
		if bit, isMod := macMods[e.Code]; isMod {
			if e.Down {
				q.mods |= bit
			} else {
				q.mods &^= bit
			}
		}
		ev := q.keyb(0, kc, e.Down)
		q.flags(ev, q.mods)
		send(ev)
	}
}

// send posts an event to the system and frees it.
func send(ev uintptr) {
	if ev == 0 {
		return
	}
	q.post(0, ev) // kCGHIDEventTap
	q.release(ev)
}

// KeyboardEvent.code to macOS virtual key code (kVK_*).
var macKeys = map[string]uint16{
	"KeyA": 0x00, "KeyS": 0x01, "KeyD": 0x02, "KeyF": 0x03, "KeyH": 0x04, "KeyG": 0x05, "KeyZ": 0x06, "KeyX": 0x07, "KeyC": 0x08, "KeyV": 0x09,
	"KeyB": 0x0B, "KeyQ": 0x0C, "KeyW": 0x0D, "KeyE": 0x0E, "KeyR": 0x0F, "KeyY": 0x10, "KeyT": 0x11, "Digit1": 0x12, "Digit2": 0x13,
	"Digit3": 0x14, "Digit4": 0x15, "Digit6": 0x16, "Digit5": 0x17, "Equal": 0x18, "Digit9": 0x19, "Digit7": 0x1A, "Minus": 0x1B,
	"Digit8": 0x1C, "Digit0": 0x1D, "BracketRight": 0x1E, "KeyO": 0x1F, "KeyU": 0x20, "BracketLeft": 0x21, "KeyI": 0x22, "KeyP": 0x23,
	"Enter": 0x24, "KeyL": 0x25, "KeyJ": 0x26, "Quote": 0x27, "KeyK": 0x28, "Semicolon": 0x29, "Backslash": 0x2A, "Comma": 0x2B,
	"Slash": 0x2C, "KeyN": 0x2D, "KeyM": 0x2E, "Period": 0x2F, "Tab": 0x30, "Space": 0x31, "Backquote": 0x32, "Backspace": 0x33,
	"Escape": 0x35, "MetaRight": 0x36, "MetaLeft": 0x37, "ShiftLeft": 0x38, "CapsLock": 0x39, "AltLeft": 0x3A, "ControlLeft": 0x3B,
	"ShiftRight": 0x3C, "AltRight": 0x3D, "ControlRight": 0x3E, "NumpadDecimal": 0x41, "NumpadMultiply": 0x43, "NumpadAdd": 0x45,
	"NumpadDivide": 0x4B, "NumpadEnter": 0x4C, "NumpadSubtract": 0x4E, "Numpad0": 0x52, "Numpad1": 0x53, "Numpad2": 0x54,
	"Numpad3": 0x55, "Numpad4": 0x56, "Numpad5": 0x57, "Numpad6": 0x58, "Numpad7": 0x59, "Numpad8": 0x5B, "Numpad9": 0x5C,
	"F5": 0x60, "F6": 0x61, "F7": 0x62, "F3": 0x63, "F8": 0x64, "F9": 0x65, "F11": 0x67, "F10": 0x6D, "F12": 0x6F, "Insert": 0x72,
	"Home": 0x73, "PageUp": 0x74, "Delete": 0x75, "F4": 0x76, "End": 0x77, "F2": 0x78, "PageDown": 0x79, "F1": 0x7A,
	"ArrowLeft": 0x7B, "ArrowRight": 0x7C, "ArrowDown": 0x7D, "ArrowUp": 0x7E,
}

// Modifier keys and the event flag each one sets while held.
var macMods = map[string]uint64{
	"ShiftLeft": 0x20000, "ShiftRight": 0x20000, "ControlLeft": 0x40000, "ControlRight": 0x40000,
	"AltLeft": 0x80000, "AltRight": 0x80000, "MetaLeft": 0x100000, "MetaRight": 0x100000,
}
