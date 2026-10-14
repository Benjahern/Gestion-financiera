# Backend de Orden

Go 1.26+, PostgreSQL 17, `pgx/v5` y `pgxpool`, HTTP estándar de Go.

## Estructura

- `cmd/orden`: servidor y comandos administrativos.
- `internal/orden`: configuración, middleware, autenticación, finanzas y pruebas.
- `internal/orden/migrations`: migraciones SQL embebidas y versionadas.

Las migraciones se aplican al iniciar, dentro de una transacción y con bloqueo
advisory para evitar carreras entre procesos. El usuario autorizado está fijado
en el servidor y reforzado con restricciones SQL.

## Comandos

Todos requieren la conexión PostgreSQL y `APP_ORIGIN` configurados:

```sh
go run ./cmd/orden serve
go run ./cmd/orden migrate
go run ./cmd/orden activation-code
go run ./cmd/orden recovery-code
```

El código solo aparece en la salida del comando, nunca en un endpoint ni en los
logs del servidor. Cada emisión invalida el código anterior del mismo tipo.
La activación no puede reabrirse después de crear al propietario. Emitir un código
de recuperación revoca inmediatamente todas las sesiones y bloquea el login con
la contraseña anterior durante los 15 minutos de validez. Al consumirlo se
cambia la contraseña, se invalidan los códigos y se crea una sesión nueva.

## API

Todas las rutas están bajo `/api`. Solo salud y los pasos de autenticación no
requieren sesión; incluso esos pasos verifican origen, JSON y límites de intentos.

| Método | Ruta | Función |
| --- | --- | --- |
| GET | `/health` | Disponibilidad, sin datos privados |
| POST | `/auth/start` | Correo autorizado y siguiente paso |
| POST | `/auth/activate` | Correo, código y contraseña nueva |
| POST | `/auth/login` | Correo y contraseña |
| POST | `/auth/recover` | Correo, código de recuperación y contraseña nueva |
| GET | `/auth/session` | Sesión y token CSRF |
| POST | `/auth/logout` | Revocar sesión actual |
| POST | `/auth/logout-all` | Revocar todas las sesiones |
| GET/POST | `/accounts` | Consultar/crear cuentas |
| PUT/DELETE | `/accounts/{id}` | Editar/eliminar cuenta |
| GET/POST | `/movements` | Consultar/crear movimientos |
| PUT/DELETE | `/movements/{id}` | Editar/eliminar movimiento |
| GET | `/overview?currency=CLP&month=2026-10` | Saldos y estadísticas |

Movimientos admite `currency`, `month`, `kind`, `category` exacta, `search` por
concepto, `limit` (máximo 100) y `offset`. Los importes son **strings decimales**
en JSON y `numeric` en PostgreSQL, nunca flotantes para almacenar/calcular dinero.
La moneda del movimiento la determina su cuenta. Una cuenta con movimientos no
puede borrarse ni cambiar de moneda. Las ediciones recalculan saldos automáticamente.

Las mutaciones autenticadas requieren `Origin` coincidente con `APP_ORIGIN`,
`Content-Type: application/json` y `X-CSRF-Token`. No hay CORS abierto.

## Límites y operación

- Contraseñas: mínimo 12 caracteres, máximo 256 bytes.
- Sesión: siete días de caducidad absoluta, revocable.
- Códigos: 256 bits aleatorios, un uso, 15 minutos.
- Login/activación/recuperación: diez intentos por operación cada 15 minutos.
- Inicio por correo: 30 intentos cada 15 minutos.

Los límites son globales por operación, deliberadamente conservadores para una
aplicación de un solo propietario detrás de un túnel. No se confía en cabeceras
IP aportadas por visitantes. Un atacante podría provocar un bloqueo temporal;
Cloudflare Access restringido a tu correo es una defensa adicional recomendable.

Los endpoints financieros siempre están asociados al propietario en el servidor.
Las tablas contienen restricciones de moneda, importes y relaciones; las consultas
usan parámetros. Los errores públicos no incluyen SQL, credenciales ni trazas.
La aplicación de Compose usa un rol PostgreSQL no superusuario.
