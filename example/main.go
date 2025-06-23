package main

import (
	"fmt"
	"syscall/js"

	"github.com/cardinalby/depo/example/internal/usecase"
	"github.com/cardinalby/depo/example/internal/wasm"
)

func main() {
	logFn := func(msg string, args ...any) {
		fmt.Printf(msg+"\n", args...)
	}
	uc := usecase.NewUsecase(logFn)
	h := wasm.NewWasmHandler(uc)
	js.Global().Set("wasmHandler", h)
	<-make(chan struct{})
}
