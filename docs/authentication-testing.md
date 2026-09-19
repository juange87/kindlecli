# Prueba de autenticación de Amazon

## Qué está implementado

- Diagnóstico real de la sesión con `auth status`, también en JSON.
- Recuperación desde `login`, sin ejecutar `logout` primero.
- Reutilización de la sesión del navegador cuando Amazon lo permite. `--fresh`
  permite solicitar autenticación nueva; para probarlo con una sesión CLI válida,
  combinar con `--force`.
- Login en dos pasos con PKCE, almacenamiento privado y caducidad de 10 minutos.
- Conservación del archivo de credenciales anterior hasta guardar el nuevo.
  Esto no garantiza que Amazon mantenga vigente el registro anterior: el servicio
  controla la validez del dispositivo durante el registro remoto.
- Diagnóstico de presencia de `refresh_token` con `--verbose`, sin mostrar su
  contenido ni guardarlo. No se ha implementado renovación silenciosa.

## Prueba recomendada en este Mac

Desde la raíz del repositorio:

```bash
go build -o kindlecli .
./kindlecli auth status --json
./kindlecli login --verbose
./kindlecli auth status --json
./kindlecli devices
./kindlecli login
```

1. La sesión anterior devolvió `rejected` el 19 de septiembre de 2026. El primer
   comando de estado debe terminar con código distinto de cero mientras siga así.
2. `login` abre el navegador. Si ya hay una sesión de Amazon, observar si entra
   directamente o pide contraseña/MFA. Completar cualquier desafío personalmente.
3. Al llegar a Send to Kindle, pegar la URL completa en el prompt **del proceso
   login**, no en el shell ni en una conversación. Alternativamente usar
   `./kindlecli login --verbose --clipboard`, copiar la URL y pulsar Enter.
4. El segundo `auth status` debe devolver `valid` y código 0. `devices` debe listar
   los dispositivos. No hace falta enviar un documento para validar autenticación.
5. El último `login` debe informar que la sesión es válida sin abrir el navegador.

Información útil para continuar: si Amazon pidió contraseña/MFA, si el estado
final fue `valid`, y la línea `Refresh token returned: true/false`. No compartir
URLs de retorno, códigos, contraseñas, claves ni los archivos JSON de credenciales.

No ejecutar `logout` como preparación de la prueba: daría de baja el dispositivo.
No usar otra carpeta de configuración como supuesto aislamiento remoto: el
protocolo heredado usa un número de serie fijo en el registro de Amazon.

## Prueba del flujo para una skill

Si la CLI ya tiene una sesión válida, `--start` devolverá `valid`. Solo para probar
un nuevo registro de forma deliberada, usar `--force --start`.

```bash
./kindlecli login --start --json
./kindlecli login --finish --json
./kindlecli auth status --json
```

El agente debe leer `state`. Para `pending`, abre `url` en el navegador habitual,
conserva la URL de retorno localmente y la entrega a stdin de `--finish`, terminada
en salto de línea. No interpolarla en un comando de shell ni imprimirla en el chat.
El proceso `--finish` necesita el mismo directorio `--config` que `--start`.
Con herramientas que solo permiten copiar desde el navegador, puede usar
`--finish --clipboard --json`.

La CLI no controla por sí sola el navegador: estos comandos proporcionan la
interfaz para que una skill con herramientas de navegador complete el flujo.
Si aparece un desafío de Amazon, el agente debe dejarlo al usuario. Un fallo
`unavailable` requiere resolver red/servicio, no reiniciar automáticamente el login.
Un `rejected` permite iniciar el flujo de recuperación. Una skill no debe repetir
un lote completo que tuvo envíos aceptados.

Un intento caducado necesita un nuevo `--start`. Un nuevo inicio sustituye al
anterior. Si un proceso muere durante login, comprobar que ya no está ejecutándose
y eliminar únicamente el directorio vacío `.login-lock` antes de reintentar.

## Investigación de renovación silenciosa

El token OAuth de acceso se usa para registrar el dispositivo; las operaciones
posteriores se firman con una clave RSA y un token ADP. Caducar un access token no
implica por sí mismo caducar el dispositivo.

El cliente upstream mantiene este mismo intercambio `DeviceLegacy`, solicita
`access_token` y registra un dispositivo descrito como duradero. No ofrece en ese
módulo una implementación de renovación:
https://github.com/maxdjohnson/stkclient/blob/main/src/stkclient/api.py

Amazon documenta refresh tokens para Login with Amazon. Esa documentación no
confirma que el cliente interno `device_auth_access` de Send to Kindle acepte el
mismo flujo. No debemos añadir a ciegas `grant_type=refresh_token` ni un callback
localhost: el cliente y su URL de retorno no son nuestros.

- https://developer.amazon.com/docs/login-with-amazon/refresh-token.html
- https://developer.amazon.com/docs/login-with-amazon/authorization-code-grant.html
- https://openid.net/specs/openid-provider-authentication-policy-extension-1_0.html

El próximo experimento es observar únicamente si el intercambio actual devuelve
`refresh_token`. `true` indicaría una vía que investigar; no demostraría todavía
que permita renovar el registro del dispositivo. `false` tampoco demuestra que
ningún otro flujo interno de Amazon lo soporte. Antes de implementar renovación,
necesitamos demostrar el intercambio y el registro con ese token y verificar
`GetListOfOwnedDevices` sin abrir el navegador. Requerirá una sesión real; no se
puede confirmar con los mocks de los tests ni con la sesión antigua rechazada.

## Resultado de la prueba del 19 de septiembre de 2026

El login interactivo terminó correctamente, aunque Amazon pidió un OTP al usuario.
El intercambio devolvió `Refresh token returned: false`. Después, las consultas
reales confirmaron `auth status = valid` y el listado de dispositivos; una segunda
invocación de `login` reutilizó la sesión sin abrir el navegador. Queda validada la
recuperación y reutilización de la sesión CLI, pero no un login de navegador sin
intervención ni la renovación silenciosa del token de dispositivo.
