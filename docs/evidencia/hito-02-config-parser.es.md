# Hito 2 — Parsing y validación de configuración

[English](hito-02-config-parser.md)

Estado: QA independiente aprobada; pendiente de CI antes de la aceptación.

`internal/config` convierte exactamente un documento YAML o JSON en un único
modelo tipado. Antes de canonizar rechaza entradas mayores de 8 MiB, más de 64
contenedores anidados, documentos adicionales, aliases, tags personalizados,
claves duplicadas y claves YAML que no sean strings. Los diagnósticos contienen
solo códigos y rutas estables, sin copiar valores rechazados.

La validación implementa tipos y límites del schema, defaults sin perder si el
campo fue omitido, `null` explícito cuando está permitido, URLs HTTP(S), permiso
explícito para hosts privados literales, referencias, borrados sobre un catálogo
efectivo y afinidad de proveedor. El export JSON ordena recursos por identidad y
aplica el redactor compartido antes de escapar strings.

Las pruebas cubren el ejemplo con los diez tipos de recursos, round-trip canónico,
inputs hostiles, URLs, tombstones, borrados contra estado existente, defaults y
presencia, `null`, canaries, Unicode y los límites de 10.000 recursos. En la VM
representativa de 998.465.536 bytes, la suite Linux AMD64 pasó con 51.152 KiB de
RSS máximo y cero swap; el hash del binario se verificó después de transferirlo.
La cobertura local de `internal/config` fue 83,4%.

Límites: la resolución DNS y protección frente a cambios de DNS/redirects pertenece
al cliente de salida del Hito 4. La integración con plan/apply pertenece a los
siguientes puntos del Hito 2. La tercera revisión independiente cerró los hallazgos
anteriores sin bloqueos; falta CI antes de aceptar este punto.
