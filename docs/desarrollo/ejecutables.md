# Ejecutables: qué es cada uno y dónde queda

En local, todo lo que se ejecuta o se copia a otro equipo sale de **un solo comando** y queda en
**una sola carpeta**, `dist\local`, con una subcarpeta por sistema (`windows` y `linux`):

```powershell
.\scripts\compilar.ps1                    # las dos apps y los dos instaladores (Inno Setup 6)
.\scripts\compilar.ps1 -Solo consola      # solo la consola portable, sin instaladores
.\scripts\compilar.ps1 -Solo consola -Abrir
.\scripts\compilar.ps1 -Solo alumno       # solo la app del alumno portable
.\scripts\compilar.ps1 -Version 0.2.0-dev # otra versión (por defecto, docs/ux/notas/VERSION + -dev)
.\scripts\compilar.ps1 -Linux             # además, los paquetes de Linux bajados del CI
.\scripts\compilar.ps1 -Solo linux        # solo bajar los paquetes de Linux del CI
```

Los paquetes de Linux no se compilan en Windows (Wails en Linux necesita cgo y WebKitGTK): se
bajan con `gh` de la última corrida verde del CI en `develop`, así que son del último push, no de
tus cambios sin subir (el script lo avisa).

El script compila como el workflow de release (`.github/workflows/release.yml`): sin cgo, con
rutas recortadas y la versión embebida; solo le falta la firma.

## Lo que queda en `dist\local`

| Archivo | Qué es | Para qué |
|---|---|---|
| `windows\bin\aula-consola.exe` | La consola del profesor, portable | Probar la consola con el aula simulada (`probar-consola-local.md`) o en una VM sin instalar |
| `windows\bin\aula-cliente.exe` | La app del alumno, portable (ventana, ayudante y servicio en un solo ejecutable) | Probar sin instalar (`docs/prueba-vms.md`) |
| `windows\Aula-Consola-Setup-<versión>.exe` | Instalador de la consola (Inno Setup) | Equipo o VM del profesor (`instaladores-locales.md`) |
| `windows\Aula-Alumno-Setup-<versión>.exe` | Instalador del alumno; instala el servicio `AulaAgente` | Equipos o VMs de alumno |
| `linux\aula-consola_<versión>_amd64.deb`, `linux\aula-cliente_<versión>_amd64.deb`, sus `.rpm` y `linux\aula-consola-<versión>-x86_64.AppImage` | Paquetes de Linux del CI (con `-Linux` o `-Solo linux`) | VMs Linux (`docs/prueba-vms.md` §31.0) |

Con `-Solo consola` o `-Solo alumno` se reemplaza solo ese `windows\bin\*.exe`; los instaladores
que ya estuvieran ahí siguen siendo los de la última compilación completa (que antes vacía
`dist\local\windows`, sin tocar `dist\local\linux`). Con `-Linux` o `-Solo linux` se reemplaza
toda `dist\local\linux`.

## Lo que no se usa

- **`apps\teacher\build\bin\aula-consola.exe`, `apps\student\build\bin\aula-cliente.exe`:** es
  donde `wails build` deja siempre su compilación (Wails no permite cambiar esa carpeta). Es un
  paso intermedio: el script la copia a `dist\local\windows\bin`. No ejecutes nada desde ahí; puede ser
  de otra versión o de otra compilación.
- **`teacher.exe`, `student.exe` sueltos** (en la raíz o en `apps\*`): salen de
  `go build ./apps/teacher` o de `go build` dentro de la carpeta. **No sirven:** se compilan sin
  las etiquetas de Wails (`desktop,production`) y la app no abre. Bórralos si aparecen.
  - Para comprobar que todo compila, usa `go build ./...` (con varios paquetes no escribe
    ningún archivo) o `go vet ./...`.
  - Para iterar en el frontend, `wails dev` dentro de `apps\teacher` o `apps\student`.

## En el repositorio

Nada de esto se sube: `.gitignore` excluye `*.exe`, `apps/*/build/bin/` y `/dist/`. Las
versiones publicadas salen del workflow de release (`publicar.md`), no de `dist\local`.
