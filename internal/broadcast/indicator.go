package broadcast

import (
	"sync"

	"github.com/bayalad7/aula/internal/platform"
)

// IndicatorMux shares the one native badge of the agent between the screen
// stream (thumbnails, remote control, screenshots: internal/screen) and a
// presentation. While a presentation is live its text wins («Tu pantalla se
// está mostrando a la clase»); when it ends, the badge goes back to what the
// stream shows, or hides. Neither layer can hide what the other shows.
type IndicatorMux struct {
	Indicator platform.ScreenIndicator

	mu     sync.Mutex
	layers [2]layer // 0: the stream, 1: the presentation
	shown  string   // text on screen; "" when hidden
}

type layer struct {
	on   bool
	text string
}

// Stream is the layer of internal/screen.
func (m *IndicatorMux) Stream() platform.ScreenIndicator { return muxLayer{m, 0} }

// Presentation is the layer of the Presenter.
func (m *IndicatorMux) Presentation() platform.ScreenIndicator { return muxLayer{m, 1} }

func (m *IndicatorMux) set(i int, l layer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := m.layers[i]
	m.layers[i] = l
	want := ""
	switch {
	case m.layers[1].on:
		want = m.layers[1].text
	case m.layers[0].on:
		want = m.layers[0].text
	}
	if want == m.shown {
		return nil
	}
	var err error
	if want == "" {
		err = m.Indicator.Hide()
	} else {
		err = m.Indicator.Show(want)
	}
	if err != nil {
		if l.on {
			m.layers[i] = prev // the caller must know it is not on screen
		}
		return err
	}
	m.shown = want
	return nil
}

type muxLayer struct {
	m *IndicatorMux
	i int
}

func (l muxLayer) Show(text string) error { return l.m.set(l.i, layer{on: true, text: text}) }
func (l muxLayer) Hide() error            { return l.m.set(l.i, layer{}) }
