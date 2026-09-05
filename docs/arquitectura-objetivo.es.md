# Arquitectura objetivo inicial de ModelCairn

- Estado: borrador de arquitectura
- Alcance: primera versión operable y extensiones previstas

## Objetivos

- Proporcionar un endpoint estable compatible inicialmente con OpenAI Chat
  Completions.
- Ejecutarse junto al sistema operativo en una VM de 1 GB de RAM.
- Mantener configuración, secretos y decisiones en un nodo principal.
- Utilizar relays ligeros para egresos alternativos sin desplegar la aplicación
  completa ni persistir secretos en ellos.
- Permitir que protocolos, proveedores y estrategias se amplíen mediante contratos
  definidos.

## Vista de componentes

```text
Clientes y agentes
        |
        v
API de compatibilidad ──> autenticación y políticas
        |                         |
        v                         v
Router y estrategias <── estado operativo/estimador
        |
        +──> egreso directo ─────────────> proveedor
        |
        +──> túnel cifrado ──> relay ────> proveedor

Consola/PWA ──> API administrativa ──> configuración y secretos
                                      └─> eventos, métricas y auditoría
```

## Nodo principal

La primera implementación favorecerá un **monolito modular**: un despliegue único
con límites internos claros. Esto reduce memoria, instalación y coordinación sin
impedir separar componentes en el futuro.

Módulos lógicos:

1. **API de compatibilidad:** valida y normaliza solicitudes de clientes.
2. **Identidad y políticas:** autentica agentes y determina permisos.
3. **Router:** ejecuta una versión publicada de la estrategia.
4. **Adaptadores:** conectan protocolos y proveedores.
5. **Estado operativo:** cooldown, circuit breakers, salud y carga.
6. **Estimador:** deriva capacidad, recuperación y confianza.
7. **Plano de control:** API administrativa para configuración y operación.
8. **Consola/PWA:** interfaz para onboarding, rutas, métricas y diagnóstico.
9. **Persistencia embebida:** configuración, eventos, métricas y auditoría.
10. **Almacén de secretos:** protege las credenciales y entrega su valor solamente
    al componente que crea la conexión autorizada.

La consola se construirá con TypeScript, React y Vite. En producción sus activos
estáticos se incrustarán en el ejecutable Go, por lo que Node.js no será un proceso
residente ni consumirá RAM en la VM.

“Monolito modular” no significa un archivo gigante. Significa que los módulos se
ejecutan juntos, pero tienen contratos y responsabilidades separados.

## Relay de salida

El relay será un servicio sin interfaz administrativa pública general. Su contrato
mínimo será:

- aceptar únicamente conexiones autenticadas del nodo principal;
- permitir solamente destinos autorizados para evitar un proxy abierto;
- transportar el flujo sin terminar TLS por defecto;
- no persistir cuerpos, credenciales ni respuestas;
- publicar salud, versión y métricas mínimas sin contenido;
- imponer límites de conexiones, tiempo y tamaño;
- cerrarse de forma segura si pierde su configuración o confianza.

El protocolo concreto del túnel se seleccionará mediante ADR después de comparar
una solución estándar existente con un relay propio. No se implementará
criptografía ni transporte propietario si un estándar pequeño satisface el caso.
La salud del relay se expondrá solamente por el canal restringido al nodo principal,
no como una API administrativa abierta a Internet.

## Persistencia

SQLite será la base de datos embebida del nodo principal porque simplifica una
instalación personal y reduce recursos. La decisión deberá validarse con
concurrencia, streaming, retención prolongada, backups y migraciones.

Los secretos no se tratarán como campos ordinarios. La arquitectura separará:

- datos operativos consultables;
- valores secretos cifrados en reposo;
- clave maestra o mecanismo necesario para descifrarlos;
- exportaciones sin secretos;
- backup explícito de secretos con protección adicional.

## Fuente de verdad de configuración

Existirá una sola fuente de verdad persistida y versionada en SQLite. La consola y la API
administrativa modificarán esa fuente mediante validaciones y auditoría. Los
archivos servirán para importación declarativa, exportación y automatización, no
como una segunda copia observada continuamente sin reglas de precedencia.

Toda entidad modificable tendrá un esquema compartido entre API, archivos y web.
Los secretos y parámetros de bootstrap podrán tener superficies más restringidas.
El contrato se verificará mediante casos de aplicación, exportación y round-trip.

## Camino de una solicitud

1. El cliente se autentica y envía una solicitud a una ruta lógica.
2. La API valida tamaño, protocolo, capacidades y presupuesto.
3. El router carga la estrategia publicada y obtiene destinos elegibles.
4. El planificador combina política explícita, salud y estimación con incertidumbre.
5. El nodo principal crea el intento mediante egreso directo o túnel cifrado.
6. El adaptador conserva streaming y capacidades soportadas.
7. Un error se clasifica antes de permitir espera, cooldown o fallback.
8. La respuesta vuelve al cliente y se registran eventos sin contenido por defecto.

## Límites de confianza

- El cliente no es de confianza hasta autenticarse y validar su solicitud.
- La consola requiere una sesión administrativa independiente de los tokens de
  agentes.
- El relay se considera infraestructura propia pero potencialmente comprometible;
  no recibe secretos legibles cuando el modo túnel lo permite.
- El proveedor es externo y recibe necesariamente el contenido y credencial que su
  API necesita.
- Los archivos importados, respuestas de proveedor y errores son entradas no
  confiables y deben validarse o redactarse.

## Línea base de seguridad

La seguridad será proporcional al riesgo de una aplicación autohospedada:

- TLS para clientes cuando se accede fuera de localhost y para tráfico externo;
- canal autenticado y cifrado entre nodo principal y relays;
- API keys cifradas en reposo y nunca recuperables completas desde la interfaz;
- contraseñas almacenadas mediante hash de contraseñas resistente;
- sesiones administrativas seguras y separación de tokens de agentes;
- redacción centralizada de logs, errores, métricas y exportaciones;
- permisos mínimos de archivos, backups protegidos y rotación posible;
- límites de solicitud y protección para que un relay no sea un proxy abierto.

Las URLs de proveedores son entradas sensibles aunque las configure un
administrador. Antes de permitir conexiones, el sistema deberá controlar esquemas,
resolución DNS, direcciones locales/privadas, redirects y puertos para evitar SSRF.
Los endpoints privados seguirán siendo posibles, pero requerirán una autorización
explícita y visible del operador en vez de quedar permitidos accidentalmente.

No se exige alta disponibilidad, hardware criptográfico, consenso distribuido ni
controles militares para la primera versión.

## Disponibilidad y degradación

El nodo principal es un punto único de fallo aceptado para la primera versión. Si
un relay falla, los destinos ligados a ese egreso dejan de ser elegibles; sus API
keys no cambian automáticamente de egreso. La solicitud solo puede continuar por
otro destino ya autorizado por la estrategia.

Reiniciar el nodo principal debe conservar configuración, afinidades, cooldown
relevante y datos confirmados. El estado puramente transitorio puede reconstruirse
sin presentar estimaciones antiguas como actuales.

## Decisiones diferidas

- protocolo o herramienta exacta para el túnel de egreso.
- contrato detallado del YAML y posible flujo GitOps posterior;
- separación futura de procesos si las mediciones la justifican.

Las primitivas y el formato de backup se cerraron en
[ADR-0005](decisiones/0005-criptografia-y-formato-de-backup.es.md), y el stack de la
consola en [ADR-0006](decisiones/0006-stack-de-consola-web.es.md).
