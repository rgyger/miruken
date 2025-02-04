package miruken

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/promise"
	"reflect"
)

// ContravariantPolicy matches related input values.
type ContravariantPolicy struct {
	FilteredScope
}


var (
	ErrConResultsExceeded = errors.New("contravariant: cannot accept more than 2 results")
	ErrConMissingCallback = errors.New("contravariant: missing callback argument")
)


func (p *ContravariantPolicy) VariantKey(
	key any,
) (variant bool, unknown bool) {
	if typ, ok := key.(reflect.Type); ok {
		return true, internal.IsAny(typ)
	}
	return false, false
}

func (p *ContravariantPolicy) MatchesKey(
	key, otherKey any,
	invariant     bool,
) (matches bool, exact bool) {
	if key == otherKey {
		return true, true
	} else if invariant {
		return false, false
	} else if bt, isType := key.(reflect.Type); isType {
		if internal.IsAny(bt) {
			return true, false
		} else if kt, isType := otherKey.(reflect.Type); isType {
			if kt.AssignableTo(bt) {
				return true, false
			}
			if kt.Kind() == reflect.Ptr && kt.Elem().AssignableTo(bt) {
				return true, false
			}
		}
	}
	return false, false
}

func (p *ContravariantPolicy) Strict() bool {
	return true
}

func (p *ContravariantPolicy) Less(
	binding, otherBinding Binding,
) bool {
	if binding == nil {
		panic("binding cannot be nil")
	}
	if otherBinding == nil {
		panic("otherBinding cannot be nil")
	}
	matches, exact := p.MatchesKey(otherBinding.Key(), binding.Key(), false)
	return !exact && matches
}

func (p *ContravariantPolicy) AcceptResults(
	results []any,
) (result any, accepted HandleResult) {
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
		default:
			return results[0], Handled
		}
	}
	return nil, NotHandled.WithError(ErrConResultsExceeded)
}

func (p *ContravariantPolicy) NewMethodBinding(
	method reflect.Method,
	spec   *bindingSpec,
	key    any,
) (Binding, error) {
	if args, key, err := validateContravariantFunc(method.Type, spec, key,1); err != nil {
		return nil, &MethodBindingError{method, err}
	} else {
		return &methodBinding{
			funcCall{method.Func, args},
			BindingBase{
				FilteredScope{spec.filters},
				spec.flags, spec.metadata,
			}, key, method, spec.lt,
		}, nil
	}
}

func (p *ContravariantPolicy) NewFuncBinding(
	fun  reflect.Value,
	spec *bindingSpec,
	key  any,
) (Binding, error) {
	if args, key, err := validateContravariantFunc(fun.Type(), spec, key,0); err != nil {
		return nil, &FuncBindingError{fun, err}
	} else {
		return &funcBinding{
			funcCall{fun, args},
			BindingBase{
				FilteredScope{spec.filters},
				spec.flags, spec.metadata,
			}, key, spec.lt,
		}, nil
	}
}


func validateContravariantFunc(
	funType reflect.Type,
	spec    *bindingSpec,
	key     any,
	skip    int,
) (args []arg, ck any, err error) {
	ck       = key
	numArgs := funType.NumIn()
	numOut  := funType.NumOut()
	args     = make([]arg, numArgs-skip)
	args[0]  = spec.arg
	index   := 1

	// Source argument must be present if spec
	if len(args) > 1 {
		if arg := funType.In(1+skip); arg.AssignableTo(callbackType) {
			args[1] = CallbackArg{}
			if ck == nil {
				ck = internal.AnyType
			}
		} else {
			if ck == nil {
				ck = arg
			}
			args[1] = sourceArg{}
		}
		index++
	} else if _, isSpec := spec.arg.(zeroArg); isSpec {
		err = ErrConMissingCallback
	} else if ck == nil {
		ck = internal.AnyType
	}

	if err2 := buildDependencies(funType, index+skip, numArgs, args, index); err2 != nil {
		err = multierror.Append(err, fmt.Errorf("contravariant: %w", err2))
	}

	resIdx := -1

	for i := 0; i < numOut; i++ {
		out := funType.Out(i)
		if out.AssignableTo(internal.ErrorType) {
			if i != numOut-1 {
				err = multierror.Append(err, fmt.Errorf(
					"contravariant: error found at index %v must be last return", i))
			}
		} else if out.AssignableTo(handleResType) {
			if i != numOut-1 {
				err = multierror.Append(err, fmt.Errorf(
					"contravariant: HandleResult found at index %v must be last return", i))
			}
		} else if out.AssignableTo(sideEffectType) {
			// ignore side-effects
		} else if resIdx >= 0 {
			err = multierror.Append(err, fmt.Errorf(
				"contravariant: effective return at index %v conflicts with index %v", i, resIdx))
		} else {
			resIdx = i
			if lt, ok := promise.Inspect(out); ok {
				spec.flags = spec.flags | bindingAsync
				out = lt
			}
			spec.setLogicalOutputType(out)
		}
	}
	return
}
