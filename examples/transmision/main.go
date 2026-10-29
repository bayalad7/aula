// Command transmision is a teacher console without a window that broadcasts
// a screen to the agents of a channel as soon as it starts, and prints every
// 5 s the receivers, the video bitrate, the estimated upload and the CPU it
// uses (docs/prueba-vms.md §12). By default it sends a synthetic pattern of
// code text, so nothing private leaves the machine; -monitor N sends a real
// monitor.
//
//	go run ./examples/transmision                      # pattern, channel 1, port 47830
//	go run ./examples/transmision -monitor 0 -kbps 1500
//
// Agents connect as usual (installed with the same key, or aula-cliente
// -canal 1 -clave aula-ed25519:… -directo IP:47830).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

func main() {
	canal := flag.String("canal", "1", "canal del aula")
	puerto := flag.Int("puerto", 47830, "puerto WebSocket de la consola; la transmisión usa el UDP siguiente")
	nombre := flag.String("nombre", "Transmisión de prueba", "nombre visible de la consola")
	dir := flag.String("dir", "", "directorio de la clave y el certificado (por defecto, el de aula-consola keygen)")
	monitor := flag.Int("monitor", -1, "monitor a transmitir (0, 1…); -1 envía un patrón sintético de código")
	kbps := flag.Int("kbps", broadcast.DefaultMaxKbps, "tope de la tasa de video en kbit/s")
	verbose := flag.Bool("v", false, "logs detallados")
	flag.Parse()

	level := slog.LevelWarn
	if *verbose {
		level = slog.LevelInfo
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, log, *canal, *puerto, *nombre, *dir, *monitor, *kbps); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, channel string, port int, name, dir string, monitor, kbps int) error {
	if dir == "" {
		var err error
		if dir, err = security.DefaultDir(); err != nil {
			return err
		}
	}
	id, err := security.LoadIdentity(dir)
	if errors.Is(err, fs.ErrNotExist) {
		id, err = security.CreateIdentity(dir, false)
	}
	if err != nil {
		return err
	}
	cert, err := security.LoadOrCreateCertificate(dir)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("escuchar en el puerto %d: %w", port, err)
	}
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: log})

	var capture platform.DisplayCapture = &pattern{}
	if monitor >= 0 {
		if capture, err = platform.NewDisplayCapture(); err != nil {
			return err
		}
	} else {
		monitor = 0
	}
	stats := &encodeStats{}
	var console *session.Console
	b := &broadcast.Broadcaster{
		Capture: capture,
		NewEncoder: func(cfg platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
			enc, err := platform.NewH264Encoder(cfg)
			if err != nil {
				return nil, err
			}
			return &countingEncoder{VideoEncoder: enc, stats: stats}, nil
		},
		Send: func(ctx context.Context, sid string, p protocol.Payload) (string, error) {
			return console.Send(ctx, sid, p)
		},
		Media:   broadcast.MediaOptions{UDPPort: port + 1},
		MaxKbps: kbps,
		Logger:  log,
	}
	console = &session.Console{Channel: channel, Registry: reg, Identity: id, CertSHA256: cert.SHA256, Logger: log,
		OnChange: func() { b.Sync(reg.List(channel)) },
		OnMessage: func(_ context.Context, conn *transport.Conn, env protocol.Envelope) {
			b.Handle(conn.ID(), env)
		},
	}
	announcer := &discovery.Announcer{Logger: log, Announcement: discovery.Announcement{
		Channel: channel, ConsoleName: name, WSPort: port, CertFingerprint: cert.Fingerprint(),
	}}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() { defer wg.Done(); _ = console.Run(ctx, srv.Events()) }()
	go func() { defer wg.Done(); _ = announcer.Run(ctx) }()

	if err := b.Start(monitor, reg.List(channel)); err != nil {
		return err
	}
	fmt.Printf("Consola %q en el canal %s: TCP %d, UDP %d. Ctrl+C para salir.\n", name, channel, port, port+1)
	fmt.Printf("Clave pública para los alumnos: %s\n", security.EncodePublicKey(id.Public()))
	report(ctx, b, stats)
	b.Stop()
	wg.Wait()
	return nil
}

// report prints the counts every 5 s until ctx is canceled.
func report(ctx context.Context, b *broadcast.Broadcaster, stats *encodeStats) {
	const every = 5 * time.Second
	t := time.NewTicker(every)
	defer t.Stop()
	lastCPU, lastAt := processCPU(), time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s := b.Status()
		bytes, frames, enc := stats.take()
		now, cpu := time.Now(), processCPU()
		secs := now.Sub(lastAt).Seconds()
		videoKbps := float64(bytes) * 8 / 1000 / secs
		cpuPct := (cpu - lastCPU).Seconds() / secs * 100 // of one core
		lastCPU, lastAt = cpu, now
		perFrame := time.Duration(0)
		if frames > 0 {
			perFrame = enc / time.Duration(frames)
		}
		fmt.Printf("%s  receptores %d/%d · video %.0f kbit/s · subida ≈ %.1f Mbit/s · %.1f cuadros/s · codificar %v/cuadro · CPU %.0f %% de un núcleo\n",
			now.Format("15:04:05"), s.Receivers, s.Eligible, videoKbps, videoKbps*float64(s.Receivers)/1000,
			float64(frames)/secs, perFrame.Round(100*time.Microsecond), cpuPct)
	}
}

type encodeStats struct {
	bytes, frames atomic.Int64
	encode        atomic.Int64 // nanoseconds
}

func (s *encodeStats) take() (bytes, frames int64, encode time.Duration) {
	return s.bytes.Swap(0), s.frames.Swap(0), time.Duration(s.encode.Swap(0))
}

type countingEncoder struct {
	platform.VideoEncoder
	stats *encodeStats
}

func (e *countingEncoder) Encode(img *image.RGBA, forceKey bool) ([]byte, error) {
	start := time.Now()
	au, err := e.VideoEncoder.Encode(img, forceKey)
	e.stats.encode.Add(int64(time.Since(start)))
	e.stats.frames.Add(1)
	e.stats.bytes.Add(int64(len(au)))
	return au, err
}

// pattern is a 1920×1080 "monitor" showing a code listing whose cursor
// blinks and whose lines scroll every 4 s, like a teacher typing. Each
// listing is drawn once; a frame is a copy plus the cursor, so the CPU the
// report shows is the broadcast's, not the pattern's.
type pattern struct {
	mu    sync.Mutex
	pages map[int]*image.RGBA
	frame *image.RGBA
}

func (*pattern) Displays() ([]platform.Display, error) {
	return []platform.Display{{Index: 0, Bounds: image.Rect(0, 0, 1920, 1080), Primary: true}}, nil
}

var code = []string{
	"def factorial(n):",
	"    \"\"\"Devuelve n! de forma recursiva.\"\"\"",
	"    if n <= 1:",
	"        return 1",
	"    return n * factorial(n - 1)",
	"",
	"for i in range(10):",
	"    print(i, factorial(i))  # 0 1 2 6 24 120 ...",
	"",
	"class Pila:",
	"    def __init__(self):",
	"        self.datos = []",
	"    def push(self, x):",
	"        self.datos.append(x)",
	"    def pop(self):",
	"        return self.datos.pop()",
}

func (p *pattern) Grab(context.Context, int) (*image.RGBA, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	offset := int(now.Unix()/4) % len(code)
	page := p.pages[offset]
	if page == nil {
		page = drawPage(offset)
		if p.pages == nil {
			p.pages = map[int]*image.RGBA{}
		}
		p.pages[offset] = page
	}
	if p.frame == nil {
		p.frame = image.NewRGBA(page.Bounds())
	}
	copy(p.frame.Pix, page.Pix)
	// A blinking cursor that moves along the first line.
	if now.UnixMilli()/500%2 == 0 {
		x := 2 * (16 + 7*(5+int(now.UnixMilli()/200)%30))
		draw.Draw(p.frame, image.Rect(x, 18, x+4, 44), image.NewUniform(color.RGBA{255, 200, 0, 255}), image.Point{}, draw.Src)
	}
	return p.frame, nil
}

// drawPage draws the listing scrolled by offset at half size with the 7×13
// bitmap font and doubles it: 14×26 px glyphs, like a 20 pt editor font on a
// 1080p monitor.
func drawPage(offset int) *image.RGBA {
	small := image.NewRGBA(image.Rect(0, 0, 960, 540))
	draw.Draw(small, small.Bounds(), image.NewUniform(color.RGBA{30, 30, 30, 255}), image.Point{}, draw.Src)
	d := font.Drawer{Dst: small, Src: image.NewUniform(color.RGBA{220, 220, 220, 255}), Face: basicfont.Face7x13}
	for i := range 38 {
		d.Dot = fixed.P(16, 20+i*14)
		d.DrawString(fmt.Sprintf("%3d  %s", i+1, code[(i+offset)%len(code)]))
	}
	big := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := range 1080 {
		for x := range 1920 {
			o := small.PixOffset(x/2, y/2)
			copy(big.Pix[big.PixOffset(x, y):], small.Pix[o:o+4])
		}
	}
	return big
}
