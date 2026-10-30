package main

import (
	"strings"
	"sync"

	"github.com/bayalad7/aula/internal/protocol"
)

// A student answers with a protocol error message (docs/protocol.md §2.1)
// when it cannot process an order. The console shows the last one on the
// student's row and, when it answers a delivery, on that delivery. The
// frontend explains each code (textos.es-MX.js, errorAlumno.<code>) and links
// it to the manual (agentErrorHelp).

// AgentError is the last error a student reported.
type AgentError struct {
	Code string `json:"code"` // protocol error code
	At   string `json:"at"`   // "15:04:05"
}

// agentErrorHelp is the section of the manual for each protocol error code.
// TestAgentErrorCodesLinkToTheManual checks every code of internal/protocol
// has one, and its texts.
var agentErrorHelp = map[protocol.ErrorCode]string{
	protocol.CodeUnsupportedVersion:    "solucion-de-problemas#el-equipo-del-alumno-no-pudo-hacerlo",
	protocol.CodeInvalidMessage:        "solucion-de-problemas#el-equipo-del-alumno-no-pudo-hacerlo",
	protocol.CodeMessageTooLarge:       "solucion-de-problemas#el-equipo-del-alumno-no-pudo-hacerlo",
	protocol.CodeUnknownType:           "solucion-de-problemas#el-equipo-del-alumno-no-pudo-hacerlo",
	protocol.CodeUnauthorized:          "solucion-de-problemas#la-clave-no-coincide",
	protocol.CodeUnsupportedCapability: "conceptos#qué-puede-hacer-cada-equipo",
	protocol.CodeInternal:              "solucion-de-problemas#el-equipo-del-alumno-no-pudo-hacerlo",
}

// agentErrorKey is the code as the frontend names its texts:
// "unsupported_capability" → "unsupportedCapability".
func agentErrorKey(code protocol.ErrorCode) string {
	parts := strings.Split(string(code), "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// agentErrors keeps the last error of each session. It has its own lock
// because studentsIn runs with App.mu held.
type agentErrors struct {
	mu sync.Mutex
	m  map[string]AgentError
}

func (e *agentErrors) set(sessionID string, err AgentError) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.m == nil {
		e.m = map[string]AgentError{}
	}
	e.m[sessionID] = err
}

func (e *agentErrors) get(sessionID string) *AgentError {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err, ok := e.m[sessionID]; ok {
		return &err
	}
	return nil
}

// keep forgets the sessions that are no longer in the list.
func (e *agentErrors) keep(present map[string]bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id := range e.m {
		if !present[id] {
			delete(e.m, id)
		}
	}
}

// onAgentError takes an error message from a student (real or simulated).
func (a *App) onAgentError(sessionID string, p protocol.ErrorPayload) {
	a.log.Info("student reported an error", "conn", sessionID, "code", p.Code)
	a.agentErrs.set(sessionID, AgentError{Code: string(p.Code), At: a.now().Format("15:04:05")})
	a.sendMu.Lock() // the delivery it answers may still be registering
	a.mu.Lock()
	_, rejected := a.deliveries.Reject(p.RefID, string(p.Code))
	a.mu.Unlock()
	a.sendMu.Unlock()
	if rejected {
		a.emitDeliveries()
	}
	if class := a.currentClass(); class != nil {
		a.emit(EventStudents, a.studentsIn(class))
	}
}
