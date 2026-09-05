# Glosario y modelo de dominio de ModelCairn

- Estado: borrador para revisión conjunta
- Etapa: descubrimiento y definición
- Alcance: vocabulario común y relaciones conceptuales; no define todavía tablas ni clases

## Propósito

Este documento evita que una misma palabra represente conceptos distintos durante
el diseño, la implementación y las conversaciones del proyecto. Cuando la
interfaz use una palabra más sencilla, la documentación técnica deberá relacionarla
con el término de dominio correspondiente.

## Actores

### Operador

Persona que instala y administra una instancia de ModelCairn. Controla sus datos,
proveedores, credenciales, rutas, relays y políticas.

### Administrador

Identidad humana con permisos completos dentro de una instalación. En la primera
fase podrá coincidir con el operador; el modelo permite incorporar más usuarios y
roles posteriormente.

### Cliente

Programa que consume la API estable de ModelCairn, por ejemplo un agente, una
aplicación o una herramienta de desarrollo.

### Agente

Identidad lógica atribuida a un cliente o conjunto de solicitudes. Permite aplicar
políticas, observar actividad y distribuir concurrencia sin depender solamente de
la dirección IP.

## Infraestructura

### Instalación

Conjunto autohospedado administrado por un operador. No depende de un servicio
central de ModelCairn ni envía telemetría al proyecto.

### Nodo principal

Máquina que ejecuta el plano de control y el plano de datos: API, router, consola,
persistencia, secretos, métricas y estimador. También puede proporcionar un egreso
directo. Es el único custodio persistente de las API keys en la topología inicial.

### Relay de salida

Servicio auxiliar ligero que proporciona un egreso alternativo. Transporta
conexiones autorizadas desde el nodo principal sin decidir rutas ni persistir API
keys, prompts o respuestas. No es una segunda instalación completa.

### Egreso

Camino e identidad de red por los que una conexión llega al proveedor. Puede ser
el egreso directo del nodo principal o el de un relay.

### Proxy

Término general para un intermediario de tráfico. En ModelCairn se preferirá
**relay de salida** cuando se hable del componente propio, porque “proxy” no indica
si termina TLS, toma decisiones o almacena información.

### Plano de control

Funciones de administración: consola web, configuración, secretos, publicación de
estrategias, auditoría y estado de nodos.

### Plano de datos

Camino utilizado por las solicitudes reales: recepción, autenticación del cliente,
selección de ruta, llamada al proveedor, streaming, fallback y respuesta.

## Proveedores y acceso

### Proveedor

Servicio externo autorizado que ofrece modelos mediante una API. Una cuenta puede
tener una o más credenciales y límites distintos.

### Conexión de proveedor

Configuración versionada de un endpoint concreto: proveedor, URL base, adaptador,
opciones de red y capacidades declaradas. Dos endpoints del mismo proveedor no se
consideran automáticamente equivalentes.

### Adaptador de protocolo

Componente que implementa un contrato de entrada o salida, por ejemplo OpenAI Chat
Completions o Anthropic Messages. Traducir entre protocolos requiere un adaptador;
cambiar el nombre del modelo no realiza esa traducción.

### Credencial

Referencia administrada a un secreto que autoriza llamadas a un proveedor. La
interfaz puede mostrar su etiqueta, estado y últimos caracteres permitidos, pero
nunca recuperar el valor completo después de guardarlo.

### API key

Tipo frecuente de secreto utilizado como credencial. No todas las credenciales
futuras tienen que ser API keys; podrían existir tokens temporales u otros métodos.

### Afinidad de egreso

Asignación persistente entre una credencial y un egreso autorizado. No cambia
automáticamente. Una modificación manual debe advertir sus consecuencias y quedar
registrada.

### Modelo físico

Identificador real que entiende el proveedor, junto con sus capacidades declaradas
u observadas.

### Alias de modelo

Nombre estable expuesto al cliente. Puede apuntar a uno o varios destinos físicos
según una estrategia, sin afirmar que modelos diferentes sean semánticamente
idénticos.

### Capacidad

Función que una ruta puede conservar, por ejemplo streaming, herramientas, visión,
salida estructurada o parámetros específicos. Una ruta incompatible no debe usarse
como fallback silencioso si degrada una capacidad requerida.

## Routing

### Solicitud

Operación recibida desde un cliente. Tiene una identidad, un protocolo, requisitos,
un presupuesto total y un resultado final.

### Destino

Combinación elegible de conexión de proveedor, modelo físico, credencial y egreso. Es la
unidad concreta a la que el router puede dirigir un intento.

### Ruta lógica

Punto de entrada estable que vincula un alias con una estrategia y sus políticas.
El cliente utiliza la ruta lógica sin conocer cada destino subyacente.

### Estrategia

Grafo o política versionada que decide qué destinos pueden intentarse, en qué
orden, con qué condiciones y dentro de qué presupuestos.

### Intento

Una llamada concreta a un destino dentro de una solicitud. Una solicitud puede
producir varios intentos cuando su estrategia permite fallback.

### Fallback

Transición autorizada desde un intento fallido o no disponible hacia otro destino.
No significa reintentar indefinidamente ni ocultar incompatibilidades.

### Presupuesto de ejecución

Límites máximos de tiempo, intentos y, cuando corresponda, coste o tokens que una
estrategia puede consumir para resolver una solicitud.

### Política de error

Reglas que clasifican un resultado y deciden si se responde inmediatamente, se
espera, se abre un cooldown o se permite fallback.

### Cooldown

Periodo durante el cual un destino reduce o suspende temporalmente su elegibilidad
después de una señal como `429`. No prueba por sí solo cuál es la cuota oficial.

### Circuit breaker

Mecanismo que deja de enviar tráfico temporalmente a un destino con fallos
repetidos y realiza pruebas controladas de recuperación.

## Estimación y observabilidad

### Evento operativo

Registro estructurado de algo que ocurrió, sin guardar por defecto prompts ni
respuestas. Puede incluir tiempos, tokens, destino, estado, decisión y error
redactado.

### Métrica

Medición numérica derivada de eventos, como latencia, TTFT, tasa de éxito o tokens
por segundo.

### Estimador adaptativo de capacidad y recuperación

Componente que utiliza observaciones históricas para estimar disponibilidad,
capacidad y recuperación. Expresa incertidumbre y nunca presenta una inferencia
como cuota oficial.

### Confianza

Medida de cuánta evidencia respalda una estimación. Debe considerar cantidad,
recencia y consistencia de las observaciones.

### Salud

Estado operativo observado de un nodo, relay, proveedor o destino. Salud no
equivale a compatibilidad ni garantiza disponibilidad futura.

### Trazabilidad

Capacidad de explicar qué estrategia y señales produjeron una decisión sin revelar
secretos o contenido privado.

## Configuración y ciclo de vida

### Fuente de verdad

Estado autoritativo desde el cual opera ModelCairn. Los archivos y la consola no
deben mantener copias independientes que puedan contradecirse.

### Configuración declarativa

Descripción versionable del estado deseado. Su formato exacto y la forma de
conciliar cambios web siguen siendo una decisión de arquitectura.

### Bootstrap

Configuración mínima y segura necesaria para iniciar la instalación por primera
vez, incluida la identidad administrativa y el modo de acceso.

### Onboarding

Flujo posterior que ayuda a configurar proveedores, credenciales, modelos y una
primera ruta funcional.

### Migración

Cambio versionado del esquema de datos o configuración. Debe ser verificable y
tener una estrategia explícita de recuperación o rollback.

### Auditoría

Registro de acciones administrativas relevantes: quién cambió qué, cuándo y con
qué resultado. No debe contener el valor de secretos.

## Relaciones esenciales

1. Una instalación tiene exactamente un nodo principal en la topología inicial.
2. Una instalación puede registrar cero o más relays de salida.
3. Un egreso pertenece al nodo principal o a un relay.
4. Una credencial pertenece al almacén de secretos del nodo principal y tiene una
   afinidad con un egreso.
5. Un proveedor tiene una o más conexiones versionadas, ofrece modelos físicos y
   acepta determinadas credenciales.
6. Un destino combina conexión, modelo físico, credencial y egreso.
7. Una ruta lógica expone un alias y referencia una estrategia versionada.
8. Una estrategia selecciona destinos compatibles mediante políticas explícitas.
9. Una solicitud produce uno o más intentos y un único resultado final.
10. Los intentos generan eventos; las métricas y estimaciones se derivan de ellos.

## Reglas invariantes iniciales

- Ningún relay persiste secretos ni contenido de solicitudes.
- Ninguna API key aparece completa en logs, métricas, auditoría o exportaciones.
- El router no cambia automáticamente la afinidad de egreso de una credencial.
- Un fallback no puede exceder el presupuesto total de la solicitud.
- Una capacidad requerida no se elimina silenciosamente durante una traducción o
  fallback.
- Las reglas oficiales configuradas por el operador prevalecen sobre las
  estimaciones observadas.
- Las decisiones publicadas son versionadas; editar un borrador no altera tráfico
  activo hasta su publicación.
