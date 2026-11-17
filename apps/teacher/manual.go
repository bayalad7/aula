package main

import (
	"encoding/base64"
	"path"
	"strings"
	"sync"

	"github.com/bayalad7/aula/docs"
	"github.com/bayalad7/aula/internal/health"
	"github.com/bayalad7/aula/internal/manual"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Manual (docs/administracion, embedded from the docs package) ------------------

// diagnosticDiagram is the flowchart «¿Falta un alumno?» is built from.
const diagnosticDiagram = "12-diagnostico-conexion"

// ManualView is everything the Ayuda view and the contextual help draw. Pages
// are data, never HTML: the frontend builds them with DOM calls.
type ManualView struct {
	Pages    []manual.Page             `json:"pages"`
	Diagrams map[string]manual.Diagram `json:"diagrams"`
	Images   map[string]string         `json:"images"` // file name → data: URI
	Errors   map[string]string         `json:"errors"` // error code → manual link
	// AgentErrors links the protocol error codes the students send.
	AgentErrors map[protocol.ErrorCode]string `json:"agentErrors"`
	Tree        *manual.Tree                  `json:"diagnosis"` // «¿Falta un alumno?»; nil if it could not be read
	// HealthAlerts links each health alert of Laboratorio to what to do
	// (prompt 10.25).
	HealthAlerts map[health.AlertID]string `json:"healthAlerts"`
}

// healthAlertHelp is the section of the manual each health alert links to.
// TestHealthAlertsLinkToTheManual checks every alert has one and it exists.
var healthAlertHelp = map[health.AlertID]string{
	health.AlertDiskSystem:    "solucion-de-problemas#un-equipo-tiene-el-disco-casi-lleno",
	health.AlertDiskDocuments: "solucion-de-problemas#un-equipo-tiene-el-disco-casi-lleno",
	health.AlertDiskFailing:   "solucion-de-problemas#el-disco-de-un-equipo-tiene-fallas",
	health.AlertMemory:        "solucion-de-problemas#un-equipo-tiene-poca-memoria-o-el-procesador-saturado",
	health.AlertCPUBusy:       "solucion-de-problemas#un-equipo-tiene-poca-memoria-o-el-procesador-saturado",
	health.AlertCPUHot:        "solucion-de-problemas#el-procesador-de-un-equipo-está-limitado",
	health.AlertReboot:        "solucion-de-problemas#un-equipo-espera-un-reinicio",
	health.AlertClock:         "solucion-de-problemas#el-reloj-de-un-equipo-está-desfasado",
	health.AlertNoDNS:         "solucion-de-problemas#un-equipo-no-tiene-internet",
	health.AlertNoInternet:    "solucion-de-problemas#un-equipo-no-tiene-internet",
	health.AlertLinkSlow:      "solucion-de-problemas#el-cable-de-red-de-un-equipo-va-lento",
	health.AlertWiFiWeak:      "solucion-de-problemas#un-equipo-se-conecta-por-wi-fi",
	health.AlertWiFiCable:     "solucion-de-problemas#un-equipo-se-conecta-por-wi-fi",
	health.AlertOnBattery:     "solucion-de-problemas#un-equipo-funciona-con-batería",
	health.AlertBatteryLow:    "solucion-de-problemas#un-equipo-funciona-con-batería",
	health.AlertBatteryWorn:   "solucion-de-problemas#un-equipo-funciona-con-batería",
}

var (
	manualOnce sync.Once
	manualData *manual.Manual
	manualView ManualView
	manualErr  error
)

// loadManual reads the embedded manual once.
func (a *App) loadManual() (*manual.Manual, ManualView, error) {
	manualOnce.Do(func() {
		m, err := manual.Load(docs.Files(), docs.ManualDir, docs.DiagramsDir)
		if err != nil {
			manualErr = err
			return
		}
		v := ManualView{Pages: m.Pages, Diagrams: m.Diagrams, Images: map[string]string{}, Errors: errorHelp,
			AgentErrors: agentAndFileHelp(), HealthAlerts: healthAlertHelp}
		for name, b := range m.Images {
			v.Images[name] = "data:" + imageType(name) + ";base64," + base64.StdEncoding.EncodeToString(b)
		}
		if d, ok := m.Diagrams[diagnosticDiagram]; ok {
			if t, err := manual.ParseTree(d.Mermaid); err == nil {
				v.Tree = &t
			} else {
				a.log.Warn("diagnostic tree unreadable", "err", err)
			}
		}
		manualData, manualView = m, v
	})
	return manualData, manualView, manualErr
}

// Manual returns the manual for the Ayuda view.
func (a *App) Manual() (ManualView, error) {
	_, v, err := a.loadManual()
	if err != nil {
		a.log.Error("manual unreadable", "err", err)
		return ManualView{}, codedf(codeManualFailed, "no se pudo leer el manual: %w", err)
	}
	return v, nil
}

// ManualSearch finds the sections and diagrams that mention every word of q.
func (a *App) ManualSearch(q string) []manual.Result {
	m, _, err := a.loadManual()
	if err != nil {
		return []manual.Result{}
	}
	return m.Search(q)
}

// imageType is the media type of a manual image by its extension: the
// captures are WebP; older ones may still be PNG.
func imageType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	}
	return "image/png"
}
