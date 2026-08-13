package tui

import "strings"

// sanitizeTerminalText removes terminal control sequences from untrusted model
// and tool output before it reaches the viewport. Private CSI modes can change
// horizontal margins and persist after a render.
func sanitizeTerminalText(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		if text[i] != '\x1b' {
			if text[i] >= 0x20 || text[i] == '\n' || text[i] == '\t' {
				out.WriteByte(text[i])
			}
			i++
			continue
		}

		if i+1 >= len(text) {
			break
		}
		switch text[i+1] {
		case '[': // CSI: consume through its final byte.
			i += 2
			for i < len(text) {
				if text[i] >= 0x40 && text[i] <= 0x7e {
					i++
					break
				}
				i++
			}
		case ']': // OSC: terminated by BEL or ST (ESC \\).
			i += 2
			for i < len(text) {
				if text[i] == '\a' {
					i++
					break
				}
				if text[i] == '\x1b' && i+1 < len(text) && text[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			// Two-byte ESC controls (RIS, save/restore cursor, etc.).
			i += 2
		}
	}
	return out.String()
}
