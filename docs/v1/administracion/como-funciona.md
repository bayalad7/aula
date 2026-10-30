---
titulo: "Cómo funciona Aula"
resumen: "Los diagramas de Aula: quién se conecta con quién, cómo viaja tu código y qué ve el alumno."
grupo: "Cómo funciona Aula"
orden: 110
pantallas: [ayuda]
---

# Cómo funciona Aula

Estos diagramas explican Aula por dentro. No hace falta entenderlos para dar clase, pero ayudan a
saber qué pasa cuando algo no funciona.

## Quién se conecta con quién

La consola anuncia tu canal en la red y los equipos de los alumnos de ese canal se conectan a
ella. Nunca al revés.

{{diagrama:01-contexto}}

## Cómo encuentra el alumno a su consola

{{diagrama:03-descubrimiento-canal}}

## Cómo viaja tu código

{{diagrama:06-emision-codigo}}

## Qué ve el alumno

Cada acción tuya tiene un aviso visible en el equipo del alumno.

{{diagrama:09-transparencia-avisos}}

## Conectado y desconectado

{{diagrama:05-ciclo-sesion}}

## El aula simulada

{{diagrama:10-aula-simulada}}

## Para el responsable de TI

- [Seguridad y la clave del centro](diagrama:04-autenticacion-tls)
- [Aula como servicio de Windows](diagrama:13-agente-servicio)
- [Qué guarda Aula en cada equipo](diagrama:11-modelo-datos)
- [Cómo está hecho Aula por dentro](diagrama:02-componentes)
