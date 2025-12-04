//go:build js && wasm
// +build js,wasm

package input

import (
	"fmt"
	"syscall/js"
)

// Dummy replacements for WASM
// (no stdin, no readline — browser uses JS for input)

type DummyInput struct{}

var Rl *DummyInput

func NewInputPrompt(str string) (*DummyInput, error) {
	return &DummyInput{}, nil
}

func (_ *DummyInput) Readline() (string, error) {
	return "", nil // or return an error
}

func (_ *DummyInput) SetPrompt(_ string) {
	return
}

func CloseInput(_ *DummyInput) {}

func (_ *DummyInput) Close() {}

func SetInnerHtml(str_id, target string) error {
	document := js.Global().Get("document")
	element := document.Call("getElementById", str_id) // Replace "output" with your element's ID
	if element.Truthy() {                              // Check if the element exists
		element.Set("innerHTML", target)
		return nil
	} else {
		return fmt.Errorf("cannot find specified element: \"%s\"", str_id)
	}
}
