package main

import (
	"errors"
	"time"

	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/simclass"
)

// --- Aula simulada (docs/v1/ux/profesor, Fase 6) ------------------------------------------
//
// Simulated students run inside this process and connect to a loopback
// listener of the open class (simulationHost): they speak the real protocol,
// are never announced, never receive power, restrictions or the broadcast,
// and are counted apart from real students everywhere.

// DefaultSimulated is how many students SimulationStart creates.
const DefaultSimulated = 2

// SimRequest is a simulated student to add.
type SimRequest struct {
	OS           string   `json:"os"`           // windows, linux or darwin; "" = windows
	Capabilities []string `json:"capabilities"` // nil = code, notices and screen
}

// SimulationView is the state of the aula simulada.
type SimulationView struct {
	Active       bool               `json:"active"`
	Students     []simclass.Student `json:"students"`
	Max          int                `json:"max"`
	Capabilities []string           `json:"capabilities"` // those a simulated student may declare
}

var (
	errSimUnavailable = codedf(codeSimUnavailable, "esta clase no admite alumnos simulados")
	errSimNotActive   = codedf(codeSimClosed, "el aula simulada no está abierta")
)

// Simulation returns the state of the aula simulada.
func (a *App) Simulation() SimulationView {
	a.mu.Lock()
	sim := a.sim
	a.mu.Unlock()
	v := SimulationView{Max: simclass.MaxStudents, Students: []simclass.Student{}}
	for _, c := range simclass.Capabilities {
		v.Capabilities = append(v.Capabilities, string(c))
	}
	if sim != nil {
		v.Active = true
		v.Students = sim.Students()
	}
	return v
}

// SimulationStart opens the aula simulada with n students (0 means
// DefaultSimulated). If it is already open, it only adds them.
func (a *App) SimulationStart(n int) (SimulationView, error) {
	if n <= 0 {
		n = DefaultSimulated
	}
	for range n {
		if _, err := a.SimulationAdd(SimRequest{}); err != nil {
			return a.Simulation(), err
		}
	}
	return a.Simulation(), nil
}

// SimulationAdd adds one simulated student, opening the aula simulada first
// if needed.
func (a *App) SimulationAdd(req SimRequest) (SimulationView, error) {
	sim, err := a.ensureSimulation()
	if err != nil {
		return a.Simulation(), err
	}
	var caps []protocol.Capability
	if req.Capabilities != nil {
		caps = []protocol.Capability{}
		for _, c := range req.Capabilities {
			caps = append(caps, protocol.Capability(c))
		}
	}
	if _, err := sim.Add(protocol.OS(req.OS), caps); err != nil {
		switch {
		case errors.Is(err, simclass.ErrFull):
			err = codedf(codeSimFull, "ya hay %d alumnos simulados, el máximo", simclass.MaxStudents)
		case errors.Is(err, simclass.ErrBadCapability):
			err = codedf(codeSimCapability, "un alumno simulado solo puede recibir código y avisos, mostrar su pantalla, levantar la mano y escribir")
		}
		return a.Simulation(), err
	}
	return a.Simulation(), nil
}

// SimulationRemove removes one simulated student.
func (a *App) SimulationRemove(id string) (SimulationView, error) {
	a.mu.Lock()
	sim := a.sim
	a.mu.Unlock()
	if sim == nil {
		return a.Simulation(), errSimNotActive
	}
	if err := sim.Remove(id); err != nil {
		return a.Simulation(), codedf(codeSimGone, "ese alumno simulado ya no existe")
	}
	return a.Simulation(), nil
}

// SimulationFault makes a simulated student fail: "none", "slow" (delayMs
// late), "noack" or "disconnected".
func (a *App) SimulationFault(id, fault string, delayMs int) (SimulationView, error) {
	a.mu.Lock()
	sim := a.sim
	a.mu.Unlock()
	if sim == nil {
		return a.Simulation(), errSimNotActive
	}
	err := sim.SetFault(id, simclass.Fault(fault), time.Duration(delayMs)*time.Millisecond)
	switch {
	case errors.Is(err, simclass.ErrUnknownStudent):
		err = codedf(codeSimGone, "ese alumno simulado ya no existe")
	case errors.Is(err, simclass.ErrBadFault):
		err = codedf(codeSimFault, "falla %q desconocida", fault)
	}
	return a.Simulation(), err
}

// SimulationStop closes the aula simulada: its students leave and are
// forgotten. Real students are not touched.
func (a *App) SimulationStop() SimulationView {
	if class := a.currentClass(); class != nil {
		a.stopSimulationOf(class)
		a.onChange()
	}
	return a.Simulation()
}

// ensureSimulation opens the simulated side of the class and the student set.
func (a *App) ensureSimulation() (*simclass.Class, error) {
	class := a.currentClass()
	if class == nil {
		return nil, errNoClass
	}
	host, ok := class.(simulationHost)
	if !ok {
		return nil, errSimUnavailable
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sim != nil {
		return a.sim, nil
	}
	con, err := host.StartSimulation()
	if err != nil {
		return nil, err
	}
	sim, err := simclass.New(con, version, a.emitSimulation, a.log)
	if err != nil {
		host.StopSimulation()
		return nil, codedf(codeSimFailed, "no se pudo abrir el aula simulada: %w", err)
	}
	a.sim = sim
	return sim, nil
}

func (a *App) emitSimulation() { a.emit(EventSimulation, a.Simulation()) }

// SimulationHand makes a simulated student raise its hand (up) or lower it,
// to try «Participación» without a lab (docs/protocol.md §5.18).
func (a *App) SimulationHand(id string, up bool) (SimulationView, error) {
	return a.simParticipation(func(sim *simclass.Class) error { return sim.Raise(id, up) })
}

// SimulationMessage makes a simulated student write to the teacher; an empty
// text picks a sample question.
func (a *App) SimulationMessage(id, text string) (SimulationView, error) {
	return a.simParticipation(func(sim *simclass.Class) error { return sim.Write(id, text) })
}

func (a *App) simParticipation(do func(*simclass.Class) error) (SimulationView, error) {
	a.mu.Lock()
	sim := a.sim
	a.mu.Unlock()
	if sim == nil {
		return a.Simulation(), errSimNotActive
	}
	err := do(sim)
	switch {
	case err == nil:
	case errors.Is(err, simclass.ErrUnknownStudent):
		err = codedf(codeSimGone, "ese alumno simulado ya no existe")
	case errors.Is(err, simclass.ErrBadCapability):
		err = codedf(codeSimCapability, "ese alumno simulado no levanta la mano ni escribe")
	case errors.Is(err, simclass.ErrNotConnected):
		err = codedf(codeStudentGone, "ese alumno simulado no está conectado")
	case errors.Is(err, participation.ErrTooSoon), errors.Is(err, participation.ErrWaiting):
		err = codedf(codeSimTooSoon, "espera unos segundos antes de repetirlo")
	}
	return a.Simulation(), err
}
