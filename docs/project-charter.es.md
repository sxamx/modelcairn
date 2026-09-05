# Project Charter de ModelCairn

[English](project-charter.md)

- Estado: aprobado como base de la Fase 1; evolucionará mediante ADR y fases
- Fecha de apertura: 2026-09-05
- Etapa: descubrimiento y definición
- Audiencia de este documento: mantenedores, contribuidores y revisores del proyecto

## Cómo usar este documento

Este Project Charter, o acta de constitución del proyecto, define por qué existe
ModelCairn y qué resultado pretende conseguir. Todavía no define la solución
técnica completa. Las afirmaciones confirmadas forman la base del proyecto; las
propuestas requieren revisión del propietario; las preguntas abiertas no deben
convertirse en requisitos ni tareas de implementación.

## Resumen

ModelCairn es un proyecto destinado a publicarse como open source para construir un gateway de proveedores de
IA ligero, self-hosted y configurable. Se ubica entre clientes o agentes y los
proveedores, aplica estrategias de routing y fallback definidas por el operador,
observa la capacidad y recuperación de cada ruta y ofrece una consola web para
administrar el sistema.

El proyecto busca que los clientes dependan de un endpoint estable aunque cambien
los proveedores, modelos o condiciones operativas disponibles. Las decisiones de
routing deben ser visibles, explicables y compatibles con las políticas y cuentas
que el operador esté autorizado a utilizar.

## Problema

Los proveedores y modelos de IA cambian con frecuencia. Una integración directa
obliga a cada cliente a conocer credenciales, endpoints, disponibilidad, límites,
errores y reemplazos. Los gateways existentes pueden consumir demasiados recursos,
incluir funciones innecesarias para instalaciones pequeñas o limitar la forma en
que el operador define rutas y fallback.

ModelCairn pretende concentrar esa complejidad en una capa operable desde archivos
y desde una interfaz web, capaz de ejecutarse en infraestructura pequeña y crecer
hacia varios nodos sin perder trazabilidad.

## Visión confirmada

- Proyecto destinado a publicarse en GitHub con una licencia open source.
- Gateway intermedio entre clientes de IA y proveedores autorizados.
- API estable para evitar reconfigurar cada cliente cuando cambia una ruta.
- Proveedores, modelos, credenciales, ubicaciones de ejecución y egresos
  autorizados configurables.
- Custodia centralizada de credenciales en el nodo principal. Los nodos de salida
  retransmiten solicitudes sin persistir las API keys y cada credencial mantiene
  una afinidad estable con un egreso autorizado.
- Routing y fallback configurables, incluida una representación visual avanzada.
- Identificación de clientes o agentes para aplicar políticas explícitas.
- Distribución consciente de solicitudes concurrentes según capacidad y política.
- Estimación adaptativa de capacidad y recuperación basada en observaciones.
- Configuración operativa gestionable mediante un esquema común desde archivos y
  consola web. Los secretos, el bootstrap y la recuperación tendrán límites
  explícitos para evitar exposición o fuentes de verdad contradictorias.
- Arquitectura ligera con una instalación objetivo en VM de 1 GB de RAM.
- Compatibilidad con redes privadas como Tailscale sin depender exclusivamente de
  una tecnología de red.
- Desarrollo dividido en fases completas, documentadas y verificables.
- QA independiente en entregas relevantes para reducir el sesgo del implementador.
- Comunicación pública precisa y atractiva para facilitar adopción, contribuciones
  y patrocinio del proyecto.

## Principios confirmados

### Documentación antes de implementación

Las decisiones de producto y arquitectura se documentan antes de desarrollar la
fase que depende de ellas. La documentación debe reducir ambigüedad, explicar las
consecuencias y permitir corregir el rumbo temprano.

### Fases completas

Cada fase tendrá alcance, criterios de aceptación, pruebas y documentación. Una
fase no se considera terminada por contener una demostración parcial o una
interfaz visual.

### Control del operador

El operador define proveedores, rutas, prioridades, fallbacks, afinidades y
políticas. Las decisiones automáticas deben poder explicarse y respetar límites
configurados.

### Eficiencia medible

El objetivo de funcionar en una VM de 1 GB se validará con mediciones y pruebas de
carga reproducibles. La elección de tecnologías deberá justificarse contra ese
presupuesto.

### Extensibilidad

Agregar proveedores, modelos, nodos o estrategias no debería exigir reescribir el
núcleo. Los contratos de extensión formarán parte de la documentación pública.

### Privacidad y seguridad

Credenciales, tokens, rutas locales y datos sensibles no deben aparecer en
documentación, logs o exportaciones públicas. La seguridad se diseña como parte de
cada fase y no como una corrección final.

El nivel objetivo será proporcional a una aplicación administrativa autohospedada,
no el de una infraestructura militar o de alta clasificación. La prioridad será
proteger las API keys; después, impedir accesos no autorizados a la consola y
proteger en tránsito la comunicación entre el nodo principal, los relays y los
proveedores. El diseño incluirá cifrado en tránsito, protección de secretos en
reposo, redacción de logs, autenticación entre nodos y copias de seguridad seguras,
sin añadir complejidad que no responda a un riesgo real del proyecto.

### Autohospedaje sin telemetría externa

Cada instalación pertenece a quien la opera y conserva localmente su configuración
y datos operativos. ModelCairn no enviará telemetría, analíticas, prompts,
respuestas ni identificadores al mantenedor o a terceros. Las métricas locales
necesarias para routing, diagnóstico y estimación de capacidad no implican un
servicio central de ModelCairn.

### Documentación bilingüe

La discusión y primera redacción se realizarán en español para facilitar la
revisión del mantenedor. Después de aprobar cada documento, se mantendrá una
versión pública canónica en inglés y una traducción oficial en español. El proceso
de documentación deberá definir cómo detectar y evitar divergencias.

## Stakeholders identificados

- **Mantenedor principal durante la etapa inicial:** facilita la visión y aprueba
  decisiones de alcance mientras se define la gobernanza futura.
- **Operador:** instala ModelCairn, configura infraestructura, proveedores y
  políticas, y responde a incidentes.
- **Usuario de clientes o agentes:** utiliza el endpoint del gateway y espera un
  comportamiento estable y comprensible.
- **Contribuidor:** estudia la documentación, propone cambios e implementa una
  parte sin necesitar reconstruir decisiones desde conversaciones privadas.
- **Revisor:** evalúa calidad, seguridad, rendimiento, diseño o comunicación con
  criterio independiente.

## Usuario prioritario y experiencia

La experiencia inicial se diseñará para una persona con conocimientos técnicos
básicos o intermedios que comprende conceptos como API, proveedor, modelo y VM,
pero no debería necesitar experiencia avanzada en administración de sistemas.

El conocimiento técnico del usuario no justifica una interfaz compleja. La consola
debe aplicar **divulgación progresiva**: mostrar primero las decisiones habituales,
explicar sus efectos y mantener los controles avanzados disponibles sin imponerlos
al flujo básico.

El sistema también deberá ser útil para operadores expertos y evolucionar hacia
pequeños equipos, pero esas necesidades no deben degradar la instalación personal
ni convertir la interfaz básica en un panel hostil.

## Compatibilidad inicial

La primera superficie compatible será `POST /v1/chat/completions`, incluyendo los
comportamientos necesarios de streaming y llamadas a herramientas que soporte cada
ruta. Esta prioridad fija el orden de implementación, no limita la visión completa.

Cada ruta lógica podrá exponer un alias de modelo distinto del identificador real
del proveedor. Los alias forman parte del routing general y no implican por sí
solos compatibilidad con otro protocolo.

La arquitectura deberá permitir incorporar otros protocolos mediante adaptadores.
Entre los casos futuros se incluye una interfaz compatible con Anthropic para
clientes como Claude Code. La compatibilidad no se considerará completa si una
traducción pierde silenciosamente herramientas, reasoning, contenido o semántica
requerida por el cliente.

Responses API, Anthropic Messages y otros protocolos se evaluarán en su propio
contrato de compatibilidad antes de asignarlos a una fase.

## Instalación y onboarding

La experiencia objetivo comienza con un comando de instalación. Después, un
asistente interactivo en la terminal realiza únicamente el bootstrap necesario:

- comprobar sistema operativo, arquitectura y recursos;
- instalar o validar el runtime requerido;
- crear el servicio y el directorio de datos;
- crear la cuenta o contraseña administrativa inicial;
- elegir cómo se accederá a la consola;
- iniciar ModelCairn y mostrar la URL y los siguientes pasos.

La configuración operativa de proveedores, modelos, credenciales y estrategias se
realizará después desde la consola web o mediante archivos. El asistente no debe
pedir API keys innecesariamente ni obligar a reinstalar para cambiar la
configuración.

La ruta principal será un ejecutable Go administrado por systemd. El instalador
preguntará si debe iniciarse automáticamente al reiniciar, recomendando habilitarlo.
Se publicarán artefactos Linux AMD64 y ARM64. Docker se incorporará como alternativa
después de validar la instalación nativa y sus diferencias operativas.

## Acceso web y PWA

La consola será una Progressive Web App (PWA) instalable en dispositivos móviles y
de escritorio. El instalador ofrecerá modos de acceso comprensibles:

- solo en la máquina local;
- red privada o LAN;
- tailnet privada mediante Tailscale Serve y HTTPS;
- exposición pública mediante un reverse proxy HTTPS administrado por el usuario.

La integración con Tailscale es opcional. ModelCairn puede detectar su
disponibilidad y generar instrucciones, pero no depende de Tailscale ni debe hacer
pública una instancia privada. Tailscale Funnel no será el mecanismo recomendado
para una consola administrativa.

## Restricciones conocidas

- Despliegue objetivo inicial en Linux sobre una VM con 1 GB de RAM.
- Posibilidad de operar al menos dos nodos propios y crecer a más nodos.
- Consola web servida con un presupuesto de recursos reducido.
- Configuración comprensible para una instalación personal, pero extensible a
  escenarios más complejos.
- El repositorio público nuevo debe comenzar con un historial limpio y
  documentación revisada.
- Las fases pueden desarrollarse rápidamente, pero ninguna puede omitir sus
  pruebas, documentación o criterios de aceptación.

## Hipótesis que debemos validar

- Una API compatible con OpenAI cubre la integración inicial de la mayoría de los
  clientes objetivo.
- Un proceso ligero con persistencia embebida puede cumplir el presupuesto de una
  VM de 1 GB junto con el sistema operativo y servicios básicos.
- La configuración gestionada desde web y archivos puede compartir un esquema sin
  producir dos fuentes de verdad, manteniendo secretos referenciados y parámetros
  de bootstrap fuera de las superficies que no correspondan.
- La capacidad y recuperación pueden estimarse de forma útil sin interpretar unas
  pocas observaciones como reglas permanentes.
- Un nodo principal puede custodiar los secretos y coordinar nodos de salida
  ligeros sin que estos persistan credenciales ni contenido de las solicitudes.

## Resultados que definirán el éxito

Las métricas numéricas se acordarán durante los requisitos no funcionales. Como
resultado general, ModelCairn será exitoso cuando:

- un operador pueda instalarlo y configurar una ruta sin modificar código;
- un operador pueda cambiar la ruta detrás de un identificador lógico compatible
  sin reconfigurar el cliente;
- un fallo permitido produzca un fallback trazable dentro del presupuesto de
  tiempo e intentos;
- el estimador comunique capacidad, recuperación e incertidumbre sin presentar
  conjeturas como límites confirmados;
- una instalación de referencia funcione dentro del presupuesto real de la VM;
- un contribuidor pueda comprender arquitectura, contratos y proceso de entrega a
  partir del repositorio;
- cada decisión automática importante pueda explicarse desde la consola y los
  registros permitidos.

## Métricas locales y retención

Las métricas operativas son una función central: alimentan el estimador adaptativo,
explican el routing y permiten analizar el comportamiento histórico. Permanecen en
la instalación del operador y no se envían al proyecto.

La política inicial propuesta distingue los datos originales de sus vistas
derivadas:

- eventos detallados con retención predeterminada de 30 días;
- estadísticas históricas calculadas a partir de esos eventos para mostrar, entre
  otros datos, latencia, tokens, tokens por segundo, errores, disponibilidad y
  comportamiento por modelo, proveedor y ruta.

Las estadísticas agregadas no sustituyen ni obligan a eliminar los eventos. El
operador podrá conservar eventos detallados indefinidamente. La interfaz deberá
estimar el crecimiento de disco, advertir antes de agotar espacio y permitir
exportar, compactar o eliminar datos de forma controlada. La retención
predeterminada propuesta es 30 días, con una opción explícita para no eliminar
nunca. Los periodos exactos y la granularidad se cerrarán como requisitos no
funcionales.

## Topología, salud y afinidad

La topología inicial será de **control centralizado**:

- el **nodo principal** ejecuta el gateway, la consola, la persistencia, el
  estimador, el planificador y la custodia de todas las API keys;
- el nodo principal también puede actuar como salida directa a Internet;
- los **nodos de salida** son relays ligeros: transportan conexiones autorizadas
  del nodo principal hacia el proveedor mediante su propio egreso;
- los nodos de salida no constituyen una segunda instalación completa, no toman
  decisiones de routing y no guardan API keys de forma persistente.

El diseño preferirá un túnel de egreso en el que el cifrado TLS termine en el
proveedor, no en el relay. De ese modo el nodo de salida transporta tráfico, pero
no recibe en texto legible la API key, el prompt ni la respuesta. Si una futura
integración necesitara terminar TLS o reconstruir peticiones en el relay, se
considerará un modo distinto y deberá pasar una revisión de seguridad explícita.

Esta decisión concentra la superficie de administración y los secretos, pero
convierte al nodo principal en un componente crítico. La arquitectura deberá
proteger el canal entre nodos, autenticar ambos extremos, evitar que un relay
acepte tráfico arbitrario y establecer qué ocurre si el nodo principal o un relay
dejan de estar disponibles.

La consola incluirá una vista de los nodos registrados. Para cada nodo deberá
mostrar, como mínimo:

- estado de conectividad y última comprobación;
- versión de ModelCairn;
- recursos declarados y observados cuando estén disponibles;
- solicitudes activas y carga reciente;
- proveedores configurados;
- cantidad de credenciales asignadas, sin revelar sus secretos;
- rutas y modelos que puede atender;
- estado de mantenimiento y motivo de indisponibilidad.

Una credencial pertenecerá al almacén de secretos del nodo principal y tendrá
afinidad estable con un egreso autorizado. El planificador no cambiará esa
asignación automáticamente. Una
reasignación requerirá una acción explícita del operador, comprobación de
dependencias, registro de auditoría y una advertencia de que cambiarán las
condiciones operativas y posiblemente la identidad de red observada por el
proveedor.

La afinidad no implica que nodo, proxy y egreso sean el mismo objeto. Esa relación
se definirá en el modelo de dominio y deberá admitir instalaciones sin proxy,
proxies compartidos y redes privadas como Tailscale.

## Propuesta de límites del proyecto

Los siguientes límites son una propuesta para discusión, no una decisión tomada:

- ModelCairn administra routing de solicitudes de IA; no pretende convertirse en
  una plataforma general de agentes.
- No aloja modelos ni ofrece inferencia propia.
- No intenta reemplazar observabilidad general, gestores de secretos o redes
  privadas externas; se integra con ellos mediante contratos simples.
- No garantiza disponibilidad cuando no existe ninguna ruta autorizada y sana.
- No presenta inferencias estadísticas como cuotas oficiales del proveedor.
- La consola administra ModelCairn; no pretende ser un panel universal para todas
  las funciones de cada proveedor.

## Validación prevista de hipótesis

Esta tabla propone cómo evitar que los supuestos se conviertan en decisiones sin
evidencia. Los responsables y umbrales concretos se fijarán durante la
planificación.

| Hipótesis | Evidencia prevista | Etapa |
|---|---|---|
| Compatibilidad inicial suficiente | Matriz de clientes y pruebas de contrato | Requisitos y arquitectura |
| Operación dentro de 1 GB | Perfil de memoria y carga en una VM representativa | Validación técnica |
| Esquema común para archivo y web | Prototipo de esquema, round-trip y casos de secretos/bootstrap | Arquitectura |
| Estimación adaptativa útil | Simulaciones, datos sintéticos y proveedores de prueba | Diseño estadístico |
| Secretos centralizados y relays sin persistencia | Threat model, inspección del relay y prueba multinodo | Seguridad y arquitectura |

## Decisiones resueltas y postergadas deliberadamente

Las decisiones necesarias para comenzar la Fase 1 están resueltas a continuación.
Los elementos postergados explícitamente pertenecen a hitos posteriores y no
reabren este charter.

### Usuarios prioritarios

Decisión tomada: la experiencia inicial prioriza a una persona con conocimientos
técnicos básicos o intermedios y una VM propia. La interfaz seguirá siendo
intuitiva y aplicará divulgación progresiva. Operadores expertos y pequeños equipos
forman parte de la evolución prevista.

### Superficie inicial de compatibilidad

Decisión tomada: OpenAI Chat Completions será la primera interfaz. La arquitectura
permitirá adaptadores posteriores; la compatibilidad Anthropic para clientes como
Claude Code queda como caso explícito para el futuro contrato de protocolos.

### Modelo de despliegue

Decisión tomada: ejecutable Go y systemd como ruta principal, con inicio automático
elegible durante el bootstrap. Se soportarán Linux AMD64 y ARM64; Docker será una
alternativa posterior, no una dependencia de la primera instalación.

### Exposición de red

Decisión tomada: la configuración inicial favorecerá localhost o una red privada.
Se soportarán Tailscale Serve para HTTPS privado y exposición pública detrás de un
reverse proxy HTTPS, sin activar acceso público automáticamente.

### Idioma de documentación

Decisión tomada: inglés como versión pública canónica y español como traducción
oficial, trabajando primero los borradores en español. Ambas versiones se revisan
juntas para la base inicial; una comprobación automática de divergencia queda
postergada.

### Licencia y gobernanza

Decisión tomada: [ADR-0002 adopta Apache-2.0](decisiones/0002-licencia-y-atribucion.es.md) con `NOTICE`, atribución en la interfaz oficial y una futura política de marca. El flujo inicial de contribución y la autoridad de decisión están definidos en [gestión del proyecto](governance/project-management.es.md); la gobernanza comunitaria madura queda postergada hasta que la base de contribuidores la requiera.

### Telemetría y contenido

Decisión tomada: no habrá telemetría externa ni servicio central de recopilación.
Por defecto no se almacenarán prompts ni respuestas: se conservarán solo métricas
y metadatos operativos que no necesiten reproducir el contenido. Una función
futura podrá permitir almacenamiento local explícito por política, pero deberá
definir antes finalidad, advertencias de privacidad, cifrado, redacción,
retención, acceso y eliminación. El estimador adaptativo se diseñará para operar
sin leer ni conservar el contenido de los mensajes.

### Configuración, secretos y recuperación

Decisión tomada: SQLite será la fuente de verdad; YAML permitirá aplicar y exportar
configuración mediante el mismo esquema usado por web y API. Las API keys se
cifrarán con una clave maestra local. Habrá exportación sin secretos y backup
completo cifrado con contraseña independiente, cuya restauración deberá recuperar
una ruta funcional. La contraseña administrativa se restablecerá desde la CLI local.

### Nodos, proxies y egreso

Decisión tomada: el nodo principal aloja la aplicación completa, guarda todas las
credenciales y también puede proporcionar un egreso. Los nodos auxiliares serán
relays de salida ligeros, no instalaciones completas ni almacenes persistentes de
secretos. Cada credencial queda vinculada a un egreso y solo el operador puede
cambiar esa afinidad. El protocolo entre nodos, su autenticación, cifrado,
comportamiento ante fallos y presupuesto de recursos quedan postergados hasta el
hito de relays y deberán contratarse antes de implementarlos.

### Compatibilidad y autoridad del proveedor

La superficie de la Fase 1 se rige por la matriz de compatibilidad de Chat
Completions. La compatibilidad de protocolos posteriores y la representación de
límites y términos publicados por proveedores quedan postergadas a sus hitos. La
información oficial y las instrucciones explícitas del proveedor prevalecen sobre
las estimaciones observadas; antes de implementar routing adaptativo deberá
contratarse una política detallada de conflictos.

## Criterio de aprobación

Este charter fue aceptado para la Fase 1 después de la revisión del mantenedor sobre
la visión y los límites, la definición de métodos de validación para sus hipótesis y
una revisión independiente sin hallazgos bloqueantes pendientes. Un ADR o contrato
de fase posterior podrá evolucionarlo sin reescribir esta aprobación histórica.
