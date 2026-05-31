package scriptmgr

import (
	"reflect"

	"send2nlm/sdk"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Engine manages yaegi interpreter instances for loading scripts.
type Engine struct{}

// NewEngine creates a new script engine.
func NewEngine() *Engine {
	return &Engine{}
}

// newInterp creates a fresh yaegi interpreter pre-loaded with stdlib and SDK symbols.
func (e *Engine) newInterp() *interp.Interpreter {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(sdk.Symbols)
	return i
}

// EvalScript evaluates a .go source file in a fresh interpreter.
// Returns the interpreter so callers can extract exported variables.
func (e *Engine) EvalScript(path string) (*interp.Interpreter, error) {
	i := e.newInterp()
	_, err := i.EvalPath(path)
	return i, err
}

// ExtractProducer extracts the "Producer" variable from an evaluated script.
// The script must declare: var Producer sdk.Producer = &SomeType{}
func ExtractProducer(i *interp.Interpreter) (sdk.Producer, error) {
	v, err := i.Eval("producer.Producer")
	if err != nil {
		return nil, err
	}
	return v.Interface().(sdk.Producer), nil
}

// ExtractReceiver extracts the "Receiver" variable from an evaluated script.
// The script must declare: var Receiver sdk.Receiver = &SomeType{}
func ExtractReceiver(i *interp.Interpreter) (sdk.Receiver, error) {
	v, err := i.Eval("receiver.Receiver")
	if err != nil {
		return nil, err
	}
	return v.Interface().(sdk.Receiver), nil
}

// Ensure interpreter implements necessary interfaces.
var _ reflect.Value
