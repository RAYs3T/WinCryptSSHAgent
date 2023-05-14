package utils

import (
	"syscall"
	"unsafe"

	"github.com/electricbubble/go-toast"

	_ "embed"
)

var (
	moduser32      = syscall.NewLazyDLL("user32.dll")
	procMessageBox = moduser32.NewProc("MessageBoxW")
)

const (
	MB_OK                = 0x00000000
	MB_OKCANCEL          = 0x00000001
	MB_ABORTRETRYIGNORE  = 0x00000002
	MB_YESNOCANCEL       = 0x00000003
	MB_YESNO             = 0x00000004
	MB_RETRYCANCEL       = 0x00000005
	MB_CANCELTRYCONTINUE = 0x00000006
	MB_ICONHAND          = 0x00000010
	MB_ICONQUESTION      = 0x00000020
	MB_ICONEXCLAMATION   = 0x00000030
	MB_ICONASTERISK      = 0x00000040
	MB_USERICON          = 0x00000080
	MB_ICONWARNING       = MB_ICONEXCLAMATION
	MB_ICONERROR         = MB_ICONHAND
	MB_ICONINFORMATION   = MB_ICONASTERISK
	MB_ICONSTOP          = MB_ICONHAND

	MB_DEFBUTTON1 = 0x00000000
	MB_DEFBUTTON2 = 0x00000100
	MB_DEFBUTTON3 = 0x00000200
	MB_DEFBUTTON4 = 0x00000300

	IDOK     = 1
	IDCANCEL = 2
	IDABORT  = 3
	IDRETRY  = 4
	IDIGNORE = 5
	IDYES    = 6
	IDNO     = 7
)

func MessageBox(title, text string, style uintptr) int {
	pText, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return -1
	}
	pTitle, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return -1
	}
	ret, _, _ := syscall.Syscall6(procMessageBox.Addr(),
		4,
		0,
		uintptr(unsafe.Pointer(pText)),
		uintptr(unsafe.Pointer(pTitle)),
		style,
		0,
		0)
	return int(ret)
}

//go:embed icons/fingerprint.png
var ICON_FINGERPRINT []byte

//go:embed icons/alert-circle.png
var ICON_ALERT []byte

//go:embed icons/check-bold.png
var ICON_CHECK []byte

//go:embed icons/key-minus.png
var ICON_KEY_MINUS []byte

//go:embed icons/key-plus.png
var ICON_KEY_PLUS []byte

//go:embed icons/key-remove.png
var ICON_KEY_REMOVE []byte

func Notify(title string, message string, sound toast.Audio, icon []byte) {
	_ = toast.Push(message,
		toast.WithTitle(title),
		toast.WithAppID("WinCrypt SSH Agent"),
		toast.WithAudio(sound),
		toast.WithShortDuration(),
		toast.WithIconRaw(icon),
	)
}
