package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxLogSize is the size at which the log is rotated (one previous copy kept).
const maxLogSize = 1 << 20

// OpenLogFile opens the app's log for appending, in LogDir(app), e.g.
// %LOCALAPPDATA%\Aula\alumno\aula-cliente.log. When it exceeds 1 MiB it is
// renamed to <name>.1 (replacing the previous copy) and a new one starts. It
// returns the file and its path.
func OpenLogFile(app, name string) (*os.File, string, error) {
	dir, err := LogDir(app)
	if err != nil {
		return nil, "", fmt.Errorf("log dir: %w", err)
	}
	return OpenLogFileIn(dir, name)
}

// OpenLogFileIn is OpenLogFile in a given directory, e.g. AgentServiceLogDir
// for the agent service, which has no user profile of its own.
func OpenLogFileIn(dir, name string) (*os.File, string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", fmt.Errorf("log dir: %w", err)
	}
	path := filepath.Join(dir, name)
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open log: %w", err)
	}
	return f, path, nil
}
