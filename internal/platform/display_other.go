//go:build !linux

package platform

func displayServer() string { return DisplayNative }
