package notify

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"color code", "\x1b[32mgreen\x1b[0m", "green"},
		{"cursor move", "\x1b[2J\x1b[H", ""},
		{"bold text", "\x1b[1mbold\x1b[0m text", "bold text"},
		{"256 color", "\x1b[38;5;196mred\x1b[0m", "red"},
		{"rgb color", "\x1b[38;2;255;0;0mred\x1b[0m", "red"},
		{"mixed", "\x1b[1;32m? \x1b[0mDo you want? \x1b[36m[Y/n]\x1b[0m", "? Do you want? [Y/n]"},
		{"OSC title", "\x1b]0;window title\x07rest", "rest"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLastVisibleLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "hello", "hello"},
		{"multi line", "line1\nline2\nline3", "line3"},
		{"trailing newline", "line1\nline2\n", "line2"},
		{"with ansi", "line1\n\x1b[32m? prompt\x1b[0m", "? prompt"},
		{"empty", "", ""},
		{"only newlines", "\n\n\n", ""},
		{"carriage return", "overwritten\rprompt> ", "prompt> "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LastVisibleLine(tt.input)
			if got != tt.want {
				t.Errorf("LastVisibleLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
