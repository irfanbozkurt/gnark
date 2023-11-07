// Tip: This package is used to defer function calls in the circuit builder.
// Golang defers are LIFO style stack (last in first out).
// We need FIFO style stack (first in first out) to defer function calls in the circuit builder.
// And we need to collect the stack trace of the function call.
package circuitdefer

import (
	"runtime"

	"github.com/consensys/gnark/internal/kvstore"
	"github.com/consensys/gnark/profile"
)

type deferKey struct{}

type CallBackWithStack struct {
	CallBack any
	Stack    []uintptr
}

func Push(builder any, cb any) {
	kv, ok := builder.(kvstore.Store)
	if !ok {
		panic("builder does not implement kvstore.Store")
	}

	var lifo []CallBackWithStack
	if val := kv.GetKeyValue(deferKey{}); val != nil {
		var ok bool
		if lifo, ok = val.([]CallBackWithStack); !ok {
			panic("stored deferred functions not []func(frontend.API) error")
		}
	}

	var pc []uintptr
	if profile.HasActiveSessions() {
		// we are profiling, we collect the stack trace.
		pc = make([]uintptr, 20)
		n := runtime.Callers(2, pc)
		if n == 0 {
			return
		}
		pc = pc[:n]
	}

	lifo = append(lifo, CallBackWithStack{cb, pc})
	kv.SetKeyValue(deferKey{}, lifo)

}

func Pop(builder any) (cb CallBackWithStack, ok bool) {
	kv, ok := builder.(kvstore.Store)
	if !ok {
		panic("builder does not implement kvstore.Store")
	}
	val := kv.GetKeyValue(deferKey{})
	if val == nil {
		return CallBackWithStack{}, false
	}
	lifo, ok := val.([]CallBackWithStack)
	if !ok {
		panic("stored deferred functions not []func(frontend.API) error")
	}
	if len(lifo) == 0 {
		return CallBackWithStack{}, false
	}
	r := lifo[0]

	// TODO @gbotrel set the stack trace for current deferred call.

	kv.SetKeyValue(deferKey{}, lifo[1:])
	return r, true
}

// func Put[T any](builder any, cb T) {
// 	// we use generics for type safety but to avoid import cycles.
// 	// TODO: compare with using any and type asserting at caller
// 	kv, ok := builder.(kvstore.Store)
// 	if !ok {
// 		panic("builder does not implement kvstore.Store")
// 	}

// 	var deferred []T
// 	if val := kv.GetKeyValue(deferKey{}); val != nil {
// 		var ok bool
// 		deferred, ok = val.([]T)
// 		if !ok {
// 			panic("stored deferred functions not []func(frontend.API) error")
// 		}
// 	}
// 	deferred = append(deferred, cb)
// 	kv.SetKeyValue(deferKey{}, deferred)
// }

// func GetAll[T any](builder any) []T {
// 	kv, ok := builder.(kvstore.Store)
// 	if !ok {
// 		panic("builder does not implement kvstore.Store")
// 	}
// 	val := kv.GetKeyValue(deferKey{})
// 	if val == nil {
// 		return nil
// 	}
// 	deferred, ok := val.([]T)
// 	if !ok {
// 		panic("stored deferred functions not []func(frontend.API) error")
// 	}
// 	return deferred
// }
