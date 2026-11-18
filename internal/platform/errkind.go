package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"syscall"
)

// kinds are the errors ErrorKind names by their fixed text. None carries a
// path or anything the student wrote.
var kinds = []error{
	ErrFileExists, ErrLinkInPath, ErrNoSpace, ErrDenied, ErrPathTooLong, ErrFileTooLarge,
	ErrOffline, ErrReadOnly, ErrDataMismatch, ErrNotSupported,
	fs.ErrNotExist, fs.ErrExist, fs.ErrPermission, fs.ErrClosed,
	context.Canceled, context.DeadlineExceeded, io.ErrUnexpectedEOF, io.EOF,
}

// ErrorKind names the class of err for a log line, never its text: an
// *os.PathError, or a %q added on the way up, carries the path, which names the
// student and their files (docs/privacidad.md rules 5, 48, 59 and 65). It is
// the text of the first sentinel err wraps, from known (the caller's own,
// tried first) or from this package and the standard library; else the
// operating system's error number; else "other". It is "" for a nil err.
func ErrorKind(err error, known ...error) string {
	if err == nil {
		return ""
	}
	for _, list := range [][]error{known, kinds} {
		for _, s := range list {
			if errors.Is(err, s) {
				return s.Error()
			}
		}
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return fmt.Sprintf("errno %d", uint(errno))
	}
	return "other"
}
