# Métricas, precios y navegación contextual

Estado: implementación inicial para revisión visual; no equivale a facturación.

## Semántica de los datos

- `GET /api/v1/admin/metrics` exige sesión y CSRF. Resume solo filas operativas
  retenidas localmente; no lee ni entrega prompts, respuestas ni valores de claves.
- Los totales de tokens suman exclusivamente los conteos reportados. Una ausencia
  de uso no se interpreta como cero. `usageKnownRequests` cuenta solicitudes con
  entrada y salida presentes. El gráfico diario se limita a 90 días UTC.
- El desglose por modelo usa el último intento y su destino. Si ese intento o
  modelo desapareció, los tokens siguen en el total pero no se asignan a un
  modelo. El consumo de intentos fallidos sin conteo puede quedar fuera. La
  latencia media usa la duración completa de solicitudes con uso reportado;
  tokens/s divide los tokens de salida por la suma de esas duraciones válidas.
  Es un rendimiento observado aproximado, no una medición aislada de generación.
- La tasa de éxito de las últimas 24 horas se llama **disponibilidad observada**;
  no es uptime. Uptime requerirá sondeos independientes y definición de ventanas.
- La retención elegida por el administrador gobierna estos datos. Al purgar
  eventos también disminuyen los totales; no prometemos un acumulado vitalicio.

## Precio estimado

Cada modelo puede declarar `pricing` con moneda USD y tarifas de entrada/salida
por un millón de tokens. Ausencia de tarifa significa **desconocido**, mientras
que `0` explícito significa gratuito. El cálculo usa los tokens atribuidos al
modelo y la tarifa **actual**; editar el precio recalcula el pasado visible. Por
eso no se presenta como factura, gasto real ni estimación histórica exacta.
Una estimación parcial debe mostrar cuántas solicitudes con uso tienen tarifa.
No se consultan precios en Internet ni se ejecutan llamadas al proveedor.

La vista de ejemplo de Datos es local al componente: no genera solicitudes,
no guarda registros, no escribe recursos y se identifica visualmente. Al salir,
vuelve al mismo conjunto de métricas reales.

## Navegación y rutas

Los enlaces Modelo desde Proveedor conservan el proveedor y su pestaña de
Modelos como origen. Claves guardadas conserva Proveedor > API keys. Un enlace
directo sin origen vuelve al catálogo general. Este comportamiento también debe
funcionar después de recargar, no depender solo del historial del navegador.

La ruta visual muestra el alias solicitado y el orden de opciones de destino.
El editor crea nuevas opciones eligiendo modelo y clave del mismo proveedor,
además de reutilizar o reordenar opciones existentes. La salida de red se hereda
de la credencial. Crear una opción guarda un destino aún no usado; guardar el
recorrido crea una nueva versión de estrategia como borrador; **Publicar** es un
acto separado que afecta solicitudes futuras. Si se cancela tras crear una
opción, el destino queda disponible pero sin asociar a la ruta. Una operación
atómica que abarque creación y publicación está pendiente.

## Próximos controles

- La conexión API ya se puede añadir desde Proveedor > Configuración; el
  formulario inicial usa Chat Completions y HTTPS. Proveedor > API keys permite
  vincular una clave previamente guardada. El vínculo se revisa antes de
  aplicarse; si faltan cuenta o salida directa, se crean en la misma operación.
  Guardar el secreto continúa siendo un paso separado. Falta un asistente único
  que reúna ambos pasos y la asignación de modelos.
- Constructor visual completo de rutas nuevas a partir de recursos existentes,
  con revisión previa y transacción atómica de opción + recorrido.
- Uptime por sondeos, TTFT y ventanas/muestras configurables por modelo;
  distinguir errores de red de límites de cuota.
- Si se propone volver a revelar un secreto, diseñar autorización reforzada
  (por ejemplo, reautenticación o 2FA), auditoría y política de no exposición.
  No habilitar un botón de «mostrar clave» solo porque el servidor pueda
  descifrarla.
