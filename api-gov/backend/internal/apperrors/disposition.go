package apperrors

import (
	"errors"
)

type Disposition string

const (
	Ack        Disposition = "ACK"
	Retry      Disposition = "RETRY"
	BadRequest Disposition = "BAD_REQUEST"
)

func (d Disposition) Error() string { return string(d) }

type DispositionError interface {
	error
	Disposition() Disposition
}

type dispositionError struct {
	err  error
	disp Disposition
}

func (e dispositionError) Error() string            { return e.err.Error() }
func (e dispositionError) Unwrap() error            { return e.err }
func (e dispositionError) Disposition() Disposition { return e.disp }

func Mark(err error, disp Disposition) error {
	if err == nil {
		return nil
	}
	return dispositionError{err: err, disp: disp}
}

func GetDisposition(err error) (Disposition, bool) {
	var de DispositionError
	if errors.As(err, &de) {
		return de.Disposition(), true
	}
	return "", false
}
