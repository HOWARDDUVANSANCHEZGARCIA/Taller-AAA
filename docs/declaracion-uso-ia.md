# Declaración de uso responsable de IA (borrador)

> Borrador para completar y ajustar por el equipo antes de incluirlo en el PDF.
> Debe reflejar con exactitud cómo se usó la herramienta: corrijan lo que no
> corresponda y agreguen lo que cada integrante hizo por su cuenta.

## Herramienta

Claude (Anthropic), usada como asistente de programación durante el desarrollo
del taller. Los commits en los que intervino llevan la línea
`Co-Authored-By: Claude` en el historial del repositorio.

## Para qué se usó

- Revisión del código de registro y autenticación (integrante 1). Detectó que
  el login permitía saber qué correos estaban registrados por el tiempo de
  respuesta, y que el proyecto no compilaba sin fijar la versión de Gin.
- Generación de código, pruebas y documentación de:
  - vehículos y catálogo (HU-VIS-01, HU-VEN-01),
  - autorización y administración (HU-ADM-01),
  - accounting (RF-01),
  - las páginas web con plantillas de Gin y CSS.
- Elaboración de los diagramas de estructura y de comportamiento.

## Qué hizo el equipo

- [Completar: decisiones que tomó el equipo, por ejemplo la división del
  trabajo, los datos de un vehículo, que el front se haga con Gin, el orden de
  la demostración.]
- [Completar: qué revisó y probó cada integrante y qué cambió del código
  propuesto.]
- [Completar: cómo verificaron que entienden el código: lectura, ejecución de
  las pruebas, recorrido de la demostración.]

## Verificación

Todo el código se verificó con las pruebas automáticas del repositorio
(`go test ./...`, incluida una prueba de integración de punta a punta de AAA)
y con el recorrido manual descrito en `docs/guia-ejecucion.md`.

## Responsabilidad

El equipo revisó el código entregado, responde por su contenido y puede
explicar su funcionamiento en la sustentación.
