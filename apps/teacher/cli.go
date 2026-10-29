package main

import (
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/security"
)

const usage = `Uso: aula-consola [comando]

Sin comando abre la consola. Comandos:

  Clave del centro (una por centro de cómputo; requiere administrador):
  centro crear [-force]               la crea en este equipo (primera consola del centro)
  centro importar ARCHIVO [-force]    instala aula-centro.key exportada de otra consola
  centro exportar ARCHIVO             guarda la clave privada para otras consolas (secreta)
  centro publica [-o ARCHIVO]         guarda la clave pública (aula-centro.pub) para los alumnos

  pubkey                              muestra la clave pública que usa esta consola
  keygen [-dir DIR] [-force]          crea una clave personal (equipos sin clave del centro)
  version                             muestra la versión
`

// runCLI runs a command-line subcommand and returns the process exit code.
// It never prints the private key.
func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "keygen":
		err = keygen(args[1:], stdout)
	case "pubkey":
		err = pubkey(args[1:], stdout)
	case "centro":
		err = centre(args[1:], stdout)
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, "aula-consola "+version)
		return 0
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "comando desconocido %q\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func keygen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(stdout)
	dir := fs.String("dir", "", "directorio de la clave (por defecto, el de configuración del usuario)")
	force := fs.Bool("force", false, "reemplazar la clave existente (habrá que reconfigurar todos los alumnos)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := keyDir(*dir)
	if err != nil {
		return err
	}
	id, err := security.CreateIdentity(d, *force)
	if errors.Is(err, security.ErrIdentityExists) {
		existing, lerr := security.LoadIdentity(d)
		if lerr != nil {
			return err
		}
		fmt.Fprintf(stdout, "Ya existe una clave en %s.\n", filepath.Join(d, security.IdentityFile))
		printPublic(stdout, existing)
		return fmt.Errorf("no se reemplazó: usa -force solo si quieres una clave nueva y reconfigurar todos los alumnos")
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Clave de la consola creada en %s\n", filepath.Join(d, security.IdentityFile))
	fmt.Fprintln(stdout, "Solo este usuario puede leerla. No la copies a los equipos de los alumnos.")
	fmt.Fprintln(stdout)
	printPublic(stdout, id)
	return nil
}

func pubkey(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("pubkey", flag.ContinueOnError)
	fs.SetOutput(stdout)
	dir := fs.String("dir", "", "directorio de la clave")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := keyDir(*dir)
	if err != nil {
		return err
	}
	id, source, err := consoleKey(siteKeyDir(), d, false)
	if err != nil {
		return fmt.Errorf("%w (crea la clave del centro con: aula-consola centro crear)", err)
	}
	if source == keyCentre {
		fmt.Fprintln(stdout, "Esta consola usa la clave del centro.")
	} else {
		fmt.Fprintln(stdout, "Este equipo no tiene clave del centro: la consola usa una clave personal.")
	}
	printPublic(stdout, id)
	return nil
}

func printPublic(w io.Writer, id *security.Identity) {
	fmt.Fprintln(w, "Clave pública para configurar los alumnos:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+security.EncodePublicKey(id.Public()))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Huella para verificar de palabra: "+security.KeyFingerprint(id.Public()))
}

func keyDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return security.DefaultDir()
}

// siteKeyDir is where "centro" works: platform.SiteKeyDir(), or
// AULA_SITE_KEY_DIR for tests.
func siteKeyDir() string {
	if d := os.Getenv("AULA_SITE_KEY_DIR"); d != "" {
		return d
	}
	return platform.SiteKeyDir()
}

// centre manages the site key (clave del centro), docs/protocol.md §10.1.
func centre(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("falta la acción: crear, importar, exportar o publica")
	}
	fs := flag.NewFlagSet("centro "+args[0], flag.ContinueOnError)
	fs.SetOutput(stdout)
	force := fs.Bool("force", false, "reemplazar la clave existente (habrá que reinstalar consolas y alumnos)")
	out := fs.String("o", "", "archivo de salida")
	// Flags may follow the file name: parse what comes after the first
	// positional argument too.
	rest := args[1:]
	var positional []string
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	dir := siteKeyDir()
	admin := func(err error) error {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("%w: ejecuta la terminal como administrador", err)
		}
		return err
	}

	switch args[0] {
	case "crear":
		id, err := security.CreateSiteKey(dir, *force)
		if errors.Is(err, security.ErrSiteKeyExists) {
			return fmt.Errorf("este equipo ya tiene clave del centro (%s); usa -force solo si quieres reemplazarla y reinstalar todo", security.KeyFingerprint(mustPublic(dir)))
		}
		if err != nil {
			return admin(err)
		}
		fmt.Fprintf(stdout, "Clave del centro creada en %s\n", filepath.Join(dir, security.SiteKeyFile))
		fmt.Fprintf(stdout, "Huella: %s\n", security.KeyFingerprint(id.Public()))
		fmt.Fprintln(stdout, "Siguiente paso: aula-consola centro publica -o aula-centro.pub (para instalar los alumnos).")
		return nil
	case "importar":
		if len(positional) != 1 {
			return errors.New("uso: aula-consola centro importar aula-centro.key")
		}
		data, err := os.ReadFile(positional[0])
		if err != nil {
			return err
		}
		id, err := security.ImportSiteKey(dir, data, *force)
		if errors.Is(err, security.ErrSiteKeyExists) {
			return fmt.Errorf("este equipo ya tiene clave del centro; usa -force para reemplazarla")
		}
		if err != nil {
			return admin(err)
		}
		fmt.Fprintf(stdout, "Clave del centro instalada. Huella: %s\n", security.KeyFingerprint(id.Public()))
		return nil
	case "exportar":
		if len(positional) != 1 {
			return errors.New("uso: aula-consola centro exportar aula-centro.key")
		}
		id, err := security.LoadSiteKey(dir)
		if err != nil {
			return fmt.Errorf("%w (crea la clave con: aula-consola centro crear)", err)
		}
		data, err := security.ExportSiteKeyPEM(id)
		if err != nil {
			return err
		}
		if err := os.WriteFile(positional[0], data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Clave del centro exportada a %s.\n", positional[0])
		fmt.Fprintln(stdout, "Es SECRETA: quien la tenga controla a todos los alumnos del centro. Úsala solo para instalar otras consolas y bórrala después.")
		return nil
	case "publica":
		id, err := security.LoadSiteKey(dir)
		if err != nil {
			return fmt.Errorf("%w (crea la clave con: aula-consola centro crear)", err)
		}
		line := security.EncodePublicKey(id.Public()) + "\n"
		if *out == "" {
			_, err = io.WriteString(stdout, line)
			return err
		}
		if err := os.WriteFile(*out, []byte(line), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Clave pública del centro guardada en %s (huella %s).\n", *out, security.KeyFingerprint(id.Public()))
		fmt.Fprintln(stdout, "Ponla junto a Aula-Alumno-Setup-*.exe para instalar los equipos de los alumnos.")
		return nil
	default:
		return fmt.Errorf("acción desconocida %q: crear, importar, exportar o publica", args[0])
	}
}

func mustPublic(dir string) ed25519.PublicKey {
	if id, err := security.LoadSiteKey(dir); err == nil {
		return id.Public()
	}
	return nil
}
