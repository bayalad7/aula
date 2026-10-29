package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Class restrictions (docs/protocol.md §5.13–5.14) ------------------------------------

// RestrictionRequest is what the Restricciones panel applies.
type RestrictionRequest struct {
	Kind  string   `json:"kind"`  // "apps" | "web"
	Mode  string   `json:"mode"`  // "deny" | "allow"
	Items []string `json:"items"` // app names or domains, as typed
	Hosts []string `json:"hosts"` // marked students; empty: the whole channel
}

// RestrictionsView is the panel's state.
type RestrictionsView struct {
	Apps *policy.Rule `json:"apps"`
	Web  *policy.Rule `json:"web"`
}

// Restrictions returns the restrictions of the class.
func (a *App) Restrictions() RestrictionsView {
	r := a.restr.Rules()
	return RestrictionsView{Apps: r[policy.KindApps], Web: r[policy.KindWeb]}
}

// SetRestriction applies a list of apps or domains to the class (or to the
// marked students). Students who reconnect get it again.
func (a *App) SetRestriction(req RestrictionRequest) (RestrictionsView, error) {
	if a.currentClass() == nil {
		return a.Restrictions(), errNoClass
	}
	mode := protocol.PolicyMode(req.Mode)
	switch req.Kind {
	case string(policy.KindApps):
		set := protocol.AppBlockSet{PolicyID: protocol.NewID(), Mode: mode, Apps: cleanApps(req.Items)}
		if err := set.Validate(); err != nil {
			return a.Restrictions(), friendlyPolicy(err, "aplicaciones")
		}
		a.restr.SetApps(set, req.Hosts)
	case string(policy.KindWeb):
		set := protocol.WebFilterSet{PolicyID: protocol.NewID(), Mode: mode, Domains: cleanDomains(req.Items)}
		if err := set.Validate(); err != nil {
			return a.Restrictions(), friendlyPolicy(err, "sitios")
		}
		a.restr.SetWeb(set, req.Hosts)
	default:
		return a.Restrictions(), fmt.Errorf("tipo de restricción %q desconocido", req.Kind)
	}
	a.log.Info("restriction set", "kind", req.Kind, "mode", req.Mode, "items", len(req.Items), "hosts", len(req.Hosts))
	a.syncRestrictions()
	return a.Restrictions(), nil
}

// ClearRestriction removes the apps or the web restriction of the class.
func (a *App) ClearRestriction(kind string) RestrictionsView {
	a.restr.Clear(policy.Kind(kind))
	a.log.Info("restriction cleared", "kind", kind)
	a.syncRestrictions()
	return a.Restrictions()
}

// ClearAllRestrictions is "Quitar restricciones".
func (a *App) ClearAllRestrictions() RestrictionsView {
	a.restr.Clear(policy.KindApps)
	a.restr.Clear(policy.KindWeb)
	a.log.Info("all restrictions cleared")
	a.syncRestrictions()
	return a.Restrictions()
}

// syncRestrictions sends what each agent is missing: the policy to those
// that must have it, the clear to those that no longer must.
func (a *App) syncRestrictions() {
	class := a.currentClass()
	if class == nil {
		return
	}
	a.sendRestrictions(class, a.restr.Plan(class.Sessions()))
	a.emit(EventRestrictions, a.Restrictions())
}

func (a *App) sendRestrictions(class classroom, cmds []policy.Command) {
	if len(cmds) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, c := range cmds {
		if _, err := class.Send(ctx, c.SessionID, c.Payload); err != nil {
			a.log.Debug("restriction not sent", "type", c.Payload.MessageType(), "err", err)
			continue
		}
		a.restr.Sent(c)
	}
}

// onPolicyAck records an agent's acknowledgement; its card shows it.
func (a *App) onPolicyAck(sessionID string, p protocol.Payload) {
	a.restr.Ack(sessionID, p)
	if class := a.currentClass(); class != nil {
		a.emit(EventStudents, a.studentsOf(class.Sessions()))
	}
}

// studentsOf adds to each card whether the class's restrictions are active
// on it.
func (a *App) studentsOf(list []session.Session) Students {
	st := studentsOf(list)
	for i := range st.Students {
		s := &st.Students[i]
		s.AppsActive = a.restr.Active(s.ID, policy.KindApps)
		s.WebActive = a.restr.Active(s.ID, policy.KindWeb)
	}
	return st
}

// cleanApps takes app names as a teacher types them: one per line, with or
// without ".exe", even a full path.
func cleanApps(items []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		it = strings.Trim(strings.TrimSpace(it), `"`)
		if i := strings.LastIndexAny(it, `\/`); i >= 0 {
			it = it[i+1:]
		}
		k := protocol.AppKey(it)
		if it == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, it)
	}
	return out
}

// cleanDomains takes sites as a teacher types or pastes them: "https://www.
// youtube.com/watch?v=…" becomes "youtube.com" (subdomains are included).
func cleanDomains(items []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		d := strings.ToLower(strings.TrimSpace(it))
		if i := strings.Index(d, "://"); i >= 0 {
			d = d[i+3:]
		}
		if i := strings.IndexAny(d, "/?#"); i >= 0 {
			d = d[:i]
		}
		if h, _, err := splitPort(d); err == nil {
			d = h
		}
		d = strings.TrimPrefix(strings.TrimSuffix(d, "."), "www.")
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func splitPort(d string) (string, string, error) {
	if i := strings.LastIndexByte(d, ':'); i > 0 && !strings.Contains(d[i:], "]") {
		return d[:i], d[i+1:], nil
	}
	return d, "", errors.New("no port")
}

func friendlyPolicy(err error, what string) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "must have 1-"):
		limit := protocol.MaxBlockedApps
		if what == "sitios" {
			limit = protocol.MaxFilterDomain
		}
		return fmt.Errorf("escribe entre 1 y %d %s, uno por línea", limit, what)
	case strings.Contains(msg, "mode must be"):
		return errors.New("elige si se bloquean esos o solo se permiten esos")
	case strings.Contains(msg, "app name"):
		return errors.New("un nombre de aplicación no es válido: usa el nombre del ejecutable, p. ej. steam o steam.exe")
	case strings.Contains(msg, "invalid domain"):
		return errors.New("un sitio no es válido: escribe dominios como youtube.com (sin acentos; usa la forma xn-- para ellos)")
	default:
		return err
	}
}
