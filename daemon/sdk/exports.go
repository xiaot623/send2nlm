package sdk

import (
	"reflect"

	"github.com/traefik/yaegi/interp"
)

// Symbols exports the SDK package symbols for the yaegi interpreter.
// Key format: "importPath/packageName"
var Symbols = interp.Exports{
	"send2nlm/sdk/sdk": {
		"Producer":   reflect.ValueOf((*Producer)(nil)).Elem(),
		"Receiver":   reflect.ValueOf((*Receiver)(nil)).Elem(),
		"Resource":   reflect.ValueOf((*Resource)(nil)).Elem(),
		"Config":     reflect.ValueOf((*Config)(nil)).Elem(),
		"LoadConfig": reflect.ValueOf(LoadConfig),
	},
}
