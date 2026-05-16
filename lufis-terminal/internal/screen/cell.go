package screen

import (
	"image/color"
)

type ColorIndex uint8

const (
	ColorDefault ColorIndex = iota
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
	ColorBrightBlack
	ColorBrightRed
	ColorBrightGreen
	ColorBrightYellow
	ColorBrightBlue
	ColorBrightMagenta
	ColorBrightCyan
	ColorBrightWhite
)

var AnsiColors = [...]color.RGBA{
	ColorDefault:       {0, 0, 0, 0},
	ColorBlack:         {0x00, 0x00, 0x00, 0xff},
	ColorRed:           {0xcd, 0x31, 0x31, 0xff},
	ColorGreen:         {0x45, 0xb7, 0x3b, 0xff},
	ColorYellow:        {0xcc, 0xc0, 0x3a, 0xff},
	ColorBlue:          {0x3b, 0x6a, 0xd9, 0xff},
	ColorMagenta:       {0xc4, 0x3a, 0xcc, 0xff},
	ColorCyan:          {0x3b, 0xbf, 0xbf, 0xff},
	ColorWhite:         {0xc0, 0xc0, 0xc0, 0xff},
	ColorBrightBlack:   {0x66, 0x66, 0x66, 0xff},
	ColorBrightRed:     {0xe6, 0x4a, 0x4a, 0xff},
	ColorBrightGreen:   {0x5f, 0xe6, 0x5f, 0xff},
	ColorBrightYellow:  {0xe6, 0xe6, 0x5f, 0xff},
	ColorBrightBlue:    {0x5f, 0x8a, 0xe6, 0xff},
	ColorBrightMagenta: {0xe6, 0x5f, 0xe6, 0xff},
	ColorBrightCyan:    {0x5f, 0xe6, 0xe6, 0xff},
	ColorBrightWhite:   {0xff, 0xff, 0xff, 0xff},
}

type Color struct {
	Index  ColorIndex
	True   color.RGBA
	IsTrue bool
}

func IndexColor(idx ColorIndex) Color {
	return Color{Index: idx}
}

func TrueColor(r, g, b uint8) Color {
	return Color{IsTrue: true, True: color.RGBA{R: r, G: g, B: b, A: 255}}
}

func (c Color) RGBA(defaultFg, defaultBg color.RGBA) color.RGBA {
	if c.IsTrue {
		return c.True
	}
	if c.Index == ColorDefault {
		return defaultFg
	}
	return AnsiColors[c.Index]
}

type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrHidden
	AttrStrikethrough
)

type Cell struct {
	Rune rune
	Fg   Color
	Bg   Color
	Attr Attr
	Wide bool
}

func (c Cell) IsDefault() bool {
	return c.Rune == 0 && !c.Fg.IsTrue && c.Fg.Index == ColorDefault &&
		!c.Bg.IsTrue && c.Bg.Index == ColorDefault && c.Attr == 0
}

type Cursor struct {
	X, Y    int
	Visible bool
	Shape   CursorShape
}

type CursorShape int

const (
	CursorDefault CursorShape = iota
	CursorBlock
	CursorUnderline
	CursorBeam
)

func CursorShapeFromString(s string) CursorShape {
	switch s {
	case "block":
		return CursorBlock
	case "underline":
		return CursorUnderline
	case "beam":
		return CursorBeam
	default:
		return CursorDefault
	}
}

type Line struct {
	Cells []Cell
	Dirty bool
}

func IndexedRGBA(idx ColorIndex) color.RGBA {
	if idx >= ColorDefault && idx <= ColorBrightWhite {
		return AnsiColors[idx]
	}
	return AnsiColors[ColorDefault]
}
