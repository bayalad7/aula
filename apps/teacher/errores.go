package main

import (
	"errors"
	"fmt"
)

// Errors returned by the bindings carry a stable code, so the frontend can
// show its own message ("qué pasó + qué hacer", textos.es-MX.js keys
// error.<code>.titulo and error.<code>.texto) and keep the Go text as the
// technical detail. On the wire (Wails turns errors into strings) a coded
// error reads "aula:<code>: <detail>".

// Error codes. Every one must have its texts; TestErrorCodesHaveTexts checks it.
const (
	codeNoClass              = "noClass"
	codePortInUse            = "portInUse"
	codeSiteKey              = "siteKey"
	codeConsoleKey           = "consoleKey"
	codeCertificate          = "certificate"
	codeInvalidChannel       = "invalidChannel"
	codeInvalidName          = "invalidName"
	codeInvalidMode          = "invalidMode"
	codeEmptyCode            = "emptyCode"
	codeTitleTooLong         = "titleTooLong"
	codeCodeTooLarge         = "codeTooLarge"
	codeInvalidNotice        = "invalidNotice"
	codeUnknownLanguage      = "unknownLanguage"
	codeSnippetGone          = "snippetGone"
	codeDeliveryGone         = "deliveryGone"
	codeClipboardUnavailable = "clipboardUnavailable"
	codeClipboardEmpty       = "clipboardEmpty"
	codeClipboardTooLarge    = "clipboardTooLarge"
	codeNoH264               = "noH264"
	codeHideConsole          = "hideConsole"
	codeAlreadyBroadcasting  = "alreadyBroadcasting"
	codeBroadcastFailed      = "broadcastFailed"
	codeInvalidPower         = "invalidPower"
	codeNoPowerTargets       = "noPowerTargets"
	codeInvalidRestriction   = "invalidRestriction"
	codeInvalidApp           = "invalidApp"
	codeInvalidDomain        = "invalidDomain"
	codeSimUnavailable       = "simUnavailable"
	codeSimClosed            = "simClosed"
	codeSimFull              = "simFull"
	codeSimCapability        = "simCapability"
	codeSimGone              = "simGone"
	codeSimFault             = "simFault"
	codeSimFailed            = "simFailed"
	codeManualFailed         = "manualFailed"
	codeCheckRunning         = "checkRunning"
	codeAlreadyControlling   = "alreadyControlling"
	codeRemoteUnsupported    = "remoteUnsupported"
	codeRemoteGone           = "remoteGone"
	codeSnapshotUnsupported  = "snapshotUnsupported"
	codeSnapshotTooMany      = "snapshotTooMany"
	codeSnapshotGone         = "snapshotGone"
	codeSnapshotSaveFailed   = "snapshotSaveFailed"
	codeSnapshotFolder       = "snapshotFolder"
	codeHandGone             = "handGone"
	codeMessageGone          = "messageGone"
	codeStudentGone          = "studentGone"
	codeSimTooSoon           = "simTooSoon"
)

// errorCodes lists them all, for the test and the frontend contract.
var errorCodes = []string{
	codeNoClass, codePortInUse, codeSiteKey, codeConsoleKey, codeCertificate, codeInvalidChannel,
	codeInvalidName, codeInvalidMode, codeEmptyCode, codeTitleTooLong, codeCodeTooLarge, codeInvalidNotice,
	codeUnknownLanguage, codeSnippetGone, codeDeliveryGone, codeClipboardUnavailable, codeClipboardEmpty,
	codeClipboardTooLarge, codeNoH264, codeHideConsole, codeAlreadyBroadcasting, codeBroadcastFailed,
	codeInvalidPower, codeNoPowerTargets, codeInvalidRestriction, codeInvalidApp, codeInvalidDomain,
	codeSimUnavailable, codeSimClosed, codeSimFull, codeSimCapability, codeSimGone, codeSimFault, codeSimFailed, codeManualFailed,
	codeCheckRunning, codeAlreadyControlling, codeRemoteUnsupported, codeRemoteGone,
	codeSnapshotUnsupported, codeSnapshotTooMany, codeSnapshotGone, codeSnapshotSaveFailed, codeSnapshotFolder,
	codeHandGone, codeMessageGone, codeStudentGone, codeSimTooSoon,
}

// errorHelp is the section of the manual (docs/v1/administracion) that each
// error links to. TestErrorCodesLinkToTheManual checks every code has one and
// that it exists.
var errorHelp = map[string]string{
	codeNoClass:              "primeros-pasos#abrir-tu-primera-clase",
	codePortInUse:            "solucion-de-problemas#la-consola-no-abre",
	codeSiteKey:              "solucion-de-problemas#la-consola-no-abre",
	codeConsoleKey:           "solucion-de-problemas#la-consola-no-abre",
	codeCertificate:          "solucion-de-problemas#la-consola-no-abre",
	codeInvalidChannel:       "solucion-de-problemas#el-alumno-está-en-otro-canal",
	codeInvalidName:          "ajustes#clase",
	codeInvalidMode:          "codigo#enviar",
	codeEmptyCode:            "codigo#enviar",
	codeTitleTooLong:         "codigo#el-editor",
	codeCodeTooLarge:         "codigo#enviar",
	codeInvalidNotice:        "tareas-frecuentes#mandar-un-aviso",
	codeUnknownLanguage:      "codigo#el-editor",
	codeSnippetGone:          "codigo#historial-de-la-clase",
	codeDeliveryGone:         "codigo#últimos-envíos",
	codeClipboardUnavailable: "solucion-de-problemas#el-portapapeles-no-funciona",
	codeClipboardEmpty:       "solucion-de-problemas#el-portapapeles-no-funciona",
	codeClipboardTooLarge:    "solucion-de-problemas#el-portapapeles-no-funciona",
	codeNoH264:               "solucion-de-problemas#no-puedo-mostrar-mi-pantalla",
	codeHideConsole:          "solucion-de-problemas#no-puedo-mostrar-mi-pantalla",
	codeAlreadyBroadcasting:  "solucion-de-problemas#no-puedo-mostrar-mi-pantalla",
	codeBroadcastFailed:      "solucion-de-problemas#no-puedo-mostrar-mi-pantalla",
	codeInvalidPower:         "tareas-frecuentes#apagar-o-reiniciar-los-equipos",
	codeNoPowerTargets:       "tareas-frecuentes#apagar-o-reiniciar-los-equipos",
	codeInvalidRestriction:   "tareas-frecuentes#bloquear-aplicaciones-o-sitios",
	codeInvalidApp:           "tareas-frecuentes#bloquear-aplicaciones-o-sitios",
	codeInvalidDomain:        "tareas-frecuentes#bloquear-aplicaciones-o-sitios",
	codeSimUnavailable:       "probar-sin-laboratorio#abrir-el-aula-simulada",
	codeSimClosed:            "probar-sin-laboratorio#abrir-el-aula-simulada",
	codeSimFull:              "probar-sin-laboratorio#agregar-alumnos-y-simular-fallas",
	codeSimCapability:        "probar-sin-laboratorio#qué-hacen-los-alumnos-simulados",
	codeSimGone:              "probar-sin-laboratorio#agregar-alumnos-y-simular-fallas",
	codeSimFault:             "probar-sin-laboratorio#agregar-alumnos-y-simular-fallas",
	codeSimFailed:            "probar-sin-laboratorio#abrir-el-aula-simulada",
	codeManualFailed:         "",
	codeCheckRunning:         "solucion-de-problemas#comprobar-la-conexión",
	codeAlreadyControlling:   "tareas-frecuentes#ayudar-a-un-alumno-tomando-el-control",
	codeRemoteUnsupported:    "tareas-frecuentes#ayudar-a-un-alumno-tomando-el-control",
	codeRemoteGone:           "tareas-frecuentes#ayudar-a-un-alumno-tomando-el-control",
	codeSnapshotUnsupported:  "tareas-frecuentes#tomar-una-captura-de-la-pantalla-de-un-alumno",
	codeSnapshotTooMany:      "tareas-frecuentes#tomar-una-captura-de-la-pantalla-de-un-alumno",
	codeSnapshotGone:         "tareas-frecuentes#tomar-una-captura-de-la-pantalla-de-un-alumno",
	codeSnapshotSaveFailed:   "ajustes#capturas-de-pantalla",
	codeSnapshotFolder:       "ajustes#capturas-de-pantalla",
	codeHandGone:             "participacion#manos-levantadas",
	codeMessageGone:          "participacion#mensajes-de-los-alumnos",
	codeStudentGone:          "participacion#responder-a-un-alumno",
	codeSimTooSoon:           "probar-sin-laboratorio#qué-hacen-los-alumnos-simulados",
}

// codedError is an error the UI can explain by its code.
type codedError struct {
	code string
	err  error
}

func (e *codedError) Error() string { return "aula:" + e.code + ": " + e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// coded attaches a code to err; a nil err stays nil, and an error that
// already has a code keeps it.
func coded(code string, err error) error {
	if err == nil {
		return nil
	}
	var c *codedError
	if errors.As(err, &c) {
		return err
	}
	return &codedError{code: code, err: err}
}

// codedf builds a coded error from a format.
func codedf(code, format string, args ...any) error {
	return &codedError{code: code, err: fmt.Errorf(format, args...)}
}

// errorCode returns the code of err, or "" if it has none.
func errorCode(err error) string {
	var c *codedError
	if errors.As(err, &c) {
		return c.code
	}
	return ""
}
