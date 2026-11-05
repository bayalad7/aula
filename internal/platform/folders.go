package platform

// DocumentsDir is the user's Documents folder: on Windows the real one
// (FOLDERID_Documents, which follows a redirection to OneDrive or a network
// share), elsewhere ~/Documents. The console files the screenshots the
// teacher saves under it (docs/privacidad.md rule 46).
func DocumentsDir() (string, error) { return documentsDir() }

// OpenFolder shows a folder in the system's file manager (Explorer). It
// fails with ErrNotSupported where Aula has no implementation yet.
func OpenFolder(path string) error { return openFolder(path) }
