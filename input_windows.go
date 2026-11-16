//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
	procMsgBox    = user32.NewProc("MessageBoxW")
	procMapVK     = user32.NewProc("MapVirtualKeyW")
)

// INPUT is 40 bytes on 64-bit Windows; both layouts below match it exactly.
type mouseInput struct {
	typ       uint32
	_         uint32
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	_         uint32
	extra     uintptr
}

type keybdInput struct {
	typ   uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	_     uint32
	extra uintptr
	_     [8]byte
}

func canInject() bool { return true }

func platformNote() string { return "" }

func confirm(tech, title string) bool {
	text := tech + " is asking to control your mouse and keyboard"
	if title != "" {
		text += " for: " + title
	}
	text += ".\n\nOnly click Yes if you contacted this person for help and you trust them.\nNever allow this for someone who called you unexpectedly.\n\nYou can stop at any time by closing the helper window.\n\nAllow remote control?"
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString("Workspace Remote Helper")
	// MB_YESNO | MB_ICONWARNING | MB_DEFBUTTON2 | MB_SYSTEMMODAL | MB_SETFOREGROUND | MB_TOPMOST
	r, _, _ := procMsgBox.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x4|0x30|0x100|0x1000|0x10000|0x40000)
	return r == 6 // IDYES
}

func sendMouse(dx, dy int32, data uint32, flags uint32) {
	in := mouseInput{typ: 0, dx: dx, dy: dy, mouseData: data, flags: flags}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func abs(v float64) int32 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return int32(v * 65535)
}

func inject(e event) {
	switch e.T {
	case "move":
		sendMouse(abs(e.X), abs(e.Y), 0, 0x0001|0x8000) // MOVE | ABSOLUTE (primary screen)
	case "down", "up":
		if e.X >= 0 {
			sendMouse(abs(e.X), abs(e.Y), 0, 0x0001|0x8000)
		}
		flags := [3][2]uint32{{0x0002, 0x0004}, {0x0020, 0x0040}, {0x0008, 0x0010}}[e.B] // left, middle, right
		if e.T == "down" {
			sendMouse(0, 0, 0, flags[0])
		} else {
			sendMouse(0, 0, 0, flags[1])
		}
	case "wheel":
		d := -e.DY
		if d > 360 {
			d = 360
		}
		if d < -360 {
			d = -360
		}
		sendMouse(0, 0, uint32(int32(d)), 0x0800)
	case "key":
		k, ok := keyCodes[e.Code]
		if !ok {
			return
		}
		scan, _, _ := procMapVK.Call(uintptr(k.code), 0)
		var flags uint32
		if k.ext {
			flags |= 0x0001
		}
		if !e.Down {
			flags |= 0x0002
		}
		in := keybdInput{typ: 1, vk: k.code, scan: uint16(scan), flags: flags}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	}
}
