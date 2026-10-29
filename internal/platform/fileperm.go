package platform

import "fmt"

// RestrictToOwner leaves path readable and writable only by the current user.
// On Windows it replaces the DACL with a protected one granting full control to
// the current user only (inherited by files when path is a directory); on other
// systems it sets mode 0600 for files and 0700 for directories.
func RestrictToOwner(path string) error {
	if err := restrictToOwner(path); err != nil {
		return fmt.Errorf("restrict %s to owner: %w", path, err)
	}
	return nil
}

// AttachParentConsole makes a GUI-subsystem executable (as produced by
// wails build on Windows) write its standard output to the console of the
// terminal that launched it, so command-line subcommands can print. It reports
// whether output now reaches a console or a redirect. Elsewhere it is a no-op
// returning true.
func AttachParentConsole() bool {
	return attachParentConsole()
}
