package main

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Opening a page or an app (docs/protocol.md §5.22) -----------------------------
//
// The panel «Abrir página o app» of the action bar, the action of the student's
// file and the big screen, and Ajustes › Apps permitidas. Only real students:
// simulated ones never declare open.* (it changes the computer, rule 70).

// allowedAppsFile is the catalog of allowed apps: in the site key folder for
// the whole centre (C:\ProgramData\Aula\centro, written by an administrator)
// when it exists there; otherwise in the console's folder.
const allowedAppsFile = "apps-permitidas.json"

// Sources of AllowedApps.
const (
	appsCentre   = "centro"   // the centre's file, shared by its consoles
	appsConsole  = "consola"  // this console's file
	appsExamples = "ejemplos" // no file yet: the examples
)

// AllowedApps is what Ajustes › Apps permitidas shows.
type AllowedApps struct {
	Apps   []openapp.App `json:"apps"`
	Source string        `json:"source"`
	Path   string        `json:"path"` // where it is (or will be) saved
}

func (a *App) allowedAppsPaths() (centre, console string) {
	if a.siteKeyDir != "" {
		centre = filepath.Join(a.siteKeyDir, allowedAppsFile)
	}
	return centre, filepath.Join(a.dir, allowedAppsFile)
}

// AllowedApps returns the catalog: the centre's, this console's or the
// examples, in that order.
func (a *App) AllowedApps() AllowedApps {
	centre, console := a.allowedAppsPaths()
	for _, c := range []struct{ path, source string }{{centre, appsCentre}, {console, appsConsole}} {
		if c.path == "" {
			continue
		}
		data, err := os.ReadFile(c.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil {
			if apps, err := openapp.Parse(data); err == nil {
				return AllowedApps{Apps: apps, Source: c.source, Path: c.path}
			}
		}
		a.log.Warn("allowed apps file unreadable, using the examples", "source", c.source, "err", err)
		break
	}
	apps, _ := openapp.Normalize(openapp.Defaults())
	return AllowedApps{Apps: apps, Source: appsExamples, Path: console}
}

// DefaultAllowedApps returns the examples, for «Restaurar ejemplos».
func (a *App) DefaultAllowedApps() []openapp.App {
	apps, _ := openapp.Normalize(openapp.Defaults())
	return apps
}

// SaveAllowedApps checks and saves the catalog where AllowedApps reads it:
// the centre's file if it exists, otherwise this console's.
func (a *App) SaveAllowedApps(apps []openapp.App) (AllowedApps, error) {
	norm, err := openapp.Normalize(apps)
	if err != nil {
		return a.AllowedApps(), friendlyAllowedApp(err, apps)
	}
	cur := a.AllowedApps()
	path, source := cur.Path, cur.Source
	if source == appsExamples {
		source = appsConsole
	}
	data, err := openapp.Marshal(norm)
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
			err = os.WriteFile(path, data, 0o644)
		}
	}
	if err != nil {
		if source == appsCentre && errors.Is(err, fs.ErrPermission) {
			return cur, codedf(codeAllowedAppsNotSaved, "la lista es la del centro (%s) y solo un administrador puede cambiarla: %w", path, err)
		}
		return cur, codedf(codeAllowedAppsNotSaved, "no se pudo guardar la lista en %s: %w", path, err)
	}
	a.log.Info("allowed apps saved", "source", source, "apps", len(norm))
	out := AllowedApps{Apps: norm, Source: source, Path: path}
	a.emit(EventAllowedApps, out)
	return out, nil
}

func friendlyAllowedApp(err error, apps []openapp.App) error {
	var ee *openapp.EntryError
	if !errors.As(err, &ee) {
		return codedf(codeAllowedAppInvalid, "%w", err)
	}
	if ee.Field == openapp.FieldList {
		return codedf(codeAllowedAppInvalid, "la lista admite hasta %d apps", openapp.MaxApps)
	}
	name := strings.TrimSpace(apps[ee.Index].Name)
	if name == "" {
		name = "sin nombre"
	}
	if ee.Field == openapp.FieldName {
		return codedf(codeAllowedAppInvalid, "app %d (%s): el nombre falta, es muy largo o ya lo tiene otra app", ee.Index+1, name)
	}
	return codedf(codeAllowedAppInvalid, "app %d (%s): la ruta debe ser un .exe instalado en Program Files, Windows o %%LOCALAPPDATA%%\\Programs, y no una herramienta del sistema", ee.Index+1, name)
}

// OpenRequest is what «Abrir página o app» sends.
type OpenRequest struct {
	URL   string   `json:"url"`   // the page, for OpenPage
	AppID string   `json:"appId"` // the app of the catalog, for OpenApp
	Hosts []string `json:"hosts"` // marked students; empty: the whole channel
}

// OpenResult says where an order went.
type OpenResult struct {
	OpenID      string   `json:"openId"`
	Sent        int      `json:"sent"`
	Unsupported []string `json:"unsupported"` // hostnames whose agent cannot open it
	Failed      []string `json:"failed"`      // hostnames the message did not reach
}

// OpenCheck is what the panel warns about before sending.
type OpenCheck struct {
	URL string `json:"url"` // as it will be sent (https:// added if it had no scheme)
	// Filtered are the target hostnames whose web filter blocks the page
	// (docs/privacidad.md rule 69).
	Filtered []string `json:"filtered"`
}

// pageURL adds https:// to an address typed without a scheme and checks it.
func pageURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	if err := protocol.CheckOpenURL(raw); err != nil {
		return "", codedf(codeInvalidURL, "la dirección debe empezar con http:// o https:// y llevar un sitio, sin espacios ni usuario y contraseña")
	}
	return raw, nil
}

// CheckOpenPage checks a page and says which targets would not load it
// because of the class's web filter.
func (a *App) CheckOpenPage(raw string, hosts []string) (OpenCheck, error) {
	u, err := pageURL(raw)
	if err != nil {
		return OpenCheck{Filtered: []string{}}, err
	}
	out := OpenCheck{URL: u, Filtered: []string{}}
	rule := a.Restrictions().Web
	class := a.currentClass()
	if rule == nil || class == nil {
		return out, nil
	}
	parsed, _ := url.Parse(u)
	wp := policy.NewWebPolicy(protocol.WebFilterSet{Mode: rule.Mode, Domains: rule.Items})
	if !wp.Blocks(strings.ToLower(parsed.Hostname()), nil) {
		return out, nil
	}
	ruled := map[string]bool{}
	for _, h := range rule.Hosts {
		ruled[strings.ToLower(h)] = true
	}
	for _, s := range targetsOf(class.Sessions(), hosts) {
		if s.Can(protocol.CapWebFilter) && (len(ruled) == 0 || ruled[strings.ToLower(s.Hostname)]) {
			out.Filtered = append(out.Filtered, s.Hostname)
		}
	}
	return out, nil
}

// OpenPage opens a page in the browser of the class or of the marked students.
func (a *App) OpenPage(req OpenRequest) (OpenResult, error) {
	u, err := pageURL(req.URL)
	if err != nil {
		return OpenResult{Unsupported: []string{}, Failed: []string{}}, err
	}
	p := protocol.OpenURL{OpenID: protocol.NewID(), URL: u}
	return a.sendOpen(p, p.OpenID, openapp.KindURL, u, protocol.CapOpenURL, req.Hosts)
}

// OpenApp starts an app of the catalog in the class or the marked students.
func (a *App) OpenApp(req OpenRequest) (OpenResult, error) {
	for _, app := range a.AllowedApps().Apps {
		if app.ID == req.AppID {
			p := protocol.OpenApp{OpenID: protocol.NewID(), AppID: app.ID, Name: app.Name, Path: app.Path}
			return a.sendOpen(p, p.OpenID, openapp.KindApp, app.Name, protocol.CapOpenApp, req.Hosts)
		}
	}
	return OpenResult{Unsupported: []string{}, Failed: []string{}},
		codedf(codeOpenAppGone, "esa app ya no está en la lista de apps permitidas")
}

func (a *App) sendOpen(p protocol.Payload, id, kind, label string, c protocol.Capability, hosts []string) (OpenResult, error) {
	res := OpenResult{OpenID: id, Unsupported: []string{}, Failed: []string{}}
	class := a.currentClass()
	if class == nil {
		return res, errNoClass
	}
	targets := targetsOf(class.Sessions(), hosts) // real students only (rule 70)
	if !anyCan(targets, c) {
		return res, codedf(codeNoOpenTargets, "ningún alumno conectado de la selección puede abrirlo (su equipo no tiene Aula 0.12 como servicio con la sesión iniciada)")
	}
	a.opens.Start(id, kind, label, a.now())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range targets {
		if !s.Can(c) {
			res.Unsupported = append(res.Unsupported, s.Hostname)
			a.opens.Skipped(id, s.Hostname, true)
			continue
		}
		envID, err := class.Send(ctx, s.ID, p)
		if err != nil {
			a.log.Debug("open not sent", "session", s.ID, "err", err)
			res.Failed = append(res.Failed, s.Hostname)
			a.opens.Skipped(id, s.Hostname, false)
			continue
		}
		a.opens.Sent(id, s.ID, s.Hostname, envID)
		res.Sent++
	}
	// Never the URL nor the app (docs/privacidad.md rule 70).
	a.log.Info("open sent", "open_id", id, "kind", kind, "sent", res.Sent, "unsupported", len(res.Unsupported), "failed", len(res.Failed))
	a.emit(EventOpens, a.OpenOrders())
	return res, nil
}

func anyCan(list []session.Session, c protocol.Capability) bool {
	for _, s := range list {
		if s.Can(c) {
			return true
		}
	}
	return false
}

// OpenOrders returns the last openings, newest first.
func (a *App) OpenOrders() []openapp.Order { return a.opens.Recent() }

// onOpenMessage takes the agents' open.ack and the errors that answer an
// opening.
func (a *App) onOpenMessage(sessionID string, env protocol.Envelope) {
	changed := false
	switch p := env.Payload.(type) {
	case protocol.OpenAck:
		changed = a.opens.Ack(sessionID, p)
	case protocol.ErrorPayload:
		changed = a.opens.Error(sessionID, p.RefID)
	}
	if changed {
		a.emit(EventOpens, a.OpenOrders())
	}
}
