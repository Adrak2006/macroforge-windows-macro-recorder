//go:build windows

// win32.go: enlaces a la API de Windows. Sin cgo ni dependencias externas,
// igual que ClearGlass: todo con syscall.NewLazyDLL.
package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")

	pSetWindowsHookEx   = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHook  = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx     = user32.NewProc("CallNextHookEx")
	pGetMessage         = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessage    = user32.NewProc("DispatchMessageW")
	pIsDialogMessage    = user32.NewProc("IsDialogMessageW")
	pPostMessage        = user32.NewProc("PostMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pDefWindowProc      = user32.NewProc("DefWindowProcW")
	pRegisterClassEx    = user32.NewProc("RegisterClassExW")
	pCreateWindowEx     = user32.NewProc("CreateWindowExW")
	pShowWindow         = user32.NewProc("ShowWindow")
	pUpdateWindow       = user32.NewProc("UpdateWindow")
	pDestroyWindow      = user32.NewProc("DestroyWindow")
	pSendMessage        = user32.NewProc("SendMessageW")
	pSetWindowText      = user32.NewProc("SetWindowTextW")
	pGetWindowText      = user32.NewProc("GetWindowTextW")
	pGetWindowTextLen   = user32.NewProc("GetWindowTextLengthW")
	pSendInput          = user32.NewProc("SendInput")
	pGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	pGetAsyncKeyState   = user32.NewProc("GetAsyncKeyState")
	pMapVirtualKey      = user32.NewProc("MapVirtualKeyW")
	pLoadCursor         = user32.NewProc("LoadCursorW")
	pLoadIcon           = user32.NewProc("LoadIconW")
	pSetFocus           = user32.NewProc("SetFocus")
	pGetWindowRect      = user32.NewProc("GetWindowRect")
	pMoveWindow         = user32.NewProc("MoveWindow")
	pAdjustWindowRect   = user32.NewProc("AdjustWindowRect")
	pIsWindowVisible    = user32.NewProc("IsWindowVisible")
	pEnableWindow       = user32.NewProc("EnableWindow")
	pMessageBox         = user32.NewProc("MessageBoxW")
	pSetForegroundWin   = user32.NewProc("SetForegroundWindow")
	pCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	pAppendMenu         = user32.NewProc("AppendMenuW")
	pDestroyMenu        = user32.NewProc("DestroyMenu")
	pTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	pGetCursorPos       = user32.NewProc("GetCursorPos")
	pSetProcDpiCtx      = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	pGetDpiForWindow    = user32.NewProc("GetDpiForWindow")

	pGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	pCreateMutex     = kernel32.NewProc("CreateMutexW")

	pCreateFont   = gdi32.NewProc("CreateFontW")
	pDeleteObject = gdi32.NewProc("DeleteObject")

	pShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

	pTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
)

// ---------------------------------------------------------------- constantes

const (
	WH_KEYBOARD_LL = 13
	WH_MOUSE_LL    = 14

	HC_ACTION = 0

	LLKHF_INJECTED = 0x10
	LLKHF_EXTENDED = 0x01
	LLMHF_INJECTED = 0x01

	WM_CREATE      = 0x0001
	WM_DESTROY     = 0x0002
	WM_SIZE        = 0x0005
	WM_SETFOCUS    = 0x0007
	WM_CLOSE       = 0x0010
	WM_QUIT        = 0x0012
	WM_SETFONT     = 0x0030
	WM_GETFONT     = 0x0031
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105
	WM_COMMAND     = 0x0111
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONDOWN = 0x0204
	WM_RBUTTONUP   = 0x0205
	WM_MBUTTONDOWN = 0x0207
	WM_MBUTTONUP   = 0x0208
	WM_MOUSEWHEEL  = 0x020A
	WM_XBUTTONDOWN = 0x020B
	WM_XBUTTONUP   = 0x020C
	WM_MOUSEHWHEEL = 0x020E
	WM_APP         = 0x8000

	// mensajes propios
	WM_TRAY      = WM_APP + 1
	WM_CAPTURED  = WM_APP + 2
	WM_REFRESH   = WM_APP + 3
	WM_SETSTATUS = WM_APP + 4

	WS_OVERLAPPED       = 0x00000000
	WS_POPUP            = 0x80000000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_DISABLED         = 0x08000000
	WS_BORDER           = 0x00800000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_THICKFRAME       = 0x00040000
	WS_MINIMIZEBOX      = 0x00020000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_TABSTOP          = 0x00010000
	WS_GROUP            = 0x00020000
	WS_OVERLAPPEDWINDOW = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX

	WS_EX_CLIENTEDGE = 0x00000200
	WS_EX_DLGMODALFR = 0x00000001
	WS_EX_TOOLWINDOW = 0x00000080

	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9

	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001

	IDC_ARROW       = 32512
	IDI_APPLICATION = 32512

	COLOR_BTNFACE = 15

	// ListBox
	LBS_NOTIFY      = 0x0001
	LBS_HASSTRINGS  = 0x0040
	LB_ADDSTRING    = 0x0180
	LB_RESETCONTENT = 0x0184
	LB_SETCURSEL    = 0x0186
	LB_GETCURSEL    = 0x0188
	LBN_DBLCLK      = 2
	LBN_SELCHANGE   = 1

	// Button
	BS_PUSHBUTTON    = 0x0000
	BS_DEFPUSHBUTTON = 0x0001
	BS_AUTOCHECKBOX  = 0x0003
	BM_GETCHECK      = 0x00F0
	BM_SETCHECK      = 0x00F1
	BST_CHECKED      = 1

	// Edit
	ES_AUTOHSCROLL = 0x0080
	EM_SETSEL      = 0x00B1

	// Static
	SS_LEFT = 0x0000

	MB_OKCANCEL     = 0x00000001
	MB_YESNO        = 0x00000004
	MB_ICONWARNING  = 0x00000030
	MB_ICONQUESTION = 0x00000020
	MB_ICONERROR    = 0x00000010
	MB_ICONINFO     = 0x00000040
	IDOK            = 1
	IDYES           = 6

	MF_STRING       = 0x0000
	MF_SEPARATOR    = 0x0800
	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	// SendInput
	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1

	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_SCANCODE    = 0x0008

	MOUSEEVENTF_MOVE        = 0x0001
	MOUSEEVENTF_LEFTDOWN    = 0x0002
	MOUSEEVENTF_LEFTUP      = 0x0004
	MOUSEEVENTF_RIGHTDOWN   = 0x0008
	MOUSEEVENTF_RIGHTUP     = 0x0010
	MOUSEEVENTF_MIDDLEDOWN  = 0x0020
	MOUSEEVENTF_MIDDLEUP    = 0x0040
	MOUSEEVENTF_XDOWN       = 0x0080
	MOUSEEVENTF_XUP         = 0x0100
	MOUSEEVENTF_WHEEL       = 0x0800
	MOUSEEVENTF_HWHEEL      = 0x1000
	MOUSEEVENTF_VIRTUALDESK = 0x4000
	MOUSEEVENTF_ABSOLUTE    = 0x8000

	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79

	// Shell_NotifyIcon
	NIM_ADD     = 0x00000000
	NIM_MODIFY  = 0x00000001
	NIM_DELETE  = 0x00000002
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	// Teclas virtuales usadas por nombre
	VK_LBUTTON  = 0x01
	VK_RBUTTON  = 0x02
	VK_MBUTTON  = 0x04
	VK_BACK     = 0x08
	VK_TAB      = 0x09
	VK_RETURN   = 0x0D
	VK_SHIFT    = 0x10
	VK_CONTROL  = 0x11
	VK_MENU     = 0x12
	VK_PAUSE    = 0x13
	VK_CAPITAL  = 0x14
	VK_ESCAPE   = 0x1B
	VK_SPACE    = 0x20
	VK_PRIOR    = 0x21
	VK_NEXT     = 0x22
	VK_END      = 0x23
	VK_HOME     = 0x24
	VK_LEFT     = 0x25
	VK_UP       = 0x26
	VK_RIGHT    = 0x27
	VK_DOWN     = 0x28
	VK_SNAPSHOT = 0x2C
	VK_INSERT   = 0x2D
	VK_DELETE   = 0x2E
	VK_LWIN     = 0x5B
	VK_RWIN     = 0x5C
	VK_NUMPAD0  = 0x60
	VK_F1       = 0x70
	VK_F9       = 0x78
	VK_F10      = 0x79
	VK_F12      = 0x7B
	VK_NUMLOCK  = 0x90
	VK_SCROLL   = 0x91
	VK_LSHIFT   = 0xA0
	VK_RSHIFT   = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LMENU    = 0xA4
	VK_RMENU    = 0xA5
)

// firmaMF marca los eventos que inyectamos nosotros, para no volver a
// capturarlos en el hook (si no, una macro se grabaría a sí misma).
const firmaMF = uintptr(0x4D46_0BAD)

// ------------------------------------------------------------------ structs

type POINT struct{ X, Y int32 }

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	_       uint32
}

type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type RECT struct{ Left, Top, Right, Bottom int32 }

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSLLHOOKSTRUCT struct {
	Pt          POINT
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// INPUT de 40 bytes en x64. data es [4]uint64 para forzar alineación a 8,
// que es lo que exige el dwExtraInfo (ULONG_PTR) dentro de la unión.
type INPUT struct {
	Type uint32
	_    uint32
	data [4]uint64
}

type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	_           uint32
	DwExtraInfo uintptr
}

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	_           uint32
	DwExtraInfo uintptr
}

func (in *INPUT) mouse() *MOUSEINPUT { return (*MOUSEINPUT)(unsafe.Pointer(&in.data[0])) }
func (in *INPUT) key() *KEYBDINPUT   { return (*KEYBDINPUT)(unsafe.Pointer(&in.data[0])) }

type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	_                uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

// ----------------------------------------------------------------- helpers

func u16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func fromU16(buf []uint16) string { return syscall.UTF16ToString(buf) }

func getModuleHandle() uintptr {
	h, _, _ := pGetModuleHandle.Call(0)
	return h
}

func getSystemMetrics(i int32) int32 {
	r, _, _ := pGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}

func keyIsDown(vk uint32) bool {
	r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

func mapVirtualKey(vk, mapType uint32) uint32 {
	r, _, _ := pMapVirtualKey.Call(uintptr(vk), uintptr(mapType))
	return uint32(r)
}

func sendMessage(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessage.Call(hwnd, uintptr(msg), w, l)
	return r
}

func postMessage(hwnd uintptr, msg uint32, w, l uintptr) {
	pPostMessage.Call(hwnd, uintptr(msg), w, l)
}

func setWindowText(hwnd uintptr, s string) {
	pSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(u16(s))))
}

func getWindowText(hwnd uintptr) string {
	n, _, _ := pGetWindowTextLen.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	pGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return fromU16(buf)
}

func msgBox(parent uintptr, texto, titulo string, flags uint32) int32 {
	r, _, _ := pMessageBox.Call(parent,
		uintptr(unsafe.Pointer(u16(texto))),
		uintptr(unsafe.Pointer(u16(titulo))),
		uintptr(flags))
	return int32(r)
}
