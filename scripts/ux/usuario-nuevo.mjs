// Prueba del usuario nuevo en el arnés (prompt 18, Fase 7): sigue «Primeros
// pasos» del manual al pie de la letra, con el aula simulada. En cada paso
// busca en la consola el control con la etiqueta exacta que nombra el manual
// (si el manual y la interfaz no coinciden, falla), lo usa y cuenta clics y
// teclas. Los dobles son fijos por escena, así que cada paso abre la escena en
// la que queda la consola tras el anterior.
//   node usuario-nuevo.mjs
import fs from "node:fs";
import path from "node:path";
import { abrir, raiz, sleep } from "./arnes.mjs";

const manual = fs.readFileSync(path.join(raiz, "docs", "v1", "administracion", "primeros-pasos.md"), "utf8");
const { foto, cerrar } = await abrir();
const fallas = [];
const pasos = [];
let clics = 0, teclas = 0;

// Lo que el manual pone en negritas o entre comillas debe existir en la pantalla.
const nombrado = (etiqueta) => {
  if (!manual.includes(etiqueta)) fallas.push(`el manual ya no nombra «${etiqueta}»: actualiza esta prueba`);
};
// Pulsa el control visible cuyo texto es exactamente etiqueta.
const pulsar = (etiqueta) => async (p) => {
  nombrado(etiqueta);
  const ok = await p.evaluate((t) => {
    const c = [...document.querySelectorAll("button, a, [role=tab]")]
      .find((e) => e.textContent.trim() === t && e.getClientRects().length && !e.closest("[hidden]"));
    if (!c) return false;
    c.click();
    return true;
  }, etiqueta);
  if (!ok) { fallas.push(`no hay un control visible «${etiqueta}»`); return; }
  clics++;
  await sleep(250);
};
// Comprueba que un texto del manual se ve en la pantalla.
// delManual: false para lo que el manual describe sin nombrarlo («dos alumnos de prueba»).
const ver = (etiqueta, delManual = true) => async (p) => {
  if (delManual) nombrado(etiqueta);
  const ok = await p.evaluate((t) => [...document.querySelectorAll("body *")]
    .some((e) => e.children.length === 0 && e.textContent.includes(t) && e.getClientRects().length && !e.closest("[hidden]")), etiqueta);
  if (!ok) fallas.push(`no se ve «${etiqueta}»`);
};
const paso = async (titulo, escena, ruta, acciones) => {
  const antes = clics + teclas;
  await foto(path.join("salida-usuario", String(pasos.length + 1).padStart(2, "0") + ".webp"), escena, { ruta, revisar: false,
    pasos: async (p) => { for (const a of acciones) await a(p); } });
  pasos.push({ titulo, acciones: clics + teclas - antes });
};

// 1. Abrir tu primera clase: se abre sola; el canal se ve en la tarjeta del menú.
await paso("La clase ya está abierta", "vacia", "", [ver("Clase abierta · canal")]);
// 2. Ver a tus alumnos: sin nadie, «Probar con alumnos simulados».
await paso("Abrir el aula simulada", "vacia", "", [ver("Alumnos"), pulsar("Probar con alumnos simulados")]);
// 3. Los simulados aparecen en Alumnos.
await paso("Ver a los alumnos simulados", "simulada", "", [ver("Simulado 1", false), ver("Simulado 2", false)]);
// 4. Enviar tu primer código: Código, escribir, Emitir.
const codigo = "print('Hola')";
await paso("Escribir y emitir el primer código", "simulada", "", [
  pulsar("Código"),
  async (p) => {
    await p.evaluate(() => window.monaco && window.monaco.editor.getEditors()[0].focus());
    await p.keyboard.type(codigo);
    teclas += 1; // escribir el código cuenta como una acción
    await sleep(200);
  },
  ver("Pegar del portapapeles"),
  pulsar("Emitir (reemplaza lo que ven)"),
]);
// 5. Ver quién lo recibió: Últimos envíos, «Quién falta», «Reenviar a los que faltan».
await paso("Ver quién lo recibió y reenviar", "simulada", "codigo", [
  ver("Últimos envíos"),
  async (p) => {
    nombrado("Quién falta");
    const ok = await p.evaluate(() => {
      const b = [...document.querySelectorAll(".envio__desplegar")].find((e) => e.textContent.includes("Quién falta"));
      if (b) b.click();
      return !!b;
    });
    if (!ok) fallas.push("no hay «Quién falta» en un envío");
    else clics++;
    await sleep(250);
  },
  ver("Reenviar a los que faltan"),
]);

await cerrar();
console.log("| # | Paso | Acciones |\n|---|---|---|");
pasos.forEach((x, i) => console.log(`| ${i + 1} | ${x.titulo} | ${x.acciones} |`));
console.log(`\nTotal: ${clics} clics y ${teclas} escritura (sin contar leer).`);
if (fallas.length) {
  fallas.forEach((l) => console.error("✗ " + l));
  process.exit(1);
}
console.log("El manual y la consola coinciden en cada paso.");
