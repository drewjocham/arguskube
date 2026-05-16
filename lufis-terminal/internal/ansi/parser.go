package ansi

import (
	"strconv"
	"strings"

	"github.com/argus/terminal/internal/screen"
)

type Parser struct {
	buffer   *screen.Buffer
	state    state
	params   []int
	intermed []byte
	private  byte
	runeBuf  []rune
	oscBuf   strings.Builder
}

type state int

const (
	stateGround state = iota
	stateEscape
	stateCSIEntry
	stateCSIParam
	stateCSIIntermediate
	stateCSIIgnore
	stateOSCString
	stateDCS
)

func NewParser(buf *screen.Buffer) *Parser {
	return &Parser{
		buffer:  buf,
		state:   stateGround,
		params:  make([]int, 0, 16),
		runeBuf: make([]rune, 0, 4),
	}
}

func (p *Parser) Buffer() *screen.Buffer { return p.buffer }

func (p *Parser) Write(data []byte) {
	for _, b := range data {
		p.advance(b)
	}
}

func (p *Parser) WriteString(s string) {
	for i := 0; i < len(s); i++ {
		p.advance(s[i])
	}
}

func (p *Parser) advance(b byte) {
	switch p.state {
	case stateGround:
		p.ground(b)
	case stateEscape:
		p.escape(b)
	case stateCSIEntry:
		p.csiEntry(b)
	case stateCSIParam:
		p.csiParam(b)
	case stateCSIIntermediate:
		p.csiIntermediate(b)
	case stateCSIIgnore:
		p.csiIgnore(b)
	case stateOSCString:
		p.oscString(b)
	case stateDCS:
		p.dcs(b)
	}
}

func (p *Parser) clearParams() {
	p.params = p.params[:0]
	p.intermed = p.intermed[:0]
	p.private = 0
}

func (p *Parser) addParam(v int) {
	if len(p.params) < 32 {
		p.params = append(p.params, v)
	}
}

func (p *Parser) param(i int) int {
	if i < len(p.params) {
		return p.params[i]
	}
	return 0
}

func (p *Parser) paramOr(i int, def int) int {
	if i < len(p.params) && p.params[i] != 0 {
		return p.params[i]
	}
	return def
}

func (p *Parser) paramClamp(i int, def int) int {
	v := p.paramOr(i, def)
	if v <= 0 {
		v = def
	}
	return v
}

func (p *Parser) ground(b byte) {
	switch {
	case b == 0x1b:
		p.state = stateEscape
	case b == 0x00 || b == 0x7f:
	case b == 0x07:
		p.buffer.Print('\a')
	case b == 0x08:
		p.buffer.CUB(1)
	case b == 0x09:
		p.buffer.Print('\t')
	case b == 0x0a:
		p.buffer.IND()
	case b == 0x0b:
		p.buffer.CUD(1)
	case b == 0x0c:
		p.buffer.Print('\f')
	case b == 0x0d:
		p.state = stateGround
		p.buffer.CUP(p.buffer.Cursor().Y, 0)
	case 0x20 <= b && b <= 0x7e:
		p.buffer.Print(rune(b))
	case b >= 0x80:
		p.decodeUTF8(b)
	}
}

func (p *Parser) escape(b byte) {
	p.state = stateGround
	switch b {
	case '[':
		p.clearParams()
		p.state = stateCSIEntry
	case ']':
		p.oscBuf.Reset()
		p.state = stateOSCString
	case 'P':
		p.clearParams()
		p.state = stateDCS
	case '7':
		p.buffer.SaveCursor()
	case '8':
		p.buffer.RestoreCursor()
	case 'c':
		p.buffer.Reset()
	case 'D':
		p.buffer.IND()
	case 'E':
		p.buffer.NEL()
	case 'H':
		p.buffer.SetMargins(0, 0)
	case 'M':
		p.buffer.RI()
	case 'Z':
	case '=':
		p.buffer.SetApplicationKP(true)
	case '>':
		p.buffer.SetApplicationKP(false)
	default:
		if ' ' <= b && b <= '/' {
			p.intermed = append(p.intermed, b)
		}
	}
}

func (p *Parser) csiEntry(b byte) {
	switch {
	case 0x30 <= b && b <= 0x39:
		p.state = stateCSIParam
		p.addParam(int(b - 0x30))
	case b == ';':
		p.state = stateCSIParam
		p.addParam(0)
	case b == '?' || b == '>' || b == '!' || b == '\'' || b == '"':
		p.private = b
	case 0x20 <= b && b <= 0x2f:
		p.state = stateCSIIntermediate
		p.intermed = append(p.intermed, b)
	case 0x40 <= b && b <= 0x7e:
		p.dispatchCSI(b)
	case b == 0x1b:
		p.state = stateEscape
	default:
		p.state = stateGround
	}
}

func (p *Parser) csiParam(b byte) {
	switch {
	case 0x30 <= b && b <= 0x39:
		v := p.params[len(p.params)-1]*10 + int(b-0x30)
		p.params[len(p.params)-1] = v
	case b == ';':
		p.addParam(0)
	case 0x20 <= b && b <= 0x2f:
		p.state = stateCSIIntermediate
		p.intermed = append(p.intermed, b)
	case 0x40 <= b && b <= 0x7e:
		p.dispatchCSI(b)
	case b == 0x1b:
		p.state = stateEscape
	default:
		p.state = stateGround
	}
}

func (p *Parser) csiIntermediate(b byte) {
	switch {
	case 0x20 <= b && b <= 0x2f:
		p.intermed = append(p.intermed, b)
	case 0x40 <= b && b <= 0x7e:
		p.dispatchCSI(b)
	case b == 0x1b:
		p.state = stateEscape
	default:
		p.state = stateGround
	}
}

func (p *Parser) csiIgnore(b byte) {
	if 0x40 <= b && b <= 0x7e {
		p.state = stateGround
	} else if b == 0x1b {
		p.state = stateEscape
	}
}

func (p *Parser) oscString(b byte) {
	switch {
	case b == 0x07:
		p.dispatchOSC()
		p.state = stateGround
	case b == 0x1b:
		p.oscBuf.Reset()
		p.state = stateEscape
	case b == 0x9c:
		p.dispatchOSC()
		p.state = stateGround
	default:
		p.oscBuf.WriteByte(b)
	}
}

func (p *Parser) dcs(b byte) {
	if b == 0x1b {
		p.state = stateEscape
	} else if b == 0x9c || b == 0x07 {
		p.state = stateGround
	}
}

func (p *Parser) dispatchCSI(b byte) {
	p.state = stateGround
	hasIntermed := len(p.intermed) > 0

	if hasIntermed && p.intermed[0] == ' ' {
		switch b {
		case 'q':
			return
		}
	}

	switch b {
	case '@':
		n := p.paramClamp(0, 1)
		for i := 0; i < n; i++ {
			p.buffer.Print(' ')
		}
	case 'A':
		p.buffer.CUU(p.paramClamp(0, 1))
	case 'B':
		p.buffer.CUD(p.paramClamp(0, 1))
	case 'C':
		p.buffer.CUF(p.paramClamp(0, 1))
	case 'D':
		p.buffer.CUB(p.paramClamp(0, 1))
	case 'E':
		n := p.paramClamp(0, 1)
		p.buffer.CUD(n)
		p.buffer.CUP(p.buffer.Cursor().Y, 0)
	case 'F':
		n := p.paramClamp(0, 1)
		p.buffer.CUU(n)
		p.buffer.CUP(p.buffer.Cursor().Y, 0)
	case 'G':
		p.buffer.CHA(p.paramClamp(0, 1) - 1)
	case 'H', 'f':
		row := p.paramOr(0, 1) - 1
		col := p.paramOr(1, 1) - 1
		p.buffer.CUP(row, col)
	case 'J':
		p.buffer.ED(p.param(0))
	case 'K':
		p.buffer.EL(p.param(0))
	case 'L':
		p.buffer.IL(p.paramClamp(0, 1))
	case 'M':
		p.buffer.DL(p.paramClamp(0, 1))
	case 'P':
		p.buffer.DCH(p.paramClamp(0, 1))
	case 'S':
		p.buffer.SU(p.paramClamp(0, 1))
	case 'T':
		p.buffer.SD(p.paramClamp(0, 1))
	case 'X':
		p.buffer.ECH(p.paramClamp(0, 1))
	case 'd':
		p.buffer.VPA(p.paramClamp(0, 1) - 1)
	case 'e':
		n := p.paramClamp(0, 1)
		p.buffer.CUD(n)
	case 'h':
		p.dispatchMode(true)
	case 'l':
		p.dispatchMode(false)
	case 'm':
		p.dispatchSGR()
	case 'r':
		if len(p.params) >= 2 {
			p.buffer.SetMargins(p.paramOr(0, 1)-1, p.paramOr(1, 1)-1)
		} else {
			p.buffer.SetMargins(0, p.buffer.Height()-1)
		}
	case 's':
		p.buffer.SaveCursor()
	case 'u':
		p.buffer.RestoreCursor()
	}
}

func (p *Parser) dispatchMode(set bool) {
	if p.private == '?' {
		for _, param := range p.params {
			switch param {
			case 1:
				p.buffer.SetApplicationKP(set)
			case 3:
			case 4:
				p.buffer.SetOriginMode(set)
			case 7:
				p.buffer.SetAutoWrap(set)
			case 25:
			case 47:
			case 1047:
			case 1048:
			case 1049:
			}
		}
	}
}

func (p *Parser) dispatchSGR() {
	if len(p.params) == 0 {
		p.buffer.ResetAttrs()
		return
	}
	for i := 0; i < len(p.params); i++ {
		param := p.params[i]
		switch {
		case param == 0:
			p.buffer.ResetAttrs()
		case param == 1:
			p.buffer.SetAttr(screen.AttrBold)
		case param == 2:
			p.buffer.SetAttr(screen.AttrDim)
		case param == 3:
			p.buffer.SetAttr(screen.AttrItalic)
		case param == 4:
			p.buffer.SetAttr(screen.AttrUnderline)
		case param == 5 || param == 6:
			p.buffer.SetAttr(screen.AttrBlink)
		case param == 7:
			p.buffer.SetAttr(screen.AttrReverse)
		case param == 8:
			p.buffer.SetAttr(screen.AttrHidden)
		case param == 9:
			p.buffer.SetAttr(screen.AttrStrikethrough)
		case 21 <= param && param <= 29:
		case param == 22:
			p.buffer.ClearAttr(screen.AttrBold)
			p.buffer.ClearAttr(screen.AttrDim)
		case param == 23:
			p.buffer.ClearAttr(screen.AttrItalic)
		case param == 24:
			p.buffer.ClearAttr(screen.AttrUnderline)
		case param == 25:
			p.buffer.ClearAttr(screen.AttrBlink)
		case param == 27:
			p.buffer.ClearAttr(screen.AttrReverse)
		case param == 28:
			p.buffer.ClearAttr(screen.AttrHidden)
		case param == 29:
			p.buffer.ClearAttr(screen.AttrStrikethrough)
		case 30 <= param && param <= 37:
			p.buffer.SetFG(screen.IndexColor(screen.ColorIndex(param - 30 + 1)))
		case param == 38:
			c, skip := p.parseTrueColor(i)
			if skip > 0 {
				p.buffer.SetFG(c)
				i += skip
			}
		case param == 39:
			p.buffer.SetFG(screen.IndexColor(screen.ColorDefault))
		case 40 <= param && param <= 47:
			p.buffer.SetBG(screen.IndexColor(screen.ColorIndex(param - 40 + 1)))
		case param == 48:
			c, skip := p.parseTrueColor(i)
			if skip > 0 {
				p.buffer.SetBG(c)
				i += skip
			}
		case param == 49:
			p.buffer.SetBG(screen.IndexColor(screen.ColorDefault))
		case 90 <= param && param <= 97:
			p.buffer.SetFG(screen.IndexColor(screen.ColorIndex(param - 90 + 9)))
		case 100 <= param && param <= 107:
			p.buffer.SetBG(screen.IndexColor(screen.ColorIndex(param - 100 + 9)))
		}
	}
}

func (p *Parser) parseTrueColor(i int) (screen.Color, int) {
	if i+1 >= len(p.params) {
		return screen.IndexColor(screen.ColorDefault), 0
	}
	sub := p.params[i+1]
	switch sub {
	case 2:
		if i+4 < len(p.params) {
			r := clampU8(p.params[i+2])
			g := clampU8(p.params[i+3])
			b := clampU8(p.params[i+4])
			return screen.TrueColor(r, g, b), 4
		}
	case 5:
		if i+2 < len(p.params) {
			idx := p.params[i+2]
			if idx < 256 {
				return screen.IndexColor(screen.ColorIndex(idx + 1)), 2
			}
		}
	}
	return screen.IndexColor(screen.ColorDefault), 0
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func (p *Parser) dispatchOSC() {
	raw := p.oscBuf.String()
	idx := strings.IndexByte(raw, ';')
	if idx < 0 {
		return
	}
	cmdStr := raw[:idx]
	cmd, err := strconv.Atoi(cmdStr)
	if err != nil {
		return
	}
	data := raw[idx+1:]
	switch cmd {
	case 0, 1, 2:
	case 4:
	case 10:
	case 11:
	case 52:
	}
	_ = data
}

func (p *Parser) decodeUTF8(b byte) {
	p.runeBuf = p.runeBuf[:0]
	p.runeBuf = append(p.runeBuf, rune(b))
}
