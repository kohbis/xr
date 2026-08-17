// Package exitcode carries a process exit status through the cobra error path
// for commands that have already reported the failure in their own output.
//
// Commands that print a per-repository summary (for example xr repo sync) must
// still exit non-zero so callers and CI notice failures, but returning an
// ordinary error would make cobra print "Error: ..." and the usage text on top
// of the summary the command already wrote. Returning Silent instead keeps the
// output untouched and only changes the exit status.
package exitcode

import (
	"errors"
	"fmt"
)

// Error is an error that carries an exit status and no user-facing message.
type Error struct {
	Code int
}

func (e *Error) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Silent returns an error requesting the given exit status without output.
// Callers must silence cobra's error and usage reporting on the command that
// returns it, since the command is responsible for reporting the failure.
func Silent(code int) error {
	return &Error{Code: code}
}

// From reports the exit status requested by err, if any.
func From(err error) (int, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, true
	}
	return 0, false
}
