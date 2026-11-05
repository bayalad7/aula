// Textos de la app del alumno (español de México). Una clave por texto; la
// interfaz los toma con T("clave", {variables}). apps/student/textos_test.go
// exige que cada clave usada exista y que no sobre ninguna.
"use strict";
window.AulaTextos = Object.freeze({
  // Página
  "app.titulo": "Aula",
  "app.contenido": "Código de la clase",

  // Título de la ventana y de la barra de tareas (docs/privacidad.md, reglas 1, 6, 12, 20 y 26)
  "titulo.connected": "Aula — conectado (canal {canal} - {profesor})",
  "titulo.searching": "Aula — buscando canal {canal}",
  "titulo.disconnected": "Aula — sin conexión (canal {canal}); reintentando",
  "titulo.unconfigured": "Aula — sin configurar",
  "titulo.vista": "Aula — tu pantalla está siendo vista por {profesor} (canal {canal})",
  "titulo.recibiendo": "Aula — viendo la pantalla de {profesor} (canal {canal})",
  "titulo.restricciones": " · restricciones activas",
  "titulo.pendiente.reboot": " · reinicio pendiente",
  "titulo.pendiente.shutdown": " · apagado pendiente",
  "titulo.pendiente.logoff": " · cierre de sesión pendiente",

  // Tira de identidad
  "estado.connected": "Conectado al canal {canal} · {profesor}",
  "estado.searching": "Buscando la clase del canal {canal}…",
  "estado.disconnected": "Se perdió la conexión con {profesor}. Reconectando…",
  "estado.unconfigured": "Aula aún no está lista en este equipo. Avisa a tu profesor.",
  "estado.detalles": "Detalles para TI",

  // Código
  "codigos.vacio": "Cuando tu profesor comparta código, aparecerá aquí listo para copiar.",
  "codigo.sinTitulo": "Código sin título",
  "codigo.ejercicio": "Ejercicio {n}",
  "codigo.copiar": "Copiar",
  "codigo.copiado": "Copiado",
  "codigo.noSeCopio": "No se pudo copiar: {motivo}",
  "codigo.region": "Código: {titulo}",

  // Avisos del profesor
  "aviso.info": "Aviso",
  "aviso.warning": "Importante",
  "aviso.entendido": "Entendido",

  // Bandas de transparencia
  "banda.vista": "Tu pantalla la está viendo {profesor} (canal {canal})",
  "banda.recibiendo": "Estás viendo la pantalla de {profesor} (canal {canal})",
  "banda.tamanoReal": "Tamaño real",
  "banda.ajustar": "Ajustar a la ventana",
  "banda.conectando": "Conectando con la pantalla del profesor…",
  "banda.restricciones": "Restricciones de la clase activas: {partes}",
  "banda.restricciones.appsDeny": "aplicaciones bloqueadas",
  "banda.restricciones.appsAllow": "solo se permiten algunas aplicaciones",
  "banda.restricciones.webDeny": "sitios web filtrados",
  "banda.restricciones.webAllow": "solo se permiten algunos sitios",
  "banda.restricciones.separador": " · ",
  "banda.verDetalles": "Ver detalles",
  "banda.ocultarDetalles": "Ocultar detalles",
  "banda.lista.apps": "Aplicaciones",
  "banda.lista.web": "Sitios",
  "banda.lista.soloEstas": " (solo estas)",
  "banda.lista.mas": " y {n} más",
  "banda.appCerrada": "Aula cerró «{app}»: está bloqueada durante la clase.",

  // Energía (docs/privacidad.md, reglas 26-28)
  "energia.cuenta.reboot": "{profesor} (canal {canal}) va a reiniciar este equipo en {tiempo}. Guarda tu trabajo.",
  "energia.cuenta.shutdown": "{profesor} (canal {canal}) va a apagar este equipo en {tiempo}. Guarda tu trabajo.",
  "energia.cuenta.logoff": "{profesor} (canal {canal}) va a cerrar tu sesión en {tiempo}. Guarda tu trabajo.",
  "energia.ejecutando.reboot": "{profesor} (canal {canal}) está reiniciando este equipo.",
  "energia.ejecutando.shutdown": "{profesor} (canal {canal}) está apagando este equipo.",
  "energia.ejecutando.logoff": "{profesor} (canal {canal}) está cerrando tu sesión.",
  "energia.fallo.reboot": "No se pudo reiniciar este equipo. Puedes seguir trabajando.",
  "energia.fallo.shutdown": "No se pudo apagar este equipo. Puedes seguir trabajando.",
  "energia.fallo.logoff": "No se pudo cerrar tu sesión. Puedes seguir trabajando.",
  "energia.profesor": "Tu profesor",
  "energia.posponer": "Posponer 5 minutos",
  "energia.quedan.uno": "(1 vez más)",
  "energia.quedan.varios": "({n} veces más)",
  "energia.yaNo": "Ya no se puede posponer",
  "energia.entendido": "Entendido",
});
