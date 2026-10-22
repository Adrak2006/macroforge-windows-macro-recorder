//go:build windows

// hook.go: hooks globales de teclado y ratón (WH_KEYBOARD_LL / WH_MOUSE_LL).
//
// Los hooks se instalan en un hilo propio con su propio bucle de mensajes: si
// se instalaran en el hilo de la GUI, cualquier pausa de la ventana retrasaría
// TODO el teclado del sistema, y Windows acabaría desinstalando el hook por
// superar LowLevelHooksTimeout.
package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Los callbacks se crean una sola vez: cada syscall.NewCallback consume una
// entrada de una tabla global limitada.
var (
	cbTeclado = syscall.NewCallback(kbProc)
	cbRaton   = syscall.NewCallback(msProc)
)

const WM_ENDREC = WM_APP + 5

var (
	hookMu   sync.Mutex
	grabando atomic.Bool
	buffer   []Event
	ultimoEv time.Time
	ultimoMv time.Time

	// tragadas: teclas cuyo keydown hemos interceptado. El valor es el
	// remapeo que lo provocó (nil si solo había que tragarse la tecla).
	tragadas = map[uint32]*Remap{}

	capturando     atomic.Bool
	capturaMu      sync.Mutex
	capturaRes     Hotkey
	capturaHwnd    atomic.Uintptr
	dialogoAbierto atomic.Bool

	// eventos de la grabación recién terminada, a la espera de que el hilo
	// de la GUI los convierta en macro (guardar en disco desde el hook es
	// demasiado lento).
	pendientes []Event

	hookTeclado uintptr
	hookRaton   uintptr
)

func iniciarHooks() {
	go func() {
		runtime.LockOSThread() // los hooks pertenecen a ESTE hilo para siempre

		hookTeclado, _, _ = pSetWindowsHookEx.Call(WH_KEYBOARD_LL, cbTeclado, getModuleHandle(), 0)
		hookRaton, _, _ = pSetWindowsHookEx.Call(WH_MOUSE_LL, cbRaton, getModuleHandle(), 0)

		var msg MSG
		for {
			r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

func siguienteHook(hook uintptr, nCode, w, l uintptr) uintptr {
	r, _, _ := pCallNextHookEx.Call(hook, nCode, w, l)
	return r
}

// modsActuales lee el estado físico de los modificadores. GetAsyncKeyState
// refleja la tecla aunque el hook se la trague, que es justo lo que queremos.
func modsActuales() (ctrl, alt, shift, win bool) {
	return keyIsDown(VK_CONTROL), keyIsDown(VK_MENU), keyIsDown(VK_SHIFT),
		keyIsDown(VK_LWIN) || keyIsDown(VK_RWIN)
}

// ------------------------------------------------------------------ teclado

func kbProc(nCode, wparam, lparam uintptr) uintptr {
	if nCode != HC_ACTION {
		return siguienteHook(hookTeclado, nCode, wparam, lparam)
	}
	ks := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lparam))

	// Eventos que hemos inyectado nosotros: pasan de largo sin tocarlos.
	if ks.DwExtraInfo == firmaMF {
		return siguienteHook(hookTeclado, nCode, wparam, lparam)
	}

	vk := ks.VkCode
	down := wparam == WM_KEYDOWN || wparam == WM_SYSKEYDOWN
	inyectado := ks.Flags&LLKHF_INJECTED != 0

	// --- modo captura de atajo: nos tragamos todo hasta tener una tecla
	if capturando.Load() {
		if down && !esModificador(vk) {
			var h Hotkey
			if vk != VK_ESCAPE { // Esc cancela y devuelve una hotkey vacía
				c, a, s, w := modsActuales()
				h = Hotkey{VK: vk, Ctrl: c, Alt: a, Shift: s, Win: w}
			}
			capturaMu.Lock()
			capturaRes = h
			capturaMu.Unlock()
			capturando.Store(false)
			postMessage(capturaHwnd.Load(), WM_CAPTURED, 0, 0)
		}
		return 1
	}

	hookMu.Lock()
	defer hookMu.Unlock()

	if !down {
		// Cierre de una tecla que interceptamos al bajar.
		if r, ok := tragadas[vk]; ok {
			delete(tragadas, vk)
			if r != nil && r.Macro == "" && !r.To.Empty() {
				// Síncrono a propósito: con una goroutine el "soltar" podría
				// adelantar al "pulsar" en una pulsación muy rápida.
				enviarHotkey(r.To, false)
			}
			return 1
		}
	} else if _, ok := tragadas[vk]; ok {
		return 1 // autorrepetición de una tecla ya interceptada
	}

	if down && !inyectado && !dialogoAbierto.Load() {
		ctrl, alt, shift, win := modsActuales()

		cfgMu.RLock()
		hkGrabar := cfg.RecordHotkey
		hkParar := cfg.StopHotkey
		remaps := cfg.Remaps
		macros := cfg.Macros
		cfgMu.RUnlock()

		// 1) parar reproducción (solo se traga la tecla si hay algo sonando)
		if reproduciendo.Load() && hkParar.coincide(vk, ctrl, alt, shift, win) {
			pararReproduccion()
			tragadas[vk] = nil
			return 1
		}

		// 2) grabar / parar de grabar
		if hkGrabar.coincide(vk, ctrl, alt, shift, win) {
			alternarGrabacion()
			tragadas[vk] = nil
			return 1
		}

		// 3) remapeos
		for _, r := range remaps {
			if !r.Enabled || !r.From.coincide(vk, ctrl, alt, shift, win) {
				continue
			}
			tragadas[vk] = r
			if r.Macro != "" {
				if m := buscarMacroSeguro(r.Macro); m != nil {
					go reproducir(m)
				}
			} else if !r.To.Empty() {
				enviarHotkey(r.To, true)
			}
			return 1 // To vacío = la tecla queda anulada
		}

		// 4) atajos de macro
		for _, m := range macros {
			if m.Hotkey.coincide(vk, ctrl, alt, shift, win) {
				mm := m
				if reproduciendo.Load() {
					pararReproduccion()
				} else {
					go reproducir(mm)
				}
				tragadas[vk] = nil
				return 1
			}
		}
	}

	// 5) grabación
	if grabando.Load() && !inyectado {
		registrar(Event{
			Kind: EvKey,
			VK:   vk,
			Scan: ks.ScanCode,
			Up:   !down,
			Ext:  ks.Flags&LLKHF_EXTENDED != 0,
		})
	}

	return siguienteHook(hookTeclado, nCode, wparam, lparam)
}

// -------------------------------------------------------------------- ratón

func msProc(nCode, wparam, lparam uintptr) uintptr {
	if nCode != HC_ACTION {
		return siguienteHook(hookRaton, nCode, wparam, lparam)
	}
	if !grabando.Load() {
		return siguienteHook(hookRaton, nCode, wparam, lparam)
	}
	ms := (*MSLLHOOKSTRUCT)(unsafe.Pointer(lparam))
	if ms.DwExtraInfo == firmaMF || ms.Flags&LLMHF_INJECTED != 0 {
		return siguienteHook(hookRaton, nCode, wparam, lparam)
	}

	hookMu.Lock()
	switch wparam {
	case WM_MOUSEMOVE:
		cfgMu.RLock()
		grabarMov := cfg.RecordMouseMove
		cfgMu.RUnlock()
		// Un ratón normal genera ~125 muestras/s; una cada 8 ms basta y
		// evita macros de decenas de miles de eventos.
		if grabarMov && time.Since(ultimoMv) >= 8*time.Millisecond {
			ultimoMv = time.Now()
			registrar(Event{Kind: EvMouseMove, X: ms.Pt.X, Y: ms.Pt.Y})
		}
	case WM_LBUTTONDOWN, WM_LBUTTONUP, WM_RBUTTONDOWN, WM_RBUTTONUP,
		WM_MBUTTONDOWN, WM_MBUTTONUP, WM_XBUTTONDOWN, WM_XBUTTONUP:
		var btn uint32
		up := false
		switch wparam {
		case WM_LBUTTONDOWN:
			btn = BtnLeft
		case WM_LBUTTONUP:
			btn, up = BtnLeft, true
		case WM_RBUTTONDOWN:
			btn = BtnRight
		case WM_RBUTTONUP:
			btn, up = BtnRight, true
		case WM_MBUTTONDOWN:
			btn = BtnMiddle
		case WM_MBUTTONUP:
			btn, up = BtnMiddle, true
		case WM_XBUTTONDOWN, WM_XBUTTONUP:
			btn = BtnX1
			if ms.MouseData>>16 == 2 {
				btn = BtnX2
			}
			up = wparam == WM_XBUTTONUP
		}
		// Los clics llevan su posición: así una macro grabada sin
		// movimientos de ratón sigue haciendo clic donde tocaba.
		registrar(Event{Kind: EvMouseButton, Btn: btn, Up: up, X: ms.Pt.X, Y: ms.Pt.Y})
	case WM_MOUSEWHEEL, WM_MOUSEHWHEEL:
		registrar(Event{
			Kind:  EvWheel,
			Delta: int32(int16(ms.MouseData >> 16)),
			Horiz: wparam == WM_MOUSEHWHEEL,
			X:     ms.Pt.X, Y: ms.Pt.Y,
		})
	}
	hookMu.Unlock()

	return siguienteHook(hookRaton, nCode, wparam, lparam)
}

// ---------------------------------------------------------------- grabación

// registrar añade un evento al búfer. Debe llamarse con hookMu tomado.
func registrar(e Event) {
	ahora := time.Now()
	if len(buffer) > 0 {
		d := ahora.Sub(ultimoEv) / time.Millisecond
		if d < 0 {
			d = 0
		}
		e.Delay = uint32(d)
	}
	ultimoEv = ahora
	buffer = append(buffer, e)
}

func alternarGrabacion() {
	if grabando.Load() {
		grabando.Store(false)
		pendientes = buffer
		buffer = nil
		postMessage(hwndMain, WM_ENDREC, 0, 0)
		return
	}
	buffer = nil
	ultimoEv = time.Now()
	ultimoMv = time.Time{}
	grabando.Store(true)
	postMessage(hwndMain, WM_REFRESH, 0, 0)
}

// buscarMacroSeguro busca una macro tomando el candado de configuración.
func buscarMacroSeguro(nombre string) *Macro {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return buscarMacro(nombre)
}

// ------------------------------------------------------------------ captura

// pedirCaptura pone el hook en modo "la siguiente tecla es el atajo".
func pedirCaptura(hwnd uintptr) {
	capturaMu.Lock()
	capturaRes = Hotkey{}
	capturaMu.Unlock()
	capturaHwnd.Store(hwnd)
	capturando.Store(true)
}

func cancelarCaptura() { capturando.Store(false) }

func resultadoCaptura() Hotkey {
	capturaMu.Lock()
	defer capturaMu.Unlock()
	return capturaRes
}
