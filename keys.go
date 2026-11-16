package main

// KeyboardEvent.code to Windows virtual key. The bool marks extended keys.
type vk struct {
	code uint16
	ext  bool
}

var keyCodes = func() map[string]vk {
	m := map[string]vk{
		"Enter": {0x0D, false}, "NumpadEnter": {0x0D, true}, "Escape": {0x1B, false}, "Backspace": {0x08, false}, "Tab": {0x09, false}, "Space": {0x20, false},
		"ArrowLeft": {0x25, true}, "ArrowUp": {0x26, true}, "ArrowRight": {0x27, true}, "ArrowDown": {0x28, true},
		"Delete": {0x2E, true}, "Insert": {0x2D, true}, "Home": {0x24, true}, "End": {0x23, true}, "PageUp": {0x21, true}, "PageDown": {0x22, true},
		"ShiftLeft": {0xA0, false}, "ShiftRight": {0xA1, false}, "ControlLeft": {0xA2, false}, "ControlRight": {0xA3, true},
		"AltLeft": {0xA4, false}, "AltRight": {0xA5, true}, "MetaLeft": {0x5B, true}, "MetaRight": {0x5C, true}, "ContextMenu": {0x5D, true},
		"CapsLock": {0x14, false}, "Minus": {0xBD, false}, "Equal": {0xBB, false}, "BracketLeft": {0xDB, false}, "BracketRight": {0xDD, false},
		"Backslash": {0xDC, false}, "Semicolon": {0xBA, false}, "Quote": {0xDE, false}, "Comma": {0xBC, false}, "Period": {0xBE, false},
		"Slash": {0xBF, false}, "Backquote": {0xC0, false},
		"NumpadAdd": {0x6B, false}, "NumpadSubtract": {0x6D, false}, "NumpadMultiply": {0x6A, false}, "NumpadDivide": {0x6F, true}, "NumpadDecimal": {0x6E, false},
	}
	for i := 0; i < 26; i++ {
		m["Key"+string(rune('A'+i))] = vk{uint16(0x41 + i), false}
	}
	for i := 0; i < 10; i++ {
		m["Digit"+string(rune('0'+i))] = vk{uint16(0x30 + i), false}
		m["Numpad"+string(rune('0'+i))] = vk{uint16(0x60 + i), false}
	}
	for i := 1; i <= 12; i++ {
		name := "F" + string(rune('0'+i))
		if i >= 10 {
			name = "F1" + string(rune('0'+i-10))
		}
		m[name] = vk{uint16(0x6F + i), false}
	}
	return m
}()
