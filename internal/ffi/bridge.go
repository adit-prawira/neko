package ffi

/*
#cgo LDFLAGS: -L${SRCDIR}/../../engine/target/release -lneko_engine
#include "bridge.h"
*/
import "C"
import "fmt"

func Version() string {
	code := C.neko_version()

	if code != 0 {
		return fmt.Sprintf("neko v0.1.0 (engine error: %d)", int(code))
	}

	return "neko v0.1.0"
}
