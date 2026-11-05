# Entrega: captura de pantalla de un alumno (prompt 10.5)

Estado: **para aprobación**. Diseño aprobado el 2026-11-05, con el agregado de «Tomar captura»
también durante el control remoto. Reglas 7 y 44–49 en `docs/privacidad.md`; protocolo en
`docs/protocol.md` §5.17.

## Consola (`captura/`)

| Captura | Qué muestra |
|---|---|
| `62-ficha-tomar-captura.png` | «Tomar captura» en la ficha, con su nota («El alumno verá un aviso…»), y «Tomar una captura de su pantalla: Sí». |
| `63-captura-vista-previa.png` | La vista previa (`#captura`): imagen, «1920×1080 · hora · tamaño», «Descartar» y «Guardar». |
| `64-captura-guardada.png` | Tras «Guardar»: «Guardada en …\Documentos\Aula\capturas\LAB3-PC07 (alumno07)\2026-11-05\10-42-06.jpg», «Abrir carpeta» y «Listo». |
| `65-captura-fallida.png` | La sesión del alumno estaba bloqueada: «Su equipo no pudo capturar la pantalla» con qué hacer; nada que guardar. |
| `66-captura-tamano-real.png` | Un clic en la imagen la muestra a tamaño real, con barras de desplazamiento. |
| `67-captura-lote.png` | Panel «Captura» sobre 3 seleccionados: uno no lo permite y se omite; miniaturas con casillas y «Guardar 2». |
| `68-captura-lote-guardadas.png` | «2 capturas guardadas en <carpeta>» con «Abrir carpeta». La barra de abajo, con 5 botones, pasa a solo iconos cuando no cabe. |
| `69-captura-durante-control.png` | La vista previa encima de la vista del control: el control sigue (franja roja arriba). |
| `70-captura-oscuro-900x600.png` | Modo oscuro a 900×600, ya guardada. |
| `71-ajustes-carpeta-capturas.png` | Ajustes › Acerca de: «Capturas de pantalla», la ruta y «Abrir carpeta». |

## App del alumno (`../alumno/captura/`)

| Captura | Qué muestra |
|---|---|
| `41-captura-tomada.png` | Aviso en la esquina «Captura de pantalla · Prof. Ayala (canal 1) tomó una captura de tu pantalla (10:42 a.m.).», con icono de cámara; no es una banda. |
| `42-captura-con-pantalla-vista.png` | Durante las miniaturas: la banda «pantalla vista» sigue y el aviso va aparte. |
| `43-captura-oscuro.png` | Modo oscuro. |
| `44-captura-angosta.png` | A 480×360. |

## Verificado

- `node scripts/ux/recorrido.mjs` (71 capturas) y `node scripts/ux/recorrido-alumno.mjs`
  (44 capturas): sin errores de JS ni violaciones serias de axe.
- `node scripts/ux/pruebas-alumno.mjs`: 30 casos, con `aviso-captura` (no quita el foco, no es
  banda, cuenta como «sin ver», se cierra con Esc).
- `go test -race ./...`, `go vet`, `staticcheck` y `gofmt -l` limpios.

## Cambio visible fuera de la función

- La barra de acciones de Alumnos tiene un quinto botón, «Captura». Para que el texto de la
  selección no se corte, los botones pasan a solo icono por debajo de 54rem de ancho de la barra
  (antes 46rem) en `internal/ui/componentes.css`; con un panel lateral abierto se ven así.

## Falta probar a mano

- C-49 a C-53 y A-12 y A-13 (`docs/desarrollo/probar-consola-local.md`).
- La prueba en VM, `docs/prueba-vms.md` §17 (SN-A, SN-B y SN-C para aceptar): el distintivo
  nativo de 5 s, la resolución real, el Explorador y que nada quede en el equipo del alumno.
