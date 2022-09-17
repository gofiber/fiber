package eventemitter

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	ErrEmptyName      = errors.New("event name cannot be empty")
	ErrNotAFunction   = errors.New("callback must be a function or a pointer to a function")
	ErrEventNotExists = errors.New("event does not exist")
)

type Emitter struct {
	listeners sync.Map
}

type argsError struct {
	event    string
	expected int
	got      int
}

type argsTypeError struct {
	event    string
	pos      int
	expected reflect.Type
	got      reflect.Type
}

func (e *argsError) Error() string {
	return fmt.Sprintf("Wrong number of arguments. Event %s expected %d arguments, got %d.", e.event, e.expected, e.got)
}

func (e *argsTypeError) Error() string {
	return fmt.Sprintf("Wrong argument type. Event %s expected argument %d to be %s, got %s.", e.event, e.pos, e.expected, e.got)
}

// New returns a new event emitter.
func New() *Emitter {
	return &Emitter{}
}

// AddListener adds a listener for the specified event.
// Returns an error if the eventName is empty, or the listener is not a function.
// No checks are made to see if the listener has already been added. Multiple
// calls passing the same combination of eventName and listener will result in the
// listener being added, and called, multiple times.
// By default, event listeners are invoked in the order they are added.
func (e *Emitter) AddListener(eventName string, listener any) error {
	return e.on(eventName, false, listener)
}

// On is an alias for .AddListener(eventName, listener).
func (e *Emitter) On(eventName string, listener any) error {
	return e.on(eventName, false, listener)
}

func (e *Emitter) Once(eventName string, listener any) error {
	return e.on(eventName, true, listener)
}

// RemoveListener removes the specified listener from the specified event.
// Returns true if the listener was removed, false otherwise.
// RemoveListener will remove, at most, one instance of a listener from the
// listener map. If any single listener has been added multiple times to the
// listener map for the specified eventName, then RemoveListener must be called
// multiple times to remove each instance.
// When a single function has been added as a handler multiple times for a single
// event, RemoveListener will remove the most recently added instance.
func (e *Emitter) RemoveListener(eventName string, listener any) (ok bool, err error) {
	if !e.isFunction(listener) {
		return false, ErrNotAFunction
	}

	listeners, err := e.getListeners(eventName)
	if err != nil {
		return false, err
	}

	for i := len(listeners) - 1; i >= 0; i-- {
		if e.isEqual(listener, listeners[i]) {
			if len(listeners) == 1 {
				e.listeners.Delete(eventName)
			} else {
				e.listeners.Store(eventName, append(listeners[:i], listeners[i+1:]...))
			}

			return true, nil
		}
	}

	return false, nil
}

// Off is an alias for .RemoveListener(eventName, listener).
func (e *Emitter) Off(eventName string, listener any) (ok bool, err error) {
	return e.RemoveListener(eventName, listener)
}

// RemoveAllListeners removes all listeners, or those of the specified eventName.
func (e *Emitter) RemoveAllListeners(eventName ...string) {
	if len(eventName) == 0 {
		eventName = e.EventNames()
	}

	for _, event := range eventName {
		e.listeners.Delete(event)
	}
}

// Emit synchronously calls each of the listeners registered for the event
// named eventName, in the order they were registered, passing the supplied
// arguments to each.
// Returns an error if the event does not exist.
func (e *Emitter) Emit(eventName string, arguments ...any) error {
	return e.emit(eventName, arguments)
}

// EventNames returns a slice of strings listing the events for which the emitter
// has registered listeners.
func (e *Emitter) EventNames() []string {
	var names []string

	e.listeners.Range(func(eventName, listener any) bool {
		names = append(names, eventName.(string))

		return true
	})

	return names
}

// Listeners returns a slice of functions registered to the specified event.
// Returns an error if the event does not exist.
func (e *Emitter) Listeners(eventName string) ([]any, error) {
	return e.getListeners(eventName)
}

// ListenersCount returns the number of listeners for the specified event.
// Returns an error if the event does not exist.
func (e *Emitter) ListenerCount(eventName string) (int, error) {
	listeners, err := e.getListeners(eventName)

	if err != nil {
		return 0, err
	}

	return len(listeners), nil
}

func (e *Emitter) on(eventName string, once bool, listener any) error {
	if len(eventName) == 0 {
		return ErrEmptyName
	}

	if !e.isFunction(listener) {
		return ErrNotAFunction
	}

	if once {
		if listeners, ok := e.listeners.LoadAndDelete(eventName); ok {
			e.listeners.Store(eventName, append(listeners.([]any), listener))
		} else {
			e.listeners.Store(eventName, []any{listener})
		}
	} else {
		if listeners, ok := e.listeners.Load(eventName); ok {
			e.listeners.Store(eventName, append(listeners.([]any), listener))
		} else {
			e.listeners.Store(eventName, []any{listener})
		}
	}

	return nil
}

func (e *Emitter) emit(eventName string, arguments []any) error {
	listeners, err := e.getListeners(eventName)
	if err != nil {
		return err
	}

	args := make([]reflect.Value, 0, len(arguments))
	for _, arg := range arguments {
		args = append(args, reflect.ValueOf(arg))
	}

	for _, listener := range listeners {
		fn := reflect.ValueOf(listener)

		// If the listener is a pointer to a function, get the function.
		if fn.Kind() == reflect.Pointer {
			fn = fn.Elem()
		}

		// Check the number of arguments and their types.
		if err := e.checkArguments(eventName, fn, args); err != nil {
			panic(err)
		}

		// Call the listener.
		go fn.Call(args)
	}

	return nil
}

func (e *Emitter) getListeners(eventName string) ([]any, error) {
	if len(eventName) == 0 {
		return nil, ErrEmptyName
	}

	if listeners, ok := e.listeners.Load(eventName); ok {
		return listeners.([]any), nil
	}

	return nil, ErrEventNotExists
}

func (e *Emitter) checkArguments(eventName string, fn reflect.Value, args []reflect.Value) error {
	fnType := fn.Type()
	isVariadic := fnType.IsVariadic()
	noParams := fnType.NumIn()

	// Check arguments length.
	if isVariadic {
		if (noParams - 1) > len(args) {
			return &argsError{eventName, noParams - 1, len(args)}
		}
	} else if noParams != len(args) {
		return &argsError{eventName, noParams, len(args)}
	}

	// Check arguments type.
	for i := 0; i < noParams; i++ {
		if isVariadic && i == (noParams-1) {
			args = args[i:] // Variadic arguments.

			for j := 0; j < len(args); j++ {
				if !args[j].Type().AssignableTo(fnType.In(i).Elem()) {
					return &argsTypeError{eventName, i + 1, fnType.In(i).Elem(), args[j].Type()}
				}
			}
		} else if !args[i].Type().AssignableTo(fnType.In(i)) {
			return &argsTypeError{eventName, i + 1, fnType.In(i), args[i].Type()}
		}
	}

	return nil
}

func (e *Emitter) isFunction(fn any) bool {
	if fn != nil {
		kind := reflect.TypeOf(fn).Kind()
		if kind == reflect.Func || (kind == reflect.Pointer && reflect.ValueOf(fn).Elem().Kind() == reflect.Func) {
			return true
		}
	}

	return false
}

func (e *Emitter) isEqual(listener, storedListener any) bool {
	if reflect.TypeOf(listener).Kind() == reflect.Pointer {
		if reflect.TypeOf(storedListener).Kind() == reflect.Pointer && storedListener == listener {
			return true
		}
	} else {
		if reflect.TypeOf(storedListener).Kind() != reflect.Pointer && reflect.ValueOf(storedListener).Pointer() == reflect.ValueOf(listener).Pointer() {
			return true
		}
	}

	return false
}
