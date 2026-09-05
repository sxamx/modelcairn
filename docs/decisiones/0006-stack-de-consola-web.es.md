# ADR-0006: Stack de la consola web

[English](0006-stack-de-consola-web.md)

- Estado: aceptada
- Fecha: 2026-09-05

## Decisión

La consola será una SPA/PWA construida con TypeScript, React y Vite. El resultado
estático se incrustará en el ejecutable Go para producción; Node.js será una
dependencia de compilación y desarrollo, no un proceso residente en la VM.

Se limitarán inicialmente las dependencias de runtime. El cliente generado desde
OpenAPI compartirá tipos con los formularios; no se replicarán manualmente los
contratos del backend. La navegación, formularios y caché de servidor podrán usar
bibliotecas pequeñas solo tras justificar su coste. El editor visual futuro se
evaluará por separado.

## Motivos

- TypeScript reduce divergencias entre API y formularios.
- React facilita una consola compleja y componible, incluida la futura pizarra.
- Vite produce activos estáticos y mantiene un flujo de desarrollo sencillo.
- Incrustar los activos evita consumir RAM con un servidor frontend separado.

## Presupuestos iniciales

- JavaScript inicial comprimido: objetivo menor de 250 KiB, máximo provisional
  400 KiB antes de exigir división o justificación.
- Carga diferida para métricas pesadas y futuros editores.
- Sin SSR en la Fase 1.
- Lighthouse y accesibilidad automatizada como señales, no sustitutos de pruebas
  reales en móvil/lectores de pantalla.

## Fuentes

- [React con TypeScript](https://react.dev/learn/typescript)
- [Guía oficial de React para una aplicación con Vite](https://react.dev/learn/build-a-react-app-from-scratch)
