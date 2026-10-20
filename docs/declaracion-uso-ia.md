# Declaración de uso responsable de IA

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

Howard Sánchez - Registro, autenticación y usuarios
HU-VEN-01 · Authentication


Responsabilidades
- Implementar el registro autónomo de vendedores.
- Implementar el mecanismo de autenticación de usuarios.
- Definir cómo se identifica al solicitante en las peticiones protegidas.
- Asegurar que el rol del usuario provenga de información confiable del sistema, no de un dato arbitrario enviado por el cliente.
- Coordinar con el integrante 3 la información que necesitará el control de permisos.

Entregables: código de registro y autenticación, pruebas de acceso autenticado y no autenticado, e instrucciones de integración.



Daniel Cárdenas Ortiz - Vehículos y catálogo público
HU-VIS-01 · HU-VEN-01


Responsabilidades
- Implementar la consulta pública del catálogo.
- Permitir consultar la información del vehículo y el contacto del vendedor asociado.
- Implementar la creación de publicaciones vinculadas al vendedor autenticado.
- Registrar las publicaciones nuevas en estado pendiente de aprobación.
- Implementar el reporte de venta de un vehículo.

Entregables: operaciones de consulta, publicación y reporte de ventas, con pruebas de los estados y datos exigidos por las historias de usuario.



Sebastián Arango - Autorización y administración
HU-ADM-01 · Authorization


Responsabilidades
- Implementar el control de permisos según los roles visitante, vendedor y administrador.
- Permitir que el administrador apruebe publicaciones pendientes.
- Cambiar el estado de una publicación aprobada a publicada.
- Implementar la eliminación de publicaciones y usuarios por parte del administrador.
- Coordinar con el integrante 1 la verificación de identidad y roles.

Entregables: controles de autorización, operaciones administrativas y pruebas que demuestren que los roles no pueden ejecutar acciones ajenas a sus permisos.



Jacobo Woodcock - Accounting, estructura común e integración
RF-01 · Accounting


Responsabilidades
- Diseñar e implementar el registro trazable e íntegro de las operaciones.
- Acordar con el grupo el formato común de los registros: operación, actor identificado cuando corresponda, recurso, resultado y momento.
- Preparar la estructura inicial compartida del proyecto y coordinar las interfaces entre componentes.
- Integrar los registros de actividad en las operaciones desarrolladas por los otros tres integrantes.
- Preparar y mantener las pruebas generales de integración.

Entregables: componente de accounting, mecanismo de integración, pruebas de trazabilidad y guía breve de ejecución.
Para equilibrar la carga: los cuatro integrantes deben implementar y probar sus propios componentes; el integrante 4 coordina la integración, pero no tiene que reparar ni escribir el código de todos.

## Verificación

Todo el código se verificó con las pruebas automáticas del repositorio
(`go test ./...`, incluida una prueba de integración de punta a punta de AAA)
y con el recorrido manual descrito en `docs/guia-ejecucion.md`.

## Responsabilidad

El equipo revisó el código entregado, responde por su contenido y puede
explicar su funcionamiento en la sustentación.