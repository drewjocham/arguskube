package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type FontFace struct {
	face   font.Face
	cellW  int
	cellH  int
	ascent int
}

func NewFontFace(size int, dpi float64) (*FontFace, error) {
	if size < 8 {
		size = 8
	}
	if dpi <= 0 {
		dpi = 72
	}

	ttf, err := loadSystemFont()
	if err != nil {
		return nil, fmt.Errorf("no font available: %w", err)
	}

	face := truetype.NewFace(ttf, &truetype.Options{
		Size:    float64(size),
		DPI:     dpi,
		Hinting: font.HintingFull,
	})

	metrics := face.Metrics()
	cellH := metrics.Ascent.Ceil() + metrics.Descent.Ceil()

	advance, ok := face.GlyphAdvance('M')
	var cellW int
	if ok {
		cellW = advance.Ceil()
	} else {
		cellW = cellH * 6 / 10
	}

	if cellW < 5 {
		cellW = 5
	}

	return &FontFace{
		face:   face,
		cellW:  cellW,
		cellH:  cellH,
		ascent: metrics.Ascent.Ceil(),
	}, nil
}

func (f *FontFace) DrawGlyph(dst *image.RGBA, r rune, x, y int, fg color.RGBA) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(fg),
		Face: f.face,
		Dot:  fixed.P(x, y+f.ascent),
	}
	d.DrawString(string(r))
}

func (f *FontFace) FillBg(dst *image.RGBA, x, y int, bg color.RGBA) {
	draw.Draw(dst, image.Rect(x, y, x+f.cellW, y+f.cellH),
		image.NewUniform(bg), image.Point{}, draw.Src)
}

func (f *FontFace) CellWidth() int  { return f.cellW }
func (f *FontFace) CellHeight() int { return f.cellH }
func (f *FontFace) Face() font.Face { return f.face }

func loadSystemFont() (*truetype.Font, error) {
	paths := fontPaths()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if isTTC(data) {
			f, err := extractFirstFont(data)
			if err == nil {
				return f, nil
			}
			continue
		}
		ttf, err := truetype.Parse(data)
		if err == nil {
			return ttf, nil
		}
	}
	return nil, fmt.Errorf("no monospace font found")
}

func fontPaths() []string {
	paths := []string{
		"/System/Library/Fonts/SFNSMono.ttf",
		"/System/Library/Fonts/Menlo.ttc",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/System/Library/Fonts/Courier.ttf",
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		fd := filepath.Join(home, "Library", "Fonts")
		entries, _ := os.ReadDir(fd)
		for _, e := range entries {
			ext := filepath.Ext(e.Name())
			if !e.IsDir() && (ext == ".ttf" || ext == ".otf" || ext == ".ttc") {
				paths = append(paths, filepath.Join(fd, e.Name()))
			}
		}
	}
	return paths
}

func isTTC(data []byte) bool {
	return len(data) > 4 && string(data[:4]) == "ttcf"
}

func extractFirstFont(data []byte) (*truetype.Font, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("invalid ttc")
	}
	major := int(data[4])<<8 | int(data[5])
	if major != 1 && major != 2 {
		return nil, fmt.Errorf("unsupported ttc version %d", major)
	}
	numFonts := int(data[8])<<24 | int(data[9])<<16 | int(data[10])<<8 | int(data[11])
	if numFonts < 1 {
		return nil, fmt.Errorf("no fonts in ttc")
	}
	offset := int(data[12])<<24 | int(data[13])<<16 | int(data[14])<<8 | int(data[15])
	if offset < 0 || offset >= len(data) {
		return nil, fmt.Errorf("invalid font offset")
	}
	return truetype.Parse(data[offset:])
}
