//go:build windows

// MacroForge: grabador/reproductor de macros y remapeador de teclas para
// Windows. Sin dependencias externas ni cgo: todo Win32 por syscall.
//
//	Uso: MacroForge.exe [-oculto]
//	  -oculto  arranca directamente en la bandeja del sistema
package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

func arranqueOculto() bool {
	for _, a := range os.Args[1:] {
		if a == "-oculto" || a == "/oculto" || a == "--oculto" {
			return true
		}
	}
	return false
}

// instanciaUnica evita dos MacroForge a la vez: dos juegos de hooks harían
// que cada tecla se procesara (y se remapeara) dos veces.
func instanciaUnica() bool {
	nombre := u16("MacroForgeSingleInstance")
	h, _, e := pCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(nombre)))
	if h == 0 {
		return true // si falla el mutex, mejor arrancar que no arrancar
	}
	const ERROR_ALREADY_EXISTS = 183
	if errno, ok := e.(syscall.Errno); ok && errno == ERROR_ALREADY_EXISTS {
		return false
	}
	return true
}

func main() {
	runtime.LockOSThread() // la GUI vive siempre en el mismo hilo

	if !instanciaUnica() {
		msgBox(0, "MacroForge ya se está ejecutando (mira la bandeja del sistema).",
			"MacroForge", MB_ICONINFO)
		return
	}

	// DPI real: sin esto las coordenadas del hook (físicas) y las de
	// SendInput no cuadran en pantallas escaladas.
	if pSetProcDpiCtx.Find() == nil {
		pSetProcDpiCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 == (HANDLE)-4
	} else {
		pSetProcessDPIAware.Call()
	}

	// Resolución de 1 ms para los time.Sleep entre eventos.
	pTimeBeginPeriod.Call(1)

	cargarConfig()
	registrarClaseDialogo()
	crearVentana(!arranqueOculto())
	crearIconoBandeja()
	iniciarHooks()
	setEstado("Listo — " + textoAyuda())

	var msg MSG
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if r2, _, _ := pIsDialogMessage.Call(hwndMain, uintptr(unsafe.Pointer(&msg))); r2 != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	guardarConfig()
}

func textoAyuda() string {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg.RecordHotkey.String() + " graba/para de grabar · " +
		cfg.StopHotkey.String() + " detiene la reproducción · la X esconde en la bandeja"
}
