package miruken

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"reflect"
)

// covariantPolicy

type covariantPolicy struct{}

func (p *covariantPolicy) Variance() Variance {
	return Covariant
}

func (p *covariantPolicy) AcceptResults(
	results []interface{},
) (result interface{}, accepted HandleResult) {
	switch len(results) {
	case 0:
		return nil, Handled
	case 1:
		if result = results[0]; result == nil {
			return nil, NotHandled
		}
		return result, Handled
	case 2:
		result = results[0]
		switch err := results[1].(type) {
		case error:
			return result, NotHandled.WithError(err)
		case HandleResult:
			if result == nil {
				return nil, err.And(NotHandled)
			}
			return result, err
		default:
			if result == nil {
				return nil, NotHandled
			}
			return result, Handled
		}
	}
	return nil, NotHandled.WithError(
		errors.New("covariant policy: cannot accept more than 2 results"))
}

func (p *covariantPolicy) Less(
	binding, otherBinding Binding,
) bool {
	if binding == nil {
		panic("binding cannot be nil")
	}
	if otherBinding == nil {
		panic("otherBinding cannot be nil")
	}
	constraint := binding.Constraint()
	if otherBinding.Matches(constraint, Invariant) {
		return false
	} else if otherBinding.Matches(constraint, Covariant) {
		return true
	}
	return false
}

func (p *covariantPolicy) newMethodBinding(
	method  reflect.Method,
	spec   *bindingSpec,

) (binding Binding, invalid error) {
	methodType := method.Type
	numArgs    := methodType.NumIn()
	args       := make([]arg, numArgs)

	args[0] = _receiverArg
	args[1] = _zeroArg  // policy/binding placeholder

	for i := 2; i < numArgs; i++ {
		if methodType.In(i) == _interfaceType {
			invalid = multierror.Append(invalid,
				fmt.Errorf(
					"covariant policy: %v dependency at index %v not allowed",
					_interfaceType, i))

		} else {
			args[i] = _dependencyArg
		}
	}

	switch methodType.NumOut() {
	case 0:
		invalid = multierror.Append(invalid,
			errors.New("covariant policy: must have a return value"))
	case 1:
		if err := validateCovariantReturn(methodType.Out(0), spec); err != nil {
			invalid = multierror.Append(invalid, err)
		}
	case 2:
		if err := validateCovariantReturn(methodType.Out(0), spec); err != nil {
			invalid = multierror.Append(invalid, err)
		}
		switch methodType.Out(1) {
		case _errorType, _handleResType: break
		default:
			invalid = multierror.Append(invalid, fmt.Errorf(
				"covariant policy: when two return values, second must be %v or %v",
				_errorType, _handleResType))
		}
	default:
		invalid = multierror.Append(invalid, fmt.Errorf(
			"covariant policy: at most two return values allowed and second must be %v or %v",
			_errorType, _handleResType))
	}

	if invalid != nil {
		return nil, &MethodBindingError{method, invalid}
	}

	return &methodBinding{
		spec:   spec,
		method: method,
		args:   args,
	}, nil
}

func validateCovariantReturn(
	returnType  reflect.Type,
	spec       *bindingSpec,
) error {
	switch returnType {
	case _errorType, _handleResType:
		return fmt.Errorf(
			"covariant policy: primary return value must not be %v or %v",
			_errorType, _handleResType)
	default:
		if spec.constraint == nil {
			if !spec.strict {
				switch returnType.Kind() {
				case reflect.Slice, reflect.Array:
					spec.constraint = returnType.Elem()
					return nil
				}
			}
			spec.constraint = returnType
		}
		return nil
	}
}
