# Compilar y empaquetar para Linux

Cómo se generan los paquetes de Linux de Aula (prompt 13, decisión D-023). El release los hace
solo al empujar una etiqueta `v*` (`.github/workflows/release.yml`, jobs `linux` y
`linux-instalar`); esta guía es para generarlos a mano o para cambiar cómo se hacen.

## Qué sale

| Archivo | Qué es |
|---|---|
| `aula-cliente_X.Y.Z_amd64.deb`, `aula-cliente-X.Y.Z-1.x86_64.rpm` | Agente del alumno: `/usr/bin/aula-cliente`, el servicio `aula-agente.service` (lo instala y arranca el paquete si hay systemd), el arranque con cada sesión (`/etc/xdg/autostart`) y su lanzador. |
| `aula-consola_X.Y.Z_amd64.deb`, `aula-consola-X.Y.Z-1.x86_64.rpm` | Consola: `/usr/bin/aula-consola`, su lanzador, el grupo `aula` y `/etc/Aula/centro`, y el firewall (`ufw` o `firewalld`, solo si está activo: TCP 47830 y mDNS). |
| `aula-consola-X.Y.Z-x86_64.AppImage` | La consola sin instalar (usa el GTK y el WebKitGTK del sistema). El agente no tiene AppImage: debe arrancar con cada sesión. |
| `aula-cliente-X.Y.Z-linux-amd64`, `aula-consola-X.Y.Z-linux-amd64` | Binarios sueltos (los del release). |

Una versión con guion (`0.2.0-rc.1`) sale como `0.2.0~rc.1` en `.deb` y `.rpm`, para que se
ordene antes de `0.2.0`.

## Requisitos (Ubuntu 24.04)

```sh
sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev file curl
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1   # la misma versión de go.mod
```

Wails en Linux **necesita cgo** (GTK y WebKitGTK); es la única plataforma donde los binarios
publicados llevan cgo. Ubuntu 24.04 trae WebKitGTK **4.1**: compila siempre con
`-tags webkit2_41`.

## Compilar

```sh
for app in student teacher; do
  (cd apps/$app && CGO_ENABLED=1 wails build -clean -trimpath -tags webkit2_41 \
     -ldflags "-X main.version=X.Y.Z")
done
mkdir -p dist/bin
cp apps/student/build/bin/aula-cliente apps/teacher/build/bin/aula-consola dist/bin/
```

## Empaquetar

```sh
bash empaquetado/linux/empaquetar.sh X.Y.Z dist/bin dist/release
```

`empaquetar.sh` instala nfpm y descarga appimagetool y el runtime de AppImage en
`~/.cache/aula-empaquetado` (o `$AULA_TOOLS`), comprueba su SHA-256 cada vez y deja los cinco
paquetes en `dist/release`. Las versiones están fijadas en el script:

| Herramienta | Versión | Cómo se comprueba |
|---|---|---|
| nfpm | `v2.48.0` | `go install` con la versión exacta (go.sum del módulo) |
| appimagetool | `1.9.1` (x86_64) | SHA-256 en `empaquetar.sh` |
| runtime type2 de AppImage | `20251108` (x86_64) | SHA-256 en `empaquetar.sh` |

Para subir una, cambia la versión y su SHA-256 (los publica GitHub en cada versión:
`gh api repos/AppImage/appimagetool/releases/tags/<versión> --jq '.assets[] | .name, .digest'`)
y corre la prueba de abajo.

Los paquetes los describen `empaquetado/linux/nfpm-alumno.yaml` y `nfpm-consola.yaml`, con sus
scripts en `empaquetado/linux/scripts/`. El mantenedor y la licencia son provisionales hasta la
comercialización (prompt 16).

## Probar los paquetes

**Cambia el equipo**: instala, crea una clave del centro, configura y desinstala. Úsalo en un
contenedor, una VM o el CI, nunca en el equipo de un maestro.

```sh
docker run --rm -v "$PWD:/src:ro" ubuntu:24.04 \
  bash /src/empaquetado/linux/probar-paquetes.sh deb /src/dist/release X.Y.Z
docker run --rm -v "$PWD:/src:ro" fedora:42 \
  bash /src/empaquetado/linux/probar-paquetes.sh rpm /src/dist/release X.Y.Z
```

Comprueba que las dos apps arrancan y dicen su versión, el grupo `aula` y los permisos de la
clave, `aula-cliente configurar`, la AppImage (con `APPIMAGE_EXTRACT_AND_RUN=1`, sin FUSE) y que
quitar los paquetes borra todo menos la clave del centro.

## Pruebas de Go con X11

Las pruebas del distintivo, la captura y el atajo necesitan un servidor X. Se saltan sin él;
en el CI y en un contenedor corren bajo Xvfb, **nunca** en el escritorio de una persona (el
atajo se prueba con teclas sintéticas de XTEST):

```sh
GOFLAGS=-tags=webkit2_41 xvfb-run -a go test ./internal/platform/ ./apps/student/
```

## El servicio con systemd real

El job `servicio-linux` del CI instala la unidad de prueba `aula-agente-prueba` y corre su ciclo
(arrancar, matarlo y ver que systemd lo levanta, detener, quitar), y las pruebas que lanzan el
ayudante como otro usuario (`TestLaunchAsTheStudent`), como root. En local sirve un contenedor
con systemd como PID 1 (privilegiado; nunca en el equipo de un maestro):

```sh
docker run -d --name aula-sd --privileged --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw -v "$PWD:/src" <imagen con systemd> /lib/systemd/systemd
docker exec aula-sd bash -c 'cd /src && AULA_SERVICE_TEST=1 \
  go test -tags servicetest -run TestServiceLifecycle ./apps/student/'
```

`probar-paquetes.sh` también comprueba el servicio (habilitado y en marcha) donde hay systemd: un
runner o una VM, no un contenedor común.

## Lo que se prueba a mano

Lo que Xvfb no ve (un gestor de ventanas real, la sesión «Ubuntu en Xorg» frente a Wayland, el
servicio con una sesión gráfica real de logind, el portapapeles con otras apps) está en
`docs/prueba-vms.md` §31 y §32 y en los casos C-162 a C-168 y A-66 a A-71 de
`probar-consola-local.md`.
