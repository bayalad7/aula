# Privacidad y transparencia

Aula es una herramienta de aula **transparente**: el alumno siempre sabe que su equipo está
conectado a una clase y a cuál. No hay modo oculto ni evasión de detección.

## Reglas vigentes (hasta la etapa 06)

1. **El agente siempre se identifica en pantalla.** La ventana del cliente muestra, fijo arriba
   y también en el título de la ventana y en la barra de tareas:
   - Conectado: **"Aula — conectado a la clase de <profesor> (canal <X>)"**
   - Buscando: "Aula — buscando la clase del canal <X>"
   - Sin conexión: "Aula — sin conexión con la clase de <profesor> (canal <X>); reintentando"
   - Sin configurar: "Aula — sin configurar" (con la explicación)
2. **Al cerrar la ventana, el agente se detiene.** No queda ningún proceso en segundo plano.
3. **El agente se presenta con su nombre real** (equipo y usuario del sistema) y solo después de
   comprobar que la consola es la autorizada (`docs/protocol.md` §3.1). Sabe quién lo ve.
4. **Lo que recibe el alumno se ve.** El código y los avisos aparecen en pantalla y la ventana
   se trae al frente. Las confirmaciones (`code.ack`, `msg.ack`) solo dicen "se mostró", no
   "se leyó" ni "se copió".
5. **Registros.** No se registran el código, los textos de los avisos, las claves, los nonces ni
   las firmas. Sí el tipo de mensaje, los identificadores, el equipo y los tamaños.

## Pendiente para las etapas de monitoreo (09 en adelante)

Cuando la pantalla del alumno se vea o se transmita (miniaturas, transmisión), su equipo mostrará
un indicador visible mientras ocurra. No se guardarán capturas en disco salvo petición explícita
del profesor. Los textos de aviso de esas funciones se agregarán aquí antes de implementarlas.
