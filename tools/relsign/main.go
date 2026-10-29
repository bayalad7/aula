// Command relsign manages the release signature: it creates the Ed25519 key
// pair, writes checksums.txt, signs it and verifies it. See
// docs/desarrollo/publicar.md.
//
//	relsign keygen -pub empaquetado/llave-releases.pub | gh secret set RELEASE_SIGNING_KEY
//	relsign sums -o dist/checksums.txt dist/*.exe
//	relsign sign -in dist/checksums.txt -out dist/checksums.txt.sig   (key in $RELEASE_SIGNING_KEY)
//	relsign verify -in dist/checksums.txt -sig dist/checksums.txt.sig [-pub file]
//
// The private key is only ever printed to stdout (keygen) or read from the
// environment (sign); it is never written to disk by this tool.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bayalad7/aula/empaquetado"
	"github.com/bayalad7/aula/internal/security"
)

const keyEnv = "RELEASE_SIGNING_KEY"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "relsign:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return errors.New("usage: relsign keygen|sums|sign|verify [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "keygen":
		pubPath := fs.String("pub", "", "file for the public key (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *pubPath == "" {
			return errors.New("keygen: -pub is required")
		}
		return keygen(*pubPath, stdout)
	case "sums":
		out := fs.String("o", "", "output file (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *out == "" || fs.NArg() == 0 {
			return errors.New("sums: -o and at least one file are required")
		}
		data, err := empaquetado.Checksums(fs.Args())
		if err != nil {
			return err
		}
		return os.WriteFile(*out, data, 0o644)
	case "sign":
		in := fs.String("in", "", "checksums.txt")
		out := fs.String("out", "", "signature file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		priv, err := parsePrivate(getenv(keyEnv))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(*in)
		if err != nil {
			return err
		}
		return os.WriteFile(*out, empaquetado.Sign(priv, data), 0o644)
	case "verify":
		in := fs.String("in", "", "checksums.txt")
		sigPath := fs.String("sig", "", "signature file")
		pubPath := fs.String("pub", "", "public key file (default: the key embedded in this build)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		pub, err := publicKey(*pubPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(*in)
		if err != nil {
			return err
		}
		sig, err := os.ReadFile(*sigPath)
		if err != nil {
			return err
		}
		if err := empaquetado.Verify(pub, data, sig); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "OK: %s firmado con la llave de releases %s\n", filepath.Base(*in), security.KeyFingerprint(pub))
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// keygen writes the public key file and prints the private key (PKCS#8 PEM)
// to stdout, meant to be piped straight into the repository secret.
func keygen(pubPath string, stdout io.Writer) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte(security.EncodePublicKey(pub)+"\n"), 0o644); err != nil {
		return err
	}
	_, err = stdout.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	return err
}

func parsePrivate(s string) (ed25519.PrivateKey, error) {
	if s == "" {
		return nil, fmt.Errorf("sign: $%s is empty (the release key secret is missing)", keyEnv)
	}
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, fmt.Errorf("sign: $%s is not a PEM private key", keyEnv)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("sign: key is %T, want Ed25519", key)
	}
	return priv, nil
}

func publicKey(path string) (ed25519.PublicKey, error) {
	if path == "" {
		return empaquetado.ReleasePublicKey()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return security.ParsePublicKey(string(data))
}
