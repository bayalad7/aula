//go:build !windows

package platform

func newWorkFolder() (WorkFolder, error) { return nil, ErrNotSupported }

func newWorkFolderReader() (WorkFolderReader, error) { return nil, ErrNotSupported }
