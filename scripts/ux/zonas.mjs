// Vocabulario de la interfaz: el nombre de cada zona de las dos apps y cómo encontrarla. Es la
// única fuente de los nombres que usan el mapa (mapa.mjs → docs/ux/MAPA.md), el anotador
// (anotar.mjs) y las conversaciones sobre la interfaz: «en la Ficha del alumno, en Qué puede
// hacer su equipo». Si una zona cambia de lugar o de selector, se cambia aquí.
//
// Cada zona: id estable, nombre en español (como se le dice), selector CSS, la zona que la
// contiene (para armar «Consola › Código › Últimos envíos») y una línea de qué es.

export const zonas = {
  profesor: [
    // Marco
    { id: "menu", nombre: "Menú", selector: ".nav-lateral", que: "Columna izquierda: la clase abierta, las vistas y Ayuda y Ajustes." },
    { id: "menu-clase", dentro: "menu", nombre: "Selector de clase", selector: "#nav-clase", que: "Canal y nombre de la clase abierta; abre «Cambiar de canal»." },
    { id: "franja", nombre: "Franja global", selector: ".principal > .franja", que: "Lo que sigue activo durante la clase (transmitir, proyectar a un alumno, control, pantallas en pausa, energía, temporizador, encuesta abierta, aula simulada), con su botón para detenerlo." },
    { id: "cabecera", nombre: "Cabecera de la vista", selector: ".vista__cabecera", que: "Título de la vista, su resumen y el «?» de ayuda contextual." },
    { id: "avisos", nombre: "Avisos (toasts)", selector: "#avisos", que: "Mensajes breves abajo a la derecha tras una acción." },

    // Alumnos
    { id: "alumnos", nombre: "Vista Alumnos", selector: "[data-vista-de=alumnos]", que: "Los alumnos del canal, en lista o miniaturas." },
    { id: "alumnos-modo", dentro: "alumnos", nombre: "Selector Lista | Miniaturas", selector: "[data-vista-de=alumnos] .segmentado", que: "Cambia entre la tabla y las miniaturas de pantalla." },
    { id: "alumnos-lista", dentro: "alumnos", nombre: "Lista de alumnos", selector: "#alumnos-lista", que: "Tabla con un alumno por fila." },
    { id: "fila-alumno", dentro: "alumnos-lista", nombre: "Fila del alumno", selector: "#alumnos-filas > tr", que: "Un alumno: casilla, equipo, estado, insignias y el botón de su ficha." },
    { id: "alumnos-miniaturas", dentro: "alumnos", nombre: "Miniaturas", selector: "#alumnos-miniaturas", que: "Cuadrícula con la pantalla de cada alumno." },
    { id: "tarjeta-alumno", dentro: "alumnos-miniaturas", nombre: "Tarjeta del alumno", selector: "#alumnos-cuadricula > li", que: "La miniatura de un alumno con su nombre; un clic abre la Pantalla grande." },
    { id: "barra-acciones", dentro: "alumnos", nombre: "Barra de acciones", selector: ".barra-acciones", que: "Abajo: lo que se aplica a la selección (aviso, transmitir, pausar, restricciones, captura, temporizador, encuesta, energía…)." },

    // Paneles y diálogos que salen de Alumnos
    { id: "ficha", nombre: "Ficha del alumno", selector: "#panel-alumno", que: "Panel derecho con los datos de un alumno y qué puede hacer su equipo; su fila queda marcada. Las acciones están en la Pantalla grande." },
    { id: "ficha-puede", dentro: "ficha", nombre: "Qué puede hacer su equipo", selector: "#ficha-puede", que: "Lista de capacidades del equipo del alumno." },
    { id: "pantalla-grande", nombre: "Pantalla grande", selector: "#pantalla-grande", que: "La pantalla de un alumno en grande; también la vista del control remoto y la de la proyección de un alumno." },
    { id: "pg-acciones", dentro: "pantalla-grande", nombre: "Acciones sobre el alumno", selector: "#pg-acciones", que: "Tomar control, Mensaje privado, Tomar captura, Proyectar a la clase, Pausar pantalla y Abrir página o app (ACCIONES_ALUMNO en main.js)." },
    { id: "dlg-privado", nombre: "Diálogo Mensaje privado", selector: "#dlg-privado", que: "Escribir un mensaje solo para un alumno, con su tipo; se abre desde la Pantalla grande." },
    { id: "dlg-abrir", nombre: "Diálogo Abrir en un alumno", selector: "#dlg-abrir", que: "Abrir una página o una app permitida en el equipo de un alumno; se abre desde la Pantalla grande." },
    { id: "dlg-plantilla", nombre: "Diálogo Nueva plantilla", selector: "#dlg-plantilla", que: "Crear una plantilla con su nombre y sus archivos; se abre desde Plantillas (nota de interfaz 17)." },
    { id: "dlg-pausa", nombre: "Diálogo Cambiar mensaje de la pausa", selector: "#dlg-pausa", que: "Cambiar el mensaje de las pantallas en pausa; se abre desde la Franja global (prompt 10.21)." },
    { id: "panel-aviso", nombre: "Panel Aviso", selector: "#panel-aviso", que: "Mandar un aviso a la selección." },
    { id: "panel-pausa", nombre: "Panel Pausar pantallas", selector: "#panel-pausa", que: "Pausar las pantallas de la selección o de todos, con un mensaje opcional, o reanudarlas (Ctrl+Mayús+P)." },
    { id: "panel-pantalla", nombre: "Panel Transmitir", selector: "#panel-pantalla", que: "Transmitir tu pantalla a la clase." },
    { id: "panel-restricciones", nombre: "Panel Restricciones", selector: "#panel-restricciones", que: "Bloquear aplicaciones y filtrar sitios." },
    { id: "panel-captura", nombre: "Panel Captura", selector: "#panel-captura", que: "Tomar capturas de varios alumnos." },
    { id: "panel-temporizador", nombre: "Panel Temporizador", selector: "#panel-temporizador", que: "Lanzar un temporizador de ejercicio." },
    { id: "panel-energia", nombre: "Panel Energía", selector: "#panel-energia", que: "Apagar, reiniciar o cerrar sesión." },
    { id: "panel-encuesta", nombre: "Panel Encuesta", selector: "#panel-encuesta", que: "La pregunta, el tipo de respuesta, las opciones y «Ver quién respondió qué»; Lanzar a todos o a la selección." },
    { id: "panel-abrir", nombre: "Panel Abrir página o app", selector: "#panel-abrir", que: "Abrir una página en el navegador o una app permitida en los equipos de la selección, y las últimas que abriste." },
    { id: "captura", nombre: "Vista previa de captura", selector: "#captura", que: "La captura de un alumno, con Guardar y Descartar." },

    // Código
    { id: "codigo", nombre: "Vista Código", selector: "[data-vista-de=codigo]", que: "Preparar y enviar código a los alumnos." },
    { id: "codigo-editor", dentro: "codigo", nombre: "Tarjeta del editor", selector: "[data-vista-de=codigo] .codigo-rejilla > .tarjeta:first-child", que: "Título, lenguaje, editor, «Enviar a» y los botones de emitir." },
    { id: "editor", dentro: "codigo-editor", nombre: "Editor", selector: "#editor", que: "El editor de código (Monaco)." },
    { id: "enviar-a", dentro: "codigo-editor", nombre: "Enviar a", selector: "[data-vista-de=codigo] fieldset", que: "Todos los conectados o la selección." },
    { id: "envios", dentro: "codigo", nombre: "Últimos envíos", selector: "[data-vista-de=codigo] .tarjeta--estirada", que: "Cada envío con quién lo tiene y quién falta." },
    { id: "historial", dentro: "codigo", nombre: "Historial de la clase", selector: "[data-vista-de=codigo] .vista__cuerpo > .tarjeta", que: "Lo emitido en la clase, para reabrir o reemitir." },

    // Archivos
    { id: "archivos", nombre: "Vista Archivos", selector: "[data-vista-de=archivos]", que: "Enviar archivos y plantillas a los alumnos, y recoger su trabajo." },
    { id: "archivos-pestanas", dentro: "archivos", nombre: "Selector Enviar | Recolecciones", selector: "[data-vista-de=archivos] .segmentado", que: "Cambia entre enviar archivos y las recolecciones." },
    { id: "archivos-enviar", dentro: "archivos", nombre: "Enviar archivos", selector: "[data-vista-de=archivos] .codigo-rejilla > .tarjeta:first-child", que: "Plantilla o archivos elegidos, «Guardar en», «Reemplazar», «Enviar a» y Enviar." },
    { id: "archivos-envios", dentro: "archivos", nombre: "Envíos de archivos", selector: "[data-vista-de=archivos] .tarjeta--estirada", que: "Cada envío de archivos con quién lo tiene y quién falta." },
    { id: "plantillas", dentro: "archivos", nombre: "Plantillas", selector: "section[aria-labelledby=h-plantillas]", que: "Las plantillas de Documentos\\Aula\\plantillas: Nueva plantilla, Usar, Abrir carpeta y Eliminar." },
    { id: "archivos-historial", dentro: "archivos", nombre: "Archivos enviados", selector: "section[aria-labelledby=h-historial-archivos]", que: "Los envíos de la clase, para reenviarlos." },
    { id: "recolecciones", dentro: "archivos", nombre: "Recolecciones", selector: "#archivos-panel-recolecciones", que: "Crear la carpeta de una actividad en los equipos y recoger lo que guardó cada alumno." },
    { id: "reco-nueva", dentro: "recolecciones", nombre: "Nueva recolección", selector: "section[aria-labelledby=h-reco-nueva]", que: "Actividad, tipos de archivo y «Crear carpeta en los equipos»." },
    { id: "recoleccion", dentro: "recolecciones", nombre: "Tarjeta de recolección", selector: "#reco-lista > .recoleccion", que: "Una actividad: Recoger, Recoger a los que faltan, Abrir carpeta y la tabla por alumno." },
    { id: "reco-fila", dentro: "recoleccion", nombre: "Fila de la recolección", selector: "#reco-lista .reco-filas > tr", que: "Un alumno: estado, qué llegó, cuándo, Abrir, Guardar como… y Recoger otra vez." },

    // Participación
    { id: "participacion", nombre: "Vista Participación", selector: "[data-vista-de=participacion]", que: "Lo que responden los alumnos en vivo." },
    { id: "manos", dentro: "participacion", nombre: "Manos levantadas", selector: "#manos", que: "La fila de manos, por orden de llegada." },
    { id: "mensajes", dentro: "participacion", nombre: "Mensajes de los alumnos", selector: "#mensajes", que: "La bandeja de mensajes con Responder." },
    { id: "part-encuestas", dentro: "participacion", nombre: "Sección Encuestas", selector: "#encuestas", que: "La encuesta abierta con sus barras en vivo y Cerrar, y las anteriores con Compartir resultados." },
    { id: "part-temporizador", dentro: "participacion", nombre: "Sección Temporizador", selector: "#temporizador-activo, #temporizador-vacio", que: "Quién ve el temporizador y quién falta." },

    // Otras vistas
    { id: "simulada", nombre: "Vista Aula simulada", selector: "[data-vista-de=simulada]", que: "Alumnos de prueba y sus fallas." },
    { id: "ayuda", nombre: "Vista Ayuda (manual)", selector: "[data-vista-de=ayuda]", que: "El manual, con búsqueda e índice." },
    { id: "panel-ayuda", nombre: "Panel de ayuda contextual", selector: "#panel-ayuda", que: "La página del manual de la vista actual, al costado." },
    { id: "falta", nombre: "Vista ¿Falta un alumno?", selector: "[data-vista-de=falta]", que: "Comprobar la conexión y el asistente paso a paso." },
    { id: "ajustes", nombre: "Ajustes", selector: "#ajustes", que: "Diálogo con Clase, Apariencia, Apps permitidas y Acerca de." },
    { id: "ajustes-apps", dentro: "ajustes", nombre: "Apps permitidas", selector: "#aj-apps", que: "Las apps que se pueden abrir en los equipos: nombre y ruta del .exe, con ejemplos." },
    { id: "confirmar", nombre: "Diálogo de confirmación", selector: "#confirmar", que: "«¿Seguro?» antes de una acción con consecuencias." },
    { id: "arranque", nombre: "Pantalla de arranque", selector: "#arranque", que: "Abrir la clase: canal y nombre." },
  ],

  alumno: [
    { id: "tira", nombre: "Tira de identidad", selector: "#estado", que: "Arriba: a qué clase está conectado, el temporizador y los botones del alumno." },
    { id: "tira-botones", dentro: "tira", nombre: "Botones de la tira", selector: "#mano, #escribir, #capacidades, #compactar", que: "Levantar la mano, Mensaje al profesor, Qué puede hacer Aula en este equipo y Modo compacto." },
    { id: "tira-temporizador", dentro: "tira", nombre: "Insignia del temporizador", selector: "#temporizador", que: "Cuánto falta del ejercicio." },
    { id: "bandas", nombre: "Bandas de transparencia", selector: "#bandas", que: "Lo que el profesor hace con el equipo (energía, control, pantalla en pausa, pantalla vista, tu pantalla mostrada a la clase, restricciones); no se cierran." },
    { id: "banda-mostrando", dentro: "bandas", nombre: "Banda «Tu pantalla se está mostrando»", selector: "#banda-mostrando", que: "Mientras sus compañeros ven su pantalla; sin botones: la termina el profesor." },
    { id: "mano-estado", nombre: "Estado de la mano", selector: "#mano-estado", que: "«Tu mano está levantada» y el motivo." },
    { id: "redactar", nombre: "Diálogo Mensaje al profesor", selector: "#dlg-mensaje", que: "El diálogo para escribirle al profesor." },
    { id: "capacidades", nombre: "Qué puede hacer Aula en este equipo", selector: "#dlg-capacidades", que: "La grilla de lo que el profesor puede hacer con este equipo, como la ficha de la consola." },
    { id: "enviados", nombre: "Tus mensajes", selector: "#enviados", que: "Los mensajes enviados y si llegaron." },
    { id: "vacio", nombre: "Estado vacío", selector: "#empty", que: "Lo que se ve cuando no hay código ni archivos." },
    { id: "encuestas", nombre: "Encuestas", selector: "#encuestas", que: "Una tarjeta por encuesta del profesor, arriba de la lista." },
    { id: "tarjeta-encuesta", dentro: "encuestas", nombre: "Tarjeta de encuesta", selector: "#encuestas > li", que: "La pregunta, si es anónima, las opciones con Enviar y, si el profesor los comparte, los resultados de la clase." },
    { id: "recolecciones", nombre: "Recolecciones", selector: "#recolecciones", que: "Una tarjeta por recolección del profesor." },
    { id: "tarjeta-recoleccion", dentro: "recolecciones", nombre: "Tarjeta de recolección", selector: "#recolecciones > li", que: "Dónde guardar el ejercicio, qué se recogió y cuándo, y Abrir carpeta." },
    { id: "archivos", nombre: "Archivos recibidos", selector: "#archivos", que: "Una tarjeta por envío de archivos." },
    { id: "tarjeta-archivo", dentro: "archivos", nombre: "Tarjeta de archivo", selector: "#archivos > li", que: "Qué llegó, dónde quedó y Abrir carpeta." },
    { id: "codigos", nombre: "Lista de código", selector: "#snippets", que: "El código que mandó el profesor, lo más nuevo arriba." },
    { id: "tarjeta-codigo", dentro: "codigos", nombre: "Tarjeta de código", selector: "#snippets > li", que: "Un fragmento con su ejercicio, título, lenguaje y Copiar." },
    { id: "avisos", nombre: "Avisos en la esquina", selector: "#avisos", que: "Avisos del profesor y de Aula, abajo a la derecha." },
    { id: "compacto", nombre: "Modo compacto", selector: "#compacto", que: "La ventana encogida a una tira." },
    { id: "transmision", nombre: "Vista de la transmisión", selector: "#broadcast", que: "La pantalla del profesor, o la de un compañero, ocupando la ventana." },
  ],
};

// Escenas de datos de demostración (dobles.js y dobles-alumno.js), con lo que muestran.
export const escenas = {
  profesor: {
    lab: "Laboratorio: 5 equipos reales",
    datos: "Dos simulados con historial",
    vacia: "Clase abierta sin alumnos",
    "lab-errores": "Laboratorio con errores de alumnos",
    "lab-control": "Controlando un alumno",
    "lab-control-pausa": "Control en pausa",
    "lab-transmitiendo": "Transmitiendo tu pantalla",
    "lab-proyectar-conectando": "Proyectando: conectando con el alumno",
    "lab-proyectando": "Proyectando la pantalla de un alumno",
    "lab-restr-activa": "Restricciones activas",
    "lab-energia-pendiente": "Reinicio pendiente",
    "lab-pausa": "Pantallas en pausa, una sin acusar",
    "lab-captura-falla": "Captura que falló",
    "lab-participacion": "Manos y mensajes",
    "lab-temporizador": "Temporizador corriendo",
    "lab-temporizador-fin": "Temporizador terminado",
    "lab-archivos": "Envío de archivos con un fallo",
    simulada: "Aula simulada abierta",
    "simulada-sola": "Solo alumnos simulados",
    "clave-personal": "Consola con clave personal",
    "red-publica": "Red pública (comprobar conexión)",
    "setup-error": "Arranque con error",
    "atajo-ocupado": "Atajo de teclado ocupado",
  },
  alumno: {
    conectado: "Conectado, sin nada",
    "varios-codigos": "Tres fragmentos de código",
    "un-codigo": "Un fragmento",
    archivos: "Archivos recibidos",
    "archivo-renombrado": "Archivo renombrado",
    "archivo-fallo": "Archivo que no se guardó",
    "aviso-info": "Un aviso",
    "varios-avisos": "Varios avisos",
    "aviso-privado": "Mensaje privado",
    mano: "Mano levantada",
    mensajes: "Mensajes y respuesta",
    temporizador: "Temporizador",
    "pantalla-vista": "Pantalla vista",
    control: "Controlado",
    pausa: "Pantalla en pausa",
    restricciones: "Restricciones",
    "energia-cuenta": "Cuenta para reiniciar",
    transmision: "Transmisión del profesor",
    "transmision-alumno": "Viendo la pantalla de un compañero",
    mostrando: "Tu pantalla se está mostrando",
    todo: "Todas las bandas",
    buscando: "Buscando la clase",
    "sin-conexion": "Sin conexión",
    "sin-configurar": "Sin configurar",
  },
};

// Las vistas de la consola, para ir a ellas desde el anotador (ruta = hash).
export const rutas = { alumnos: "", codigo: "codigo", archivos: "archivos", participacion: "participacion", simulada: "simulada",
  ayuda: "ayuda", falta: "falta" };
