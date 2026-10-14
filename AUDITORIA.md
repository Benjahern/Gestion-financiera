# Auditoría de Orden

## Cierre de preparación para producción

Los hallazgos del informe inicial de abajo fueron corregidos en el código y la
configuración actuales. Ese informe se conserva como historial; sus referencias
a Compose de desarrollo y números de línea no describen ya el estado actual.

- Propietario privado configurado exclusivamente con `OWNER_EMAIL`; sin signup
  ni correo personal incrustado en código, documentación o frontend compilado.
- `.gitignore`, contextos Docker y revisión pública excluyen hojas de cálculo,
  exportaciones, dumps, claves, archivos de credenciales y configuración privada.
- Vite limitado al frontend y metadatos Figma concretos, escucha local y deniega
  documentos/secretos. Producción no ejecuta Vite ni incluye Node o fuentes web.
- Filtros Unicode corregidos y probados con tildes; rate limits por cliente
  identificado únicamente por el proxy interno autenticado, no un contador global.
- Menos consultas innecesarias y CSS sobrante; cuentas con tarjetas vinculadas
  no pueden cambiar de moneda. Gmail tiene cursor incremental, cola de avisos sin
  cuenta, tratamiento de avisos eliminados y tiempos acotados por operación.
- Compose de producción para Coolify, sin puertos de host de API/PostgreSQL;
  frontend estático no-root, cookies Secure, CSP estricta para scripts y secretos
  solo de runtime. Instrucciones completas: [deploy/README.md](deploy/README.md).
- Go actualizado a 1.26.9; paquetes del runtime actualizados. Escaneo Trivy de
  las tres imágenes construidas: **0 avisos HIGH/CRITICAL**, sin excluir los
  avisos sin parche. `pnpm audit`: **0 avisos**. Govulncheck: sin llamadas
  vulnerables; OpenPGP no se importa ni se usa.
- Tests Go de integración/race/vet, filtros Unicode, separación de límites por
  IP, build, comprobación de Vite y navegador desktop/móvil con HTTPS simulado:
  pasan. Bases Docker disponibles en ARM64/AMD64 y binario Go compilado para ARM64;
  no se ha ejecutado aún en el servidor ARM real ni desplegado en Coolify.
- CI incluye revisión pública, Gitleaks, tests y escaneo de imágenes. Hooks
  locales bloquean commits/push directos a `main` y exigen correo GitHub noreply.

### Pendientes del operador

1. Configurar variables privadas, dominio, TLS y proxy confiable en Coolify según
   la guía; validar su cadena con Cloudflare antes de abrir acceso público.
2. Activar protección remota de `main` en GitHub: los hooks locales no sustituyen
   reglas del servidor. Publicar esta preparación mediante rama release y PR.
3. El propietario autorizó anonimizar autor/committer del historial original
   usando su usuario público y correo GitHub noreply, y publicar esta preparación
   en `main` con `--force-with-lease`. La nueva historia no incluye el correo
   personal anterior; esto no borra clones/copias o caches anteriores.
   No se encontraron claves reales ni documentos financieros en los blobs Git
   revisados. La revisión no garantiza detectar secretos ocultos en imágenes.
4. Preparar backup/restauración antes de guardar datos reales. Gmail real queda
   pendiente de tus credenciales y de confirmar compatibilidad de un aviso nuevo.

Esta publicación y reescritura de metadatos son una excepción autorizada a la
política de cambios por PR. No se ha desplegado en el servidor. Tus datos,
credenciales de aplicación y el volumen real no se modificaron. Las pruebas usaron
exclusivamente bases/contenedores desechables.

## Informe inicial (histórico, anterior a las correcciones)

Fecha: 12 de octubre de 2026. Alcance: código actual, dependencias, autenticación,
API, Gmail, configuración Docker local y código potencialmente sobrante.
**No se aplicaron correcciones ni se modificaron datos del usuario o credenciales.**

## Resultado

No se encontró una vulnerabilidad crítica ni un bypass de autenticación en las
comprobaciones realizadas. Esto no es una certificación de seguridad ni un
pentest completo. Hay problemas de disponibilidad, riesgos de exposición del
servidor de desarrollo y errores funcionales que conviene resolver antes del
despliegue. El Compose actual está diseñado exclusivamente para desarrollo.

## Hallazgos prioritarios

### 1. Límite global permite bloquear el acceso — media, confirmado

**Evidencia:** `backend/internal/orden/server.go:40-43,164-179`.

Los contadores usan claves compartidas (`start`, `login`, etc.), no identifican
al solicitante y cuentan incluso solicitudes rechazadas. En una base aislada,
30 solicitudes a `/api/auth/start` con un correo no autorizado devolvieron 403;
la siguiente solicitud del propietario devolvió 429. El bloqueo dura hasta el
reinicio de la ventana de 15 minutos. Para login bastan 10 solicitudes.

Un cliente no navegador puede enviar el `Origin` esperado; validar Origin es
protección CSRF, no identificación del cliente. Esto no permite entrar ni afecta
directamente las sesiones ya abiertas, pero sí impide nuevos accesos.

**Recomendación:** combinar protección en el proxy/Cloudflare con límites por
cliente confiable y por cuenta; conservar un límite global de emergencia. No
confiar ciegamente en cabeceras de IP enviadas por cualquier cliente. Revisar que
los accesos correctos no consuman el mismo presupuesto de intentos fallidos.

### 2. Vite fuera de Docker puede servir archivos personales — media, condicional

**Evidencia:** `frontend/vite.config.ts:49` y configuración efectiva obtenida con
`resolveConfig`: host `0.0.0.0`, `server.fs.allow` incluye la raíz completa del
proyecto; `fs.deny` solo incluye `.env`, `.env.*`, certificados y `.git`.

El Excel personal está fuera del frontend, pero dentro de esa raíz permitida.
Ignorarlo en Git no impide que Vite lo sirva mediante `/@fs/` si se conoce la
ruta. El riesgo requiere ejecutar Vite directamente en el host y que alguien
pueda alcanzar su puerto. No se descargó el Excel durante esta auditoría.

**Distinción importante:** el Docker actual publica solo en `127.0.0.1` y no
copia el Excel; no se observó esta exposición en ese contenedor.

**Recomendación:** denegar explícitamente archivos financieros y carpetas no
públicas, reducir `fs.allow` y limitar la escucha local cuando no sea necesaria
la integración de previsualización. No publicar Vite por un túnel.

### 3. Dependencias del frontend con avisos — prioridad de actualización media

`pnpm audit --json` encontró **2 avisos altos y 1 moderado**, sin críticos:

| Dependencia instalada | Aviso | Contexto real | Versión corregida |
| --- | --- | --- | --- |
| Vite 8.0.5 | GHSA-fx2h-pf6j-xcff, alto | Bypass de `fs.deny` en Windows; entorno actual Linux | >=8.0.16 |
| Vite 8.0.5 | GHSA-v6wh-96g9-6wx3, moderado | Divulgación NTLM por rutas UNC en Windows | >=8.0.16 |
| source-map-js 1.2.1 | GHSA-68fv-2mgg-jv7q, alto | DoS al procesar mapas de fuente manipulados, no importes o descripciones de la API | >=1.2.2 |

Son herramientas de desarrollo/build. Sin embargo, el contenedor web ejecuta
Vite, así que no conviene descartarlas solo por estar en `devDependencies`.
Los dos avisos de Windows no constituyen una explotación confirmada en Linux.
Actualizar lockfile y ejecutar nuevamente pruebas y build; no quitar
`source-map-js` a mano, porque es una dependencia transitiva necesaria.

En Go, `govulncheck -show verbose ./...` no detectó vulnerabilidades alcanzables.
El aviso GO-2026-5932 corresponde a `x/crypto/openpgp`, que la aplicación **no
importa**. No justifica eliminar `x/crypto`: Argon2id sí se utiliza.

### 4. Compose actual no es un despliegue de producción — requisito previo

**Evidencia:** `deploy/docker-compose.yml:9-10,30-36,58-71`,
`frontend/Dockerfile:11`, `frontend/security-headers.ts:7`.

Tiene contraseñas predeterminadas de desarrollo, `APP_ENV: development`, Vite/HMR,
HTTP local y cookies sin `Secure`. La CSP permite scripts inline y WebSockets
para React Refresh. Todo esto está documentado y es aceptable para el uso local
actual; no es una vulnerabilidad crítica del backend por sí mismo.

No debe reutilizarse tal cual detrás de Cloudflare Tunnel. Producción necesita
frontend compilado y servidor estático, HTTPS público, entorno de producción,
secretos propios y una CSP sin `unsafe-inline` para scripts. Los headers actuales
del frontend se configuran en `server.headers`: no se trasladan automáticamente
al servidor que publique `dist`. `vite preview` tampoco tiene hoy el proxy API.

### 5. Categorías Unicode aceptadas pero imposibles de filtrar — media, confirmado

**Evidencia:** `backend/internal/orden/classifier.go:206-209`,
`backend/internal/orden/finance.go:52-54,276`.

Crear categorías admite hasta 100 caracteres Unicode; filtrar admite 100
**bytes**. Una categoría de 80 caracteres `á` se creó correctamente, pero su
filtro devolvió 400 porque ocupa 160 bytes. Ocurre también con otros caracteres
acentuados y nombres largos. La búsqueda de movimientos tiene una discrepancia
similar con su validación de títulos.

**Recomendación:** unificar las validaciones con `validText`/conteo de runas y
añadir regresiones con tildes. El fallo se reprodujo en una base desechable.

## Gmail: problemas funcionales por corregir posteriormente

No se cambiaron sus credenciales ni se contactó con un buzón real.

- **Timeout global demasiado corto para OAuth/sync:**
  `server.go:129` da 10 segundos por solicitud. OAuth puede realizar dos llamadas
  consecutivas a Google, cada una con timeout de 7 segundos; sync puede realizar
  todavía más (`gmail.go:27,207-215,455-525`). Una conexión lenta puede consumir
  el estado OAuth y fallar, o revertir un lote de importación manual. El worker
  tiene 60 segundos, de modo que este riesgo no significa que toda importación
  esté rota. Usar límites por operación o sincronización mediante trabajo en
  segundo plano. Hallazgo por revisión, sin reproducción con Google real.
- **Relectura creciente del historial desde la conexión:**
  `gmail.go:455-458,517-521` mantiene `after=connected_at` y vuelve a la primera
  página al terminar. La deduplicación evita duplicar gastos, pero se vuelve a
  recorrer el mismo historial y crece la latencia para avisos nuevos. Una compra
  sin tarjeta asociada también impide avanzar de esa página. Adoptar un cursor
  incremental con ventana de seguridad o Gmail History API; registrar por
  separado los avisos que requieren revisión. Confirmado por lectura del flujo,
  no por pruebas de carga con un buzón real.
- **Compatibilidad pendiente:** el lector MACH depende del formato MIME y de
  resultados de autenticación concretos. Las pruebas sintéticas pasan, pero una
  captura no prueba el funcionamiento con mensajes reales ni todos los casos de
  cabeceras duplicadas. Bci sigue deshabilitado intencionalmente.

## Código sobrante y trabajo innecesario

- `tsc --noEmit --noUnusedLocals --noUnusedParameters`: sin hallazgos.
- `deadcode ./...` de Go: sin funciones de aplicación inalcanzables detectadas.
  Estas herramientas no garantizan que toda rama o estilo tenga utilidad.
- **CSS sin referencias actuales:** `.modal-subtitle` y `.amount-input`, incluidas
  sus reglas hijas/foco en `frontend/src/index.css:292,296-300`, no aparecen en
  los componentes. Candidatos claros de limpieza; conservar las partes de los
  selectores compartidos que sí estilizan inputs y selects.
- **Consultas innecesarias:** `frontend/src/App.tsx:91-95` siempre carga cuentas,
  resumen y movimientos, incluso en Ajustes; allí no se utilizan los dos últimos.
  Los filtros del listado también vuelven a pedir cuentas/resumen sin necesidad.
  Separar cargas por página y dependencias reduce consultas y fallos acoplados.
- **Estado Gmail redundante:** se persiste y devuelve `status`, pero solo se
  escribe `connected`; no hay transición a error/revocado y el frontend no lo
  consume. Implementar estados útiles o simplificar en una migración futura.
- **Scaffold Figma:** gran parte de `frontend/vite.config.ts` sirve solo a Figma
  Make/HMR. No es lógica financiera ni necesariamente código muerto en este
  entorno. Aislar plugins si el proyecto pasa a ser independiente, no borrarlos
  indiscriminadamente. Las migraciones existentes tampoco son código sobrante.

## Controles que están bien implementados

- API financiera y categorías protegidas por sesión; lecturas anónimas dan 401.
- Propietario único validado en backend y restringido en PostgreSQL; sin signup.
- Argon2id con sal aleatoria, parámetros fijos y concurrencia limitada.
- Sesiones aleatorias de 256 bits; se guarda su hash, no el token en claro.
- Cookies HttpOnly/SameSite Strict; Secure y prefijo `__Host-` en producción.
- Origin exacto, CSRF por sesión, JSON estricto y límite de cuerpo de 16 KiB.
- SQL parametrizado, importes decimales exactos y restricciones de integridad.
- Activación/recuperación CLI con códigos de un uso y consumo transaccional;
  recuperación revoca sesiones y evita carreras de login con contraseña antigua.
- OAuth con PKCE, estado de un uso ligado a sesión, caducidad y control de
  generaciones; credenciales Gmail cifradas con AES-GCM.
- Sin tokens de sesión en localStorage ni renderizado HTML de correos.
- `.env` ignorado en Git, permiso local 600; Excel ignorado y excluido del build
  Docker. No se imprimieron secretos durante la auditoría.
- Backend no root, filesystem de contenedor read-only, capacidades eliminadas;
  PostgreSQL/API sin puertos publicados y rol de aplicación no superusuario.

## Validaciones realizadas y límites

- Tests Go completos con `-race -count=1` y PostgreSQL desechable: pasan.
- `go vet`, typecheck, build y pruebas UI desktop/móvil con APIs simuladas: pasan.
- Pruebas adicionales aisladas reprodujeron bloqueo global y filtro Unicode;
  sus archivos temporales y el contenedor de auditoría se retiraron al terminar.
- Headers y rechazo de lecturas anónimas comprobados en el Docker local.
- No se ejecutaron ataques de carga contra tu instancia ni se tocaron sus datos.
- No se escanearon las imágenes del sistema operativo, el servidor ARM futuro,
  Cloudflare, permisos reales de Google o el historial Git completo. Esos puntos
  requieren una revisión de despliegue separada. No se añadieron backups.

**Orden recomendado:** actualizar dependencias y contener Vite; corregir límites
de acceso y Unicode; limpiar estilos/cargas; mejorar Gmail; preparar y auditar
por separado la configuración de producción cuando corresponda.
