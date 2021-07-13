package miruken

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"reflect"
	"unicode"
)

// arg represents a parameter to a method
type arg interface {
	resolve(
		typ      reflect.Type,
		receiver interface{},
		context  HandleContext,
	) (reflect.Value, error)
}

// receiverArg is the receiver of the method call
type receiverArg struct {}

func (a receiverArg) resolve(
	typ      reflect.Type,
	receiver interface{},
	context  HandleContext,
) (reflect.Value, error) {
	return reflect.ValueOf(receiver), nil
}

// zeroArg returns the Zero value of the argument type
type zeroArg struct {}

func (a zeroArg) resolve(
	typ      reflect.Type,
	receiver interface{},
	context  HandleContext,
) (reflect.Value, error) {
	return reflect.Zero(typ), nil
}

// callbackArg returns the callback or raw callback
type callbackArg struct {}

func (a callbackArg) resolve(
	typ      reflect.Type,
	receiver interface{},
	context  HandleContext,
) (reflect.Value, error) {
	if v := reflect.ValueOf(context.Callback); v.Type().AssignableTo(typ) {
		return v, nil
	}
	if v := reflect.ValueOf(context.RawCallback); v.Type().AssignableTo(typ) {
		return v, nil
	}
	return reflect.ValueOf(nil), fmt.Errorf("arg: unable to resolve callback: %v", typ)
}

// dependencySpec collects metadata for a dependency
type dependencySpec struct {
	index    int
	flags    bindingFlags
	resolver DependencyResolver
}

func (s *dependencySpec) bindingAt(
	index  int,
	field  reflect.StructField,
) error {
	if s.index >= 0 {
		return fmt.Errorf(
			"field index %v already designated as the dependency", s.index)
	}
	if unicode.IsLower(rune(field.Name[0])) {
		return fmt.Errorf(
			"field %v at index %v must start with an uppercase character",
			field.Name, index)
	}
	s.index = index
	return nil
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
			"only one dependency resolver allowed, found %#v", resolver)
	}
	s.resolver = resolver
	return nil
}

// dependencyArg is a parameter resolved at runtime
type dependencyArg struct {
	spec *dependencySpec
}

func (d dependencyArg) ArgType(
	typ reflect.Type,
) reflect.Type {
	if d.spec != nil && d.spec.index >= 0 {
		return typ.Field(d.spec.index).Type
	}
	return typ
}

func (d dependencyArg) resolve(
	typ      reflect.Type,
	receiver interface{},
	context  HandleContext,
) (reflect.Value, error) {
	composer := context.Composer
	if typ == _handlerType {
		return reflect.ValueOf(composer), nil
	}
	if typ == _handleCtxType {
		return reflect.ValueOf(context), nil
	}
	rawCallback := context.RawCallback
	if rawCallback != nil {
		if rawType := reflect.TypeOf(rawCallback); rawType.AssignableTo(typ) {
			return reflect.ValueOf(rawCallback), nil
		}
	}
	argIndex := -1
	var resolver DependencyResolver = &_defaultResolver
	if spec := d.spec; spec != nil {
		argIndex = spec.index
		if spec.resolver != nil {
			resolver = spec.resolver
		}
	}
	val, err := resolver.Resolve(typ, rawCallback, d, composer)
	if err == nil {
		if argIndex >= 0 {
			wrapper := reflect.New(typ.Elem())
			wrapper.Elem().Field(argIndex).Set(val)
			return wrapper, nil
		}
	}
	return val, err
}

// DependencyResolver defines how an argument value is retrieved
type DependencyResolver interface {
	Resolve(
		typ         reflect.Type,
		rawCallback interface{},
		dep         dependencyArg,
		handler     Handler,
	) (reflect.Value, error)
}

// defaultDependencyResolver retrieves the value from the Handler
type defaultDependencyResolver struct{}

func (r *defaultDependencyResolver) Resolve(
	typ         reflect.Type,
	rawCallback interface{},
	dep         dependencyArg,
	handler     Handler,
) (reflect.Value, error) {
	argType  := typ
	optional, strict := false, false

	if spec := dep.spec; spec != nil {
		optional = spec.flags & bindingOptional == bindingOptional
		strict   = spec.flags & bindingStrict == bindingStrict
		argType  = dep.ArgType(typ.Elem())
	}

	var inquiry *Inquiry
	parent, _ := rawCallback.(*Inquiry)
	builder := new(InquiryBuilder).WithParent(parent)

	if !strict && argType.Kind() == reflect.Slice {
		builder.WithKey(argType.Elem()).WithMany()
	} else {
		builder.WithKey(argType)
	}

	inquiry = builder.NewInquiry()
	if result, err := inquiry.Resolve(handler); err == nil {
		var val reflect.Value
		if inquiry.Many() {
			results := result.([]interface{})
			val = reflect.New(argType).Elem()
			CopySliceIndirect(results, val)
		} else if result != nil {
			val = reflect.ValueOf(result)
		} else if optional {
			val = reflect.Zero(argType)
		} else {
			return reflect.ValueOf(nil), fmt.Errorf(
				"arg: unable to resolve dependency %v", argType)
		}
		return val, nil
	} else {
		return reflect.ValueOf(nil), fmt.Errorf(
			"arg: unable to resolve dependency %v: %w", argType, err)
	}
}

type HandleContext struct {
	Callback    interface{}
	RawCallback interface{}
	Composer    Handler
	Results     ResultReceiver
}

// Dependency typed

var dependencyBuilders = []bindingBuilder{
	bindingBuilderFunc(optionsBindingBuilder),
	bindingBuilderFunc(resolverBindingBuilder),
}

func buildDependency(
	argType reflect.Type,
) (arg dependencyArg, err error) {
	// Is it a *Struct arg binding?
	if argType.Kind() != reflect.Ptr {
		return dependencyArg{}, nil
	}
	arg = dependencyArg{}
	argType = argType.Elem()
	if argType.Kind() == reflect.Struct &&
		argType.Name() == "" &&  // anonymous
		argType.NumField() > 0 {
		spec := &dependencySpec{index: -1}
		if err = configureBinding(argType, spec, dependencyBuilders);
			err != nil || spec.index < 0 {
			return arg, err
		}
		arg.spec = spec
		if resolver := spec.resolver; resolver != nil {
			if v, ok := resolver.(interface {
				Validate(reflect.Type, dependencyArg) error
			}); ok {
				err = v.Validate(argType, arg)
			}
		}
	}
	return arg, err
}

func resolverBindingBuilder(
	index   int,
	field   reflect.StructField,
	binding interface{},
) (err error) {
	if field.Type.AssignableTo(_depResolverType) {
		if b, ok := binding.(interface {
			setResolver(resolver DependencyResolver) error
		}); ok {
			resolver := newStructField(field).(DependencyResolver)
			if invalid := b.setResolver(resolver); invalid != nil {
				err = multierror.Append(err, fmt.Errorf(
					"binding: dependency resolver %#v at field %v (%v) failed: %w",
					resolver, field.Name, index, invalid))
			}
		}
	}
	return err
}

var (
	_handlerType     = reflect.TypeOf((*Handler)(nil)).Elem()
	_handleCtxType   = reflect.TypeOf((*HandleContext)(nil)).Elem()
	_depResolverType = reflect.TypeOf((*DependencyResolver)(nil)).Elem()
	_defaultResolver = defaultDependencyResolver{}
)
