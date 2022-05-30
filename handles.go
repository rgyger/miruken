package miruken

import (
	"github.com/miruken-go/miruken/promise"
	"reflect"
)

// Handles callbacks contravariantly.
type Handles struct {
	CallbackBase
	callback any
}

func (h *Handles) Source() any {
	return h.callback
}

func (h *Handles) Key() any {
	return reflect.TypeOf(h.callback)
}

func (h *Handles) Policy() Policy {
	return _handlesPolicy
}

func (h *Handles) CanDispatch(
	handler     any,
	binding     Binding,
) (reset func (), approved bool) {
	if guard, ok := h.callback.(CallbackGuard); ok {
		return guard.CanDispatch(handler, binding)
	}
	return nil, true
}

func (h *Handles) CanInfer() bool {
	if infer, ok := h.callback.(interface{CanInfer() bool}); ok {
		return infer.CanInfer()
	}
	return true
}

func (h *Handles) CanFilter() bool {
	if infer, ok := h.callback.(interface{CanFilter() bool}); ok {
		return infer.CanFilter()
	}
	return true
}

func (h *Handles) Dispatch(
	handler  any,
	greedy   bool,
	composer Handler,
) HandleResult {
	count := h.ResultCount()
	return DispatchPolicy(handler, h, greedy, composer).
		OtherwiseHandledIf(h.ResultCount() > count)
}

// HandlesBuilder builds Handles callbacks.
type HandlesBuilder struct {
	callback any
}

func (b *HandlesBuilder) WithCallback(
	callback any,
) *HandlesBuilder {
	if IsNil(callback) {
		panic("callback cannot be nil")
	}
	b.callback = callback
	return b
}

func (b *HandlesBuilder) NewHandles() *Handles {
	return &Handles{
		callback: b.callback,
	}
}

func Invoke[T any](
	handler  Handler,
	callback any,
) (t T, tp *promise.Promise[T], err error) {
	if handler == nil {
		panic("handler cannot be nil")
	}
	var builder HandlesBuilder
	handles := builder.
		WithCallback(callback).
		NewHandles()
	if result := handler.Handle(handles, false, nil); result.IsError() {
		err = result.Error()
	} else if !result.handled {
		err = NotHandledError{callback}
	} else {
		_, tp, err = CoerceResult[T](handles, &t)
	}
	return
}

func InvokeAll[T any](
	handler Handler,
	callback any,
) (t []T, tp *promise.Promise[[]T], err error) {
	if handler == nil {
		panic("handler cannot be nil")
	}
	var builder HandlesBuilder
	builder.WithCallback(callback)
	handles := builder.NewHandles()
	if result := handler.Handle(handles, true, nil); result.IsError() {
		err = result.Error()
	} else if !result.handled {
		err = NotHandledError{callback}
	} else {
		_, tp, err = CoerceResults[T](handles, &t)
	}
	return
}

var _handlesPolicy Policy = &ContravariantPolicy{}
