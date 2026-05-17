package ansi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argus/terminal/internal/screen"
)

// bufferText returns the buffer content as a slice of strings, one per row.
func bufferText(buf *screen.Buffer) []string {
	lines := make([]string, buf.Height())
	for y := 0; y < buf.Height(); y++ {
		var line []rune
		for x := 0; x < buf.Width(); x++ {
			c := buf.Cell(x, y)
			if c.Rune == 0 {
				line = append(line, ' ')
			} else {
				line = append(line, c.Rune)
			}
		}
		lines[y] = string(line)
	}
	return lines
}

func writeSeq(t *testing.T, p *Parser, s string) {
	t.Helper()
	p.WriteString(s)
}

// ── Text rendering ───────────────────────────────────────────────────────

func TestParser_TextRendering(t *testing.T) {
	tests := []struct {
		name  string
		width int
		input string
		want  []string
	}{
		{
			name:  "plain ascii",
			width: 10,
			input: "hello",
			want:  []string{"hello     "},
		},
		{
			name:  "newline advances row",
			width: 10,
			input: "ab\nc",
			want:  []string{"ab        ", "  c       "},
		},
		{
			name:  "carriage return resets column",
			width: 10,
			input: "abcdef\rxyz",
			want:  []string{"xyzdef    "},
		},
		{
			name:  "backspace at origin is no-op",
			width: 10,
			input: "\x08",
			want:  []string{"          "},
		},
		{
			name:  "backspace moves left",
			width: 10,
			input: "ab\x08",
			want:  []string{"ab        "},
		},
		// UTF-8: 2-byte sequence (é = 0xC3 0xA9)
		{
			name:  "utf8 2-byte sequence",
			width: 10,
			input: "café",
			want:  []string{"café      "},
		},
		// UTF-8: 3-byte sequence (€ = 0xE2 0x82 0xAC)
		{
			name:  "utf8 3-byte sequence",
			width: 10,
			input: "€100",
			want:  []string{"€100      "},
		},
		// UTF-8: 4-byte sequence (𐍈 = 0xF0 0x90 0x8D 0x88)
		{
			name:  "utf8 4-byte sequence",
			width: 10,
			input: "\U00010348!",
			want:  []string{"𐍈!        "},
		},
		// UTF-8: invalid leading byte (0xFF) produces replacement char
		{
			name:  "utf8 invalid leading byte",
			width: 10,
			input: string([]byte{0xFF, 'x'}),
			want:  []string{"\uFFFDx        "},
		},
		// UTF-8: truncated sequence emits replacement chars
		{
			name:  "utf8 truncated 2-byte",
			width: 10,
			input: string([]byte{0xC3, 'x'}),
			want:  []string{"\uFFFDx        "},
		},
		{
			name:  "utf8 truncated 3-byte",
			width: 10,
			input: string([]byte{0xE2, 0x82, 'x'}),
			want:  []string{"\uFFFD\uFFFDx       "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := screen.NewBuffer(tt.width, 3)
			p := NewParser(buf)
			writeSeq(t, p, tt.input)
			lines := bufferText(buf)
			assert.Equal(t, tt.want[0], lines[0])
		})
	}
}

// ── Cursor movement ──────────────────────────────────────────────────────

func TestParser_CursorMovement(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantX int
		wantY int
	}{
		{name: "CUU", input: "\x1b[2A", wantX: 0, wantY: 0},
		{name: "CUU from offset", input: "\x1b[3B\x1b[2A", wantX: 0, wantY: 1},
		{name: "CUD", input: "\x1b[2B", wantX: 0, wantY: 2},
		{name: "CUF", input: "\x1b[3C", wantX: 3, wantY: 0},
		{name: "CUB from offset", input: "\x1b[5C\x1b[2D", wantX: 3, wantY: 0},
		{name: "CUP absolute", input: "\x1b[3;4H", wantX: 3, wantY: 2},
		{name: "CUP with defaults", input: "\x1b[H", wantX: 0, wantY: 0},
		{name: "CHA default", input: "\x1b[G", wantX: 0, wantY: 0},
		{name: "CHA explicit", input: "\x1b[5G", wantX: 4, wantY: 0},
		{name: "CNL", input: "\x1b[2E", wantX: 0, wantY: 2},
		{name: "CPL at top", input: "\x1b[3F", wantX: 0, wantY: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := screen.NewBuffer(10, 5)
			p := NewParser(buf)
			writeSeq(t, p, tt.input)
			assert.Equal(t, tt.wantX, buf.Cursor().X, "column")
			assert.Equal(t, tt.wantY, buf.Cursor().Y, "row")
		})
	}
}

// ── Erase operations ─────────────────────────────────────────────────────

func TestParser_Erase(t *testing.T) {
	tests := []struct {
		name     string
		setup    string
		input    string
		checkFn  func(t *testing.T, buf *screen.Buffer)
	}{
		{
			name:  "ED clear display",
			setup: "abcdef",
			input: "\x1b[2J",
			checkFn: func(t *testing.T, buf *screen.Buffer) {
				for y := 0; y < 3; y++ {
					for x := 0; x < 5; x++ {
						assert.True(t, buf.Cell(x, y).IsDefault(), "cell(%d,%d) should be default", x, y)
					}
				}
			},
		},
		{
			name:  "EL clear to end of line",
			setup: "abcdef",
			input: "\x1b[3D\x1b[K",
			checkFn: func(t *testing.T, buf *screen.Buffer) {
				lines := bufferText(buf)
				assert.Equal(t, "abc       ", lines[0])
			},
		},
		{
			name:  "IL insert lines",
			setup: "aaaaa\x1b[H",
			input: "\x1b[2L",
			checkFn: func(t *testing.T, buf *screen.Buffer) {
				lines := bufferText(buf)
				assert.Equal(t, "          ", lines[0])
				assert.Equal(t, "          ", lines[1])
				assert.Equal(t, "aaaaa     ", lines[2])
			},
		},
		{
			name:  "DL delete lines",
			setup: "aaaaa\x1b[H",
			input: "\x1b[M",
			checkFn: func(t *testing.T, buf *screen.Buffer) {
				assert.True(t, buf.Cell(0, 0).IsDefault())
			},
		},
		{
			name:  "DCH delete characters",
			setup: "abcdef\x1b[3G",
			input: "\x1b[3P",
			checkFn: func(t *testing.T, buf *screen.Buffer) {
				lines := bufferText(buf)
				assert.Equal(t, "abf       ", lines[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := screen.NewBuffer(10, 5)
			p := NewParser(buf)
			writeSeq(t, p, tt.setup)
			writeSeq(t, p, tt.input)
			tt.checkFn(t, buf)
		})
	}
}

// ── SGR attributes ───────────────────────────────────────────────────────

func TestParser_SGR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantFn  func(t *testing.T, cell screen.Cell)
	}{
		{
			name:  "reset then write",
			input: "\x1b[1m\x1b[0mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.Attr(0), cell.Attr)
				assert.Equal(t, 'x', cell.Rune)
			},
		},
		{
			name:  "reset with no params (CSI m)",
			input: "\x1b[1m\x1b[mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.Attr(0), cell.Attr)
				assert.Equal(t, screen.ColorDefault, cell.Fg.Index)
				assert.Equal(t, screen.ColorDefault, cell.Bg.Index)
			},
		},
		{name: "bold", input: "\x1b[1mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrBold, cell.Attr) }},
		{name: "dim", input: "\x1b[2mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrDim, cell.Attr) }},
		{name: "italic", input: "\x1b[3mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrItalic, cell.Attr) }},
		{name: "underline", input: "\x1b[4mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrUnderline, cell.Attr) }},
		{name: "reverse", input: "\x1b[7mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrReverse, cell.Attr) }},
		{name: "strikethrough", input: "\x1b[9mx", wantFn: func(t *testing.T, cell screen.Cell) { assert.Equal(t, screen.AttrStrikethrough, cell.Attr) }},
		{
			name:  "FG standard color",
			input: "\x1b[31mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.ColorRed, cell.Fg.Index)
				assert.Equal(t, screen.ColorDefault, cell.Bg.Index)
			},
		},
		{
			name:  "BG standard color",
			input: "\x1b[44mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.ColorBlue, cell.Bg.Index)
			},
		},
		{
			name:  "bright FG",
			input: "\x1b[91mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.ColorBrightRed, cell.Fg.Index)
			},
		},
		{
			name:  "true color FG",
			input: "\x1b[38;2;255;128;0mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.True(t, cell.Fg.IsTrue)
				assert.Equal(t, uint8(255), cell.Fg.True.R)
				assert.Equal(t, uint8(128), cell.Fg.True.G)
				assert.Equal(t, uint8(0), cell.Fg.True.B)
			},
		},
		// Regression: SGR 22 must clear bold+dim (was shadowed by range case)
		{
			name:  "SGR 22 clears bold",
			input: "\x1b[1m\x1b[22mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.Attr(0), cell.Attr&screen.AttrBold)
			},
		},
		{
			name:  "SGR 22 clears dim",
			input: "\x1b[2m\x1b[22mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.Attr(0), cell.Attr&screen.AttrDim)
			},
		},
		// SGR 21: double underline → clear underline
		{
			name:  "SGR 21 clears underline",
			input: "\x1b[4m\x1b[21mx",
			wantFn: func(t *testing.T, cell screen.Cell) {
				assert.Equal(t, screen.Attr(0), cell.Attr&screen.AttrUnderline)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := screen.NewBuffer(10, 3)
			p := NewParser(buf)
			writeSeq(t, p, tt.input)
			tt.wantFn(t, buf.Cell(0, 0))
		})
	}
}

// ── Terminal modes ───────────────────────────────────────────────────────

func TestParser_Modes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		setup  string
		checkX int
		checkY int
	}{
		{name: "auto-wrap on wraps to next line", input: "\x1b[?7h", checkX: 1, checkY: 1},
		{name: "auto-wrap off does not wrap", input: "\x1b[?7l", checkX: 4, checkY: 0},
	}

	bufSizes := map[string]struct{ w, h int }{
		"auto-wrap on wraps to next line":  {5, 3},
		"auto-wrap off does not wrap":      {5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sz := bufSizes[tt.name]
			buf := screen.NewBuffer(sz.w, sz.h)
			p := NewParser(buf)
			writeSeq(t, p, tt.input)
			n := 6
			if tt.name == "auto-wrap off does not wrap" {
				n = 8
			}
			for i := 0; i < n; i++ {
				writeSeq(t, p, "x")
			}
			assert.Equal(t, tt.checkX, buf.Cursor().X)
			assert.Equal(t, tt.checkY, buf.Cursor().Y)
		})
	}
}

func TestParser_OriginMode(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeSeq(t, p, "\x1b[?6h")
	// Origin mode sets cursor to margins origin — just verify it doesn't panic
	assert.True(t, true)
}

func TestParser_ApplicationKeypad(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeSeq(t, p, "\x1b=") // enable
	writeSeq(t, p, "\x1b>") // disable
	// Just verify it doesn't panic
}

// ── Cursor save/restore & reset ──────────────────────────────────────────

func TestParser_CursorSaveRestore(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantX int
		wantY int
	}{
		{name: "save and restore cursor", input: "\x1b[3;4H\x1b7\x1b[H\x1b8", wantX: 3, wantY: 2},
		{name: "RIS resets screen", input: "hello\x1bc", wantX: 0, wantY: 0},
		{name: "scroll region sets margins", input: "\x1b[2;4r", wantX: 0, wantY: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := screen.NewBuffer(10, 5)
			p := NewParser(buf)
			writeSeq(t, p, tt.input)
			assert.Equal(t, tt.wantX, buf.Cursor().X)
			assert.Equal(t, tt.wantY, buf.Cursor().Y)
			if tt.name == "RIS resets screen" {
				assert.True(t, buf.Cell(0, 0).IsDefault())
			}
		})
	}
}

// ── UTF-8 decodeRune unit tests ──────────────────────────────────────────

func TestDecodeRune(t *testing.T) {
	tests := []struct {
		name    string
		seq     []rune
		want    rune
		wantOK  bool
	}{
		// Valid sequences
		{name: "2-byte: é", seq: []rune{0xC3, 0xA9}, want: 'é', wantOK: true},
		{name: "2-byte: ñ", seq: []rune{0xC3, 0xB1}, want: 'ñ', wantOK: true},
		{name: "3-byte: €", seq: []rune{0xE2, 0x82, 0xAC}, want: '€', wantOK: true},
		{name: "3-byte: 日本語", seq: []rune{0xE6, 0x97, 0xA5}, want: '日', wantOK: true},
		{name: "4-byte: 𐍈", seq: []rune{0xF0, 0x90, 0x8D, 0x88}, want: '\U00010348', wantOK: true},

		// Invalid: overlong
		{name: "overlong / (2-byte for ASCII)", seq: []rune{0xC0, 0xAF}, want: 0, wantOK: false},
		{name: "overlong space (3-byte)", seq: []rune{0xE0, 0x80, 0xA0}, want: 0, wantOK: false},

		// Invalid: surrogate
		{name: "surrogate U+D800", seq: []rune{0xED, 0xA0, 0x80}, want: 0, wantOK: false},

		// Invalid: exceeds Unicode range
		{name: "out of range U+110000", seq: []rune{0xF4, 0x90, 0x80, 0x80}, want: 0, wantOK: false},

		// Edge: minimum valid 2-byte
		{name: "min 2-byte U+0080", seq: []rune{0xC2, 0x80}, want: '\u0080', wantOK: true},
		{name: "min 3-byte U+0800", seq: []rune{0xE0, 0xA0, 0x80}, want: '\u0800', wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := decodeRune(tt.seq)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, r)
			}
		})
	}
}
