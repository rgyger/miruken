package miruken

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"reflect"
)

// ContravariantPolicy defines related input values.
type ContravariantPolicy struct {
	FilteredScope
}

func (p *ContravariantPolicy) Variance() Variance {
	return Contravariant
}

func (p *ContravariantPolicy) AcceptResults(
	results []interface{},
) (result interface{}, accepted HandleResult) {
	switch len(results) {
	case 0:
		return nil, Handled
	case 1:
		switch result := results[0].(type) {
		case error:
			return nil, NotHandled.WithError(result)
		case HandleResult:
			return nil, result
		default:
			return result, Handled
		}
	case 2:
		switch result := results[1].(type) {
		case error:
			return results[0], NotHandled.WithError(result)
		case HandleResult:
			return results[0], result
		}
	}
	return nil, NotHandled.WithError(
		errors.New("contravariant policy: cannot accept more than 2 results"))
}

func (p *ContravariantPolicy) Less(
	binding, otherBinding Binding,
) bool {
	if binding == nil {
		panic("binding cannot be nil")
	}
	if otherBinding == nil {
		panic("otherBinding cannot be be nil")
	}
	key := binding.Key()
	if otherBinding.Matches(key, Invariant) {
		return false
	}
	return otherBinding.Matches(key, Contravariant)
}

func (p *ContravariantPolicy) NewMethodBinding(
	method  reflect.Method,
	spec   *policySpec,
) (binding Binding, invalid error) {
	methodType := method.Type
	numArgs    := methodType.NumIn() - 1  // skip receiver
	args       := make([]arg, numArgs)
	args[0]     = spec.arg
	key        := spec.key

	// Callback argument must be present
	if len(args) > 1 {
		if key == nil {
			key = methodType.In(2)
		}
		args[1] = callbackArg{}
	} else {
		invalid = errors.New("contravariant: missing callback argument")
	}

	if err := buildDependencies(methodType, 2, numArgs, args, 2); err != nil {
		invalid = multierror.Append(invalid, fmt.Errorf("contravariant: %w", err))
	}

	switch methodType.NumOut() {
	case 0, 1: break
	case 2:
		switch methodType.Out(1) {
		case _errorType, _handleResType: break
		default:
			invalid = multierror.Append(invalid, fmt.Errorf(
				"contravariant policy: when two return values, second must be %v or %v",
				_errorType, _handleResType))
		}
	default:
		invalid = multierror.Append(invalid, fmt.Errorf(
			"contravariant policy: at most two return values allowed and second must be %v or %v",
			_errorType, _handleResType))
	}

	if invalid != nil {
		return nil, MethodBindingError{method, invalid}
	}

	return &methodBinding{
		methodInvoke{method, args},
		FilteredScope{spec.filters},
		key,
		spec.flags,
	}, nil
}
