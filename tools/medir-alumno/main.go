// Command medir-alumno mide la app del alumno en un equipo real (prompt 19,
// Fase 4): abre una consola de prueba en 127.0.0.1 (clave y certificado
// temporales, sin anuncio mDNS), lanza el agente con -directo hacia ella y
// mide CPU y RAM del agente y de sus procesos de WebView2, en reposo y
// recibiendo código. También observa, sin tocar el teclado ni el mouse, si la
// ventana de Aula pasa a primer plano cuando llega un código (le quitaría el
// foco al alumno).
//
// Con -ventana, en lugar de medir comprueba la ventana (prompt 19, entrega D)
// actuando solo sobre la ventana del agente, nunca con teclado ni mouse: la
// minimiza y le manda código (debe seguir minimizada y sin tomar el primer
// plano), le manda WM_CLOSE como la X (el agente debe seguir vivo con la
// ventana oculta) y le manda otro código (no debe reaparecer).
//
// Uso (Windows):
//
//	go run ./tools/medir-alumno -exe apps/student/build/bin/aula-cliente.exe
//	go run ./tools/medir-alumno            (compila el agente en una carpeta temporal)
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

const channel = "7"

var soloVentana bool

func main() {
	exe := flag.String("exe", "", "aula-cliente.exe a medir (vacío: se compila ./apps/student)")
	reposo := flag.Duration("reposo", 60*time.Second, "tiempo de medición en reposo")
	recibiendo := flag.Duration("recibiendo", 60*time.Second, "tiempo de medición recibiendo código")
	cada := flag.Duration("cada", 2*time.Second, "cada cuánto llega un código mientras recibe")
	ventana := flag.Bool("ventana", false, "comprobar la ventana en vez de medir")
	flag.Parse()
	soloVentana = *ventana
	if err := run(*exe, *reposo, *recibiendo, *cada); err != nil {
		fmt.Fprintln(os.Stderr, "medir-alumno:", err)
		os.Exit(1)
	}
}

func run(exe string, reposo, recibiendo, cada time.Duration) error {
	tmp, err := os.MkdirTemp("", "aula-medir-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if exe == "" {
		exe = filepath.Join(tmp, "aula-cliente.exe")
		fmt.Println("Compilando el agente…")
		cmd := exec.Command("go", "build", "-tags", "desktop,production", "-trimpath",
			"-ldflags", "-w -s -H windowsgui", "-o", exe, "./apps/student")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compilar: %w", err)
		}
	}

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	id, err := security.GenerateIdentity()
	if err != nil {
		return err
	}
	cert, err := security.LoadOrCreateCertificate(tmp)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	con := &session.Console{Channel: channel, Registry: reg, Identity: id, CertSHA256: cert.SHA256, Logger: quiet}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() { _ = srv.Serve(ctx, ln) })
	wg.Go(func() { _ = con.Run(ctx, srv.Events()) })
	defer wg.Wait()
	defer cancel()

	addr := ln.Addr().String()
	agent := exec.Command(exe, "-canal", channel, "-clave", security.EncodePublicKey(id.Public()), "-directo", addr)
	if err := agent.Start(); err != nil {
		return fmt.Errorf("lanzar el agente: %w", err)
	}
	pid := agent.Process.Pid
	defer func() { _ = agent.Process.Kill(); _, _ = agent.Process.Wait() }()
	fmt.Printf("Agente %d contra la consola de prueba %s (canal %s)\n", pid, addr, channel)

	deadline := time.Now().Add(60 * time.Second)
	for len(reg.List(channel)) == 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("el agente no se conectó en 60 s")
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println("Conectado. Se asienta 10 s…")
	time.Sleep(10 * time.Second)
	if soloVentana {
		return checkWindow(ctx, con, pid)
	}

	idle, err := measure(pid, reposo, nil)
	if err != nil {
		return err
	}

	// Foco: quién está en primer plano antes y después del primer código.
	before := foreground()
	n := 0
	send := func() {
		n++
		p := protocol.CodeShare{SnippetID: fmt.Sprintf("m-%d", n), Exercise: fmt.Sprint(n), Title: "Medición",
			Lang: "python", Content: sample, Mode: protocol.ModeAppend}
		if n%5 == 0 {
			p.Mode = protocol.ModeReplace
		}
		_, _ = con.Broadcast(ctx, p)
	}
	coversBefore := aboveForeground(pid)
	send()
	time.Sleep(500 * time.Millisecond)
	coversSoon := aboveForeground(pid)
	time.Sleep(1500 * time.Millisecond)
	after := foreground()
	coversAfter := aboveForeground(pid)

	busy, err := measure(pid, recibiendo, func(elapsed time.Duration) bool {
		if elapsed >= cada*time.Duration(n) {
			send()
		}
		return true
	})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("| Estado | CPU media (% de un núcleo) | RAM media (MB) | RAM máxima (MB) | Procesos |")
	fmt.Println("|---|---|---|---|---|")
	fmt.Printf("| Reposo (%s) | %.1f | %.0f | %.0f | %d |\n", reposo, idle.cpu, idle.ramAvg, idle.ramMax, idle.procs)
	fmt.Printf("| Recibiendo (un código cada %s, %d códigos) | %.1f | %.0f | %.0f | %d |\n", cada, n, busy.cpu, busy.ramAvg, busy.ramMax, busy.procs)
	fmt.Println()
	fmt.Println("RAM: memoria privada (commit) del agente y de sus procesos de WebView2.")
	fmt.Printf("Primer plano antes del primer código: %s\n", before)
	fmt.Printf("Primer plano 2 s después:            %s\n", after)
	fmt.Printf("Ventana de Aula encima de la del primer plano: antes %v · a los 0,5 s %v · a los 2 s %v\n",
		coversBefore, coversSoon, coversAfter)
	if after.pid != before.pid && after.inTree(pid) {
		fmt.Println("⇒ La ventana de Aula tomó el primer plano: le quita el foco al alumno.")
	} else {
		fmt.Println("⇒ El primer plano no cambió a Aula.")
	}
	return nil
}

// sample is a 25-line snippet, about what a class sends.
var sample = strings.Repeat("def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\n", 5)

// checkWindow runs the window checks of -ventana.
func checkWindow(ctx context.Context, con *session.Console, pid int) error {
	n := 0
	send := func() {
		n++
		_, _ = con.Broadcast(ctx, protocol.CodeShare{SnippetID: fmt.Sprintf("v-%d", n), Title: "Ventana",
			Lang: "python", Content: "print(1)\n", Mode: protocol.ModeAppend})
		time.Sleep(2 * time.Second)
	}
	ok := true
	check := func(cond bool, what string) {
		mark := "✓"
		if !cond {
			mark, ok = "✗", false
		}
		fmt.Println(mark, what)
	}

	w, err := agentWindow(pid)
	if err != nil {
		return err
	}
	minimize(w)
	time.Sleep(time.Second)
	fg := foreground()
	send()
	check(iconic(w), "minimizada, llega código: sigue minimizada")
	check(foreground().pid == fg.pid, "minimizada, llega código: el primer plano no cambia ("+foreground().String()+")")

	closeWindow(w)
	time.Sleep(time.Second)
	check(alive(pid), "cerrar (WM_CLOSE): el agente sigue en ejecución")
	check(!visible(w), "cerrar: la ventana queda oculta")
	send()
	check(!visible(w), "oculta, llega código: no reaparece")
	check(len(windowsAlive(pid)) > 0, "oculta: la ventana sigue existiendo")
	if !ok {
		return fmt.Errorf("la ventana no se comporta como se espera")
	}
	fmt.Println("La ventana se comporta como se espera (falta a ojo: el icono del área de notificación y el modo compacto).")
	return nil
}
