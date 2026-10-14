# Producción con Coolify en Ubuntu ARM64

Esta configuración sustituye al antiguo Compose de desarrollo. No requiere
Nginx instalado en Ubuntu, Node o Go en el servidor: todo se construye en Docker.
Comprueba `uname -m`: debe indicar `aarch64`/ARM64. Las imágenes base también
admiten AMD64; no se fuerza una arquitectura ni se usa emulación en producción.
Recomendado: al menos 2 GiB de RAM para ejecutar la pila y margen/swap para builds.

## 1. Crear la aplicación

En Coolify: proyecto → entorno de producción → **New → Public Repository**.
Selecciona este repositorio y la rama `main`. Build strategy **Docker Compose**:

- Base Directory: `/`
- Docker Compose Location: `/docker-compose.yml`
- Modo Compose normal, **no Raw**: Coolify añadirá sus redes/labels de proxy.
- Sin comandos personalizados, bind mounts del repositorio ni puertos de host.
- Variables solo de **Runtime**, no habilites su disponibilidad durante Build.
- No asignes dominios a `db` o `backend` ni conectes estos servicios a otras apps.

## 2. Variables en el panel

| Variable | Valor |
| --- | --- |
| `APP_ORIGIN` | `https://orden.example.com`, reemplazado por tu dominio real; sin ruta, sin `/` final y sin `:8080` |
| `OWNER_EMAIL` | Tu correo privado de propietario; no se compila en la web |
| `POSTGRES_PASSWORD` | Contraseña aleatoria propia del rol de aplicación |
| `POSTGRES_ADMIN_PASSWORD` | Otra contraseña aleatoria, diferente, para PostgreSQL administrador |
| `INTERNAL_PROXY_SECRET` | 64 caracteres hexadecimales, compartidos solo entre web y backend |
| `TRUSTED_PROXY_CIDR` | Dirección/subred exacta del proxy de Coolify que alcanza web; ver apartado siguiente |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `TOKEN_ENCRYPTION_KEY` | Opcionales: deja los tres vacíos si no vas a usar Gmail |

Genera valores distintos desde una terminal privada; pégalos en Coolify, no en
Git, issues, capturas o chats:

```sh
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
# Solo para Gmail:
openssl rand -base64 32
```

No uses las claves de desarrollo ni las claves desechables de las pruebas.
`APP_ENV=production` está fijo en Compose y no permite desactivar cookies Secure.
El backend rechaza arrancar sin HTTPS público, propietario o proxy secret válido.
Si una cuenta ya existe, `OWNER_EMAIL` debe coincidir: cambiar la variable no
transfiere la cuenta. Para importar una base local, conserva su propietario real.

### Dirección de cliente y proxy confiable

Nginx sobrescribe las cabeceras privadas hacia Go; Go no confía en
`X-Forwarded-For` enviado directamente. Nginx solo acepta esta cabecera de la
dirección/subred configurada en `TRUSTED_PROXY_CIDR`.

Por defecto `127.0.0.1/32` es conservador y permite arrancar, pero detrás de
Coolify agrupa inicialmente los visitantes bajo la IP del proxy. **Antes de
abrir la aplicación a Internet**, configura la dirección/subred real:

1. Identifica en Coolify/Docker la red por la que su proxy conecta con `web`.
2. Consulta esa red (`docker network inspect NOMBRE_DE_RED`), no asumas que es
   siempre la red global `coolify`: puede ser la red específica de la aplicación.
3. Introduce preferiblemente la IP fija del proxy `/32`, o la subred privada
   específica y aislada de ese ingress si su IP cambia. Nunca `0.0.0.0/0`.
4. Redeploy y verifica en los logs web que aparecen clientes distintos, no una
   sola IP Docker. Los logs no incluyen queries, cookies ni códigos OAuth.

Si ya tienes Cloudflare Tunnel, mantenlo apuntando al **proxy de Coolify**, no
directamente al backend o a Vite. Revisa también la cadena de proxies de
Traefik/Cloudflare: no habilites confianza indiscriminada en cabeceras reenviadas.
Si quieres la IP final detrás del túnel, el ingress debe validar al cloudflared
confiable y la cabecera de Cloudflare antes de reenviarla. Para esta app privada,
Cloudflare Access con allowlist del propietario y rate limiting añade protección.

## 3. Dominio y despliegue

En **Domains** asigna únicamente a **web**:

```text
https://orden.example.com:8080
```

Reemplaza el dominio. `:8080` es el puerto **interno** que Coolify debe usar;
la URL del navegador y `APP_ORIGIN` NO llevan ese puerto. Configura DNS/Tunnel y
HTTPS con el proxy existente. No crees otro túnel ni publiques 5432/8080 en Ubuntu.

Pulsa Deploy. Espera a que `db`, `backend` y `web` estén healthy. La primera
ejecución inicializa PostgreSQL y aplica migraciones automáticamente. Prueba:

- HTTPS válido y redirección HTTP → HTTPS en Coolify.
- `/api/accounts` sin sesión devuelve 401, nunca datos financieros.
- `/@fs/`, `/src/main.tsx`, `/.env` y archivos Excel no están disponibles.
- Respuestas con CSP, no-store para HTML/API, anti-frame y cookies Secure/HttpOnly.

## 4. Primer acceso y recuperación

En la terminal del contenedor **backend** de Coolify ejecuta:

```sh
orden activation-code
```

Usa el correo de `OWNER_EMAIL`, el código y una contraseña de al menos 12
caracteres. El código caduca en 15 minutos y no se reutiliza. No lo captures ni
publiques; no se puede generar por HTTP. Después de activar no hay signup.

Si pierdes el acceso, desde esa misma terminal:

```sh
orden recovery-code
```

Revoca las sesiones existentes y bloquea el login antiguo mientras el código es
válido. Nunca borres el volumen como forma de recuperar una contraseña.

## 5. Gmail opcional, cuando decidas habilitarlo

Activa Gmail API y crea un cliente Google OAuth de **Aplicación web**. Redirect:
`https://orden.example.com/api/gmail/callback`, usando exactamente tu dominio.
Scope: `https://www.googleapis.com/auth/gmail.readonly`. Agrega tu Gmail como
usuario de prueba si el consentimiento está en modo Testing.

Configura las tres variables opcionales en Coolify y redeploy. La clave AES debe
ser base64 de 32 bytes y mantenerse mientras haya conexiones activas. Asocia la
tarjeta CLP en Conexiones y autoriza. Testing suele caducar refresh tokens en
siete días; habilitar el permiso para terceros exige revisar las reglas de Google.

Orden solo importa compras MACH compatibles y autenticadas recibidas después de
conectar. Guarda identificadores para deduplicar incluso tras borrar un gasto;
no almacena el correo ni el comercio. Las compras sin cuenta se reintentan sin
bloquear páginas nuevas. Hay polling cada cinco minutos, cursor incremental y
solapamiento de cinco minutos para indexación tardía; un retraso de indexación
mayor necesita revisión/reconexión. Desconectar borra credenciales locales e
intenta revocación en Google: si falla, revoca desde tu cuenta de Google.

## Operación y datos

- PostgreSQL persiste en `postgres-data`, gestionado por Coolify. No uses
  `docker compose down -v`, ni recrees/borras el almacenamiento al redeploy.
- Cambiar `POSTGRES_PASSWORD` en el panel **no cambia** el rol de una base ya
  inicializada. Rota la contraseña dentro de PostgreSQL y actualiza el panel de
  manera coordinada; lo mismo aplica al administrador.
- No hay backup automático habilitado. Antes de guardar finanzas reales,
  configura y prueba tu política de backup/restauración en Coolify o el servidor.
  El volumen persistente no es un respaldo. No subas dumps a este repo.
- Guarda la clave AES junto a los backups en un lugar privado: perderla impide
  descifrar conexiones Gmail (puedes volver a autorizar, no recupera esa clave).
- Limita el panel Coolify y SSH; no expongas Docker socket o dashboards internos.
- Revisa actualizaciones de imágenes y dependencias; `main` no significa congelar
  parches de seguridad: prepara los cambios en otra rama, prueba y haz PR.
- Para actualizar paquetes del sistema no reutilices indefinidamente capas de
  build antiguas: fuerza rebuild sin cache y pull de bases en Coolify. CI escanea
  las tres imágenes y bloquea avisos HIGH/CRITICAL, incluidos los sin parche.

## Bloquear main y publicar esta preparación

La publicación inicial en `main` y la anonimización del historial fueron
autorizadas expresamente por el propietario. Para cambios posteriores usa una
rama feature/release y PR hacia `main`; no se publican automáticamente.

```sh
git switch -c release/production-bootstrap
# Revisa git status, añade los archivos de código/configuración y crea el commit.
python3 scripts/check-public-repo.py --history
```

En GitHub → Settings → Rules → Rulesets configura `main`: requerir pull request,
checks `Production checks / validate`, bloquear force-push/deletion y evitar
bypass de administradores si aplica. Activa Secret scanning/Push protection cuando
estén disponibles. Configura Coolify para desplegar únicamente `main`, no previews
de ramas con secretos/volúmenes de producción. Los hooks locales no sustituyen
a las reglas de GitHub y no se transmiten automáticamente al clonar.

```sh
git config core.hooksPath .githooks
# Copia el noreply exacto de GitHub → Settings → Emails (no el de OWNER_EMAIL):
git config --local user.email 'TU_ID+TU_USUARIO@users.noreply.github.com'
```

Si una clave ya fue pública, quitarla del archivo no basta: revócala primero y
después coordina limpieza del historial. Nunca reescribas historia pública sin
coordinar los clones y ramas; los correos de autor de Git también son públicos,
por lo que conviene usar un correo `noreply` para nuevos commits.

La auditoría detectó un correo personal en autor/committer del commit original.
El propietario autorizó sustituir esos metadatos por su usuario público y correo
GitHub noreply antes de esta publicación. Los clones/copias o caches anteriores
no se pueden borrar desde este repo. Los hooks nuevos bloquean commits locales con
correos no-noreply. Activa también "Keep my email addresses private" y "Block
command line pushes that expose my email" en GitHub para futuras publicaciones.
