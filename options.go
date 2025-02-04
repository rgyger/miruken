package miruken

import (
	"dario.cat/mergo"
	"fmt"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/promise"
	"reflect"
)

type (
	// Option provides an explicit representation of
	// an optional value when the Zero value is not
	// sufficient to distinguish unset values.
	Option[T any] struct {
		set bool
		val T
	}

	// FromOptions is a DependencyResolver for option arguments.
	FromOptions struct {}

	// optCallback captures option values.
 	optCallback struct {
		options any
	}

	// optionsHandler uses an option instance to merge from
	// during an option resolution.
	optionsHandler struct {
		Handler
		options any
		optionsType reflect.Type
	}

	// optionMerger defines customer merging behavior.
	optionMerger struct {}

	// mergeable enables custom merge behavior.
	mergeable interface {
		MergeFrom(options any) bool
	}
)


// Option[T]

func (o *Option[T]) Set() bool {
	return o.set
}

func (o *Option[T]) Value() T {
	return o.val
}

func (o *Option[T]) ValueOrDefault(val T) T {
	if o.set {
		return o.val
	}
	return val
}

func (o *Option[T]) SetValue(val T) {
	o.val = val
}

func (o *Option[T]) MergeFrom(option any) bool {
	if o.set {
		return false
	}
	if other, ok := option.(Option[T]); ok && other.set {
		o.val = other.val
		o.set = true
		return true
	}
	return false
}

// Set creates a new Option set to val.
func Set[T any](val T) Option[T] {
	return Option[T]{true, val}
}


// Options returns a BuilderFunc that makes the provided
// options available for merging into matching options.
func Options(options any) BuilderFunc {
	optType := reflect.TypeOf(options)
	if optType == nil {
		panic("options cannot be nil")
	}
	if optType.Kind() == reflect.Ptr {
		optType = optType.Elem()
	}
	if optType.Kind() != reflect.Struct {
		panic("options must be a struct or *struct")
	}
	return func (handler Handler) Handler {
		return &optionsHandler{handler, options, optType}
	}
}


func GetOptions[T any](handler Handler) (t T, ok bool) {
	ok = GetOptionsInto(handler, &t)
	return
}

func GetOptionsInto(handler Handler, target any) bool {
	if internal.IsNil(handler) {
		panic("handler cannot be nil")
	}
	tv := internal.TargetValue(target)
	optType := tv.Type()
	if optType.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("options: %v is not a *struct or **struct", optType))
	}
	optType = optType.Elem()

	created := false
	options := optCallback{}

	switch optType.Kind() {
	case reflect.Struct:
		options.options = tv.Interface()
	case reflect.Ptr:
		if optType.Elem().Kind() != reflect.Struct {
			panic(fmt.Sprintf("options: %v is not a *struct or **struct", optType))
		}
		created = true
		if value := reflect.Indirect(tv); value.IsNil() {
			options.options = reflect.New(optType.Elem()).Interface()
		} else {
			options.options = value.Interface()
		}
	}

	handled := handler.Handle(options, true, nil).Handled()
	if handled && created {
		internal.CopyIndirect(options.options, target)
	}
	return handled
}

// MergeOptions merges from options into options
func MergeOptions(from, into any) bool {
	return mergo.Merge(into, from,
		mergo.WithAppendSlice,
		mergo.WithTransformers(mergeOptions)) == nil
}


// optCallback

func (o optCallback) CanInfer() bool {
	return false
}

func (o optCallback) CanFilter() bool {
	return false
}

func (o optCallback) CanBatch() bool {
	return false
}


// optionsHandler

func (c *optionsHandler) Handle(
	callback any,
	greedy   bool,
	composer Handler,
) HandleResult {
	if callback == nil {
		return NotHandled
	}
	tryInitializeComposer(&composer, c)
	cb := callback
	if comp, ok := cb.(*Composition); ok {
		if cb = comp.Callback(); cb == nil {
			return c.Handler.Handle(callback, greedy, composer)
		}
	}
	if opt, ok := cb.(optCallback); ok {
		options := opt.options
		if reflect.TypeOf(options).Elem().AssignableTo(c.optionsType) {
			merged := false
			if o, ok := options.(mergeable); ok {
				merged = o.MergeFrom(c.options)
			} else {
				merged = MergeOptions(c.options, opt.options)
			}
			if merged {
				if greedy {
					return c.Handler.Handle(callback, greedy, composer).Or(Handled)
				}
				return Handled
			}
		}
	}
	return c.Handler.Handle(callback, greedy, composer)
}


// FromOptions

func (o FromOptions) Validate(
	typ reflect.Type,
	dep DependencyArg,
) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("FromOptions: %v is not a struct or *struct", typ)
	}
	return nil
}

func (o FromOptions) Resolve(
	typ reflect.Type,
	dep DependencyArg,
	ctx HandleContext,
) (options reflect.Value, _ *promise.Promise[reflect.Value], err error) {
	options = reflect.New(typ)
	if GetOptionsInto(ctx, options.Interface()) {
		if typ.Kind() == reflect.Ptr {
			return options, nil, nil
		}
		return reflect.Indirect(options), nil, nil
	}
	if dep.Optional() {
		return reflect.Zero(typ), nil, nil
	}
	var v reflect.Value
	return v, nil, fmt.Errorf("FromOptions: unable to resolve options %v", typ)
}


// optionMerger

func (t optionMerger) Transformer(
	typ reflect.Type,
) func(dst, src reflect.Value) error {
	addr := false
	if !typ.AssignableTo(mergeableType) && typ.Kind() != reflect.Ptr {
		typ = reflect.PtrTo(typ)
		addr = true
	}
	if !addr || typ.AssignableTo(mergeableType) {
		return func(dst, src reflect.Value) error {
			if addr {
				dst = dst.Addr()
			}
			if d, ok := dst.Interface().(mergeable); ok && !internal.IsNil(d) {
				if s := src.Interface(); !internal.IsNil(s) {
					d.MergeFrom(s)
				}
			}
			return nil
		}
	}
	return nil
}


var (
	mergeableType = internal.TypeOf[mergeable]()
	mergeOptions  = optionMerger{}
)