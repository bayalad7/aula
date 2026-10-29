package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SiteKeyFile is the file name of the site key (clave del centro) in its
// directory, platform.SiteKeyDir() on teacher machines.
const SiteKeyFile = "clave-centro.pem"

// ErrSiteKeyExists is returned when creating or importing over an existing
// site key without force: replacing it means reinstalling every agent.
var ErrSiteKeyExists = errors.New("site key already exists")

// The site key is one Ed25519 key per computer centre (docs/protocol.md
// §10.1). Every console of the centre signs with it, every agent of the centre
// trusts its public half. On teacher machines it is readable by local users
// (any teacher may give class) and writable only by administrators.

// CreateSiteKey generates the site key and saves it in dir.
func CreateSiteKey(dir string, force bool) (*Identity, error) {
	id, err := GenerateIdentity()
	if err != nil {
		return nil, err
	}
	if err := saveSiteKey(dir, id, force); err != nil {
		return nil, err
	}
	return id, nil
}

// ImportSiteKey saves a site key exported from another console (PKCS#8 PEM,
// the content of aula-centro.key).
func ImportSiteKey(dir string, pemData []byte, force bool) (*Identity, error) {
	id, err := parseKeyPEM(pemData)
	if err != nil {
		return nil, fmt.Errorf("import site key: %w", err)
	}
	if err := saveSiteKey(dir, id, force); err != nil {
		return nil, err
	}
	return id, nil
}

// LoadSiteKey reads the site key saved in dir.
func LoadSiteKey(dir string) (*Identity, error) {
	path := filepath.Join(dir, SiteKeyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load site key: %w", err)
	}
	id, err := parseKeyPEM(data)
	if err != nil {
		return nil, fmt.Errorf("load site key %s: %w", path, err)
	}
	return id, nil
}

// ExportSiteKeyPEM returns the private key in PKCS#8 PEM, to install it on
// another teacher machine (aula-centro.key). The result is a secret: callers
// write it only where the administrator asked, never to logs.
func ExportSiteKeyPEM(id *Identity) ([]byte, error) {
	return keyPEM(id)
}

func saveSiteKey(dir string, id *Identity, force bool) error {
	path := filepath.Join(dir, SiteKeyFile)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("save site key %s: %w", path, ErrSiteKeyExists)
		}
	}
	data, err := keyPEM(id)
	if err != nil {
		return fmt.Errorf("save site key: %w", err)
	}
	if err := writeMachineSecret(path, data); err != nil {
		return fmt.Errorf("save site key: %w", err)
	}
	return nil
}
