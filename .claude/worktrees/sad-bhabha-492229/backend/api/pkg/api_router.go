// Package pkg — typed HTTP dispatch.
//
// The frontend talks to the Go backend via Wails bindings in desktop mode and
// via this REST surface in SaaS mode. Earlier iterations dispatched on the URL
// path with reflect.MethodByName, which exposed every exported method on *App
// to anyone who could reach the /api/* endpoint.
//
// This file replaces that with an explicit allowlist: a static map of
// permitted method names to typed adapters. Each adapter knows how to decode
// its arguments from a []json.RawMessage and call the underlying typed method.
//
// To expose a new App method over HTTP, register it in routes() below using
// one of the bindNR/bindNE/bindNRE/bindNV helpers.
package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
)

// apiHandler is the uniform shape every registered method exposes. Each
// returned `any` value is JSON-encoded into APIResponse.Result; a non-nil
// error becomes APIResponse.Error.
type apiHandler func(args []json.RawMessage) (any, error)

// errArgCount is returned when a request's args slice doesn't match the
// registered method's arity.
var errArgCount = errors.New("wrong number of arguments")

// decode unmarshals one positional arg into the destination pointer. Returns
// an explanatory error so the frontend gets "GetPodLogs arg 2: …" rather
// than a bare json error.
func decode(args []json.RawMessage, idx int, dst any, where string) error {
	if idx >= len(args) {
		return fmt.Errorf("%s: missing arg %d", where, idx)
	}
	if err := json.Unmarshal(args[idx], dst); err != nil {
		return fmt.Errorf("%s: arg %d: %w", where, idx, err)
	}
	return nil
}

func wantArity(args []json.RawMessage, n int, where string) error {
	if len(args) != n {
		return fmt.Errorf("%s: %w (got %d, want %d)", where, errArgCount, len(args), n)
	}
	return nil
}

// --- Generic adapters. Names encode arity + return shape. ---
//
//	bindNV   — N args, no return values (void)
//	bindNR   — N args, returns R
//	bindNE   — N args, returns error
//	bindNRE  — N args, returns (R, error)

// 0-arg.

func bind0V(name string, fn func()) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 0, name); err != nil {
			return nil, err
		}
		fn()
		return nil, nil
	}
}
func bind0R[R any](name string, fn func() R) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 0, name); err != nil {
			return nil, err
		}
		return fn(), nil
	}
}
func bind0E(name string, fn func() error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 0, name); err != nil {
			return nil, err
		}
		return nil, fn()
	}
}
func bind0RE[R any](name string, fn func() (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 0, name); err != nil {
			return nil, err
		}
		r, err := fn()
		return r, err
	}
}

// 1-arg.

func bind1V[A any](name string, fn func(A)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 1, name); err != nil {
			return nil, err
		}
		var a A
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		fn(a)
		return nil, nil
	}
}
func bind1R[A any, R any](name string, fn func(A) R) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 1, name); err != nil {
			return nil, err
		}
		var a A
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		return fn(a), nil
	}
}
func bind1E[A any](name string, fn func(A) error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 1, name); err != nil {
			return nil, err
		}
		var a A
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		return nil, fn(a)
	}
}
func bind1RE[A any, R any](name string, fn func(A) (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 1, name); err != nil {
			return nil, err
		}
		var a A
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		r, err := fn(a)
		return r, err
	}
}

// 2-arg.

func bind2E[A, B any](name string, fn func(A, B) error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 2, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		return nil, fn(a, b)
	}
}
func bind2RE[A, B any, R any](name string, fn func(A, B) (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 2, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		r, err := fn(a, b)
		return r, err
	}
}

// 3-arg.

func bind3E[A, B, C any](name string, fn func(A, B, C) error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 3, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		return nil, fn(a, b, c)
	}
}
func bind3RE[A, B, C any, R any](name string, fn func(A, B, C) (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 3, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		r, err := fn(a, b, c)
		return r, err
	}
}

// 4-arg.

func bind4RE[A, B, C, D any, R any](name string, fn func(A, B, C, D) (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 4, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		var d D
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		if err := decode(args, 3, &d, name); err != nil {
			return nil, err
		}
		r, err := fn(a, b, c, d)
		return r, err
	}
}
func bind4E[A, B, C, D any](name string, fn func(A, B, C, D) error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 4, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		var d D
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		if err := decode(args, 3, &d, name); err != nil {
			return nil, err
		}
		return nil, fn(a, b, c, d)
	}
}

// 5-arg.

func bind5E[A, B, C, D, E any](name string, fn func(A, B, C, D, E) error) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 5, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		var d D
		var e E
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		if err := decode(args, 3, &d, name); err != nil {
			return nil, err
		}
		if err := decode(args, 4, &e, name); err != nil {
			return nil, err
		}
		return nil, fn(a, b, c, d, e)
	}
}
func bind5RE[A, B, C, D, E any, R any](name string, fn func(A, B, C, D, E) (R, error)) apiHandler {
	return func(args []json.RawMessage) (any, error) {
		if err := wantArity(args, 5, name); err != nil {
			return nil, err
		}
		var a A
		var b B
		var c C
		var d D
		var e E
		if err := decode(args, 0, &a, name); err != nil {
			return nil, err
		}
		if err := decode(args, 1, &b, name); err != nil {
			return nil, err
		}
		if err := decode(args, 2, &c, name); err != nil {
			return nil, err
		}
		if err := decode(args, 3, &d, name); err != nil {
			return nil, err
		}
		if err := decode(args, 4, &e, name); err != nil {
			return nil, err
		}
		r, err := fn(a, b, c, d, e)
		return r, err
	}
}
