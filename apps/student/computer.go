package main

import "github.com/bayalad7/aula/internal/protocol"

// Computer is «Este equipo» in «Qué puede hacer Aula en este equipo»: its
// hostname and the label the teacher gave it in the room plan (§5.26), so the
// student can tell support where they sit. Go sends the state; the window
// words it.
type Computer struct {
	Hostname string `json:"hostname"`
	Label    string `json:"label"`
}

// handleComputerLabel keeps the label of this session. It shows no notice
// and never takes the focus: it only changes a line of the dialog.
func (a *App) handleComputerLabel(env protocol.Envelope) bool {
	p, ok := env.Payload.(protocol.ComputerLabel)
	if !ok {
		return false
	}
	a.mu.Lock()
	a.label = p.Label
	a.mu.Unlock()
	return true
}

// computer is this computer for the window; call it without a.mu.
func (a *App) computer() Computer {
	h := a.currentHello()
	a.mu.Lock()
	defer a.mu.Unlock()
	return Computer{Hostname: h.Hostname, Label: a.label}
}
