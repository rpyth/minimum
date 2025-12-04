//go:build js && wasm
// +build js,wasm

package main

import (
	"fmt"
	"syscall/js"

	// update imports to match your repo layout
	"minimum/bytecode" // bytecode types if needed
	"minimum/inter"    // inter package (interpreter)
)

// NOTE: adapt these helper-registration calls to your interpreter's host function API.
// I show two variants (RegisterHostFunc / SetPair). Use whichever matches your codebase.

var (
	interpreter inter.Interpreter
)

/*
// helper: create host functions and place them under a "html" pairing/dict
func registerHTMLPair(in *inter.Interpreter) {
	// --------- ADAPT: If your interpreter provides a registration API, use it here ----------
	// Example A: interpreter.RegisterHostFunc("html.set", func(args ...interface{}) (interface{}, error) { ... })
	// Example B: interpreter.SetPair("html", pairValue) where pairValue maps strings to callables
	//
	// Below I show a simple approach assuming you can register named host functions:
	// Replace RegisterHostFunc with your real API call (RegisterNative, AddBuiltin, etc).
	// ------------------------------------------------------------------------------------

	// html.set: sets element.outerHTML
	in.RegisterHostFunc("html.set", func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("html.set requires 2 args: id, html")
		}
		id, ok1 := args[0].(string)
		html, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("html.set expects strings")
		}
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil, fmt.Errorf("no document")
		}
		el := doc.Call("getElementById", id)
		if !el.Truthy() {
			// create an element container if not found? here we return error
			return nil, fmt.Errorf("element with id %s not found", id)
		}
		// set outerHTML (replace element)
		el.Set("outerHTML", html)
		return nil, nil
	})

	// html.set_inner: sets element.innerHTML
	in.RegisterHostFunc("html.set_inner", func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("html.set_inner requires 2 args: id, html")
		}
		id, ok1 := args[0].(string)
		html, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("html.set_inner expects strings")
		}
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil, fmt.Errorf("no document")
		}
		el := doc.Call("getElementById", id)
		if !el.Truthy() {
			return nil, fmt.Errorf("element with id %s not found", id)
		}
		el.Set("innerHTML", html)
		return nil, nil
	})

	// Optionally add convenience wrappers so Minimum code can call !html.set "id","<p>hi</p>"
	// If your interpreter represents pairs/dicts differently, create a Pair named "html"
	// and set function entries to the registered host funcs above.
}
*/

func initInterpreter(code, fname string) error {
	// create interpreter instance
	in := inter.NewInterpreter(code, fname)
	interpreter = in

	// optional: make initial Nothing run, replicate main behavior
	interpreter.Nothing("Nothing")

	// register html pair/host functions
	// registerHTMLPair(interpreter)

	// If you want to export any other host functions (e.g. console.log) do it here.

	return nil
}

// Exported JS function: Init the interpreter with source code string and filename
func jsInit(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{"ok": false, "err": "Init needs (code, fname)"}
	}
	code := args[0].String()
	fname := args[1].String()

	err := initInterpreter(code, fname)
	if err != nil {
		return map[string]interface{}{"ok": false, "err": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// Exported JS function: run a node by name (e.g., "_node_123")
func jsRunNode(this js.Value, args []js.Value) interface{} {
	//if interpreter == nil {
	//	return map[string]interface{}{"ok": false, "err": "interpreter not initialized"}
	//}
	if len(args) < 1 {
		return map[string]interface{}{"ok": false, "err": "runNode needs (nodeName)"}
	}
	node := args[0].String()
	// call interpreter Run — adapt to your API if Run returns (err bool) like in your code
	err := interpreter.Run(node)
	if err {
		return map[string]interface{}{"ok": false, "err": fmt.Sprintf("Error occurred: %v", err)}
	}
	return map[string]interface{}{"ok": true}
}

// Exported JS function: compile & run a source snippet (convenience)
func jsCompileAndRun(this js.Value, args []js.Value) interface{} {
	//if interpreter == nil {
	//	return map[string]interface{}{"ok": false, "err": "interpreter not initialized"}
	//}
	if len(args) < 2 {
		return map[string]interface{}{"ok": false, "err": "compileAndRun needs (source, origin)"}
	}
	source := args[0].String()
	origin := args[1].String()

	interpreter.Compile(source, origin)
	lastNode := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
	// run lastNode
	err := interpreter.Run(lastNode)
	if err {
		return map[string]interface{}{"ok": false, "err": fmt.Sprintf("Error occurred: %v", err)}
	}
	return map[string]interface{}{"ok": true}
}

func main() {
	// expose functions to JS
	js.Global().Set("Minimum_init", js.FuncOf(jsInit))
	js.Global().Set("Minimum_runNode", js.FuncOf(jsRunNode))
	js.Global().Set("Minimum_compileAndRun", js.FuncOf(jsCompileAndRun))

	// keep Go runtime alive
	select {}
}
