//go:build !windows && !linux

package platform

func newWorkFolder() (WorkFolder, error) { return nil, ErrNotSupported }

func newWorkFolderReader() (WorkFolderReader, error) { return nil, ErrNotSupported }
