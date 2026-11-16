//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

// Linux: XTest on an X11 session (Xorg). Wayland sessions do not allow one program to move
// another's pointer, so control is reported as unavailable there.
var x struct {
	once sync.Once
	ok   bool
	dpy  uintptr
	w, h int32

	flush   func(uintptr) int32
	motion  func(uintptr, int32, int32, int32, uint64) int32
	button  func(uintptr, uint32, int32, uint64) int32
	key     func(uintptr, uint32, int32, uint64) int32
	keycode func(uintptr, uint64) uint8
}

func xInit() bool {
	x.once.Do(func() {
		if os.Getenv("DISPLAY") == "" {
			return
		}
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		tst, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		var open func(uintptr) uintptr
		var screen func(uintptr) int32
		var width, height func(uintptr, int32) int32
		purego.RegisterLibFunc(&open, lib, "XOpenDisplay")
		purego.RegisterLibFunc(&screen, lib, "XDefaultScreen")
		purego.RegisterLibFunc(&width, lib, "XDisplayWidth")
		purego.RegisterLibFunc(&height, lib, "XDisplayHeight")
		purego.RegisterLibFunc(&x.flush, lib, "XFlush")
		purego.RegisterLibFunc(&x.keycode, lib, "XKeysymToKeycode")
		purego.RegisterLibFunc(&x.motion, tst, "XTestFakeMotionEvent")
		purego.RegisterLibFunc(&x.button, tst, "XTestFakeButtonEvent")
		purego.RegisterLibFunc(&x.key, tst, "XTestFakeKeyEvent")
		x.dpy = open(0)
		if x.dpy == 0 {
			return
		}
		s := screen(x.dpy)
		x.w, x.h = width(x.dpy, s), height(x.dpy, s)
		x.ok = x.w > 0 && x.h > 0
	})
	return x.ok
}

func canInject() bool { return testMode || xInit() }

func platformNote() string {
	if testMode || xInit() {
		return ""
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "This Linux desktop uses Wayland, which does not let programs control the mouse. Log out and choose an Xorg (X11) session, or continue with screen sharing only."
	}
	return "The helper could not reach the X11 display, so it cannot control this computer."
}

func confirm(tech, title string) bool {
	if testMode {
		return testConfirm()
	}
	q := tech + " asks to control your mouse and keyboard"
	if title != "" {
		q += " for: " + title
	}
	q += ".\n\nAllow it for this session?"
	if p, err := exec.LookPath("zenity"); err == nil {
		return exec.Command(p, "--question", "--title=Workspace Remote Helper", "--text="+q, "--ok-label=Yes", "--cancel-label=No", "--default-cancel").Run() == nil
	}
	if p, err := exec.LookPath("kdialog"); err == nil {
		return exec.Command(p, "--title", "Workspace Remote Helper", "--yesno", q).Run() == nil
	}
	fmt.Print("\n" + q + " Type yes and press Enter: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "yes")
}

func inject(e event) {
	if testMode {
		testLog(e)
		return
	}
	if !xInit() {
		return
	}
	switch e.T {
	case "move":
		x.motion(x.dpy, -1, int32(clean01(e.X)*float64(x.w-1)), int32(clean01(e.Y)*float64(x.h-1)), 0)
	case "down", "up":
		if e.X >= 0 {
			x.motion(x.dpy, -1, int32(clean01(e.X)*float64(x.w-1)), int32(clean01(e.Y)*float64(x.h-1)), 0)
		}
		btn := [3]uint32{1, 2, 3}[e.B] // browser left, middle, right -> X buttons 1, 2, 3
		press := int32(0)
		if e.T == "down" {
			press = 1
		}
		x.button(x.dpy, btn, press, 0)
	case "wheel":
		btn := uint32(5) // down
		if e.DY < 0 {
			btn = 4
		}
		for i := 0; i < wheelSteps(e.DY); i++ {
			x.button(x.dpy, btn, 1, 0)
			x.button(x.dpy, btn, 0, 0)
		}
	case "key":
		sym, ok := xKeysyms[e.Code]
		if !ok {
			return
		}
		kc := x.keycode(x.dpy, sym)
		if kc == 0 {
			return
		}
		press := int32(0)
		if e.Down {
			press = 1
		}
		x.key(x.dpy, uint32(kc), press, 0)
	}
	x.flush(x.dpy)
}

// KeyboardEvent.code to X11 keysym. Modifiers are sent as their own keys, so Shift then KeyH types H.
var xKeysyms = func() map[string]uint64 {
	m := map[string]uint64{
		"Enter": 0xff0d, "NumpadEnter": 0xff8d, "Escape": 0xff1b, "Backspace": 0xff08, "Tab": 0xff09, "Space": 0x20,
		"ArrowLeft": 0xff51, "ArrowUp": 0xff52, "ArrowRight": 0xff53, "ArrowDown": 0xff54,
		"Delete": 0xffff, "Insert": 0xff63, "Home": 0xff50, "End": 0xff57, "PageUp": 0xff55, "PageDown": 0xff56,
		"ShiftLeft": 0xffe1, "ShiftRight": 0xffe2, "ControlLeft": 0xffe3, "ControlRight": 0xffe4,
		"AltLeft": 0xffe9, "AltRight": 0xffea, "MetaLeft": 0xffeb, "MetaRight": 0xffec, "ContextMenu": 0xff67, "CapsLock": 0xffe5,
		"Minus": 0x2d, "Equal": 0x3d, "BracketLeft": 0x5b, "BracketRight": 0x5d, "Backslash": 0x5c, "Semicolon": 0x3b,
		"Quote": 0x27, "Comma": 0x2c, "Period": 0x2e, "Slash": 0x2f, "Backquote": 0x60,
		"NumpadAdd": 0xffab, "NumpadSubtract": 0xffad, "NumpadMultiply": 0xffaa, "NumpadDivide": 0xffaf, "NumpadDecimal": 0xffae,
	}
	for i := 0; i < 26; i++ {
		m["Key"+string(rune('A'+i))] = uint64(0x61 + i)
	}
	for i := 0; i < 10; i++ {
		m["Digit"+string(rune('0'+i))] = uint64(0x30 + i)
		m["Numpad"+string(rune('0'+i))] = uint64(0xffb0 + i)
	}
	for i := 1; i <= 12; i++ {
		m[fmt.Sprintf("F%d", i)] = uint64(0xffbe + i - 1)
	}
	return m
}()
