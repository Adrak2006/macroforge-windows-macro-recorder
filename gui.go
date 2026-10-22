//go:build windows

// gui.go: ventana de gestión en Win32 puro (sin toolkits, sin cgo).
package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	idLstMacros = 1001
	idLstRemaps = 1002

	idGrabar       = 2001
	idReproducir   = 2002
	idParar        = 2003
	idAtajo        = 2004
	idRepeticiones = 2005
	idVelocidad    = 2006
	idRenombrar    = 2007
	idEliminar     = 2008

	idRemapTecla  = 2101
	idRemapMacro  = 2102
	idRemapQuitar = 2103
	idRemapAlt    = 2104

	idChkMouse  = 2201
	idChkScan   = 2202
	idTeclasGlb = 2203

	idTrayMostrar = 3001
	idTraySalir   = 3002

	// diálogos
	idDlgOK       = 1
	idDlgCancelar = 2
	idDlgEdit     = 3
)

var (
	hwndMain  uintptr
	lstMacros uintptr
	lstRemaps uintptr
	chkMouse  uintptr
	chkScan   uintptr
	lblEstado uintptr

	fuenteUI   uintptr
	fuenteMono uintptr
	dpi        int32 = 96

	estadoMu  sync.Mutex
	estadoTxt = "Listo"

	iconoBandeja NOTIFYICONDATA

	cbWndProc = syscall.NewCallback(wndProc)
	cbDlgProc = syscall.NewCallback(dlgProc)
)

func sc(v int32) int32 { return v * dpi / 96 }

// ------------------------------------------------------------------ estado

func setEstado(s string) {
	estadoMu.Lock()
	estadoTxt = s
	estadoMu.Unlock()
	postMessage(hwndMain, WM_SETSTATUS, 0, 0)
}

func refrescarGUI() { postMessage(hwndMain, WM_REFRESH, 0, 0) }

func estadoActual() string {
	if grabando.Load() {
		cfgMu.RLock()
		hk := cfg.RecordHotkey.String()
		cfgMu.RUnlock()
		return fmt.Sprintf("● GRABANDO…  pulsa %s para terminar", hk)
	}
	estadoMu.Lock()
	defer estadoMu.Unlock()
	return estadoTxt
}

// ---------------------------------------------------------------- creación

func crearControl(clase, texto string, estilo uint32, x, y, w, h int32, id uintptr, padre uintptr, exEstilo uint32, fuente uintptr) uintptr {
	hwnd, _, _ := pCreateWindowEx.Call(
		uintptr(exEstilo),
		uintptr(unsafe.Pointer(u16(clase))),
		uintptr(unsafe.Pointer(u16(texto))),
		uintptr(estilo),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		padre, id, getModuleHandle(), 0)
	if hwnd != 0 && fuente != 0 {
		sendMessage(hwnd, WM_SETFONT, fuente, 1)
	}
	return hwnd
}

func crearFuente(pt int32, cara string) uintptr {
	alto := -(pt * dpi / 72)
	h, _, _ := pCreateFont.Call(
		uintptr(alto), 0, 0, 0, 400, 0, 0, 0,
		0 /*ANSI_CHARSET*/, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(u16(cara))))
	return h
}

func crearVentana(mostrar bool) {
	nombreClase := u16("MacroForgeMain")
	wc := WNDCLASSEX{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:     CS_HREDRAW | CS_VREDRAW,
		WndProc:   cbWndProc,
		Instance:  getModuleHandle(),
		ClassName: nombreClase,
	}
	wc.Cursor, _, _ = pLoadCursor.Call(0, IDC_ARROW)
	wc.Icon, _, _ = pLoadIcon.Call(0, IDI_APPLICATION)
	wc.IconSm = wc.Icon
	wc.Background = COLOR_BTNFACE + 1
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	estilo := uint32(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)
	hwndMain, _, _ = pCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(nombreClase)),
		uintptr(unsafe.Pointer(u16("MacroForge"))),
		uintptr(estilo),
		0x80000000, 0x80000000, 100, 100, // CW_USEDEFAULT
		0, 0, getModuleHandle(), 0)

	if r, _, _ := pGetDpiForWindow.Call(hwndMain); r >= 72 {
		dpi = int32(r)
	}
	fuenteUI = crearFuente(9, "Segoe UI")
	fuenteMono = crearFuente(9, "Consolas")

	// Tamaño de cliente 800x556 escalado por DPI, centrado en pantalla.
	rc := RECT{0, 0, sc(800), sc(556)}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), uintptr(estilo), 0)
	ancho, alto := rc.Right-rc.Left, rc.Bottom-rc.Top
	cx := getSystemMetrics(0) // SM_CXSCREEN
	cy := getSystemMetrics(1)
	pMoveWindow.Call(hwndMain, uintptr((cx-ancho)/2), uintptr((cy-alto)/2),
		uintptr(ancho), uintptr(alto), 1)

	crearControles()
	actualizarChecks()
	refrescarListas()
	actualizarEstado()

	if mostrar {
		pShowWindow.Call(hwndMain, SW_SHOW)
		pUpdateWindow.Call(hwndMain)
	}
}

func crearControles() {
	const (
		est   = uint32(WS_CHILD | WS_VISIBLE)
		estBt = est | WS_TABSTOP | BS_PUSHBUTTON
		estLs = est | WS_TABSTOP | WS_VSCROLL | WS_BORDER | LBS_NOTIFY | LBS_HASSTRINGS
		estSt = est | SS_LEFT
		estCk = est | WS_TABSTOP | BS_AUTOCHECKBOX
	)
	bx, bw, bh, sep := int32(566), int32(220), int32(28), int32(33)

	crearControl("STATIC", "Macros grabadas", estSt, sc(14), sc(12), sc(400), sc(18), 0, hwndMain, 0, fuenteUI)
	lstMacros = crearControl("LISTBOX", "", estLs, sc(14), sc(34), sc(540), sc(232), idLstMacros, hwndMain, 0, fuenteMono)

	y := int32(34)
	botones := []struct {
		id  uintptr
		txt string
	}{
		{idGrabar, "Grabar nueva"},
		{idReproducir, "Reproducir"},
		{idParar, "Detener"},
		{idAtajo, "Asignar atajo…"},
		{idRepeticiones, "Repeticiones…"},
		{idVelocidad, "Velocidad…"},
		{idRenombrar, "Renombrar…"},
		{idEliminar, "Eliminar"},
	}
	for _, b := range botones {
		crearControl("BUTTON", b.txt, estBt, sc(bx), sc(y), sc(bw), sc(bh), b.id, hwndMain, 0, fuenteUI)
		y += sep
	}

	chkMouse = crearControl("BUTTON", "Grabar también los movimientos del ratón", estCk,
		sc(14), sc(276), sc(360), sc(22), idChkMouse, hwndMain, 0, fuenteUI)
	chkScan = crearControl("BUTTON", "Modo juego (enviar scancodes)", estCk,
		sc(310), sc(276), sc(240), sc(22), idChkScan, hwndMain, 0, fuenteUI)
	crearControl("BUTTON", "Teclas globales…", estBt,
		sc(bx), sc(272), sc(bw), sc(26), idTeclasGlb, hwndMain, 0, fuenteUI)

	crearControl("STATIC", "Remapeos de teclas", estSt, sc(14), sc(308), sc(400), sc(18), 0, hwndMain, 0, fuenteUI)
	lstRemaps = crearControl("LISTBOX", "", estLs, sc(14), sc(330), sc(540), sc(168), idLstRemaps, hwndMain, 0, fuenteMono)

	y = 330
	remBotones := []struct {
		id  uintptr
		txt string
	}{
		{idRemapTecla, "Nuevo: tecla → tecla"},
		{idRemapMacro, "Nuevo: tecla → macro"},
		{idRemapAlt, "Activar / desactivar"},
		{idRemapQuitar, "Quitar remapeo"},
	}
	for _, b := range remBotones {
		crearControl("BUTTON", b.txt, estBt, sc(bx), sc(y), sc(bw), sc(bh), b.id, hwndMain, 0, fuenteUI)
		y += sep
	}

	lblEstado = crearControl("STATIC", "", estSt, sc(14), sc(514), sc(772), sc(20), 0, hwndMain, 0, fuenteUI)
}

func actualizarChecks() {
	cfgMu.RLock()
	m, s := cfg.RecordMouseMove, cfg.ScanMode
	cfgMu.RUnlock()
	sendMessage(chkMouse, BM_SETCHECK, boolAUint(m), 0)
	sendMessage(chkScan, BM_SETCHECK, boolAUint(s), 0)
}

func boolAUint(b bool) uintptr {
	if b {
		return BST_CHECKED
	}
	return 0
}

func actualizarEstado() { setWindowText(lblEstado, estadoActual()) }

func refrescarListas() {
	selM := int32(sendMessage(lstMacros, LB_GETCURSEL, 0, 0))
	selR := int32(sendMessage(lstRemaps, LB_GETCURSEL, 0, 0))

	sendMessage(lstMacros, LB_RESETCONTENT, 0, 0)
	sendMessage(lstRemaps, LB_RESETCONTENT, 0, 0)

	cfgMu.RLock()
	for _, m := range cfg.Macros {
		p := u16(m.Etiqueta())
		sendMessage(lstMacros, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p)))
	}
	for _, r := range cfg.Remaps {
		p := u16(r.Etiqueta())
		sendMessage(lstRemaps, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p)))
	}
	nm, nr := len(cfg.Macros), len(cfg.Remaps)
	cfgMu.RUnlock()

	if nm > 0 {
		if selM < 0 || selM >= int32(nm) {
			selM = int32(nm) - 1
		}
		sendMessage(lstMacros, LB_SETCURSEL, uintptr(selM), 0)
	}
	if nr > 0 {
		if selR < 0 || selR >= int32(nr) {
			selR = int32(nr) - 1
		}
		sendMessage(lstRemaps, LB_SETCURSEL, uintptr(selR), 0)
	}
}

func macroSeleccionada() *Macro {
	i := int32(sendMessage(lstMacros, LB_GETCURSEL, 0, 0))
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	if i < 0 || int(i) >= len(cfg.Macros) {
		return nil
	}
	return cfg.Macros[i]
}

func indiceRemap() int {
	i := int32(sendMessage(lstRemaps, LB_GETCURSEL, 0, 0))
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	if i < 0 || int(i) >= len(cfg.Remaps) {
		return -1
	}
	return int(i)
}

// -------------------------------------------------------------- WndProc

func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		alComando(uint32(wparam&0xFFFF), uint32((wparam>>16)&0xFFFF))
		return 0

	case WM_REFRESH:
		refrescarListas()
		actualizarEstado()
		return 0

	case WM_SETSTATUS:
		actualizarEstado()
		return 0

	case WM_ENDREC:
		finalizarGrabacion()
		return 0

	case WM_TRAY:
		switch uint32(lparam) {
		case WM_LBUTTONDOWN, WM_LBUTTONUP:
			mostrarVentana()
		case WM_RBUTTONUP:
			menuBandeja()
		}
		return 0

	case WM_CLOSE:
		// La X esconde en la bandeja: los hooks tienen que seguir vivos.
		pShowWindow.Call(hwnd, SW_HIDE)
		return 0

	case WM_DESTROY:
		pShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&iconoBandeja)))
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

func mostrarVentana() {
	pShowWindow.Call(hwndMain, SW_RESTORE)
	pSetForegroundWin.Call(hwndMain)
}

func menuBandeja() {
	menu, _, _ := pCreatePopupMenu.Call()
	pAppendMenu.Call(menu, MF_STRING, idTrayMostrar, uintptr(unsafe.Pointer(u16("Mostrar MacroForge"))))
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	pAppendMenu.Call(menu, MF_STRING, idTraySalir, uintptr(unsafe.Pointer(u16("Salir"))))
	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWin.Call(hwndMain) // si no, el menú no se cierra al hacer clic fuera
	r, _, _ := pTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y), 0, hwndMain, 0)
	pDestroyMenu.Call(menu)
	switch r {
	case idTrayMostrar:
		mostrarVentana()
	case idTraySalir:
		pDestroyWindow.Call(hwndMain)
	}
}

// --------------------------------------------------------------- comandos

func alComando(id, notif uint32) {
	switch id {
	case idLstMacros:
		if notif == LBN_DBLCLK {
			reproducirSeleccionada()
		}
		return
	case idLstRemaps:
		if notif == LBN_DBLCLK {
			alternarRemap()
		}
		return

	case idGrabar:
		hookMu.Lock()
		alternarGrabacion()
		hookMu.Unlock()

	case idReproducir:
		reproducirSeleccionada()

	case idParar:
		pararReproduccion()

	case idAtajo:
		m := macroSeleccionada()
		if m == nil {
			return
		}
		if h, ok := pedirAtajo("Atajo para «" + m.Name + "»"); ok {
			cfgMu.Lock()
			m.Hotkey = h
			cfgMu.Unlock()
			salvar()
		}

	case idRepeticiones:
		m := macroSeleccionada()
		if m == nil {
			return
		}
		txt, ok := pedirTexto("Repeticiones",
			"Nº de repeticiones (0 = bucle infinito):", strconv.Itoa(m.Repeat))
		if !ok {
			return
		}
		n, err := strconv.Atoi(strings.TrimSpace(txt))
		if err != nil || n < 0 {
			msgBox(hwndMain, "Escribe un número entero de 0 en adelante.", "MacroForge", MB_ICONWARNING)
			return
		}
		cfgMu.Lock()
		m.Repeat = n
		cfgMu.Unlock()
		salvar()

	case idVelocidad:
		m := macroSeleccionada()
		if m == nil {
			return
		}
		txt, ok := pedirTexto("Velocidad",
			"Multiplicador de velocidad (1 = original, 2 = el doble de rápido):",
			strconv.FormatFloat(m.Speed, 'g', -1, 64))
		if !ok {
			return
		}
		v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(txt), ",", ".", 1), 64)
		if err != nil || v <= 0 {
			msgBox(hwndMain, "Escribe un número mayor que 0.", "MacroForge", MB_ICONWARNING)
			return
		}
		cfgMu.Lock()
		m.Speed = v
		cfgMu.Unlock()
		salvar()

	case idRenombrar:
		m := macroSeleccionada()
		if m == nil {
			return
		}
		txt, ok := pedirTexto("Renombrar", "Nuevo nombre:", m.Name)
		if !ok {
			return
		}
		nuevo := strings.TrimSpace(txt)
		if nuevo == "" || nuevo == m.Name {
			return
		}
		cfgMu.Lock()
		if buscarMacro(nuevo) != nil {
			cfgMu.Unlock()
			msgBox(hwndMain, "Ya hay una macro con ese nombre.", "MacroForge", MB_ICONWARNING)
			return
		}
		viejo := m.Name
		m.Name = nuevo
		for _, r := range cfg.Remaps { // los remapeos apuntan por nombre
			if r.Macro == viejo {
				r.Macro = nuevo
			}
		}
		cfgMu.Unlock()
		salvar()

	case idEliminar:
		m := macroSeleccionada()
		if m == nil {
			return
		}
		if msgBox(hwndMain, "¿Eliminar la macro «"+m.Name+"»?\nTambién se quitarán los remapeos que la usen.",
			"MacroForge", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		cfgMu.Lock()
		for i, x := range cfg.Macros {
			if x == m {
				cfg.Macros = append(cfg.Macros[:i], cfg.Macros[i+1:]...)
				break
			}
		}
		var quedan []*Remap
		for _, r := range cfg.Remaps {
			if r.Macro != m.Name {
				quedan = append(quedan, r)
			}
		}
		cfg.Remaps = quedan
		cfgMu.Unlock()
		salvar()

	case idRemapTecla:
		nuevoRemapTecla()

	case idRemapMacro:
		nuevoRemapMacro()

	case idRemapAlt:
		alternarRemap()

	case idRemapQuitar:
		i := indiceRemap()
		if i < 0 {
			return
		}
		cfgMu.Lock()
		cfg.Remaps = append(cfg.Remaps[:i], cfg.Remaps[i+1:]...)
		cfgMu.Unlock()
		salvar()

	case idTeclasGlb:
		if h, ok := pedirAtajo("Tecla para EMPEZAR / PARAR de grabar"); ok {
			cfgMu.Lock()
			cfg.RecordHotkey = h
			cfgMu.Unlock()
		}
		if h, ok := pedirAtajo("Tecla para DETENER la reproducción"); ok {
			cfgMu.Lock()
			cfg.StopHotkey = h
			cfgMu.Unlock()
		}
		salvar()
		setEstado("Listo — " + textoAyuda())

	case idChkMouse:
		v := sendMessage(chkMouse, BM_GETCHECK, 0, 0) == BST_CHECKED
		cfgMu.Lock()
		cfg.RecordMouseMove = v
		cfgMu.Unlock()
		guardarConfig()

	case idChkScan:
		v := sendMessage(chkScan, BM_GETCHECK, 0, 0) == BST_CHECKED
		cfgMu.Lock()
		cfg.ScanMode = v
		cfgMu.Unlock()
		guardarConfig()
	}
}

func salvar() {
	if err := guardarConfig(); err != nil {
		setEstado("No se pudo guardar: " + err.Error())
	}
	refrescarListas()
	actualizarEstado()
}

func reproducirSeleccionada() {
	if reproduciendo.Load() {
		pararReproduccion()
		return
	}
	if m := macroSeleccionada(); m != nil {
		go reproducir(m)
	}
}

func alternarRemap() {
	i := indiceRemap()
	if i < 0 {
		return
	}
	cfgMu.Lock()
	cfg.Remaps[i].Enabled = !cfg.Remaps[i].Enabled
	cfgMu.Unlock()
	salvar()
}

// finalizarGrabacion corre en el hilo de la GUI: convierte el búfer del hook
// en una macro y la guarda.
func finalizarGrabacion() {
	hookMu.Lock()
	evs := pendientes
	pendientes = nil
	hookMu.Unlock()

	if len(evs) == 0 {
		setEstado("Grabación vacía: no se guardó nada")
		refrescarListas()
		actualizarEstado()
		return
	}
	cfgMu.Lock()
	m := &Macro{Name: nombreLibre("Macro"), Repeat: 1, Speed: 1, Events: evs}
	cfg.Macros = append(cfg.Macros, m)
	idx := len(cfg.Macros) - 1
	cfgMu.Unlock()

	setEstado(fmt.Sprintf("Grabada «%s»: %d eventos", m.Name, len(evs)))
	salvar()
	sendMessage(lstMacros, LB_SETCURSEL, uintptr(idx), 0)
}

func nuevoRemapTecla() {
	origen, ok := pedirAtajo("Pulsa la tecla que quieres remapear")
	if !ok {
		return
	}
	destino, ok := pedirAtajo("Ahora pulsa la tecla de DESTINO")
	if !ok {
		if msgBox(hwndMain, "¿Quieres que esa tecla quede anulada (que no haga nada)?",
			"MacroForge", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		destino = Hotkey{}
	}
	if destino.VK == origen.VK && destino.Ctrl == origen.Ctrl && destino.Alt == origen.Alt &&
		destino.Shift == origen.Shift && destino.Win == origen.Win && !destino.Empty() {
		msgBox(hwndMain, "Origen y destino son la misma tecla.", "MacroForge", MB_ICONWARNING)
		return
	}
	añadirRemap(&Remap{From: origen, To: destino, Enabled: true})
}

func nuevoRemapMacro() {
	m := macroSeleccionada()
	if m == nil {
		msgBox(hwndMain, "Selecciona antes una macro en la lista de arriba.", "MacroForge", MB_ICONINFO)
		return
	}
	origen, ok := pedirAtajo("Tecla que lanzará «" + m.Name + "»")
	if !ok {
		return
	}
	añadirRemap(&Remap{From: origen, Macro: m.Name, Enabled: true})
}

func añadirRemap(r *Remap) {
	cfgMu.Lock()
	for _, x := range cfg.Remaps {
		if x.From == r.From {
			cfgMu.Unlock()
			msgBox(hwndMain, "Esa tecla ya tiene un remapeo.", "MacroForge", MB_ICONWARNING)
			return
		}
	}
	cfg.Remaps = append(cfg.Remaps, r)
	cfgMu.Unlock()
	salvar()
}

// ---------------------------------------------------------------- bandeja

func crearIconoBandeja() {
	icono, _, _ := pLoadIcon.Call(0, IDI_APPLICATION)
	iconoBandeja = NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		HWnd:             hwndMain,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: WM_TRAY,
		HIcon:            icono,
	}
	copy(iconoBandeja.SzTip[:], syscall.StringToUTF16("MacroForge"))
	pShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&iconoBandeja)))
}

// ---------------------------------------------------------------- diálogos

type estadoDialogo struct {
	hwnd     uintptr
	edit     uintptr
	texto    string
	aceptado bool
	hecho    bool
	captura  bool
}

var dlgActual *estadoDialogo

func dlgProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	d := dlgActual
	if d == nil {
		r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
		return r
	}
	switch uint32(msg) {
	case WM_COMMAND:
		switch uint32(wparam & 0xFFFF) {
		case idDlgOK:
			if d.edit != 0 {
				d.texto = getWindowText(d.edit)
			}
			d.aceptado = true
			d.hecho = true
		case idDlgCancelar:
			d.hecho = true
		}
		return 0
	case WM_CAPTURED:
		d.aceptado = !resultadoCaptura().Empty()
		d.hecho = true
		return 0
	case WM_CLOSE:
		d.hecho = true
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

func registrarClaseDialogo() {
	nombre := u16("MacroForgeDlg")
	wc := WNDCLASSEX{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
		WndProc:   cbDlgProc,
		Instance:  getModuleHandle(),
		ClassName: nombre,
	}
	wc.Cursor, _, _ = pLoadCursor.Call(0, IDC_ARROW)
	wc.Background = COLOR_BTNFACE + 1
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
}

// abrirDialogo crea la ventanita modal y devuelve su hwnd.
func abrirDialogo(titulo string, anchoLog, altoLog int32) uintptr {
	estilo := uint32(WS_POPUP | WS_CAPTION | WS_SYSMENU)
	var rp RECT
	pGetWindowRect.Call(hwndMain, uintptr(unsafe.Pointer(&rp)))

	rc := RECT{0, 0, sc(anchoLog), sc(altoLog)}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), uintptr(estilo), 0)
	w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
	x := rp.Left + (rp.Right-rp.Left-w)/2
	y := rp.Top + (rp.Bottom-rp.Top-h)/2

	hwnd, _, _ := pCreateWindowEx.Call(WS_EX_DLGMODALFR,
		uintptr(unsafe.Pointer(u16("MacroForgeDlg"))),
		uintptr(unsafe.Pointer(u16(titulo))),
		uintptr(estilo),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hwndMain, 0, getModuleHandle(), 0)
	return hwnd
}

// bucleModal bombea mensajes hasta que el diálogo se cierra.
func bucleModal(d *estadoDialogo) {
	pEnableWindow.Call(hwndMain, 0)
	pShowWindow.Call(d.hwnd, SW_SHOW)
	pSetForegroundWin.Call(d.hwnd)

	var msg MSG
	for !d.hecho {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			pPostQuitMessage.Call(0) // reponemos el WM_QUIT para el bucle principal
			break
		}
		if !d.captura && msg.Message == WM_KEYDOWN {
			if msg.WParam == VK_RETURN {
				sendMessage(d.hwnd, WM_COMMAND, idDlgOK, 0)
				continue
			}
			if msg.WParam == VK_ESCAPE {
				sendMessage(d.hwnd, WM_COMMAND, idDlgCancelar, 0)
				continue
			}
		}
		if r2, _, _ := pIsDialogMessage.Call(d.hwnd, uintptr(unsafe.Pointer(&msg))); r2 != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	pEnableWindow.Call(hwndMain, 1)
	pDestroyWindow.Call(d.hwnd)
	pSetForegroundWin.Call(hwndMain)
}

func pedirTexto(titulo, etiqueta, inicial string) (string, bool) {
	dialogoAbierto.Store(true)
	defer dialogoAbierto.Store(false)

	d := &estadoDialogo{}
	d.hwnd = abrirDialogo(titulo, 420, 150)
	dlgActual = d
	defer func() { dlgActual = nil }()

	const est = uint32(WS_CHILD | WS_VISIBLE)
	crearControl("STATIC", etiqueta, est|SS_LEFT, sc(14), sc(14), sc(392), sc(36), 0, d.hwnd, 0, fuenteUI)
	d.edit = crearControl("EDIT", inicial, est|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL,
		sc(14), sc(56), sc(392), sc(26), idDlgEdit, d.hwnd, 0, fuenteUI)
	crearControl("BUTTON", "Aceptar", est|WS_TABSTOP|BS_DEFPUSHBUTTON,
		sc(216), sc(98), sc(90), sc(28), idDlgOK, d.hwnd, 0, fuenteUI)
	crearControl("BUTTON", "Cancelar", est|WS_TABSTOP|BS_PUSHBUTTON,
		sc(316), sc(98), sc(90), sc(28), idDlgCancelar, d.hwnd, 0, fuenteUI)

	pSetFocus.Call(d.edit)
	sendMessage(d.edit, EM_SETSEL, 0, ^uintptr(0))
	bucleModal(d)
	return d.texto, d.aceptado
}

func pedirAtajo(titulo string) (Hotkey, bool) {
	dialogoAbierto.Store(true)
	defer dialogoAbierto.Store(false)

	d := &estadoDialogo{captura: true}
	d.hwnd = abrirDialogo(titulo, 420, 130)
	dlgActual = d
	defer func() { dlgActual = nil }()

	const est = uint32(WS_CHILD | WS_VISIBLE)
	crearControl("STATIC",
		"Pulsa ahora la combinación de teclas.\r\n\r\nEsc = cancelar.",
		est|SS_LEFT, sc(14), sc(16), sc(392), sc(60), 0, d.hwnd, 0, fuenteUI)
	crearControl("BUTTON", "Cancelar", est|WS_TABSTOP|BS_PUSHBUTTON,
		sc(316), sc(84), sc(90), sc(28), idDlgCancelar, d.hwnd, 0, fuenteUI)

	pedirCaptura(d.hwnd)
	bucleModal(d)
	cancelarCaptura()

	if !d.aceptado {
		return Hotkey{}, false
	}
	return resultadoCaptura(), true
}
