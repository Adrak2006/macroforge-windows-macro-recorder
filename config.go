//go:build windows

// config.go: modelo de datos (macros, remapeos) y persistencia en JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type EventKind uint8

const (
	EvKey EventKind = iota
	EvMouseMove
	EvMouseButton
	EvWheel
)

// Botones de ratón tal y como los guardamos.
const (
	BtnLeft = iota + 1
	BtnRight
	BtnMiddle
	BtnX1
	BtnX2
)

// Event es un único evento grabado. Delay son los milisegundos que pasaron
// desde el evento anterior; el primero siempre lleva Delay 0.
type Event struct {
	Kind  EventKind `json:"k"`
	Delay uint32    `json:"d"`
	VK    uint32    `json:"vk,omitempty"`
	Scan  uint32    `json:"sc,omitempty"`
	Up    bool      `json:"up,omitempty"`
	Ext   bool      `json:"ext,omitempty"`
	X     int32     `json:"x,omitempty"`
	Y     int32     `json:"y,omitempty"`
	Btn   uint32    `json:"b,omitempty"`
	Delta int32     `json:"w,omitempty"`
	Horiz bool      `json:"hz,omitempty"`
}

// Hotkey es una tecla con modificadores. VK 0 significa "sin asignar".
type Hotkey struct {
	VK    uint32 `json:"vk"`
	Ctrl  bool   `json:"ctrl,omitempty"`
	Alt   bool   `json:"alt,omitempty"`
	Shift bool   `json:"shift,omitempty"`
	Win   bool   `json:"win,omitempty"`
}

func (h Hotkey) Empty() bool { return h.VK == 0 }

func (h Hotkey) String() string {
	if h.Empty() {
		return "—"
	}
	var p []string
	if h.Ctrl {
		p = append(p, "Ctrl")
	}
	if h.Alt {
		p = append(p, "Alt")
	}
	if h.Shift {
		p = append(p, "Mayús")
	}
	if h.Win {
		p = append(p, "Win")
	}
	p = append(p, vkName(h.VK))
	return strings.Join(p, "+")
}

// mismaTecla compara solo la tecla y los modificadores declarados.
func (h Hotkey) coincide(vk uint32, ctrl, alt, shift, win bool) bool {
	if h.Empty() || h.VK != vk {
		return false
	}
	return h.Ctrl == ctrl && h.Alt == alt && h.Shift == shift && h.Win == win
}

type Macro struct {
	Name   string  `json:"name"`
	Hotkey Hotkey  `json:"hotkey"`
	Repeat int     `json:"repeat"` // 0 = infinito hasta pulsar la tecla de parada
	Speed  float64 `json:"speed"`  // 1.0 = velocidad original
	Events []Event `json:"events"`
}

// Duracion devuelve la duración total de la macro en milisegundos.
func (m *Macro) Duracion() uint32 {
	var t uint32
	for _, e := range m.Events {
		t += e.Delay
	}
	return t
}

func (m *Macro) Etiqueta() string {
	rep := fmt.Sprintf("x%d", m.Repeat)
	if m.Repeat == 0 {
		rep = "bucle"
	}
	vel := ""
	if m.Speed != 1 && m.Speed != 0 {
		vel = fmt.Sprintf("   %.2gx", m.Speed)
	}
	return fmt.Sprintf("%s  %s  %4d ev.  %5.1fs  %s%s",
		rellena(m.Name, 28), rellena(m.Hotkey.String(), 18), len(m.Events),
		float64(m.Duracion())/1000, rep, vel)
}

// Remap redirige una tecla física a otra tecla o a una macro.
type Remap struct {
	From    Hotkey `json:"from"`
	To      Hotkey `json:"to"`
	Macro   string `json:"macro,omitempty"` // si no está vacío, dispara esa macro
	Enabled bool   `json:"enabled"`
}

func (r *Remap) Etiqueta() string {
	estado := "activo "
	if !r.Enabled {
		estado = "APAGADO"
	}
	destino := r.To.String()
	if r.Macro != "" {
		destino = "macro: " + r.Macro
	} else if r.To.Empty() {
		destino = "(anular tecla)"
	}
	return fmt.Sprintf("[%s]  %s  →  %s", estado, rellena(r.From.String(), 18), destino)
}

type Config struct {
	Macros          []*Macro `json:"macros"`
	Remaps          []*Remap `json:"remaps"`
	RecordMouseMove bool     `json:"record_mouse_move"`
	ScanMode        bool     `json:"scan_mode"` // enviar scancodes (juegos/DirectInput)
	RecordHotkey    Hotkey   `json:"record_hotkey"`
	StopHotkey      Hotkey   `json:"stop_hotkey"`
}

var (
	cfgMu sync.RWMutex
	cfg   *Config
)

func configPorDefecto() *Config {
	return &Config{
		RecordMouseMove: true,
		RecordHotkey:    Hotkey{VK: VK_F9},
		StopHotkey:      Hotkey{VK: VK_ESCAPE},
	}
}

func rutaConfig() string {
	dir, err := os.UserConfigDir() // %AppData% en Windows
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "MacroForge", "config.json")
}

func cargarConfig() {
	c := configPorDefecto()
	data, err := os.ReadFile(rutaConfig())
	if err == nil {
		if err := json.Unmarshal(data, c); err != nil {
			// Config corrupta: la apartamos en vez de perderla en silencio.
			os.Rename(rutaConfig(), rutaConfig()+".roto")
			c = configPorDefecto()
		}
	}
	if c.RecordHotkey.Empty() {
		c.RecordHotkey = Hotkey{VK: VK_F9}
	}
	if c.StopHotkey.Empty() {
		c.StopHotkey = Hotkey{VK: VK_ESCAPE}
	}
	for _, m := range c.Macros {
		if m.Speed <= 0 {
			m.Speed = 1
		}
		if m.Repeat < 0 {
			m.Repeat = 1
		}
	}
	cfgMu.Lock()
	cfg = c
	cfgMu.Unlock()
}

func guardarConfig() error {
	cfgMu.RLock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	cfgMu.RUnlock()
	if err != nil {
		return err
	}
	ruta := rutaConfig()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	// Escritura atómica: si se va la luz a media escritura no se pierde todo.
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ruta)
}

// buscarMacro devuelve la macro por nombre (o nil).
func buscarMacro(nombre string) *Macro {
	for _, m := range cfg.Macros {
		if m.Name == nombre {
			return m
		}
	}
	return nil
}

func nombreLibre(base string) string {
	n := 1
	for {
		cand := fmt.Sprintf("%s %d", base, n)
		if buscarMacro(cand) == nil {
			return cand
		}
		n++
	}
}

func recorta(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// rellena recorta o rellena hasta n caracteres (no bytes: los acentos
// ocupan 2 bytes y romperían el alineado de las columnas).
func rellena(s string, n int) string {
	s = recorta(s, n)
	if d := n - len([]rune(s)); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

// ------------------------------------------------------------ nombres de VK

var vkNombres = map[uint32]string{
	VK_BACK: "Retroceso", VK_TAB: "Tab", VK_RETURN: "Intro", VK_PAUSE: "Pausa",
	VK_CAPITAL: "BloqMayús", VK_ESCAPE: "Esc", VK_SPACE: "Espacio",
	VK_PRIOR: "RePág", VK_NEXT: "AvPág", VK_END: "Fin", VK_HOME: "Inicio",
	VK_LEFT: "←", VK_UP: "↑", VK_RIGHT: "→", VK_DOWN: "↓",
	VK_SNAPSHOT: "ImprPant", VK_INSERT: "Ins", VK_DELETE: "Supr",
	VK_LWIN: "Win", VK_RWIN: "WinDcha",
	VK_NUMLOCK: "BloqNum", VK_SCROLL: "BloqDespl",
	VK_SHIFT: "Mayús", VK_CONTROL: "Ctrl", VK_MENU: "Alt",
	VK_LSHIFT: "Mayús izq", VK_RSHIFT: "Mayús dcha",
	VK_LCONTROL: "Ctrl izq", VK_RCONTROL: "Ctrl dcha",
	VK_LMENU: "Alt izq", VK_RMENU: "AltGr",
	0x6A: "Num *", 0x6B: "Num +", 0x6D: "Num -", 0x6E: "Num .", 0x6F: "Num /",
	0xBA: "ñ/;", 0xBB: "+", 0xBC: ",", 0xBD: "-", 0xBE: ".", 0xBF: "ç",
	0xC0: "´", 0xDB: "'", 0xDC: "º", 0xDD: "¡", 0xDE: "`", 0xE2: "<",
}

func vkName(vk uint32) string {
	if n, ok := vkNombres[vk]; ok {
		return n
	}
	switch {
	case vk >= '0' && vk <= '9', vk >= 'A' && vk <= 'Z':
		return string(rune(vk))
	case vk >= VK_F1 && vk <= VK_F1+23:
		return fmt.Sprintf("F%d", vk-VK_F1+1)
	case vk >= VK_NUMPAD0 && vk <= VK_NUMPAD0+9:
		return fmt.Sprintf("Num %d", vk-VK_NUMPAD0)
	}
	return fmt.Sprintf("VK_%02X", vk)
}

func nombreBoton(b uint32) string {
	switch b {
	case BtnLeft:
		return "clic izq"
	case BtnRight:
		return "clic dcho"
	case BtnMiddle:
		return "clic medio"
	case BtnX1:
		return "botón X1"
	case BtnX2:
		return "botón X2"
	}
	return "botón?"
}

// esModificador indica si un VK es una tecla modificadora (no sirve como
// tecla principal de un atajo).
func esModificador(vk uint32) bool {
	switch vk {
	case VK_SHIFT, VK_CONTROL, VK_MENU, VK_LSHIFT, VK_RSHIFT,
		VK_LCONTROL, VK_RCONTROL, VK_LMENU, VK_RMENU, VK_LWIN, VK_RWIN:
		return true
	}
	return false
}
