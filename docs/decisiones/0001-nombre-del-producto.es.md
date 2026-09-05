# ADR-0001: nombre del producto

[English](0001-nombre-del-producto.md)

- Estado: aceptado para diseño y desarrollo
- Fecha: 2026-09-04
- Decisión: adoptar **ModelCairn** como nombre del producto

## Contexto

El prototipo anterior recibió el nombre Aegis Gateway antes de que la visión del
producto estuviera definida. Ese nombre coincide con otros gateways y proyectos
de seguridad relacionados con IA, lo que dificulta distinguir el proyecto en
búsquedas, documentación y conversaciones técnicas.

El producto necesita un nombre que pueda representar el gateway completo, no
solo el fallback o el aprendizaje de cuotas. Debe admitir una identidad clara
para el panel, la malla de nodos y el estimador adaptativo.

## Decisión

El producto se llamará **ModelCairn** durante el diseño y desarrollo.

Un *cairn* es un conjunto de piedras colocado para marcar una ruta. La metáfora
describe el comportamiento central del sistema: cada observación aporta evidencia
sobre capacidad, fallos y recuperación; esa evidencia ayuda a elegir una ruta
viable según las políticas del operador.

Nombre descriptivo:

> ModelCairn — Adaptive AI Gateway

Mensaje inicial:

> Adaptive routing for the AI providers you control.

Descripción breve:

> ModelCairn is a lightweight, self-hosted AI gateway that observes provider
> capacity and recovery patterns, then routes requests through paths defined by
> the operator.

Estos textos son una base de trabajo. El documento de visión definirá la promesa
exacta antes de utilizarlos como comunicación pública.

## Nombres funcionales provisionales

- **Gateway:** plano de datos, compatibilidad de API y routing.
- **Studio:** consola web y constructor visual de estrategias.
- **Mesh:** comunicación y operación entre nodos propios.
- **Capacity Estimator:** estimador adaptativo de capacidad y recuperación.

Estos términos describen límites conceptuales y no son submarcas. Tampoco obligan
a crear procesos, paquetes o repositorios separados.

## Comprobación preliminar

El 4 de septiembre de 2026 se realizó una búsqueda exacta del nombre:

- GitHub: ningún repositorio coincidente por nombre.
- npm: ningún paquete exacto publicado.
- PyPI: ningún proyecto exacto publicado.
- crates.io: ningún crate exacto publicado.
- Búsqueda web general: no se encontró un producto tecnológico relevante con el
  nombre exacto.

Esta comprobación reduce el riesgo de confusión técnica, pero no constituye una
búsqueda jurídica de marcas ni garantiza dominios o identificadores sociales.

Una revisión independiente encontró que la raíz `Cairn` tiene numerosos usos en
software e IA. También señaló que `Model` puede reducir la percepción del producto
a los modelos, aunque el sistema administra proveedores, nodos, rutas y capacidad.

La objeción se acepta como riesgo conocido, pero no bloquea la decisión. El
compuesto exacto no presentó colisiones en los registros técnicos revisados, la
metáfora representa el comportamiento central y el nombre funciona en una
audiencia técnica internacional. Dar a la ocupación parcial de la raíz más peso
que al compuesto exacto empujaría la marca hacia términos artificiales con menor
claridad. La descripción pública
deberá explicar que ModelCairn dirige tráfico entre proveedores e infraestructura,
no que administra únicamente modelos.

## Consecuencias

- La documentación nueva utilizará ModelCairn.
- El prototipo previo puede conservar referencias históricas a Aegis Gateway; no
  se migrarán automáticamente.
- Antes del primer commit público se debe revisar que los documentos incluidos
  usen el nombre acordado de manera consistente.
- Antes de la primera release se deben revisar marcas, dominios, registros de
  paquetes e identificadores relevantes para los países y canales elegidos.
- El repositorio remoto deberá cambiar su slug temporal a `modelcairn` antes del
  primer commit público.

## Alternativas consideradas

- **Aegis Gateway:** descartado por colisiones directas con proyectos similares.
- **Tidepath:** descartado por uso activo en una empresa de automatización e IA.
- **SteadyRelay:** demasiado genérico y cercano al lenguaje de electrónica.
- **CapacityWeave:** limita la percepción del producto a capacidad y se aproxima
  conceptualmente a otras marcas de infraestructura.
- **AvailRoute:** preciso, pero difícil de convertir en una identidad memorable.
- **Navifold:** buena metáfora de distribución, descartada al encontrarse un uso
  exacto y reciente en una herramienta de mapeo químico.
- **RouteBraid:** metáfora clara de rutas trenzadas, pero más descriptivo y con la
  raíz `Braid` muy utilizada en software.
- **Trenzavia:** distintivo y expresivo en español; se mantiene como alternativa
  de reserva, con menor claridad inmediata para una audiencia internacional.

## Revisión

La decisión debe reabrirse si aparece una colisión material antes de la primera
release o si la visión aprobada demuestra que la metáfora ya no representa el
producto.
