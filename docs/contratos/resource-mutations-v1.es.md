# Mutaciones individuales de recursos v1

[English](resource-mutations-v1.md)

Estado: implementado en el Hito 3; pendiente el cierre integral del hito.
Complementa [la semántica compartida](config/semantica-apply-v1alpha1.es.md).

## Servicio y límites

Implementar `config.Manager.MutateResourceSession` con operación tipada
create/update/delete, tipo y nombre de ruta, recurso decodificado cuando corresponda,
versión esperada, sesión y CSRF. No aceptar un Actor proporcionado por HTTP.
Resultado: recurso confirmado para create/update; delete sin cuerpo. No hacer
una segunda lectura después del commit para construir la respuesta o su ETag.

El handler valida Origin/transporte, tipos de contenido, tamaño, método, identidad
de ruta/cuerpo y sintaxis de If-Match. El servicio vuelve a validar argumentos
tipados. Reutilizar el parser estricto mediante un envelope Configuration con un
solo recurso, preservando Presence, sin perder detección de claves duplicadas.
No decodificar primero con un map que elimine duplicados. Límite existente: 8 MiB.

POST/PUT aceptan state present u omitido; state absent se rechaza. DELETE construye
internamente state absent y no acepta cambios adicionales en el cuerpo.
PUT conserva campos omitidos según el contrato común; arrays explícitos reemplazan
el array. No introducir una segunda semántica de defaults bajo el nombre replace.

## Una operación, una transacción

Añadir un ejecutor interno en SecretStore que mantenga el mismo orden de bloqueo
que ExecutePlan: mutex del almacén, BeginTx, trabajo, commit y liberación. No crear
un plan artificial ni consumir nonce para CRUD individual. El ejecutor recibe
solo callbacks internos; falla si el almacén está unavailable, hace rollback al
salir por error y marca unavailable ante resultado incierto de commit.

Dentro del callback:

1. Llamar AuthorizeAdminMutationTx; obtener Actor vigente después de esperar locks.
2. Leer estado actual por tipo/nombre usando tx. Create exige ausencia (409 si
   existe); update/delete exigen existencia (404) y versión coincidente (412).
   Comprobar también uid/resourceVersion del cuerpo cuando estén presentes.
3. Usar prepareTx con un documento de un recurso; allowDelete solo para delete.
   El catálogo y los secretos se leen en esa misma tx. Prepare/Validate verifican
   el grafo efectivo completo, incluidos dependientes no enviados por el cliente.
4. Aplicar las mutaciones mediante ApplyConfigTx. Esta capa ya genera auditoría
   tipada y aumenta config_revision cuando cambia el grafo. No llamar a
   Repository.Put/Delete ni a métodos que abren otra transacción desde el callback.
5. Para un update noop, conservar versión y timestamp, pero insertar auditoría
   tipada de operación sin cambios dentro de tx. No llamar a ApplyConfigTx con una
   mutación ficticia para forzar un incremento de revisión.
6. Leer el recurso resultante dentro de tx y capturar el DTO. Devolverlo únicamente
   si commit tuvo éxito. Cualquier fallo revierte también last_seen y auditoría.

Mutaciones simultáneas con el mismo ETag: un cambio real gana; el siguiente recibe
412. Reset/logout confirmado antes de la operación invalida su autorización.
Eliminar y recrear un nombre crea otra identidad: If-Match numérico solo comprueba
la versión de la identidad actual. No afirmar protección entre recreaciones; uid
del cuerpo, cuando existe, añade esa comprobación. Evaluar ETag con identidad en
una revisión futura del contrato antes de prometer esa garantía para DELETE.

## Correspondencia HTTP

POST devuelve 201; PUT 200; ambos con ETag de la versión confirmada. DELETE 204
con no-store. If-Match ausente en PUT/DELETE: 428. Débil, wildcard, lista, signo,
ceros iniciales, duplicado o overflow: 400. Solo aceptar `"[1-9][0-9]*"`.
Tipo/nombre de cuerpo distinto de ruta: 400. Dependencias y cruce de proveedor:
409. No exponer errores SQL ni valores rechazados. Aplicar el redactor compartido
a cadenas del DTO antes de serializar; no filtrar texto JSON ya serializado.

AgentToken CRUD administra solo metadatos: no emite bearer, reemplaza token_hash ni
revive revoked_at. Mantener el upsert irreversible existente; issue/revoke tendrán
su servicio explícito. No implementar publish/test como éxito ficticio.

## Aceptación agrupada

Probar create duplicado, update/delete inexistente, ETag inválido/obsoleto y dos
escrituras concurrentes. Rechazar borrar un recurso referenciado y cambiar proveedor
de una dependencia que invalidaría un Destination no incluido en la petición.
Comprobar noop estable con auditoría, rollback ante fallo de auditoría, revocación
de sesión y no exposición de secretos en DTOs. El QA se agrupa con el bloque de
implementación; este diseño no declara implementados los endpoints.
