// Package simclass runs simulated students inside the console process, so a
// teacher can try Aula end to end without a lab (docs/v1/ux/profesor,
// Fase 6). Each student is a real session.Agent speaking the real protocol
// over TLS to a loopback listener of the console that is never announced over
// mDNS: nothing here adds protocol messages or reaches real students.
//
// Students show code like the real app (codedist.Receiver), answer screen
// thumbnails with a drawn screen (screen.Streamer), and can simulate the
// faults the console already knows how to show: disconnected, slow and no
// acknowledgement. They hold no files and log with simulated=true.
//
// It does not touch the OS: screens are drawn in memory.
package simclass
