# MacroForge

> **Estado:** funcional, pero con pocas pruebas en Windows real (sobre todo DPI, bandeja y
> diálogos). Si algo falla, abre un issue con tu versión de Windows y escalado de pantalla.

Grabador/reproductor de macros y remapeador de teclas para Windows x64.
Go puro sobre Win32 (`syscall`), **sin cgo y sin dependencias**: un único .exe
de ~2 MB que no instala nada.

## Uso rápido

| Acción | Cómo |
|---|---|
| Grabar | **F9** (o el botón *Grabar nueva*). Vuelve a pulsar F9 para terminar. |
| Reproducir | Doble clic en la macro, botón *Reproducir*, o su atajo propio |
| Detener | **Esc** mientras suena |
| Ocultar | La **X** manda el programa a la bandeja (los hooks siguen vivos) |
| Salir de verdad | Clic derecho en el icono de la bandeja → *Salir* |

`MacroForge.exe -oculto` arranca directamente en la bandeja (útil si lo pones
en el arranque de Windows).

### Macros

Se graban teclado, clics, rueda y (opcional) movimientos del ratón, con los
tiempos reales entre eventos. Por macro se puede ajustar:

- **Atajo**: cualquier combinación; volver a pulsarlo detiene la reproducción.
- **Repeticiones**: `0` = bucle infinito hasta pulsar Esc.
- **Velocidad**: multiplicador (2 = el doble de rápido, 0.5 = la mitad).

Los clics guardan su posición aunque desactives la grabación de movimientos,
así que una macro "sin ratón" sigue haciendo clic donde toca.

### Remapeos

- **tecla → tecla**: por ejemplo BloqMayús → Ctrl+C.
- **tecla → macro**: la tecla lanza la macro seleccionada.
- **anular tecla**: al pedir el destino, cancela y confirma "anular".

Los remapeos se pueden activar/desactivar sin borrarlos.

## Detalles que importan

- **Coordenadas**: el proceso es *per-monitor DPI aware*, así que las
  posiciones grabadas coinciden con las reproducidas aunque tengas escalado
  al 125/150 % o varios monitores (se usa el escritorio virtual completo).
- **Modo juego**: la casilla *scancodes* hace que las teclas se inyecten por
  scancode (`KEYEVENTF_SCANCODE`) en vez de por código virtual. Muchos juegos
  con DirectInput/RawInput solo leen eso. Actívala si una macro funciona en el
  Bloc de notas pero no en el juego.
- **Anti-bucle**: todo lo que inyectamos va marcado en `dwExtraInfo`, de modo
  que el hook lo ignora y una macro nunca se graba a sí misma.
- **Sin teclas clavadas**: si abortas a mitad, se sueltan automáticamente las
  teclas y botones que la macro había dejado pulsados.
- **Una sola instancia**: un mutex impide arrancar dos veces (dos juegos de
  hooks procesarían cada tecla dos veces).
- Los hooks corren en **su propio hilo** con su bucle de mensajes: si vivieran
  en el hilo de la ventana, cualquier atasco de la GUI retrasaría todo el
  teclado del sistema y Windows acabaría desinstalando el hook.

## Limitaciones conocidas

- Para que funcione sobre ventanas **elevadas** (juegos anticheat, apps como
  administrador), MacroForge tiene que ejecutarse también como administrador;
  si no, Windows bloquea la inyección (UIPI).
- No se puede usar un **modificador suelto** (Ctrl, Alt, Win, Mayús) como
  origen de un remapeo o de un atajo. BloqMayús sí vale.
- El origen de un remapeo es de teclado; los botones del ratón se graban en
  macros pero no sirven como disparador.
- Los antivirus miran con lupa cualquier cosa con hooks globales de teclado.
  Es un falso positivo esperable en un binario Go sin firmar.

## Archivos

	main.go     arranque, instancia única, DPI, bucle de mensajes
	gui.go      ventana, listas, diálogos, bandeja
	hook.go     WH_KEYBOARD_LL / WH_MOUSE_LL, grabación, remapeos
	play.go     SendInput, reproducción, temporización
	config.go   modelo de datos + JSON
	win32.go    enlaces a la API de Windows

Configuración: `%AppData%\MacroForge\config.json` (JSON legible y editable).

## Compilar

	./build.sh      # desde Linux, cross-compile a Windows x64

Requisitos: Go ≥ 1.21. No hace falta cgo ni MinGW. En Windows también vale:

	set GOOS=windows& set GOARCH=amd64& go build -trimpath -ldflags="-H windowsgui -s -w" -o MacroForge.exe .

## English summary

**MacroForge** is a tiny macro recorder/player and key remapper for Windows x64, written in
pure Go on top of Win32 (`syscall`, no cgo, no dependencies, single ~2 MB exe). Record
keyboard/mouse with real timings (F9), replay with per-macro hotkeys, repeat count and speed,
remap key→key or key→macro, scancode "game mode", DPI-aware coordinates, tray icon.
Build with `./build.sh` (cross-compiles from Linux). Issues and PRs welcome.

## Licencia

MIT (ver `LICENSE`).
