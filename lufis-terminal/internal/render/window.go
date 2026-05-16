package render

import (
	"fmt"
	"image"
	"log/slog"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() {
	runtime.LockOSThread()
}

type Window struct {
	width    int
	height   int
	title    string
	window   *glfw.Window
	logger   *slog.Logger
	program  uint32
	vao      uint32
	vbo      uint32
	texture  uint32
	OnKey    func(key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey)
	OnChar   func(char rune)
	OnResize func(width, height int)
}

func New(width, height int, title string, logger *slog.Logger) (*Window, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("glfw init: %w", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.CocoaRetinaFramebuffer, glfw.True)

	window, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("glfw create window: %w", err)
	}

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		window.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("opengl init: %w", err)
	}

	glfw.SwapInterval(1)

	w := &Window{
		width:  width,
		height: height,
		title:  title,
		window: window,
		logger: logger,
	}

	prog, err := w.createProgram()
	if err != nil {
		window.Destroy()
		glfw.Terminate()
		return nil, err
	}
	w.program = prog

	w.setupQuad()

	gl.GenTextures(1, &w.texture)
	gl.BindTexture(gl.TEXTURE_2D, w.texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	window.SetKeyCallback(w.onKey)
	window.SetCharCallback(w.onChar)
	window.SetSizeCallback(w.onResize)

	return w, nil
}

func (w *Window) createProgram() (uint32, error) {
	vertSrc := `#version 410 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aTex;
out vec2 vTex;
void main() {
    gl_Position = vec4(aPos, 0.0, 1.0);
    vTex = aTex;
}`

	fragSrc := `#version 410 core
in vec2 vTex;
out vec4 FragColor;
uniform sampler2D uTex;
void main() {
    FragColor = texture(uTex, vTex);
}`

	vert := gl.CreateShader(gl.VERTEX_SHADER)
	cstr, free := gl.Strs(vertSrc + "\x00")
	gl.ShaderSource(vert, 1, cstr, nil)
	free()
	gl.CompileShader(vert)
	if !checkShader(vert) {
		return 0, fmt.Errorf("vertex shader compile failed")
	}

	frag := gl.CreateShader(gl.FRAGMENT_SHADER)
	cstr2, free2 := gl.Strs(fragSrc + "\x00")
	gl.ShaderSource(frag, 1, cstr2, nil)
	free2()
	gl.CompileShader(frag)
	if !checkShader(frag) {
		return 0, fmt.Errorf("fragment shader compile failed")
	}

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vert)
	gl.AttachShader(prog, frag)
	gl.LinkProgram(prog)

	gl.DeleteShader(vert)
	gl.DeleteShader(frag)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		return 0, fmt.Errorf("program link failed")
	}

	return prog, nil
}

func checkShader(s uint32) bool {
	var status int32
	gl.GetShaderiv(s, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(s, gl.INFO_LOG_LENGTH, &logLen)
		log := strings.Repeat("\x00", int(logLen+1))
		gl.GetShaderInfoLog(s, logLen, nil, gl.Str(log))
		return false
	}
	return true
}

func (w *Window) setupQuad() {
	vertices := []float32{
		-1, -1, 0, 1,
		1, -1, 1, 1,
		1, 1, 1, 0,
		-1, 1, 0, 0,
	}

	gl.GenVertexArrays(1, &w.vao)
	gl.GenBuffers(1, &w.vbo)

	gl.BindVertexArray(w.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, w.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 4*4, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 4*4, 2*4)
}

func (w *Window) ShouldClose() bool { return w.window.ShouldClose() }

func (w *Window) PollEvents() { glfw.PollEvents() }

func (w *Window) Destroy() {
	if w.texture != 0 {
		gl.DeleteTextures(1, &w.texture)
	}
	if w.vbo != 0 {
		gl.DeleteBuffers(1, &w.vbo)
	}
	if w.vao != 0 {
		gl.DeleteVertexArrays(1, &w.vao)
	}
	if w.program != 0 {
		gl.DeleteProgram(w.program)
	}
	w.window.Destroy()
	glfw.Terminate()
}

func (w *Window) Present(img *image.RGBA) {
	fbW, fbH := w.window.GetFramebufferSize()
	gl.Viewport(0, 0, int32(fbW), int32(fbH))

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	gl.UseProgram(w.program)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, w.texture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(img.Rect.Dx()), int32(img.Rect.Dy()),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(img.Pix))

	gl.BindVertexArray(w.vao)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)

	w.window.SwapBuffers()
}

func (w *Window) FBWidth() int {
	fbW, _ := w.window.GetFramebufferSize()
	return fbW
}

func (w *Window) FBHeight() int {
	_, fbH := w.window.GetFramebufferSize()
	return fbH
}

func (w *Window) onKey(_ *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if w.OnKey != nil {
		w.OnKey(key, scancode, action, mods)
	}
}

func (w *Window) onChar(_ *glfw.Window, char rune) {
	if w.OnChar != nil {
		w.OnChar(char)
	}
}

func (w *Window) onResize(_ *glfw.Window, width, height int) {
	w.width = width
	w.height = height
	if w.OnResize != nil {
		w.OnResize(width, height)
	}
}
