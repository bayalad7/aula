// Package health is the health of the lab's computers (prompt 10.25,
// docs/protocol.md §5.27): a few data of the computer, never of the student
// (docs/privacidad.md rules 92–95).
//
// On the agent's side, Monitor samples the processor every 5 s, builds the
// health.report of the closed list, sends it every 60 s ±10 % and earlier on
// a notable change, and answers health.request; SelfTest runs the self-test
// on demand. On the console's side, Lab keeps the last report of every
// computer in memory, turns it into alerts with the thresholds of Ajustes ›
// Salud and follows the self-tests.
//
// Nothing here calls OS APIs: the data come through platform.SystemHealth,
// and the network probes use the standard library's net, like the
// connection checker. Nothing here cleans, restarts or changes the
// computer.
package health
