# ADR-0002: licencia, atribución y marca

[English](0002-licencia-y-atribucion.md)

- Estado: aceptado para diseño y primera publicación
- Fecha: 2026-09-05
- Decisión: publicar ModelCairn bajo Apache License 2.0

## Contexto

ModelCairn está destinado a ser un proyecto open source que permita uso,
modificación, redistribución, integración y prestación de servicios. El mantenedor
quiere favorecer adopción y contribuciones, conservar el reconocimiento del origen
del proyecto y permitir patrocinio voluntario.

Se distinguieron cuatro conceptos que no deben confundirse:

- **copyright:** reconoce la autoría y los derechos sobre el trabajo original;
- **licencia:** concede permisos y establece condiciones para usar el trabajo;
- **marca:** identifica el proyecto oficial y evita presentaciones engañosas;
- **patrocinio:** apoyo económico voluntario, independiente de la licencia.

## Decisión

La primera publicación utilizará **Apache License 2.0** con identificador SPDX
`Apache-2.0`. Los archivos del repositorio quedarán bajo esa licencia salvo que un
archivo o directorio indique explícitamente otra condición. El tratamiento de
logotipos y otros activos de marca se definirá por separado antes de publicarlos.

La distribución incluirá:

- `LICENSE` con el texto completo y sin modificar de Apache-2.0;
- `NOTICE` con la atribución breve del proyecto y la ubicación del repositorio
  oficial;
- avisos SPDX en archivos cuando la política de contribución lo requiera;
- una vista “Acerca de” con versión, licencia, repositorio, creador original y
  contribuidores;
- una atribución discreta en la interfaz oficial;
- una política de marca separada antes de la primera release estable.

Texto de trabajo para `NOTICE`:

```text
ModelCairn
Copyright 2026 sxamx

Originally developed by sxamx.
Official repository: https://github.com/sxamx/modelcairn
```

La identidad pública inicial del creador será `sxamx`. Podrá complementarse con un
nombre personal o entidad jurídica únicamente mediante una decisión explícita.

## Alcance de la atribución

Apache-2.0 no obliga a todo proyecto a crear un archivo `NOTICE`. ModelCairn decide
incluirlo; por ello, quienes redistribuyan obras derivadas deberán reproducir de
forma legible sus atribuciones aplicables según la sección 4 de la licencia. No
obliga a mostrar permanentemente el nombre del creador en la interfaz de cada fork.

La atribución visible en el pie y en “Acerca de” forma parte del diseño oficial de
ModelCairn. Los forks podrán modificar la interfaz dentro de los permisos de la
licencia, pero deberán cumplir las obligaciones de licencia y atribución en su
distribución.

La futura política de marca deberá aclarar qué usos del nombre y logotipo se
permiten. La licencia de código no concede por sí sola permiso para presentar una
versión modificada como publicación oficial del proyecto.

## Patrocinio

El patrocinio no es obligatorio ni una condición de uso. El proyecto podrá incluir
un archivo `.github/FUNDING.yml` y enlaces voluntarios desde GitHub y la vista
“Acerca de”. Ningún patrocinador adquiere propiedad o control del proyecto por el
solo hecho de aportar fondos.

## Cambio futuro de licencia

Una versión ya publicada conserva los permisos de la licencia con la que fue
distribuida. Cambiar la licencia de versiones futuras requiere controlar los
derechos necesarios sobre todo el código afectado.

Cuando existan contribuciones externas, un cambio podría requerir autorización de
sus titulares, un acuerdo de contribución aplicable o reemplazar los aportes que no
puedan relicenciarse. Por ello no se adoptará CPAL como paso temporal con la idea
de cambiar automáticamente a Apache más adelante.

La política de contribución deberá definir el modelo de licencia de entrada antes
de aceptar código externo significativo.

## Alternativas consideradas

- **CPAL-1.0:** permite exigir atribución limitada en una interfaz y cubre
  despliegues por red. Se descartó por ser menos conocida y por introducir más
  fricción de adopción que la requerida por el objetivo final.
- **MIT:** sencilla y ampliamente adoptada, pero Apache-2.0 ofrece términos más
  explícitos sobre patentes y redistribución.
- **AGPL-3.0:** exigiría publicar modificaciones utilizadas para prestar un
  servicio por red. Se descartó porque el mantenedor prioriza adopción y no exige
  que todas las modificaciones operadas como servicio se publiquen.

## Consecuencias

- Terceros podrán crear y distribuir forks, también con fines comerciales, si
  cumplen Apache-2.0.
- Un fork puede cambiar la interfaz y no está obligado por Apache-2.0 a conservar
  un crédito permanente en pantalla.
- Los avisos aplicables deberán conservarse según la licencia.
- El nombre y el logotipo necesitarán una política propia.
- La primera publicación contiene `LICENSE` y `NOTICE`; ambos se incorporaron al
  preparar el repositorio limpio.

## Revisión

Esta decisión debe revisarse antes de aceptar contribuciones externas si cambia el
objetivo de adopción, el modelo de gobernanza o la necesidad de copyleft. Para una
aplicación jurídica concreta se deberá obtener asesoría profesional.
