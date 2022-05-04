package miruken

import (
	"reflect"
)

type (
	// Callback represents an action.
	Callback interface {
		Key() any
		Many() bool
		Policy() Policy
		ResultType() reflect.Type
		Result() any
		SetResult(result any)
		ReceiveResult(
			result   any,
			strict   bool,
			composer Handler,
		) HandleResult
	}

	// AcceptResultFunc validates callback results.
	AcceptResultFunc func (
		result   any,
		composer Handler,
	) HandleResult

	// CallbackBase is abstract Callback implementation.
	CallbackBase struct {
		many    bool
		results []any
		result  any
		accept  AcceptResultFunc
	}
)

func (c *CallbackBase) Many() bool {
	return c.many
}

func (c *CallbackBase) ResultType() reflect.Type {
	return nil
}

func (c *CallbackBase) Result() any {
	if result := c.result; result == nil {
		if c.many {
			if c.results == nil {
				c.results = make([]any, 0, 0)
			}
			c.result = c.results
		} else {
			if len(c.results) == 0 {
				c.result = nil
			} else {
				c.result = c.results[0]
			}
		}
	}
	return c.result
}

func (c *CallbackBase) SetResult(result any) {
	c.result = result
}

func (c *CallbackBase) AddResult(
	result   any,
	composer Handler,
) HandleResult {
	if IsNil(result) {
		return NotHandled
	}
	if c.accept != nil {
		return c.accept(result, composer)
	}
	c.results = append(c.results, result)
	c.result  = nil
	return Handled
}

func (c *CallbackBase) ReceiveResult(
	result   any,
	strict   bool,
	composer Handler,
) HandleResult {
	return c.AddResult(result, composer)
}

func (c *CallbackBase) CopyResult(target any) {
	if c.Many() {
		CopySliceIndirect(c.Result().([]any), target)
	} else {
		CopyIndirect(c.Result(), target)
	}
}

type CallbackBuilder struct {
	many   bool
	accept AcceptResultFunc
}

func (b *CallbackBuilder) WithMany() *CallbackBuilder {
	b.many = true
	return b
}

func (b *CallbackBuilder) WithAcceptResult(
	accept AcceptResultFunc,
) *CallbackBuilder {
	b.accept = accept
	return b
}

func (b *CallbackBuilder) CallbackBase() CallbackBase {
	return CallbackBase{many: b.many, accept: b.accept}
}

// customizeDispatch marks customized Callback dispatch.
type customizeDispatch interface {
 	Dispatch(
		handler  any,
		greedy   bool,
		composer Handler,
	) HandleResult
}

// suppressDispatch marks a type that opts out of Callback dispatch.
type suppressDispatch interface {
	SuppressDispatch()
}

// CallbackGuard detects and prevents circular Callback dispatch.
type CallbackGuard interface {
	CanDispatch(
		handler any,
		binding Binding,
	) (reset func (), approved bool)
}
