---
titulo: "Tareas frecuentes"
resumen: "Mostrar tu pantalla, mandar avisos, restringir aplicaciones y sitios, apagar equipos y cambiar de canal."
grupo: "Tareas frecuentes"
orden: 60
pantallas: []
---

# Tareas frecuentes

Todas estas acciones están en la barra de abajo de [Alumnos](alumnos#acciones-sobre-la-selección).
Sin selección van a todos los conectados; con selección, solo a los elegidos.

## Mostrar tu pantalla

1. Pulsa **Mostrar mi pantalla**.
2. Elige el monitor y pulsa **Empezar**.

Tus alumnos ven tu pantalla en grande, con tu nombre arriba. Una franja roja en tu consola te
recuerda que estás transmitiendo; la ventana de Aula no aparece en la transmisión. Para terminar,
pulsa **Detener** en la franja. Por ahora, solo equipos con Windows pueden recibirla.

{{diagrama:14-transmision-pantalla}}

## Mandar un aviso

1. Pulsa **Aviso**.
2. Escribe el mensaje y elige **Información** o **Advertencia** (en amarillo).
3. Pulsa **Enviar aviso**.

En [Código › Últimos envíos](codigo#últimos-envíos) ves quién ya lo vio.

## Bloquear aplicaciones o sitios

1. Pulsa **Restricciones** y elige **Sitios web** o **Aplicaciones**.
2. Elige **Bloquear estos** o **Permitir solo estos**.
3. Escribe uno por línea (o usa una plantilla) y pulsa **Aplicar**.

- **Sitios**: escribe el dominio, como `youtube.com`; incluye sus subdominios.
- **Aplicaciones**: el nombre del programa, como `steam` o `steam.exe`.

Los alumnos ven un aviso mientras haya restricciones, y todo se quita solo al cerrar la clase o
si se pierde la conexión. Solo funcionan en equipos con Windows y Aula instalada como servicio.

> **Lo que no se puede garantizar:** un alumno con permisos de administrador puede detener Aula;
> renombrar un programa evita el bloqueo por nombre; entrar a un sitio por su dirección IP o con
> una VPN evita el filtro.

{{diagrama:15-restricciones}}

## Apagar o reiniciar los equipos

1. Pulsa **Energía**.
2. Elige **Reiniciar**, **Apagar** o **Cerrar sesión**.
3. Elige el tiempo para que guarden su trabajo y confirma.

Cada alumno ve un aviso con la cuenta regresiva y puede posponerla 5 minutos, hasta 2 veces. Si
marcas **Forzar ahora**, no hay cuenta y las aplicaciones se cierran sin guardar. Mientras haya
órdenes pendientes, una franja en tu consola permite **Cancelar**.

{{diagrama:16-energia}}

## Cambiar de canal o de nombre

Toca la tarjeta «Clase abierta · canal» del menú (o ve a [Ajustes](ajustes#clase)), escribe el
canal y confirma. Los alumnos del canal anterior dejan de verte, se quitan las restricciones y se
detiene la transmisión; tu historial de código se conserva.
