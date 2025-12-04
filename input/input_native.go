//go:build !js
// +build !js

package input

import (
	"fmt"

	"github.com/chzyer/readline"
)

var Rl *readline.Instance

func New(str string) {
	Rl, _ = readline.New(str)
}

func NewInputPrompt(str string) (*readline.Instance, error) {
	return readline.New(str)
}

// For example:
func ReadLine(rl *readline.Instance) (string, error) {
	return rl.Readline()
}

func CloseInput(rl *readline.Instance) {
	rl.Close()
}

func SetInnerHtml(str_id, target string) error {
	return fmt.Errorf("cannot call this function under this build")
}
