# 08 · Hito 1: integración, instaladores y prueba en el laboratorio

**Prerrequisitos:** etapas 06 y 07 completas.
**Duración estimada:** 1 sesión + una sesión de prueba presencial.
**Meta:** cerrar el Hito 1 con la distribución de código funcionando de punta a punta e instalable.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Vamos a cerrar el Hito 1: distribución de
código y mensajes, estable y empaquetado.

## Tareas
1. **Configuración**
   - Un archivo de configuración por app (profesor y estudiante): canal, puertos, nombre del
     profesor, clave pública del profesor (en el agente). Documenta el formato.
   - Una forma de generar la configuración del agente desde la consola (que ya incluya la clave
     pública y el canal) para no editarla a mano en cada equipo.
2. **Empaquetado (solo Windows en este hito)** — Módulo D
   - `wails build` para ambas apps. Instalador del agente con **Inno Setup** (como en tus otros
     proyectos; `.exe` de setup) que copie el binario y su configuración y lo deje arrancando al
     iniciar sesión el usuario. (El servicio del sistema llega en la Fase 2; por ahora corre en la
     sesión del alumno.) El instalador debe soportar instalación silenciosa (`/VERYSILENT
     /SUPPRESSMSGBOXES /NORESTART`) para el despliegue por GPO de la etapa 15.
   - Instalador o portable de la consola del profesor.
   - Documenta las reglas de firewall local necesarias para mDNS y el WebSocket.
3. **Workflow de release** (`.github/workflows/release.yml`) — primera versión, solo Windows
   - Se dispara al empujar un tag `v*`; sigue las convenciones de CI de CLAUDE.md (acciones
     ancladas, `setup-go` 1.26.x).
   - Deriva la versión del tag (`${GITHUB_REF_NAME#v}`), compila ambas apps, genera los
     instaladores Inno Setup, y comprueba que el binario reporta esa versión.
   - Genera `checksums.txt` (sha256 de todos los artefactos) y lo firma con la llave Ed25519 de
     releases tomada de un secreto del repo; verifica la firma con la llave pública versionada en
     `empaquetado/llave-releases.pub` antes de publicar.
   - Publica con `gh release create "$GITHUB_REF_NAME" --generate-notes` (prerelease si el tag
     lleva guion) adjuntando instaladores, binarios, `checksums.txt` y `checksums.txt.sig`.
   - La firma de código de Windows (Authenticode) queda opcional: solo se aplica si el secreto del
     certificado existe. Documenta los secretos en `docs/desarrollo/publicar.md`.
   - Deja el `.deb`/`.rpm` y el `.pkg` de macOS como jobs a agregar en la etapa 13.
4. **Robustez**
   - Prueba con 10+ agentes simulados a la vez (ejemplo que lanza N clientes).
   - Reconexión al reiniciar la consola: los agentes vuelven solos.
5. **Guía de uso** en `docs/uso-hito1.md` (español): instalar la consola, generar e instalar el
   agente, seleccionar canal, emitir código y usar el hotkey.

## Criterios de aceptación
- Desde una consola recién instalada, con 2–3 agentes en VMs o equipos del laboratorio:
  descubren por canal, conectan, autentican, y el código emitido se copia y pega intacto.
- Reiniciar la consola no exige reinstalar nada en los agentes.
- Empujar un tag `vX.Y.Z` produce una versión en GitHub con los instaladores y un `checksums.txt`
  firmado que verifica con la llave pública del repositorio.
- `staticcheck ./...` y `go test -race ./...` pasan.

Actualiza docs/progreso.md y marca el Hito 1 como completado.
~~~~

## Prueba en el laboratorio (tú, presencial)
- [ ] Instala la consola en tu laptop y el agente en 3–4 equipos.
- [ ] Emite un ejercicio real de tu clase y pide a los alumnos que lo copien.
- [ ] Anota qué falló o fue incómodo: alimenta la lista de ajustes antes de la Fase 2.

> A partir de aquí ya tienes algo usable en clase. Las etapas 09+ agregan el monitoreo (Módulo B).
