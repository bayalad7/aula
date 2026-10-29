package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/bayalad7/aula/internal/security"
)

const usage = `Uso: aula-consola [comando]

Sin comando abre la consola. Comandos:
  keygen [-dir DIR] [-force]   crea la clave de la consola y muestra la clave pública
  pubkey [-dir DIR]            muestra la clave pública para configurar alumnos
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
	id, err := security.LoadIdentity(d)
	if err != nil {
		return fmt.Errorf("%w (¿ejecutaste aula-consola keygen?)", err)
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
