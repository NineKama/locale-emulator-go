package launcher

import "golang.org/x/sys/windows"

const (
	debugPadding           = 4
	debugEntryOffset       = 48
	exceptionAddressOffset = 16
	contextSize            = 1232
	contextFlagsOffset     = 48
	contextIPOffset        = 248
	contextControl         = 0x100001
)

func installThreadEntry(_ windows.Handle, address uintptr) (uintptr, func(), error) {
	return address, func() {}, nil
}
