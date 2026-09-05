# ADR-0003: Base técnica y despliegue inicial

[English](0003-base-tecnica-y-despliegue.md)

- Estado: aceptada
- Fecha: 2026-09-05
- Responsables: mantenedor principal y diseño técnico

## Contexto

ModelCairn debe funcionar en una VM Linux de 1 GB de RAM, atender tráfico HTTP con
streaming, instalarse con poco esfuerzo y distribuirse para AMD64 y ARM64. El
prototipo anterior no determina la tecnología definitiva.

## Decisión

- El backend y las herramientas de sistema se implementarán en **Go**.
- La primera arquitectura será un **monolito modular** distribuido como ejecutable.
- La consola web compilada podrá incluirse en la distribución del servidor.
- **systemd** será el método principal de ejecución en Linux.
- El instalador preguntará si debe habilitar el inicio automático después de
  reiniciar la máquina. La opción recomendada y predeterminada será habilitarlo.
- Docker se ofrecerá como alternativa después de validar la ruta nativa.
- Se publicarán artefactos Linux para AMD64 y ARM64.
- La medición principal de 1 GB se ejecutará primero en la VM de referencia real y
  se repetirá una validación representativa en la otra arquitectura.

## Motivos

Go permite producir ejecutables y soporta objetivos Linux AMD64 y ARM64. Su modelo
de concurrencia y biblioteca HTTP encajan con routing y streaming, mientras que
un único proceso simplifica consumo, instalación y diagnóstico. systemd proporciona
supervisión e inicio durante el arranque sin exigir un runtime de contenedores.

La elección deberá superar el benchmark de memoria, concurrencia y streaming de la
Fase 1. Un resultado insuficiente obliga a revisar o reemplazar esta decisión
mediante otro ADR.

## Consecuencias

- El prototipo previo sirve como referencia funcional, no como base obligatoria.
- Los límites entre módulos deberán comprobarse mediante paquetes y pruebas.
- El proyecto mantendrá compilaciones y pruebas para dos arquitecturas.
- La instalación Docker podrá tener diferencias operativas documentadas respecto
  de systemd, pero no diferencias funcionales intencionales.

## Fuentes técnicas

- [Documentación oficial de compilación de Go](https://go.dev/doc/tutorial/compile-install)
- [Objetivos de sistema y arquitectura de Go](https://go.dev/doc/install/source#environment)
