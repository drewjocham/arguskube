package k8s

import (
	"context"
	"io"
	"log/slog"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// PodExecSession manages an interactive exec session into a pod container.
type PodExecSession struct {
	stdin    io.WriteCloser
	stdinW   io.Writer
	stdout   io.Reader
	resizeCh chan remotecommand.TerminalSize
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
	logger   *slog.Logger

	// OnOutput is called with each chunk of output from the container.
	OnOutput func(data string)
}

// execPipe bridges io.Reader/Writer for SPDY exec streams.
type execPipe struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newExecPipe() *execPipe {
	r, w := io.Pipe()
	return &execPipe{r: r, w: w}
}

func (p *execPipe) Read(data []byte) (int, error)  { return p.r.Read(data) }
func (p *execPipe) Write(data []byte) (int, error) { return p.w.Write(data) }
func (p *execPipe) Close() error                   { p.w.Close(); return p.r.Close() }

// ExecPodShell starts an interactive shell session in the given pod/container.
// It returns a PodExecSession that can be written to (stdin) and produces output via OnOutput.
func (c *Client) ExecPodShell(ctx context.Context, namespace, podName, container string, rows, cols uint16) (*PodExecSession, error) {
	if container == "" {
		// Pick the first container.
		pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}
	}

	// Detect available shell — try sh (always present), the command will
	// attempt /bin/sh which exists in virtually all container images.
	cmd := []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}

	req := c.cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restCfg, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	stdinPipe := newExecPipe()
	stdoutPipe := newExecPipe()

	sess := &PodExecSession{
		stdin:    stdinPipe,
		stdinW:   stdinPipe.w,
		stdout:   stdoutPipe,
		resizeCh: make(chan remotecommand.TerminalSize, 4),
		done:     make(chan struct{}),
		logger:   c.logger,
	}

	// Send initial size.
	sess.resizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}

	// Run the exec stream in background.
	go func() {
		defer close(sess.done)
		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             stdinPipe,
			Stdout:            stdoutPipe.w,
			Stderr:            stdoutPipe.w,
			Tty:               true,
			TerminalSizeQueue: sess,
		})
		if err != nil {
			sess.logger.Debug("exec stream ended", slog.String("error", err.Error()))
		}
		stdoutPipe.w.Close()
	}()

	// Read output in background.
	go sess.readLoop(stdoutPipe)

	return sess, nil
}

// Next implements remotecommand.TerminalSizeQueue.
func (s *PodExecSession) Next() *remotecommand.TerminalSize {
	size, ok := <-s.resizeCh
	if !ok {
		return nil
	}
	return &size
}

// Write sends raw input (keystrokes) to the container's stdin.
func (s *PodExecSession) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	_, err := s.stdinW.Write([]byte(data))
	return err
}

// Resize updates the terminal dimensions.
func (s *PodExecSession) Resize(rows, cols uint16) {
	select {
	case s.resizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
	}
}

// Close terminates the exec session.
func (s *PodExecSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.stdin.Close()
	close(s.resizeCh)
}

// Done returns a channel that closes when the exec stream ends.
func (s *PodExecSession) Done() <-chan struct{} {
	return s.done
}

func (s *PodExecSession) readLoop(r io.Reader) {
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if n > 0 && s.OnOutput != nil {
			s.OnOutput(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}
