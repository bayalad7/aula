package main

import (
	"strings"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Private message to one student (prompt 10.7, docs/protocol.md §5.4) ------------

// SendPrivateNotice sends a notice to one student only («Mensaje privado» in
// the student's file): a msg.show with personal set, to that session, real
// or simulated. Its label in «Últimos envíos» names the student, never the
// text, and the log only says who and how long (docs/privacidad.md rule 53).
func (a *App) SendPrivateNotice(sessionID, text, level string) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	text = strings.TrimSpace(text)
	p := protocol.MsgShow{Text: text, Level: protocol.MsgLevel(level), Personal: true}
	if err := p.Validate(); err != nil {
		return codedist.Delivery{}, friendly(err)
	}
	_, s, ok := participant(class, sessionID)
	if !ok {
		return codedist.Delivery{}, codedf(codeStudentGone, "ese alumno no está conectado")
	}
	if !s.Can(protocol.CapMsgShow) {
		return codedist.Delivery{}, codedf(codeNoticeUnsupported, "el equipo de %s no recibe avisos", s.Hostname)
	}
	d, err := a.broadcastTo("notice", privateLabel(s.Hostname), p, []string{s.ID})
	if err != nil {
		return d, err
	}
	a.log.Info("private notice", "conn", s.ID, "host", s.Hostname, "len", utf8.RuneCountInString(text))
	return d, nil
}

// privateLabel is the delivery's label: who, never what (rule 53).
func privateLabel(host string) string { return "Mensaje privado a " + host }
