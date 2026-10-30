// Command vistaprevia serves the console's frontend folder on the loopback
// interface, to open the design system catalog and the prototype in a browser
// (they load the icon sprite and fetch files, which file:// URLs block).
// Development only: nothing here is embedded in the console.
//
//	go run ./tools/vistaprevia            # then open the printed URLs
//	go run ./tools/vistaprevia -puerto 8090
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	dir := flag.String("dir", "apps/teacher/frontend", "carpeta a servir (la raíz del frontend de la consola)")
	port := flag.Int("puerto", 0, "puerto en 127.0.0.1 (0: uno libre)")
	flag.Parse()

	if _, err := os.Stat(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "vistaprevia: %v (ejecútalo desde la raíz del repositorio o usa -dir)\n", err)
		os.Exit(2)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "vistaprevia: %v\n", err)
		os.Exit(1)
	}
	base := "http://" + ln.Addr().String() + "/"
	fmt.Println("Catálogo:   " + base + "dev/catalogo.html")
	fmt.Println("Prototipo:  " + base + "dev/prototipo/alumnos.html")
	fmt.Println("Ctrl+C para terminar.")

	srv := &http.Server{
		Handler:           noCache(http.FileServer(http.Dir(*dir))),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("vistaprevia", "err", err)
		os.Exit(1)
	}
}

// noCache makes every reload show the files as they are on disk.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
