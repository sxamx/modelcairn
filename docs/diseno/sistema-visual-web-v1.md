# Sistema visual de la web · propuesta v1

Estado: propuesta para revisión del mantenedor. La [galería interactiva](../prototipos/sistema-visual-web-v1.html) es la referencia visual; este documento define las reglas que deberán trasladarse a la web real cuando se aprueben. No modifica los contratos de API ni la versión instalada.

## Principio

Los *design tokens* son nombres compartidos para decisiones visuales. Por ejemplo, `--mc-line` significa «borde de superficie» y puede tener un valor distinto en tema claro y oscuro. Las pantallas usan el nombre, no un color o margen inventado cada vez.

## Tokens base

| Familia | Token | Claro | Oscuro | Uso |
|---|---|---|---|---|
| Superficie | `--mc-bg` | `#f4f6f5` | `#111a19` | Fondo de pantalla |
| Superficie | `--mc-surface` | `#ffffff` | `#1b2927` | Tarjetas, menú, formularios |
| Superficie | `--mc-surface-soft` | `#f9fbfa` | `#22322f` | Áreas secundarias |
| Texto | `--mc-ink` | `#172c29` | `#e5f1ed` | Texto principal |
| Texto | `--mc-muted` | `#647875` | `#a5b8b1` | Metadatos y ayuda |
| Estructura | `--mc-line` | `#dce5e1` | `#344844` | Separadores y bordes |
| Marca | `--mc-accent` | `#146c60` | `#73c9b8` | Acción principal, selección, enlaces |
| Marca | `--mc-accent-soft` | `#e4f3ef` | `#284b43` | Selección tenue |

Estados: éxito verde, advertencia ámbar, fallo rojo e información azul. Cada estado combina fondo y texto propios para claro/oscuro; siempre lleva una palabra o código, nunca solo color. Un límite `429` recuperado por fallback es advertencia de un intento, no fallo final de la solicitud. Un fallo que llega al cliente sí es rojo.

## Escalas

- Espaciado: 4, 8, 12, 16, 20, 24, 32 y 40 px. La tarjeta usa 20 px internos; filas 12–16 px; separación de secciones 24–32 px.
- Radios: controles 8 px, tarjetas 13 px y estados redondeados por completo. Borde estándar de 1 px; sombra de tarjeta ligera.
- Tipografía local del sistema, sin descarga externa: página 30 px, sección 20 px, título de tarjeta 16 px, cuerpo 15 px, secundario 14 px y etiqueta 13 px. Texto principal de fila 15 px semibold. Evitar 11–12 px en contenido esencial, especialmente en pantallas de baja densidad.
- Botones y campos: al menos 40 px de alto en escritorio y 44 px en móvil. Foco visible y navegación por teclado.
- Movimiento: navegación entre secciones con desplazamiento suave; cambios breves de estado/tema. Si el sistema solicita reducir movimiento, las transiciones y el desplazamiento animado se desactivan.
- No usar márgenes negativos ni un selector general para «arreglar» el espaciado de varios componentes. Las separaciones entre piezas se expresan mediante `gap` y la escala de tokens.

## Componentes reutilizables

Tarjeta de métrica; tarjeta de contenido; fila de lista; badge de estado; botones primario/secundario/discreto/peligroso; campo con ayuda; aviso contextual; estado vacío; pestañas; panel de edición contextual. La galería los muestra con datos de ejemplo y ambos temas. Las vistas reales no deben inventar datos para llenar una tarjeta.

## Visualizaciones

| Pregunta | Componente propuesto | Regla |
|---|---|---|
| ¿Cómo cambió una métrica con el tiempo? | Línea | Ejes, unidad, periodo y origen de los datos visibles; no suavizar hasta ocultar picos. |
| ¿Qué parte aporta cada grupo? | Dona | Solo pocas categorías y total conocido; mostrar porcentajes también en texto. |
| ¿Qué categoría tiene más eventos? | Barras | Escala común, valores visibles y orden claro. |
| ¿Cuándo estuvo disponible? | Tira de intervalos + detalle temporal | Diferenciar disponible, degradado, caído y sin datos; no contar huecos como éxitos. |

El uptime requiere **sondeos independientes** y una política de ventanas/muestras aún no implementados. La disponibilidad observada de solicitudes es otra métrica. La galería usa números sintéticos expresamente marcados; la web real debe mostrar «Sin sondeos» hasta que exista el dato. Los gráficos deben ofrecer descripción textual y no depender únicamente del color.

## Criterio de integración

Una vez aprobada esta guía, extraer los tokens a la hoja CSS de la web real y migrar componentes compartidos antes de reescribir pantallas. Comprobar contraste, zoom, móvil/PWA, preferencia de movimiento reducido, carga en la VM de 1 GB y consistencia entre temas. No publicar una pantalla parcialmente migrada como si fuera el nuevo sistema completo.

La revisión del 22 de septiembre detectó que una regla de margen negativo para párrafos introductorios también alcanzaba la explicación bajo los estados y provocaba solapamiento. Se sustituyó por un selector específico y después por separaciones basadas en tokens; se revisó la sección en tema oscuro. Este incidente refuerza que declarar tokens no garantiza su uso correcto: cada componente exige revisión visual en claro, oscuro y ancho móvil.
