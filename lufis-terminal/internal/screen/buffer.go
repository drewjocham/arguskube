package screen

import (
	"image/color"
	"unicode"
)

const scrollbackLimit = 50000

type Buffer struct {
	width, height int
	cells         []Cell
	cursor        Cursor
	saved         Cursor
	attrs         Attr
	fg, bg        Color
	topMargin     int
	bottomMargin  int
	tabWidth      int
	scrollback    *ringBuffer
	dirty         bool
	autoWrap      bool
	originMode    bool
	applicationKP bool
}

func NewBuffer(width, height int) *Buffer {
	b := &Buffer{
		width:    width,
		height:   height,
		cells:    make([]Cell, width*height),
		cursor:   Cursor{X: 0, Y: 0, Visible: true, Shape: CursorBlock},
		saved:    Cursor{X: 0, Y: 0, Visible: true, Shape: CursorBlock},
		fg:       IndexColor(ColorDefault),
		bg:       IndexColor(ColorDefault),
		tabWidth: 8,
		dirty:    true,
		autoWrap: true,
	}
	b.resetMargins()
	b.scrollback = newRingBuffer(scrollbackLimit)
	return b
}

func (b *Buffer) Width() int  { return b.width }
func (b *Buffer) Height() int { return b.height }

func (b *Buffer) Cursor() Cursor { return b.cursor }
func (b *Buffer) Dirty() bool    { return b.dirty }
func (b *Buffer) ClearDirty()    { b.dirty = false }

func (b *Buffer) Cell(x, y int) Cell {
	if x < 0 || x >= b.width || y < 0 || y >= b.height {
		return Cell{}
	}
	return b.cells[y*b.width+x]
}

func (b *Buffer) Attrs() Attr { return b.attrs }
func (b *Buffer) FG() Color   { return b.fg }
func (b *Buffer) BG() Color   { return b.bg }

func (b *Buffer) Resize(w, h int) {
	if w == b.width && h == b.height {
		return
	}

	oldCells := b.cells
	oldW, oldH := b.width, b.height
	b.width = w
	b.height = h
	b.cells = make([]Cell, w*h)

	for y := 0; y < h && y < oldH; y++ {
		for x := 0; x < w && x < oldW; x++ {
			b.cells[y*w+x] = oldCells[y*oldW+x]
		}
	}

	if b.cursor.X >= w {
		b.cursor.X = w - 1
	}
	if b.cursor.Y >= h {
		b.cursor.Y = h - 1
	}

	b.resetMargins()
	b.dirty = true
}

func (b *Buffer) Reset() {
	b.cells = make([]Cell, b.width*b.height)
	b.cursor = Cursor{X: 0, Y: 0, Visible: true, Shape: CursorBlock}
	b.saved = Cursor{X: 0, Y: 0, Visible: true, Shape: CursorBlock}
	b.attrs = 0
	b.fg = IndexColor(ColorDefault)
	b.bg = IndexColor(ColorDefault)
	b.autoWrap = true
	b.originMode = false
	b.resetMargins()
	b.scrollback = newRingBuffer(scrollbackLimit)
	b.dirty = true
}

func (b *Buffer) resetMargins() {
	b.topMargin = 0
	b.bottomMargin = b.height - 1
}

func (b *Buffer) SetMargins(top, bottom int) {
	if top >= 0 && bottom > top && bottom < b.height {
		b.topMargin = top
		b.bottomMargin = bottom
		b.cursor.X = 0
		if b.originMode {
			b.cursor.Y = top
		} else {
			b.cursor.Y = 0
		}
	}
}

func (b *Buffer) Print(r rune) {
	if r == 0 {
		return
	}
	b.dirty = true

	pos := b.cursor

	var tabWidth int

	if r == '\t' {
		tabWidth = b.tabWidth - (b.cursor.X % b.tabWidth)
		if tabWidth == 0 {
			tabWidth = b.tabWidth
		}
		for i := 0; i < tabWidth; i++ {
			b.setCell(pos.X+i, pos.Y, Cell{Rune: ' ', Attr: b.attrs, Fg: b.fg, Bg: b.bg})
		}
		b.cursor.X += tabWidth
		if b.cursor.X >= b.width {
			b.cursor.X = b.width - 1
		}
		return
	}

	wide := unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r)

	if wide && pos.X+1 >= b.width {
		b.newline()
		pos = b.cursor
	}

	b.setCell(pos.X, pos.Y, Cell{Rune: r, Attr: b.attrs, Fg: b.fg, Bg: b.bg, Wide: wide})

	if wide {
		b.setCell(pos.X+1, pos.Y, Cell{Rune: 0, Attr: b.attrs, Fg: b.fg, Bg: b.bg, Wide: true})
		b.cursor.X += 2
	} else {
		b.cursor.X++
	}

	if b.cursor.X >= b.width {
		if b.autoWrap {
			b.newline()
		} else {
			b.cursor.X = b.width - 1
		}
	}
}

func (b *Buffer) setCell(x, y int, cell Cell) {
	if x < 0 || x >= b.width || y < 0 || y >= b.height {
		return
	}
	b.cells[y*b.width+x] = cell
}

func (b *Buffer) newline() {
	if b.cursor.Y >= b.bottomMargin {
		b.scrollUp(1)
	} else {
		b.cursor.Y++
	}
	b.cursor.X = 0
}

func (b *Buffer) scrollUp(n int) {
	if n <= 0 {
		return
	}
	b.dirty = true

	top := b.topMargin
	bottom := b.bottomMargin
	regionHeight := bottom - top + 1

	if n > regionHeight {
		n = regionHeight
	}

	scrolled := make([]Cell, b.width*n)
	for y := top; y < top+n && y < b.height; y++ {
		copy(scrolled[(y-top)*b.width:(y-top+1)*b.width], b.cells[y*b.width:(y+1)*b.width])
	}
	b.scrollback.push(scrolled, b.width)

	for y := top; y <= bottom-n; y++ {
		copy(b.cells[y*b.width:(y+1)*b.width], b.cells[(y+n)*b.width:(y+n+1)*b.width])
	}

	for y := bottom - n + 1; y <= bottom; y++ {
		for x := 0; x < b.width; x++ {
			b.cells[y*b.width+x] = Cell{}
		}
	}
}

func (b *Buffer) scrollDown(n int) {
	if n <= 0 {
		return
	}
	b.dirty = true

	top := b.topMargin
	bottom := b.bottomMargin

	if n > bottom-top+1 {
		n = bottom - top + 1
	}

	for y := bottom; y >= top+n; y-- {
		copy(b.cells[y*b.width:(y+1)*b.width], b.cells[(y-n)*b.width:(y-n+1)*b.width])
	}

	for y := top; y < top+n; y++ {
		for x := 0; x < b.width; x++ {
			b.cells[y*b.width+x] = Cell{}
		}
	}
}

func (b *Buffer) CUU(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	target := b.cursor.Y - n
	if target < b.topMargin {
		target = b.topMargin
	}
	b.cursor.Y = target
}

func (b *Buffer) CUD(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	target := b.cursor.Y + n
	if target > b.bottomMargin {
		target = b.bottomMargin
	}
	b.cursor.Y = target
}

func (b *Buffer) CUF(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	target := b.cursor.X + n
	if target >= b.width {
		target = b.width - 1
	}
	b.cursor.X = target
}

func (b *Buffer) CUB(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	target := b.cursor.X - n
	if target < 0 {
		target = 0
	}
	b.cursor.X = target
}

func (b *Buffer) CUP(row, col int) {
	b.dirty = true
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	if row >= b.height {
		row = b.height - 1
	}
	if col >= b.width {
		col = b.width - 1
	}
	if b.originMode {
		row += b.topMargin
		if row > b.bottomMargin {
			row = b.bottomMargin
		}
	}
	b.cursor.Y = row
	b.cursor.X = col
}

func (b *Buffer) CHA(col int) {
	b.dirty = true
	if col < 0 {
		col = 0
	}
	if col >= b.width {
		col = b.width - 1
	}
	b.cursor.X = col
}

func (b *Buffer) VPA(row int) {
	b.dirty = true
	if row < 0 {
		row = 0
	}
	if row >= b.height {
		row = b.height - 1
	}
	if b.originMode {
		row += b.topMargin
		if row > b.bottomMargin {
			row = b.bottomMargin
		}
	}
	b.cursor.Y = row
}

func (b *Buffer) EL(mode int) {
	b.dirty = true
	switch mode {
	case 0:
		for x := b.cursor.X; x < b.width; x++ {
			b.cells[b.cursor.Y*b.width+x] = Cell{}
		}
	case 1:
		for x := 0; x <= b.cursor.X; x++ {
			b.cells[b.cursor.Y*b.width+x] = Cell{}
		}
	case 2:
		for x := 0; x < b.width; x++ {
			b.cells[b.cursor.Y*b.width+x] = Cell{}
		}
	}
}

func (b *Buffer) ED(mode int) {
	b.dirty = true
	switch mode {
	case 0:
		for y := b.cursor.Y; y < b.height; y++ {
			startX := 0
			if y == b.cursor.Y {
				startX = b.cursor.X
			}
			for x := startX; x < b.width; x++ {
				b.cells[y*b.width+x] = Cell{}
			}
		}
	case 1:
		for y := 0; y <= b.cursor.Y; y++ {
			endX := b.width
			if y == b.cursor.Y {
				endX = b.cursor.X + 1
			}
			for x := 0; x < endX; x++ {
				b.cells[y*b.width+x] = Cell{}
			}
		}
	case 2:
		for y := 0; y < b.height; y++ {
			for x := 0; x < b.width; x++ {
				b.cells[y*b.width+x] = Cell{}
			}
		}
	}
}

func (b *Buffer) IL(n int) {
	if n <= 0 {
		n = 1
	}
	if b.cursor.Y < b.topMargin || b.cursor.Y > b.bottomMargin {
		return
	}
	b.dirty = true

	regionBottom := b.bottomMargin
	linesToMove := regionBottom - b.cursor.Y + 1
	if n > linesToMove {
		n = linesToMove
	}

	for y := regionBottom; y >= b.cursor.Y+n; y-- {
		copy(b.cells[y*b.width:(y+1)*b.width], b.cells[(y-n)*b.width:(y-n+1)*b.width])
	}

	for y := b.cursor.Y; y < b.cursor.Y+n; y++ {
		for x := 0; x < b.width; x++ {
			b.cells[y*b.width+x] = Cell{}
		}
	}
}

func (b *Buffer) DL(n int) {
	if n <= 0 {
		n = 1
	}
	if b.cursor.Y < b.topMargin || b.cursor.Y > b.bottomMargin {
		return
	}
	b.dirty = true

	regionBottom := b.bottomMargin
	linesToMove := regionBottom - b.cursor.Y + 1
	if n > linesToMove {
		n = linesToMove
	}

	for y := b.cursor.Y; y <= regionBottom-n; y++ {
		copy(b.cells[y*b.width:(y+1)*b.width], b.cells[(y+n)*b.width:(y+n+1)*b.width])
	}

	for y := regionBottom - n + 1; y <= regionBottom; y++ {
		for x := 0; x < b.width; x++ {
			b.cells[y*b.width+x] = Cell{}
		}
	}
}

func (b *Buffer) DCH(n int) {
	if n <= 0 {
		n = 1
	}
	b.dirty = true
	row := b.cursor.Y
	col := b.cursor.X

	copy(b.cells[row*b.width+col:], b.cells[row*b.width+col+n:row*b.width+b.width])

	for x := b.width - n; x < b.width; x++ {
		b.cells[row*b.width+x] = Cell{}
	}
}

func (b *Buffer) ECH(n int) {
	if n <= 0 {
		n = 1
	}
	b.dirty = true
	row := b.cursor.Y
	for x := b.cursor.X; x < b.cursor.X+n && x < b.width; x++ {
		b.cells[row*b.width+x] = Cell{}
	}
}

func (b *Buffer) SU(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	b.scrollUp(n)
}

func (b *Buffer) SD(n int) {
	b.dirty = true
	if n <= 0 {
		n = 1
	}
	b.scrollDown(n)
}

func (b *Buffer) RI() {
	b.dirty = true
	if b.cursor.Y == b.topMargin {
		b.scrollDown(1)
	} else {
		b.cursor.Y--
	}
}

func (b *Buffer) IND() {
	b.dirty = true
	if b.cursor.Y >= b.bottomMargin {
		b.scrollUp(1)
	} else {
		b.cursor.Y++
	}
}

func (b *Buffer) NEL() {
	b.dirty = true
	if b.cursor.Y >= b.bottomMargin {
		b.scrollUp(1)
	} else {
		b.cursor.Y++
	}
	b.cursor.X = 0
}

func (b *Buffer) SaveCursor() {
	b.saved = b.cursor
}

func (b *Buffer) RestoreCursor() {
	b.cursor = b.saved
	b.dirty = true
}

func (b *Buffer) SetAttr(attr Attr) {
	b.attrs |= attr
}

func (b *Buffer) ClearAttr(attr Attr) {
	b.attrs &^= attr
}

func (b *Buffer) ResetAttrs() {
	b.attrs = 0
	b.fg = IndexColor(ColorDefault)
	b.bg = IndexColor(ColorDefault)
}

func (b *Buffer) SetFG(c Color) { b.fg = c }
func (b *Buffer) SetBG(c Color) { b.bg = c }

func (b *Buffer) SetAutoWrap(on bool)      { b.autoWrap = on }
func (b *Buffer) SetOriginMode(on bool)    { b.originMode = on }
func (b *Buffer) SetApplicationKP(on bool) { b.applicationKP = on }

func (b *Buffer) SaveToScrollback(line []Cell) {
	b.scrollback.push(line, b.width)
}

func (b *Buffer) DefaultFG() color.RGBA { return AnsiColors[ColorBrightWhite] }
func (b *Buffer) DefaultBG() color.RGBA { return AnsiColors[ColorBlack] }
