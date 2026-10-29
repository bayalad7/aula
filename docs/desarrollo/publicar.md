# Publicar una versión

Las versiones se publican solas al empujar un tag `vX.Y.Z`
(`.github/workflows/release.yml`). Esta guía es para quien mantiene el repositorio.

Para probar `develop` en VMs **sin publicar**, genera los instaladores en tu equipo con
`instaladores-locales.md`.

## 1. Cuándo y con qué número

- Versionado semántico (CLAUDE.md): `z` corrige, `y` agrega funciones compatibles, `x` rompe el
  protocolo o el formato de configuración.
- **Estables** (`v1.2.0`): se etiquetan en `main`, después de llevar `develop` a `main`.
- **De prueba** (`v1.2.0-rc.1`, cualquier tag con guion): salen marcadas como *prerelease*;
  sirven para validar el empaquetado desde `develop` sin tocar `main`.

```sh
git switch develop && git pull
git tag -a v0.1.0-rc.1 -m "Aula 0.1.0-rc.1"
git push origin v0.1.0-rc.1
```

## 2. Qué hace el workflow

1. Deriva la versión del tag (`${GITHUB_REF_NAME#v}`) y la valida (`X.Y.Z` o `X.Y.Z-pre`).
2. Corre `go test ./...`.
3. Compila las dos apps con `wails build -trimpath -ldflags "-X main.version=<versión>"`, sin
   cgo, y pone la versión numérica en los metadatos del `.exe`.
4. **Comprueba** que `aula-consola.exe version` y `aula-cliente.exe -version` reportan esa
   versión.
5. Si existe el certificado, firma los binarios con Authenticode.
6. Genera los instaladores Inno Setup (`empaquetado/windows/*.iss`) y, si hay certificado, los
   firma.
7. Escribe `checksums.txt` (SHA-256 de todos los `.exe`) y lo firma con la llave Ed25519 de
   releases (`checksums.txt.sig`). **Verifica la firma** con la llave pública versionada antes de
   seguir.
8. Publica con `gh release create "$TAG" --generate-notes` (prerelease si el tag lleva guion).

Artefactos de cada versión:

| Archivo | Qué es |
|---|---|
| `Aula-Alumno-Setup-X.Y.Z.exe` | Instalador del agente (admite `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART`, `/CANAL=`, `/CLAVECENTRO=` y `/DIRECTO=`). |
| `Aula-Consola-Setup-X.Y.Z.exe` | Instalador de la consola (pide canal predeterminado y crea o importa la clave del centro; silencioso: `/CANAL=`, `/CENTRO=` (`crear`, `mantener` o la ruta de `aula-centro.key`), `/SALIDA=`). |
| `aula-cliente-X.Y.Z-windows-amd64.exe`, `aula-consola-X.Y.Z-windows-amd64.exe` | Portables. |
| `checksums.txt`, `checksums.txt.sig` | SHA-256 y su firma Ed25519. |

Los paquetes de Linux (`.deb`/`.rpm`) y de macOS (`.pkg`) se agregan en la etapa 13 como jobs
nuevos del mismo workflow.

## 3. Secretos del repositorio

*Settings → Secrets and variables → Actions*. Solo los usa el workflow de release.

| Secreto | Obligatorio | Contenido |
|---|---|---|
| `RELEASE_SIGNING_KEY` | **Sí** | Llave privada Ed25519 de releases, PEM PKCS#8 (`-----BEGIN PRIVATE KEY-----`). Sin ella el workflow falla al firmar. |
| `WINDOWS_CERT_PFX` | No | Certificado de firma de código (Authenticode) en `.pfx`, codificado en Base64 (`base64 -w0 cert.pfx`). Si no existe, se publica sin firma de código. |
| `WINDOWS_CERT_PASSWORD` | Solo con el anterior | Contraseña del `.pfx`. |

Sin Authenticode, Windows SmartScreen avisa "Windows protegió tu PC" al abrir los instaladores
(*Más información → Ejecutar de todas formas*). La integridad se comprueba igual con la firma de
`checksums.txt`.

## 4. La llave de releases

- **Pública:** `empaquetado/llave-releases.pub`, versionada. El paquete `empaquetado` la
  **embebe** en los binarios para que la auto-actualización (etapa 15) verifique las versiones.
- **Privada:** solo existe como el secreto `RELEASE_SIGNING_KEY`. No hay copia en disco.

Se creó así (la privada va por tubería al secreto, sin tocar el disco):

```sh
go run ./tools/relsign keygen -pub empaquetado/llave-releases.pub | gh secret set RELEASE_SIGNING_KEY
git add empaquetado/llave-releases.pub && git commit -m "…"
```

**Rotarla** (si la privada se filtra): repite el comando, versiona la pública nueva y publica una
versión. Los binarios anteriores confían en la llave vieja hasta actualizarse, así que publica
cuanto antes una versión firmada con la nueva.

Formato de la firma: Ed25519 sobre `"aula-checksums-v1\n" + <bytes exactos de checksums.txt>`,
en Base64 dentro de `checksums.txt.sig`.

## 5. Verificar una versión descargada

Con Go, desde cualquier carpeta que tenga los dos archivos:

```sh
go run github.com/bayalad7/aula/tools/relsign@v0.1.0-rc.1 verify -in checksums.txt -sig checksums.txt.sig
```

Sin `-pub` usa la llave embebida en esa versión de la herramienta. Luego compara los SHA-256:

```powershell
Get-FileHash .\Aula-Alumno-Setup-0.1.0-rc.1.exe -Algorithm SHA256   # igual a su línea en checksums.txt
```

## 6. Si algo falla

| Falla | Causa probable |
|---|---|
| "El tag … no es vX.Y.Z" | Tag mal escrito (`v1.2`, `1.2.0`). Bórralo (`git push --delete origin <tag>`) y crea otro. |
| "Check that the binaries report the version" | El `-ldflags` no llegó a `main.version`: revisa que la variable siga existiendo en `apps/*/`. |
| "$RELEASE_SIGNING_KEY is empty" | Falta el secreto o el workflow corre desde un fork. |
| La verificación de la firma falla | La pública versionada no corresponde al secreto: alguien rotó solo una de las dos. |
| `gh release create` dice que ya existe | Ya hay una versión con ese tag: bórrala en GitHub o usa otro número. |
