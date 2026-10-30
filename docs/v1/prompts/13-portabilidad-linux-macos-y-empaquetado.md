# 13 · Portabilidad Linux/macOS + empaquetado multiplataforma

**Prerrequisitos:** Hito 1 cerrado (08) y, idealmente, la etapa 09 (para portar también captura y overlay).
**Duración estimada:** 3–6 sesiones (conviene una por plataforma).
**Marco:** completar "multiplataforma", que es requisito central. Se hace por capas: primero que todo compile y corra en cada SO, luego cada función de `platform`, luego el empaquetado.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/decisiones.md, docs/protocol.md y docs/progreso.md. Vamos a llevar Aula a
Linux y macOS. Trabaja una plataforma a la vez; empieza por Linux (X11) salvo que yo indique otra.
Para cada interfaz de `internal/platform` que hoy solo tiene implementación de Windows, agrega la
de este SO; si algo no es posible en este SO, deja el fallback "no soportado" con un mensaje claro
y anótalo en una tabla de compatibilidad en docs/compatibilidad.md.

## Alcance por plataforma (repite el bloque por SO)
1. **Compilar y correr**: `wails build` de ambas apps en <SO>; resolver dependencias de cgo y de
   WebView (WebKitGTK en Linux, WKWebView en macOS).
2. **Interfaces de platform**: `ScreenCapture`, `ScreenOverlay`, portapapeles, hotkey global y,
   si ya existe, `ServiceControl`. Implementa lo posible en <SO>.
3. **Permisos**:
   - Linux: documenta X11 vs Wayland (captura y control de entrada en Wayland requieren
     aprobación por sesión; recomienda sesión X11 para el laboratorio). Reglas Polkit/DBus si
     hace falta para el overlay o el servicio.
   - macOS: permisos de Grabación de Pantalla y Accesibilidad; cómo se conceden y cómo
     preautorizarlos con un perfil MDM en un laboratorio gestionado.
4. **Empaquetado**:
   - Linux: `.deb` y `.rpm` nativos (nfpm es una buena opción) y un formato universal (AppImage o
     Flatpak; recomienda uno).
   - macOS: `.dmg`/`.pkg`; describe el proceso de firma y notarización (requiere cuenta Apple
     Developer de pago) y qué pasa si aún no la tienes (ejecución local con Gatekeeper).
5. **Extiende el workflow de release** (`.github/workflows/release.yml`, creado en la etapa 08):
   - Agrega un job que construya los `.deb`/`.rpm` a partir de los binarios ya compilados, y que
     pruebe la instalación en un runner (`apt-get install ./*.deb`, arranca, y `purge`).
   - Agrega un job de macOS que construya el `.pkg`, con firma y notarización **opcionales**
     (solo si existen los secretos de Apple), y que pruebe la instalación con `installer`.
   - Esos artefactos se suman a `checksums.txt` y a la versión de GitHub; mantén la verificación
     de la firma de releases. Reusa la matriz y las acciones ancladas de CLAUDE.md.

## Reglas
- No metas lógica específica de SO fuera de `internal/platform`.
- Mantén verde el CI en los tres sistemas para lo que sí compila.
- Cada función portada conserva el principio de transparencia (indicador visible al alumno).

## Pruebas
- `go test ./...` en <SO>.
- Prueba manual en una VM o equipo con <SO>: descubrir, conectar, recibir código y copiarlo.
- Para captura/overlay: verifica en <SO> que funcionan y que el aviso al alumno aparece.

## Criterios de aceptación
- En <SO>, un alumno se conecta al canal, recibe código y lo pega intacto.
- Existe un paquete instalable de <SO> y una guía para generarlo.
- docs/compatibilidad.md refleja qué función funciona en cada SO.

Actualiza docs/progreso.md y docs/compatibilidad.md.
~~~~

## Verificación manual
- [ ] Un equipo Linux del laboratorio: ¿descubre el canal y copia el código?
- [ ] ¿La tabla de compatibilidad por SO está completa y es honesta sobre lo no soportado?
