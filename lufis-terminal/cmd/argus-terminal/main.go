package main

import (
	"image"
	"image/color"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/argus/terminal/internal/ansi"
	"github.com/argus/terminal/internal/auth"
	"github.com/argus/terminal/internal/automate"
	"github.com/argus/terminal/internal/blocks"
	"github.com/argus/terminal/internal/config"
	"github.com/argus/terminal/internal/notes"
	"github.com/argus/terminal/internal/opencode"
	"github.com/argus/terminal/internal/pty"
	"github.com/argus/terminal/internal/render"
	"github.com/argus/terminal/internal/screen"
	"github.com/argus/terminal/internal/session"
)

var (
	defFg = color.RGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}
	defBg = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
)

type App struct {
	cfg       config.Config
	logger    *slog.Logger
	term      *pty.Terminal
	parser    *ansi.Parser
	buf       *screen.Buffer
	win       *render.Window
	font      *render.FontFace
	screenImg *image.RGBA

	notes      *notes.Store
	auth       *auth.Store
	blocks     *blocks.Store
	session    *session.Store
	automation *automate.Engine
	agent      *opencode.Agent
}

func main() {
	if err := run(); err != nil {
		slog.Error("terminal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	app := &App{}
	var err error

	app.cfg, err = config.Load()
	if err != nil {
		return err
	}

	app.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dataDir := os.Getenv("ARGUS_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = home + "/.config/argus-terminal"
	}

	app.notes, _ = notes.NewStore(dataDir)
	app.auth, _ = auth.NewStore(dataDir)
	app.blocks, _ = blocks.NewStore(dataDir)
	app.session, _ = session.NewStore(dataDir)
	app.automation = automate.New(app.logger)
	app.automation.Start()
	defer app.automation.Stop()

	app.font, err = render.NewFontFace(app.cfg.Terminal.FontSize)
	if err != nil {
		return err
	}

	app.buf = screen.NewBuffer(80, 24)
	app.parser = ansi.NewParser(app.buf)

	app.term = pty.New(app.logger)
	app.term.OnOutput = func(data string) {
		app.parser.WriteString(data)
	}

	app.win, err = render.New(app.cfg.Terminal.Width, app.cfg.Terminal.Height, "Argus Terminal", app.logger)
	if err != nil {
		return err
	}
	defer app.win.Destroy()

	if err := app.term.Start(app.cfg.Terminal.Shell, 24, 80); err != nil {
		return err
	}
	defer app.term.Close()

	cw := app.font.CellWidth()
	ch := app.font.CellHeight()

	app.win.OnResize = func(width, height int) {
		cols := width / cw
		rows := height / ch
		if cols < 10 {
			cols = 10
		}
		if rows < 2 {
			rows = 2
		}
		_ = app.term.Resize(uint16(rows), uint16(cols))
	}

	app.screenImg = image.NewRGBA(image.Rect(0, 0, 80*cw, 24*ch))

	app.win.OnKey = func(key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
		if action != glfw.Press && action != glfw.Repeat {
			return
		}

		if mods == glfw.ModControl && key == glfw.KeyO {
			_ = app.spawnOpenCode()
			return
		}

		if mods == glfw.ModControl {
			switch key {
			case glfw.KeyA:
				_ = app.term.Write("\x01")
				return
			case glfw.KeyB:
				_ = app.term.Write("\x02")
				return
			case glfw.KeyC:
				_ = app.term.Write("\x03")
				return
			case glfw.KeyD:
				_ = app.term.Write("\x04")
				return
			case glfw.KeyE:
				_ = app.term.Write("\x05")
				return
			case glfw.KeyF:
				_ = app.term.Write("\x06")
				return
			case glfw.KeyK:
				_ = app.term.Write("\x0b")
				return
			case glfw.KeyL:
				_ = app.term.Write("\x0c")
				return
			case glfw.KeyN:
				_ = app.term.Write("\x0e")
				return
			case glfw.KeyP:
				_ = app.term.Write("\x10")
				return
			case glfw.KeyR:
				_ = app.term.Write("\x12")
				return
			case glfw.KeyU:
				_ = app.term.Write("\x15")
				return
			case glfw.KeyW:
				_ = app.term.Write("\x17")
				return
			case glfw.KeyX:
				_ = app.term.Write("\x18")
				return
			case glfw.KeyY:
				_ = app.term.Write("\x19")
				return
			case glfw.KeyZ:
				_ = app.term.Write("\x1a")
				return
			}
		}

		switch key {
		case glfw.KeyEscape:
			_ = app.term.Write("\x1b")
		case glfw.KeyEnter:
			_ = app.term.Write("\r")
		case glfw.KeyBackspace:
			_ = app.term.Write("\x7f")
		case glfw.KeyDelete:
			_ = app.term.Write("\x1b[3~")
		case glfw.KeyTab:
			_ = app.term.Write("\t")
		case glfw.KeyUp:
			_ = app.term.Write("\x1b[A")
		case glfw.KeyDown:
			_ = app.term.Write("\x1b[B")
		case glfw.KeyRight:
			_ = app.term.Write("\x1b[C")
		case glfw.KeyLeft:
			_ = app.term.Write("\x1b[D")
		case glfw.KeyHome:
			_ = app.term.Write("\x1b[H")
		case glfw.KeyEnd:
			_ = app.term.Write("\x1b[F")
		}
	}

	app.win.OnChar = func(char rune) {
		_ = app.term.Write(string(char))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for !app.win.ShouldClose() {
		select {
		case <-sigCh:
			return nil
		default:
			app.win.PollEvents()
		}

		if app.buf.Dirty() {
			renderTerminal(app.buf, app.font, app.screenImg)
			app.buf.ClearDirty()
		}

		app.win.Present(app.screenImg)
	}

	return nil
}

func (app *App) spawnOpenCode() error {
	cfg := opencode.ModelConfig{
		Provider: opencode.ProviderOllama,
		Model:    "llama3",
		BaseURL:  "http://localhost:11434/v1",
	}

	if cred, ok := app.auth.Get("openai"); ok && cred.APIKey != "" {
		cfg = opencode.ModelConfig{
			Provider: opencode.ProviderOpenAI,
			Model:    "gpt-4o-mini",
			APIKey:   cred.APIKey,
		}
	}

	app.agent = opencode.NewAgent(cfg, app.logger)
	app.logger.Info("opencode spawned", "provider", cfg.Provider, "model", cfg.Model)
	return nil
}

func renderTerminal(buf *screen.Buffer, font *render.FontFace, img *image.RGBA) {
	cw := font.CellWidth()
	ch := font.CellHeight()

	for y := 0; y < buf.Height(); y++ {
		for x := 0; x < buf.Width(); x++ {
			cell := buf.Cell(x, y)
			fg, bg := resolveColors(cell)
			px := x * cw
			py := y * ch
			font.FillBg(img, px, py, bg)
			if cell.Rune != 0 {
				font.DrawGlyph(img, cell.Rune, px, py, fg)
			}
		}
	}
}

func resolveColors(cell screen.Cell) (color.RGBA, color.RGBA) {
	var fg, bg color.RGBA
	if cell.Fg.IsTrue {
		fg = cell.Fg.True
	} else if cell.Fg.Index != screen.ColorDefault {
		fg = screen.IndexedRGBA(cell.Fg.Index)
	} else {
		fg = defFg
	}
	if cell.Bg.IsTrue {
		bg = cell.Bg.True
	} else if cell.Bg.Index != screen.ColorDefault {
		bg = screen.IndexedRGBA(cell.Bg.Index)
	} else {
		bg = defBg
	}
	if cell.Attr&screen.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	if cell.Attr&screen.AttrBold != 0 {
		fg = brighten(fg)
	}
	return fg, bg
}

func brighten(c color.RGBA) color.RGBA {
	r := int(c.R) * 13 / 10
	g := int(c.G) * 13 / 10
	b := int(c.B) * 13 / 10
	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: c.A}
}
