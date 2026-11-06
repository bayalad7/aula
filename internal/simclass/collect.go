package simclass

import (
	"context"
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Collecting the work (docs/protocol.md §5.21): a simulated student answers
// with the real collect.Responder, but its activity folder is in memory and
// holds one file, its last code (the newest snippet it received), named
// after it: «Simulado 1.py». It reads no disk (docs/privacidad.md rule 65).
// Its fault applies like to any answer: "noack" never acknowledges, "slow"
// answers late.

// handleCollect applies file.collect.*; it reports whether env was one.
func (s *student) handleCollect(ctx context.Context, send filepush.Sender, env protocol.Envelope, fault Fault) bool {
	acked := func(ctx context.Context, p protocol.Payload) error {
		if _, isAck := p.(protocol.FileCollectAck); isAck && fault == FaultNoAck {
			return nil
		}
		return send(ctx, p)
	}
	reply, handled := s.collect.Handle(ctx, collect.Sender(acked), env)
	if !handled {
		return false
	}
	if reply != nil {
		if err := acked(ctx, reply); err != nil {
			s.class.log.Debug("cannot reply", "student", s.name, "type", reply.MessageType(), "err", err)
		}
	}
	return true
}

func (s *student) collected(_ collect.Collection, ev collect.Event) {
	if ev == collect.EventCollected {
		s.class.changed()
	}
}

// langExt names the file of the last code by its language.
var langExt = map[string]string{"python": "py", "javascript": "js", "typescript": "ts", "java": "java", "c": "c",
	"cpp": "cpp", "csharp": "cs", "go": "go", "html": "html", "css": "css", "sql": "sql", "php": "php",
	"ruby": "rb", "rust": "rs", "kotlin": "kt", "bash": "sh", "plaintext": "txt"}

// lastCodeFolder is the activity folder of a simulated student: whatever
// folder is asked for holds its last code.
type lastCodeFolder struct {
	name  string
	board *codedist.Board

	mu   sync.Mutex
	dirs map[string]bool
}

func (f *lastCodeFolder) file() (string, []byte, time.Time, bool) {
	list := f.board.Snippets()
	if len(list) == 0 {
		return "", nil, time.Time{}, false
	}
	last := list[len(list)-1]
	ext := langExt[strings.ToLower(last.Lang)]
	if ext == "" {
		ext = "txt"
	}
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 0x20 {
			return '_'
		}
		return r
	}, f.name) + "." + ext
	if filepush.CheckName(name) != nil {
		name = "codigo." + ext
	}
	return name, []byte(last.Content), last.ReceivedAt, true
}

func (f *lastCodeFolder) MkdirAll(rel string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dirs == nil {
		f.dirs = map[string]bool{}
	}
	f.dirs[rel] = true
	return nil
}

func (f *lastCodeFolder) List(rel string, _ int) ([]platform.WorkFile, error) {
	f.mu.Lock()
	ok := f.dirs[rel]
	f.mu.Unlock()
	if !ok {
		return nil, fs.ErrNotExist
	}
	name, data, at, has := f.file()
	if !has {
		return nil, nil
	}
	return []platform.WorkFile{{Path: name, Size: int64(len(data)), ModTime: at}}, nil
}

func (f *lastCodeFolder) Read(rel string, max int64) ([]byte, error) {
	name, data, _, has := f.file()
	if !has || path.Base(rel) != name {
		return nil, fs.ErrNotExist
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return data, nil
}
