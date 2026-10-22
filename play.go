//go:build windows

// play.go: reproducción de macros e inyección de eventos con SendInput.
package main

import (
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	reproduciendo atomic.Bool
	pararFlag     atomic.Bool
)

// ------------------------------------------------------------- SendInput

func enviarInputs(ins []INPUT) {
	if len(ins) == 0 {
		return
	}
	pSendInput.Call(uintptr(len(ins)), uintptr(unsafe.Pointer(&ins[0])),
		unsafe.Sizeof(INPUT{}))
}

// enviarTecla inyecta una pulsación. En "modo juego" se manda por scancode
// (wVk=0), que es lo único que leen muchos juegos con DirectInput.
func enviarTecla(vk, scan uint32, up, ext bool) {
	cfgMu.RLock()
	modoScan := cfg.ScanMode
	cfgMu.RUnlock()

	var in INPUT
	in.Type = INPUT_KEYBOARD
	k := in.key()
	k.DwExtraInfo = firmaMF
	if up {
		k.DwFlags |= KEYEVENTF_KEYUP
	}
	if ext {
		k.DwFlags |= KEYEVENTF_EXTENDEDKEY
	}
	if modoScan {
		if scan == 0 {
			scan = mapVirtualKey(vk, 0)
		}
		if scan != 0 {
			k.WScan = uint16(scan)
			k.DwFlags |= KEYEVENTF_SCANCODE
			enviarInputs([]INPUT{in})
			return
		}
	}
	k.WVk = uint16(vk)
	k.WScan = uint16(scan)
	enviarInputs([]INPUT{in})
}

func aAbsoluto(x, y int32) (int32, int32) {
	vx := getSystemMetrics(SM_XVIRTUALSCREEN)
	vy := getSystemMetrics(SM_YVIRTUALSCREEN)
	vw := getSystemMetrics(SM_CXVIRTUALSCREEN)
	vh := getSystemMetrics(SM_CYVIRTUALSCREEN)
	if vw < 2 {
		vw = 2
	}
	if vh < 2 {
		vh = 2
	}
	nx := int32((int64(x-vx) * 65535) / int64(vw-1))
	ny := int32((int64(y-vy) * 65535) / int64(vh-1))
	return nx, ny
}

func enviarRaton(flags uint32, x, y int32, data uint32) {
	var in INPUT
	in.Type = INPUT_MOUSE
	m := in.mouse()
	m.DwExtraInfo = firmaMF
	m.DwFlags = flags
	m.MouseData = data
	if flags&MOUSEEVENTF_ABSOLUTE != 0 {
		m.Dx, m.Dy = aAbsoluto(x, y)
	}
	enviarInputs([]INPUT{in})
}

var flagsBoton = map[uint32][2]uint32{
	BtnLeft:   {MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP},
	BtnRight:  {MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP},
	BtnMiddle: {MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP},
	BtnX1:     {MOUSEEVENTF_XDOWN, MOUSEEVENTF_XUP},
	BtnX2:     {MOUSEEVENTF_XDOWN, MOUSEEVENTF_XUP},
}

func enviarBoton(btn uint32, up bool, x, y int32) {
	f, ok := flagsBoton[btn]
	if !ok {
		return
	}
	idx := 0
	if up {
		idx = 1
	}
	var data uint32
	if btn == BtnX1 {
		data = 1
	} else if btn == BtnX2 {
		data = 2
	}
	// El clic va acompañado del movimiento absoluto para que caiga donde se grabó.
	enviarRaton(f[idx]|MOUSEEVENTF_MOVE|MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK, x, y, data)
}

// enviarHotkey pulsa (o suelta) una combinación completa.
func enviarHotkey(h Hotkey, down bool) {
	if h.Empty() {
		return
	}
	mods := []struct {
		on bool
		vk uint32
	}{{h.Ctrl, VK_CONTROL}, {h.Alt, VK_MENU}, {h.Shift, VK_SHIFT}, {h.Win, VK_LWIN}}

	if down {
		for _, m := range mods {
			if m.on {
				enviarTecla(m.vk, mapVirtualKey(m.vk, 0), false, false)
			}
		}
		enviarTecla(h.VK, mapVirtualKey(h.VK, 0), false, esExtendida(h.VK))
		return
	}
	enviarTecla(h.VK, mapVirtualKey(h.VK, 0), true, esExtendida(h.VK))
	for i := len(mods) - 1; i >= 0; i-- {
		if mods[i].on {
			enviarTecla(mods[i].vk, mapVirtualKey(mods[i].vk, 0), true, false)
		}
	}
}

func esExtendida(vk uint32) bool {
	switch vk {
	case VK_INSERT, VK_DELETE, VK_HOME, VK_END, VK_PRIOR, VK_NEXT,
		VK_LEFT, VK_RIGHT, VK_UP, VK_DOWN, VK_NUMLOCK, VK_SNAPSHOT,
		VK_RCONTROL, VK_RMENU, VK_LWIN, VK_RWIN:
		return true
	}
	return false
}

// ------------------------------------------------------------ reproducción

func pararReproduccion() { pararFlag.Store(true) }

// dormir espera troceando el tiempo para poder abortar al instante.
func dormir(d time.Duration) bool {
	const trozo = 15 * time.Millisecond
	for d > 0 {
		if pararFlag.Load() {
			return false
		}
		t := d
		if t > trozo {
			t = trozo
		}
		time.Sleep(t)
		d -= t
	}
	return !pararFlag.Load()
}

func reproducir(m *Macro) {
	if m == nil || len(m.Events) == 0 {
		return
	}
	if !reproduciendo.CompareAndSwap(false, true) {
		return // ya hay una macro en marcha
	}
	pararFlag.Store(false)
	setEstado(fmt.Sprintf("Reproduciendo «%s»…  (%s para parar)", m.Name, estadoStopHotkey()))
	refrescarGUI()

	// Registro de lo que dejamos pulsado, para no dejar teclas clavadas si
	// se aborta a mitad.
	teclasPulsadas := map[uint32]Event{}
	botonesPulsados := map[uint32]Event{}

	defer func() {
		for _, e := range teclasPulsadas {
			enviarTecla(e.VK, e.Scan, true, e.Ext)
		}
		for _, e := range botonesPulsados {
			enviarBoton(e.Btn, true, e.X, e.Y)
		}
		reproduciendo.Store(false)
		pararFlag.Store(false)
		setEstado("Listo")
		refrescarGUI()
	}()

	vel := m.Speed
	if vel <= 0 {
		vel = 1
	}

	for rep := 0; m.Repeat == 0 || rep < m.Repeat; rep++ {
		for _, e := range m.Events {
			if e.Delay > 0 {
				d := time.Duration(float64(e.Delay) * float64(time.Millisecond) / vel)
				if !dormir(d) {
					return
				}
			} else if pararFlag.Load() {
				return
			}

			switch e.Kind {
			case EvKey:
				enviarTecla(e.VK, e.Scan, e.Up, e.Ext)
				if e.Up {
					delete(teclasPulsadas, e.VK)
				} else {
					teclasPulsadas[e.VK] = e
				}
			case EvMouseMove:
				enviarRaton(MOUSEEVENTF_MOVE|MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK, e.X, e.Y, 0)
			case EvMouseButton:
				enviarBoton(e.Btn, e.Up, e.X, e.Y)
				if e.Up {
					delete(botonesPulsados, e.Btn)
				} else {
					botonesPulsados[e.Btn] = e
				}
			case EvWheel:
				f := uint32(MOUSEEVENTF_WHEEL)
				if e.Horiz {
					f = MOUSEEVENTF_HWHEEL
				}
				enviarRaton(f, 0, 0, uint32(e.Delta))
			}
		}
	}
}

func estadoStopHotkey() string {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg.StopHotkey.String()
}
