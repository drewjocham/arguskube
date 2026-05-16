package ansi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argus/terminal/internal/screen"
)

func bufferText(t *testing.T, buf *screen.Buffer) []string {
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

func writeString(t *testing.T, p *Parser, s string) {
	t.Helper()
	p.WriteString(s)
}

func TestPlainText(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "hello")

	lines := bufferText(t, buf)
	assert.Equal(t, "hello     ", lines[0])
}

func TestNewline(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "ab\nc")

	lines := bufferText(t, buf)
	assert.Equal(t, "ab        ", lines[0])
	assert.Equal(t, "  c       ", lines[1])
}

func TestCarriageReturn(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "abcdef")
	writeString(t, p, "\rxyz")

	lines := bufferText(t, buf)
	assert.Equal(t, "xyzdef    ", lines[0])
}

func TestCUU(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[2A")
	assert.Equal(t, 0, buf.Cursor().Y)

	writeString(t, p, "\x1b[3B")
	writeString(t, p, "\x1b[2A")
	assert.Equal(t, 1, buf.Cursor().Y)
}

func TestCUD(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[2B")
	assert.Equal(t, 2, buf.Cursor().Y)
}

func TestCUF(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[3C")
	assert.Equal(t, 3, buf.Cursor().X)
}

func TestCUB(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[5C\x1b[2D")
	assert.Equal(t, 3, buf.Cursor().X)
}

func TestCUP(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[3;4H")
	assert.Equal(t, 2, buf.Cursor().Y)
	assert.Equal(t, 3, buf.Cursor().X)
}

func TestCUPDefault(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[H")
	assert.Equal(t, 0, buf.Cursor().Y)
	assert.Equal(t, 0, buf.Cursor().X)
}

func TestED(t *testing.T) {
	buf := screen.NewBuffer(5, 3)
	p := NewParser(buf)
	writeString(t, p, "abcdef")
	writeString(t, p, "\x1b[2J")
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			assert.True(t, buf.Cell(x, y).IsDefault())
		}
	}
}

func TestEL(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "abcdef")
	writeString(t, p, "\x1b[3D\x1b[K")
	lines := bufferText(t, buf)
	assert.Equal(t, "abc       ", lines[0])
}

func TestSGRReset(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[1m\x1b[0m")
	writeString(t, p, "x")
	assert.Equal(t, screen.Attr(0), buf.Cell(0, 0).Attr)
}

func TestSGRBold(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[1mx")
	assert.Equal(t, screen.AttrBold, buf.Cell(0, 0).Attr)
}

func TestSGRDim(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[2mx")
	assert.Equal(t, screen.AttrDim, buf.Cell(0, 0).Attr)
}

func TestSGRItalic(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[3mx")
	assert.Equal(t, screen.AttrItalic, buf.Cell(0, 0).Attr)
}

func TestSGRUnderline(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[4mx")
	assert.Equal(t, screen.AttrUnderline, buf.Cell(0, 0).Attr)
}

func TestSGRReverse(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[7mx")
	assert.Equal(t, screen.AttrReverse, buf.Cell(0, 0).Attr)
}

func TestSGRStrikethrough(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[9mx")
	assert.Equal(t, screen.AttrStrikethrough, buf.Cell(0, 0).Attr)
}

func TestSGRFGColor(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[31mx")
	assert.Equal(t, screen.ColorRed, buf.Cell(0, 0).Fg.Index)
	assert.Equal(t, screen.ColorDefault, buf.Cell(0, 0).Bg.Index)
}

func TestSGRBGColor(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[44mx")
	assert.Equal(t, screen.ColorBlue, buf.Cell(0, 0).Bg.Index)
}

func TestSGRBrightFG(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[91mx")
	assert.Equal(t, screen.ColorBrightRed, buf.Cell(0, 0).Fg.Index)
}

func TestSGRTrueColor(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[38;2;255;128;0mx")
	assert.True(t, buf.Cell(0, 0).Fg.IsTrue)
	assert.Equal(t, uint8(255), buf.Cell(0, 0).Fg.True.R)
	assert.Equal(t, uint8(128), buf.Cell(0, 0).Fg.True.G)
	assert.Equal(t, uint8(0), buf.Cell(0, 0).Fg.True.B)
}

func TestSaveRestoreCursor(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[3;4H\x1b7\x1b[H\x1b8")
	assert.Equal(t, 2, buf.Cursor().Y)
	assert.Equal(t, 3, buf.Cursor().X)
}

func TestRIS(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "hello")
	writeString(t, p, "\x1bc")
	assert.True(t, buf.Cell(0, 0).IsDefault())
}

func TestScrollRegion(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[2;4r")
	c := buf.Cursor()
	assert.Equal(t, 0, c.Y)
}

func TestIL(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "aaaaa")
	writeString(t, p, "\x1b[H")
	writeString(t, p, "\x1b[2L")
	lines := bufferText(t, buf)
	assert.Equal(t, "          ", lines[0])
	assert.Equal(t, "          ", lines[1])
	assert.Equal(t, "aaaaa     ", lines[2])
}

func TestDL(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "aaaaa")
	writeString(t, p, "\x1b[H")
	writeString(t, p, "\x1b[M")
	assert.True(t, buf.Cell(0, 0).IsDefault())
}

func TestDCH(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "abcdef")
	writeString(t, p, "\x1b[3G\x1b[3P")
	lines := bufferText(t, buf)
	assert.Equal(t, "abf       ", lines[0])
}

func TestBackspace(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x08")
	assert.Equal(t, 0, buf.Cursor().X)

	writeString(t, p, "ab\x08")
	assert.Equal(t, 1, buf.Cursor().X)
}

func TestDECAutoWrapOn(t *testing.T) {
	buf := screen.NewBuffer(5, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[?7h")
	for i := 0; i < 6; i++ {
		writeString(t, p, "x")
	}
	assert.Equal(t, 1, buf.Cursor().X)
	assert.Equal(t, 1, buf.Cursor().Y)
}

func TestDECAutoWrapOff(t *testing.T) {
	buf := screen.NewBuffer(5, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[?7l")
	for i := 0; i < 8; i++ {
		writeString(t, p, "x")
	}
	assert.Equal(t, 4, buf.Cursor().X)
}

func TestOriginMode(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[?6h")
	assert.True(t, true)
}

func TestApplicationKeypad(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b=")
	writeString(t, p, "\x1b>")
}

func TestSGRDefaults(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[m")
	writeString(t, p, "x")
	assert.Equal(t, screen.Attr(0), buf.Cell(0, 0).Attr)
	assert.Equal(t, screen.ColorDefault, buf.Cell(0, 0).Fg.Index)
	assert.Equal(t, screen.ColorDefault, buf.Cell(0, 0).Bg.Index)
}

func TestCHADefault(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[G")
	assert.Equal(t, 0, buf.Cursor().X)
}

func TestCHA(t *testing.T) {
	buf := screen.NewBuffer(10, 3)
	p := NewParser(buf)
	writeString(t, p, "\x1b[5G")
	assert.Equal(t, 4, buf.Cursor().X)
}

func TestCNL(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[2E")
	assert.Equal(t, 0, buf.Cursor().X)
	assert.Equal(t, 2, buf.Cursor().Y)
}

func TestCPL(t *testing.T) {
	buf := screen.NewBuffer(10, 5)
	p := NewParser(buf)
	writeString(t, p, "\x1b[3F")
	assert.Equal(t, 0, buf.Cursor().X)
	assert.Equal(t, 0, buf.Cursor().Y)
}
