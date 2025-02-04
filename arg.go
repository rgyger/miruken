package miruken

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/promise"
	"reflect"
)

type (
	// arg models a parameter of a Handler method.
	arg interface {
		flags() bindingFlags
		resolve(
			reflect.Type,
			HandleContext,
		) (reflect.Value, *promise.Promise[reflect.Value], error)
	}
)


// zeroArg returns the Zero value of the argument type.
type zeroArg struct {}

func (z zeroArg) flags() bindingFlags {
	return bindingNone
}

func (z zeroArg) resolve(
	typ reflect.Type,
	ctx HandleContext,
) (reflect.Value, *promise.Promise[reflect.Value], error) {
	return reflect.Zero(typ), nil, nil
}

// CallbackArg returns the raw callback.
type CallbackArg struct {}

func (c CallbackArg) flags() bindingFlags {
	return bindingNone
}

func (c CallbackArg) resolve(
	typ reflect.Type,
	ctx HandleContext,
) (reflect.Value, *promise.Promise[reflect.Value], error) {
	val := reflect.ValueOf(ctx.Callback)
	if val.Type().AssignableTo(typ) {
		return val, nil, nil
	}
	return reflect.Zero(typ), nil, nil
}

// sourceArg returns the callback source.
type sourceArg struct {}

func (s sourceArg) flags() bindingFlags {
	return bindingNone
}

func (s sourceArg) resolve(
	typ reflect.Type,
	ctx HandleContext,
) (reflect.Value, *promise.Promise[reflect.Value], error) {
	if cb := ctx.Callback; cb != nil {
		if src := cb.Source(); src != nil {
			v := reflect.ValueOf(src)
			if t := v.Type(); t.AssignableTo(typ) {
				return v, nil, nil
			} else if t.Kind() == reflect.Ptr && t.Elem().AssignableTo(typ) {
				return v.Elem(), nil, nil
			}
		}
	}
	var v reflect.Value
	return v, nil, fmt.Errorf("arg: unable to resolve source: %v", typ)
}

// dependencySpec encapsulates dependency metadata.
type dependencySpec struct {
	logicalType reflect.Type
	resolver    DependencyResolver
	constraints []any
	flags       bindingFlags
	metadata    []any
}

func (s *dependencySpec) setStrict(
	index  int,
	field  reflect.StructField,
	strict bool,
) error {
	s.flags = s.flags | bindingStrict
	return nil
}

func (s *dependencySpec) setOptional(
	index  int,
	field  reflect.StructField,
	strict bool,
) error {
	s.flags = s.flags | bindingOptional
	return nil
}

func (s *dependencySpec) setResolver(
	resolver DependencyResolver,
) error {
	if s.resolver != nil {
		return fmt.Errorf(
			"only one dependency resolver allowed, found %T", resolver)
	}
	s.resolver = resolver
	return nil
}

func (s *dependencySpec) addConstraint(
	constraint Constraint,
) error {
	s.constraints = append(s.constraints, constraint)
	return nil
}

func (s *dependencySpec) addMetadata(
	metadata any,
) error {
	s.metadata = append(s.metadata, metadata)
	return nil
}

// DependencyArg is a parameter resolved at runtime.
type DependencyArg struct {
	spec *dependencySpec
}

func (d DependencyArg) flags() bindingFlags {
	if spec := d.spec; spec != nil {
		return spec.flags
	}
	return bindingNone
}

func (d DependencyArg) Optional() bool {
	return d.spec != nil && d.spec.flags & bindingOptional == bindingOptional
}

func (d DependencyArg) Strict() bool {
	return d.spec != nil && d.spec.flags & bindingStrict == bindingStrict
}

func (d DependencyArg) Promise() bool {
	return d.spec != nil && d.spec.flags &bindingAsync == bindingAsync
}

func (d DependencyArg) Metadata() []any {
	if spec := d.spec; spec != nil {
		return spec.metadata
	}
	return nil
}

func (d DependencyArg) logicalType(
	typ reflect.Type,
) reflect.Type {
	if spec := d.spec; spec != nil {
		if lt := spec.logicalType; lt != nil {
			return lt
		}
	}
	return typ
}

func (d DependencyArg) resolve(
	typ reflect.Type,
	ctx HandleContext,
) (reflect.Value, *promise.Promise[reflect.Value], error) {
	typ = d.logicalType(typ)
	if typ == handlerType {
		return reflect.ValueOf(ctx.Composer), nil, nil
	}
	if typ == handleCtxType {
		return reflect.ValueOf(ctx), nil, nil
	}
	if callback := ctx.Callback; callback != nil {
		if val := reflect.ValueOf(callback); val.Type().AssignableTo(typ) {
			return val, nil, nil
		} else if typ.AssignableTo(callbackType) {
			return reflect.Zero(typ), nil, nil
		}
		if src := callback.Source(); src != nil {
			if val := reflect.ValueOf(src); val.Type().AssignableTo(typ) {
				return val, nil, nil
			}
		}
	}
	var resolver DependencyResolver = &defaultResolver
	if spec := d.spec; spec != nil {
		if spec.resolver != nil {
			resolver = spec.resolver
		}
	}
	return resolver.Resolve(typ, d, ctx)
}

// DependencyResolver defines how an argument value is retrieved.
type DependencyResolver interface {
	Resolve(
		reflect.Type,
		DependencyArg,
		HandleContext,
	) (reflect.Value, *promise.Promise[reflect.Value], error)
}

// defaultDependencyResolver Resolves the value from the Handler.
type defaultDependencyResolver struct{}

func (r *defaultDependencyResolver) Resolve(
	typ reflect.Type,
	dep DependencyArg,
	ctx HandleContext,
) (v reflect.Value, pv *promise.Promise[reflect.Value], err error) {
	parent, _ := ctx.Callback.(*Provides)
	many := !dep.Strict() && typ.Kind() == reflect.Slice
	var builder ProvidesBuilder
	builder.WithParent(parent).ForOwner(ctx.Handler)
	if many {
		builder.WithKey(typ.Elem())
	} else {
		builder.WithKey(typ)
	}
	if spec := dep.spec; spec != nil {
		builder.WithConstraints(spec.constraints...)
	}
	provides := builder.New()
	if result, pr, err2 := provides.Resolve(ctx, many); err2 != nil {
		err = fmt.Errorf("arg: unable to resolve dependency %v: %w", typ, err2)
	} else if pr == nil {
		if many {
			v = reflect.New(typ).Elem()
			internal.CopySliceIndirect(result.([]any), v)
		} else if result != nil {
			v = reflect.ValueOf(result)
		} else if dep.Optional() {
			v = reflect.Zero(typ)
		} else {
			err = fmt.Errorf("arg: unable to resolve dependency %v", typ)
		}
	} else {
		pv = promise.Then(pr, func(res any) reflect.Value {
			var val reflect.Value
			if many {
				val = reflect.New(typ).Elem()
				internal.CopySliceIndirect(res.([]any), val)
			} else if res != nil {
				val = reflect.ValueOf(res)
			} else if dep.Optional() {
				val = reflect.Zero(typ)
			} else {
				panic(fmt.Errorf("arg: unable to resolve dependency %v", typ))
			}
			return val
		})
	}
	return
}


// UnresolvedArgError reports a failed resolve an arg.
type UnresolvedArgError struct {
	arg    arg
	Reason error
}

func (e *UnresolvedArgError) Error() string {
	return fmt.Sprintf("unresolved arg %+v: %v", e.arg, e.Reason)
}

func (e *UnresolvedArgError) Unwrap() error {
	return e.Reason
}


// Dependency typed

var dependencyParsers = []BindingParser{
	BindingParserFunc(parseOptions),
	BindingParserFunc(parseResolver),
	BindingParserFunc(parseConstraints),
}

func buildDependencies(
	funTyp     reflect.Type,
	startIndex int,
	endIndex   int,
	args       []arg,
	offset     int,
) (invalid error) {
	var lastSpec *dependencySpec
	for i, j := startIndex, 0; i < endIndex; i, j = i + 1, j + 1 {
		argType := funTyp.In(i)
		if arg, err := buildDependency(argType); err == nil {
			if arg.spec != nil {
				if lastSpec != nil {
					invalid = multierror.Append(invalid, fmt.Errorf(
						"expected dependency at index %v, but found spec", i))
				} else {
					lastSpec = arg.spec // capture spec for actual dependency
					args[j + offset] = zeroArg{}
				}
			} else {
				if lastSpec != nil {
					arg.spec = lastSpec // adopt last dependency spec
					if resolver := lastSpec.resolver; resolver != nil {
						if v, ok := resolver.(interface {
							Validate(reflect.Type, DependencyArg) error
						}); ok {
							if err := v.Validate(argType, arg); err != nil {
								invalid = multierror.Append(invalid, err)
							}
						}
					}
					lastSpec = nil
				}
				if lt, ok := promise.Inspect(argType); ok {
					if spec := arg.spec; spec == nil {
						arg.spec = &dependencySpec{flags: bindingAsync}
					} else {
						spec.flags = spec.flags | bindingAsync
					}
					arg.spec.logicalType = lt
				}
				args[j + offset] = arg
			}
		} else {
			invalid = multierror.Append(invalid, fmt.Errorf(
				"invalid dependency at index %v: %w", i, err))
		}
	}
	if lastSpec != nil {
		invalid = multierror.Append(invalid, fmt.Errorf(
			"missing dependency at index %v", endIndex))
	}
	return invalid
}

func buildDependency(
	argType reflect.Type,
) (arg DependencyArg, err error) {
	if internal.IsAny(argType) {
		return arg, fmt.Errorf(
			"type %v cannot be used As a dependency",
			internal.AnyType)
	}
	// Is it a *struct arg binding?
	if argType.Kind() != reflect.Ptr {
		return arg, nil
	}
	argType = argType.Elem()
	if argType.Kind() == reflect.Struct &&
		argType.Name() == "" {  // anonymous
		spec := &dependencySpec{}
		if err = parseSpec(argType, spec, dependencyParsers); err != nil {
			return arg, err
		}
		arg.spec = spec
	}
	return arg, err
}

func parseResolver(
	index   int,
	field   reflect.StructField,
	binding any,
) (bound bool, err error) {
	if dr := internal.CoerceToPtr(field.Type, depResolverType); dr != nil {
		bound = true
		if b, ok := binding.(interface {
			setResolver(DependencyResolver) error
		}); ok {
			if resolver, invalid := internal.NewWithTag(dr, field.Tag); invalid != nil {
				err = fmt.Errorf(
					"parseResolver: new dependency resolver at field %v (%v) failed: %w",
					field.Name, index, invalid)
			} else if invalid := b.setResolver(resolver.(DependencyResolver)); invalid != nil {
				err = fmt.Errorf(
					"parseResolver: dependency resolver %T at field %v (%v) failed: %w",
					resolver, field.Name, index, invalid)
			}
		}
	}
	return bound, err
}

var (
	handlerType     = internal.TypeOf[Handler]()
	handleCtxType   = internal.TypeOf[HandleContext]()
	depResolverType = internal.TypeOf[DependencyResolver]()
	defaultResolver = defaultDependencyResolver{}
)
