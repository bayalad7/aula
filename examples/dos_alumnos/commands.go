package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

const commandHelp = `Comandos:
  codigo <archivo> [título]    muestra el archivo a todos (reemplaza lo que tengan)
  agregar <archivo> [título]   lo agrega debajo de lo que ya tienen
  limpiar                      borra el código de las pantallas
  aviso <texto>                aviso informativo
  alerta <texto>               aviso de advertencia
`

// commands reads teacher commands from in until it ends or ctx is canceled.
// It stands in for the console UI (stage 07) to test code distribution.
func commands(ctx context.Context, console *session.Console, in io.Reader) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		verb, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		var p protocol.Payload
		var err error
		switch verb {
		case "codigo", "agregar":
			mode := protocol.ModeReplace
			if verb == "agregar" {
				mode = protocol.ModeAppend
			}
			p, err = shareFile(rest, mode)
		case "limpiar":
			p = protocol.CodeClear{}
		case "aviso", "alerta":
			level := protocol.LevelInfo
			if verb == "alerta" {
				level = protocol.LevelWarning
			}
			p = protocol.MsgShow{Text: rest, Level: level}
		case "ayuda":
			fmt.Print(commandHelp)
			continue
		default:
			err = fmt.Errorf("comando desconocido %q; escribe ayuda", verb)
		}
		if err == nil {
			err = p.Validate()
		}
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		sent, failed := console.Broadcast(ctx, p)
		fmt.Printf("[%s] %s enviado a %d equipo(s)", now(), p.MessageType(), len(sent))
		var noCap int
		for _, e := range failed {
			if errors.Is(e, session.ErrUnsupportedCapability) {
				noCap++
			}
		}
		if noCap > 0 {
			fmt.Printf("; %d no lo soportan", noCap)
		}
		if n := len(failed) - noCap; n > 0 {
			fmt.Printf("; %d fallaron", n)
		}
		fmt.Println()
	}
}

// shareFile builds a code.share from "<file> [title]".
func shareFile(args string, mode protocol.CodeMode) (protocol.CodeShare, error) {
	path, title, _ := strings.Cut(args, " ")
	if path == "" {
		return protocol.CodeShare{}, errors.New("falta el archivo")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return protocol.CodeShare{}, err
	}
	if title = strings.TrimSpace(title); title == "" {
		title = filepath.Base(path)
	}
	return protocol.CodeShare{
		SnippetID: protocol.NewID(),
		Title:     title,
		Lang:      langFor(path),
		Content:   strings.ReplaceAll(string(data), "\r\n", "\n"),
		Mode:      mode,
	}, nil
}

func langFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".js", ".mjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sh":
		return "shell"
	case ".json":
		return "json"
	case ".rs":
		return "rust"
	case ".kt":
		return "kotlin"
	default:
		return "plaintext"
	}
}

// ackLabel shows the last snippet the agent confirmed on screen.
func ackLabel(s session.Session) string {
	switch {
	case s.LastCodeAckAt.IsZero():
		return ""
	case s.LastCodeAck == "":
		return "pantalla limpia ✓"
	default:
		return "código " + s.LastCodeAck[:8] + " ✓"
	}
}

// studentMessages makes the console student behave like the real app for the
// protocol: it applies code and notices with codedist.Receiver, answers the
// acks, and prints a summary instead of drawing a window.
func studentMessages(host string) func(context.Context, *transport.Client, protocol.Envelope) {
	recv := &codedist.Receiver{
		Board: codedist.NewBoard(0),
		OnBoard: func(snippets []codedist.Snippet) {
			if len(snippets) == 0 {
				fmt.Printf("[%s] %s: pantalla limpia\n", now(), host)
				return
			}
			s := snippets[len(snippets)-1]
			fmt.Printf("[%s] %s: código recibido: ejercicio %s %q (%s, %d líneas); en pantalla: %d\n", now(), host,
				s.Exercise, s.Title, s.Lang, strings.Count(strings.TrimRight(s.Content, "\n"), "\n")+1, len(snippets))
		},
		OnNotice: func(n codedist.Notice) { fmt.Printf("[%s] %s: aviso (%s): %s\n", now(), host, n.Level, n.Text) },
	}
	return func(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
		if reply := recv.Handle(env); reply != nil {
			_, _ = cl.Send(ctx, reply)
		}
	}
}
