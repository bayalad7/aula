//go:build !windows

package main

import (
	"errors"
	"time"
)

// The measurement reads Windows process counters: the laboratory runs
// Windows, the platform the student app targets first (CLAUDE.md).

type result struct {
	cpu            float64
	ramAvg, ramMax float64
	procs          int
}

func measure(int, time.Duration, func(time.Duration) bool) (result, error) {
	return result{}, errors.New("medir-alumno solo funciona en Windows")
}

type fgInfo struct {
	pid  uint32
	name string
}

func (f fgInfo) String() string        { return f.name }
func (f fgInfo) inTree(int) bool       { return false }
func foreground() fgInfo               { return fgInfo{name: "?"} }
func aboveForeground(int) bool         { return false }
func agentWindow(int) (uintptr, error) { return 0, errors.New("solo en Windows") }
func windowsAlive(int) []uintptr       { return nil }
func visible(uintptr) bool             { return false }
func iconic(uintptr) bool              { return false }
func minimize(uintptr)                 {}
func closeWindow(uintptr)              {}
func alive(int) bool                   { return false }
