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
)

// errorCodes lists them all, for the test and the frontend contract.
var errorCodes = []string{
	codeNoClass, codePortInUse, codeSiteKey, codeConsoleKey, codeCertificate, codeInvalidChannel,
	codeInvalidName, codeInvalidMode, codeEmptyCode, codeTitleTooLong, codeCodeTooLarge, codeInvalidNotice,
	codeUnknownLanguage, codeSnippetGone, codeDeliveryGone, codeClipboardUnavailable, codeClipboardEmpty,
	codeClipboardTooLarge, codeNoH264, codeHideConsole, codeAlreadyBroadcasting, codeBroadcastFailed,
	codeInvalidPower, codeNoPowerTargets, codeInvalidRestriction, codeInvalidApp, codeInvalidDomain,
	codeSimUnavailable, codeSimClosed, codeSimFull, codeSimCapability, codeSimGone, codeSimFault, codeSimFailed,
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
