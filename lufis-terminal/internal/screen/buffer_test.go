package screen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBuffer(t *testing.T) {
	tests := []struct {
		name   string
		w, h   int
		wantW  int
		wantH  int
		wantCX int
		wantCY int
	}{
		{name: "default size", w: 80, h: 24, wantW: 80, wantH: 24, wantCX: 0, wantCY: 0},
		{name: "small", w: 10, h: 5, wantW: 10, wantH: 5, wantCX: 0, wantCY: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.w, tt.h)
			assert.Equal(t, tt.wantW, b.Width())
			assert.Equal(t, tt.wantH, b.Height())
			c := b.Cursor()
			assert.Equal(t, tt.wantCX, c.X)
			assert.Equal(t, tt.wantCY, c.Y)
			assert.True(t, b.Dirty())
		})
	}
}

func TestPrintChars(t *testing.T) {
	b := NewBuffer(10, 3)
	b.ClearDirty()

	b.Print('h')
	b.Print('e')
	b.Print('l')
	b.Print('l')
	b.Print('o')

	assert.Equal(t, 'h', b.Cell(0, 0).Rune)
	assert.Equal(t, 'e', b.Cell(1, 0).Rune)
	assert.Equal(t, 'l', b.Cell(2, 0).Rune)
	assert.Equal(t, 'l', b.Cell(3, 0).Rune)
	assert.Equal(t, 'o', b.Cell(4, 0).Rune)
	assert.True(t, b.Cell(5, 0).IsDefault())
	assert.True(t, b.Dirty())
}

func TestPrintNewlineWrap(t *testing.T) {
	b := NewBuffer(5, 3)
	for i := 0; i < 5; i++ {
		b.Print('x')
	}
	b.Print('y')

	assert.Equal(t, 'y', b.Cell(0, 1).Rune)
	assert.Equal(t, 1, b.Cursor().Y)
	assert.Equal(t, 1, b.Cursor().X)
}

func TestPrintNewline(t *testing.T) {
	b := NewBuffer(10, 3)
	b.Print('a')
	b.IND()
	b.Print('b')

	assert.Equal(t, 'a', b.Cell(0, 0).Rune)
	assert.Equal(t, 'b', b.Cell(1, 1).Rune)
	assert.Equal(t, 1, b.Cursor().Y)
}

func TestTab(t *testing.T) {
	b := NewBuffer(20, 3)
	b.Print('a')
	b.Print('\t')
	b.Print('b')

	assert.Equal(t, 'a', b.Cell(0, 0).Rune)
	assert.Equal(t, ' ', b.Cell(1, 0).Rune)
	assert.Equal(t, 'b', b.Cell(8, 0).Rune)
}

func TestCUU(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUP(4, 0)
	b.CUU(1)
	assert.Equal(t, 3, b.Cursor().Y)

	b.CUU(10)
	assert.Equal(t, 0, b.Cursor().Y)
}

func TestCUD(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUD(2)
	assert.Equal(t, 2, b.Cursor().Y)

	b.CUD(10)
	assert.Equal(t, 4, b.Cursor().Y)
}

func TestCUF(t *testing.T) {
	b := NewBuffer(10, 3)
	b.CUF(3)
	assert.Equal(t, 3, b.Cursor().X)

	b.CUF(10)
	assert.Equal(t, 9, b.Cursor().X)
}

func TestCUB(t *testing.T) {
	b := NewBuffer(10, 3)
	b.CUF(5)
	b.CUB(2)
	assert.Equal(t, 3, b.Cursor().X)

	b.CUB(10)
	assert.Equal(t, 0, b.Cursor().X)
}

func TestCUP(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUP(2, 3)
	assert.Equal(t, 2, b.Cursor().Y)
	assert.Equal(t, 3, b.Cursor().X)
}

func TestCUPOutOfRange(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUP(100, 100)
	assert.Equal(t, 4, b.Cursor().Y)
	assert.Equal(t, 9, b.Cursor().X)
}

func TestEL(t *testing.T) {
	b := NewBuffer(10, 3)
	b.CUP(0, 2)
	b.Print('X')
	b.CUP(0, 3)
	b.EL(0)
	assert.Equal(t, 'X', b.Cell(2, 0).Rune)
	assert.True(t, b.Cell(3, 0).IsDefault())
	assert.True(t, b.Cell(4, 0).IsDefault())
}

func TestELStart(t *testing.T) {
	b := NewBuffer(10, 3)
	for _, r := range "abcdef" {
		b.Print(r)
	}
	b.CUP(0, 4)
	b.EL(1)
	assert.True(t, b.Cell(3, 0).IsDefault())
	assert.True(t, b.Cell(4, 0).IsDefault())
	assert.True(t, b.Cell(0, 0).IsDefault())
	assert.Equal(t, 'f', b.Cell(5, 0).Rune)
}

func TestELAll(t *testing.T) {
	b := NewBuffer(5, 3)
	b.Print('a')
	b.Print('b')
	b.EL(2)
	for x := 0; x < 5; x++ {
		assert.True(t, b.Cell(x, 0).IsDefault(), "cell %d should be empty", x)
	}
}

func TestED(t *testing.T) {
	b := NewBuffer(5, 3)
	b.Print('a')
	b.CUP(2, 2)
	b.ED(0)
	assert.False(t, b.Cell(0, 0).IsDefault())
	assert.True(t, b.Cell(0, 1).IsDefault())
}

func TestEDAll(t *testing.T) {
	b := NewBuffer(5, 3)
	b.Print('a')
	b.ED(2)
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			assert.True(t, b.Cell(x, y).IsDefault())
		}
	}
}

func TestIL(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUP(2, 0)
	for x := 0; x < 5; x++ {
		b.Print('a')
	}
	b.CUP(1, 0)
	b.IL(2)

	assert.True(t, b.Cell(0, 1).IsDefault())
	assert.True(t, b.Cell(0, 2).IsDefault())
	assert.Equal(t, 'a', b.Cell(0, 4).Rune)
}

func TestDL(t *testing.T) {
	b := NewBuffer(10, 5)
	b.Print('x')
	b.CUP(0, 0)
	b.DL(1)
	assert.True(t, b.Cell(0, 0).IsDefault())
}

func TestDCH(t *testing.T) {
	b := NewBuffer(10, 3)
	for _, r := range "abcdefghij" {
		b.Print(r)
	}
	b.CUP(0, 2)
	b.DCH(3)
	assert.Equal(t, 'a', b.Cell(0, 0).Rune)
	assert.Equal(t, 'b', b.Cell(1, 0).Rune)
	assert.Equal(t, 'f', b.Cell(2, 0).Rune)
	assert.Equal(t, 'g', b.Cell(3, 0).Rune)
}

func TestECH(t *testing.T) {
	b := NewBuffer(10, 3)
	for _, r := range "abcdef" {
		b.Print(r)
	}
	b.CUP(0, 2)
	b.ECH(3)
	assert.Equal(t, 'a', b.Cell(0, 0).Rune)
	assert.True(t, b.Cell(2, 0).IsDefault())
	assert.True(t, b.Cell(3, 0).IsDefault())
	assert.True(t, b.Cell(4, 0).IsDefault())
	assert.Equal(t, 'f', b.Cell(5, 0).Rune)
}

func TestSU(t *testing.T) {
	b := NewBuffer(5, 4)
	b.CUP(2, 0)
	b.Print('x')

	t.Logf("before SU: cells[15] rune=%d", b.Cell(0, 3).Rune)
	t.Logf("before SU: cells[10] rune=%d", b.Cell(0, 2).Rune)
	t.Logf("before SU: cells[5] rune=%d", b.Cell(0, 1).Rune)

	b.SU(1)

	t.Logf("after SU: cells[15] rune=%d", b.Cell(0, 3).Rune)
	t.Logf("after SU: cells[10] rune=%d", b.Cell(0, 2).Rune)
	t.Logf("after SU: cells[5] rune=%d", b.Cell(0, 1).Rune)

	assert.Equal(t, 'x', b.Cell(0, 1).Rune)
	assert.True(t, b.Cell(0, 3).IsDefault())
}

func TestSD(t *testing.T) {
	b := NewBuffer(5, 4)
	b.CUP(1, 0)
	b.Print('x')
	b.SD(1)
	assert.Equal(t, 'x', b.Cell(0, 2).Rune)
	assert.True(t, b.Cell(0, 0).IsDefault())
}

func TestSGR(t *testing.T) {
	b := NewBuffer(10, 3)
	b.SetAttr(AttrBold)
	b.SetAttr(AttrItalic)
	b.Print('x')

	cell := b.Cell(0, 0)
	assert.Equal(t, AttrBold|AttrItalic, cell.Attr)
}

func TestSGRColor(t *testing.T) {
	b := NewBuffer(10, 3)
	b.SetFG(IndexColor(ColorRed))
	b.SetBG(IndexColor(ColorBlue))
	b.Print('x')

	cell := b.Cell(0, 0)
	assert.Equal(t, ColorRed, cell.Fg.Index)
	assert.Equal(t, ColorBlue, cell.Bg.Index)
}

func TestReset(t *testing.T) {
	b := NewBuffer(10, 3)
	b.Print('x')
	b.SetAttr(AttrBold)
	b.Reset()

	assert.True(t, b.Cell(0, 0).IsDefault())
	assert.Equal(t, Attr(0), b.Attrs())
}

func TestResize(t *testing.T) {
	b := NewBuffer(10, 5)
	for _, r := range "hello" {
		b.Print(r)
	}
	b.Resize(5, 3)
	assert.Equal(t, 5, b.Width())
	assert.Equal(t, 3, b.Height())
	assert.Equal(t, 'h', b.Cell(0, 0).Rune)
}

func TestMargins(t *testing.T) {
	b := NewBuffer(10, 5)
	b.SetMargins(1, 3)
	b.CUP(0, 0)
	b.Print('x')
	assert.Equal(t, 'x', b.Cell(0, 0).Rune)
}

func TestCursorSaveRestore(t *testing.T) {
	b := NewBuffer(10, 5)
	b.CUP(3, 7)
	b.SaveCursor()
	b.CUP(0, 0)
	b.RestoreCursor()
	c := b.Cursor()
	assert.Equal(t, 3, c.Y)
	assert.Equal(t, 7, c.X)
}

func TestAutoWrap(t *testing.T) {
	b := NewBuffer(5, 3)
	b.SetAutoWrap(true)
	for i := 0; i < 6; i++ {
		b.Print('x')
	}
	assert.Equal(t, 'x', b.Cell(0, 1).Rune)
}

func TestAutoWrapOff(t *testing.T) {
	b := NewBuffer(5, 3)
	b.SetAutoWrap(false)
	for i := 0; i < 8; i++ {
		b.Print('x')
	}
	assert.Equal(t, 'x', b.Cell(4, 0).Rune)
}

func TestCellAtBoundary(t *testing.T) {
	b := NewBuffer(5, 3)
	c := b.Cell(-1, 0)
	assert.True(t, c.IsDefault())
	c = b.Cell(0, -1)
	assert.True(t, c.IsDefault())
	c = b.Cell(10, 0)
	assert.True(t, c.IsDefault())
	c = b.Cell(0, 10)
	assert.True(t, c.IsDefault())
}
