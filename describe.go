package miruken

import (
	"container/list"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/miruken-go/miruken/promise"
	"reflect"
	"sync"
)

type (
	// policyBindings maintains Binding's for a Policy.
	policyBindings struct {
		policy    Policy
		variant   list.List
		index     map[any]*list.Element
		invariant map[any][]Binding
	}

	// policyBindingsMap maps Policy's to policyBindings.
	policyBindingsMap map[Policy]*policyBindings
)

func (p *policyBindings) insert(binding Binding) {
	key := binding.Key()
	if variant, unknown := p.policy.VariantKey(key); variant {
		indexedElem := p.index[key]
		if unknown {
			elem := p.variant.PushBack(binding)
			if indexedElem == nil {
				p.index[key] = elem
			}
			return
		}
		insert := indexedElem
		if insert == nil {
			insert = p.variant.Front()
		}
		for insert != nil && !p.policy.Less(binding, insert.Value.(Binding)) {
			insert = insert.Next()
		}
		var elem *list.Element
		if insert != nil {
			elem = p.variant.InsertBefore(binding, insert)
		} else {
			elem = p.variant.PushBack(binding)
		}
		if indexedElem == nil {
			p.index[key] = elem
		}
	} else {
		if p.invariant == nil {
			p.invariant = make(map[any][]Binding)
			p.invariant[key] = []Binding{binding}
		} else {
			bindings := append(p.invariant[key], binding)
			p.invariant[key] = bindings
		}
	}
}

func (p *policyBindings) reduce(
	key     any,
	reducer BindingReducer,
) (result HandleResult) {
	if reducer == nil {
		panic("reducer cannot be nil")
	}
	done := false
	result = NotHandled
	// Check variant keys (reflect.Type)
	if variant, _ := p.policy.VariantKey(key); variant {
		elem := p.index[key]
		if elem == nil {
			elem = p.variant.Front()
		}
		for elem != nil {
			if result, done = reducer(elem.Value.(Binding), result); done {
				break
			}
			elem = elem.Next()
		}
		return result
	// Check invariant keys (string)
	} else if p.invariant != nil {
		if bs := p.invariant[key]; bs != nil {
			for _, b := range bs {
				if result, done = reducer(b, result); done {
					return result
				}
			}
		}
	}
	// Check unknown keys (any)
	if unk := p.index[anyType]; unk != nil {
		for unk != nil {
			if result, done = reducer(unk.Value.(Binding), result); done {
				break
			}
			unk = unk.Next()
		}
	}
	return result
}

func (p policyBindingsMap) forPolicy(policy Policy) *policyBindings {
	bindings, found := p[policy]
	if !found {
		bindings = &policyBindings{
			policy: policy,
			index:  make(map[any]*list.Element),
		}
		p[policy] = bindings
	}
	return bindings
}

type (
	// HandlerDescriptor manages Handler Binding's.
	HandlerDescriptor struct {
		FilteredScope
		spec     HandlerSpec
		bindings policyBindingsMap
	}

	// HandlerSpec creates a HandlerDescriptor.
	HandlerSpec interface {
		fmt.Stringer
		PkgPath() string
		key() any
		suppress() bool
		newHandlerDescriptor(
			builder bindingSpecFactory,
			observers []HandlerDescriptorObserver,
		) (*HandlerDescriptor, error)
	}

	// HandlerTypeSpec creates a HandlerDescriptor for a set of methods.
	HandlerTypeSpec struct {
		typ reflect.Type
	}

	// HandlerFuncSpec creates a HandlerDescriptor for a single function.
	HandlerFuncSpec struct {
		fun reflect.Value
	}

	// HandlerDescriptorError reports a failed HandlerDescriptor.
	HandlerDescriptorError struct {
		Spec  HandlerSpec
		Cause error
	}
)

func (s HandlerTypeSpec) Type() reflect.Type {
	return s.typ
}

func (s HandlerTypeSpec) Name() string {
	typ := s.typ
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ.Name()
}

func (s HandlerTypeSpec) PkgPath() string {
	typ := s.typ
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ.PkgPath()
}

func (s HandlerTypeSpec) String() string {
	return fmt.Sprintf("%v in %v package", s.typ, s.PkgPath())
}

func (s HandlerTypeSpec) key() any {
	return s.typ
}

func (s HandlerTypeSpec) suppress() bool {
	return s.typ.Implements(suppressDispatchType)
}

func (s HandlerTypeSpec) newHandlerDescriptor(
	factory   bindingSpecFactory,
	observers []HandlerDescriptorObserver,
) (descriptor *HandlerDescriptor, invalid error) {
	typ        := s.typ
	bindings   := make(policyBindingsMap)
	descriptor  = &HandlerDescriptor{spec: s}

	var ctorSpec *bindingSpec
	var ctorPolicies []policyKey
	var constructor *reflect.Method
	// Add constructor implicitly
	if ctor, ok := typ.MethodByName("Constructor"); ok {
		constructor = &ctor
		ctorType   := ctor.Type
		if spec, err := factory.createSpec(ctorType, 2); err == nil {
			if spec != nil {
				ctorSpec     = spec
				ctorPolicies = spec.policies
			}
		} else {
			invalid = multierror.Append(invalid, err)
		}
	}
	if _, noImplicit := typ.MethodByName("NoConstructor"); !noImplicit {
		addProvides := true
		for _, ctorPk := range ctorPolicies {
			if _, ok := ctorPk.policy.(*providesPolicy); ok {
				addProvides = false
				break
			}
		}
		if addProvides {
			ctorPolicies = append(ctorPolicies, policyKey{policy: providesPolicyInstance})
		}
	} else if constructor != nil {
		invalid = multierror.Append(invalid, fmt.Errorf(
			"handler %v has both a Constructor and NoConstructor method", typ))
	}
	for _, ctorPk := range ctorPolicies {
		policy := ctorPk.policy
		if binder, ok := policy.(ConstructorBinder); ok {
			if ctor, err := binder.NewConstructorBinding(typ, constructor, ctorSpec, ctorPk.key); err == nil {
				for _, observer := range observers {
					observer.BindingCreated(policy, descriptor, ctor)
				}
				bindings.forPolicy(policy).insert(ctor)
			} else {
				invalid = multierror.Append(invalid, err)
			}
		}
	}
	// Add callback factory explicitly
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Name == "Constructor" || method.Name == "NoConstructor" {
			continue
		}
		methodType := method.Type
		if spec, err := factory.createSpec(methodType, 2); err == nil {
			if spec == nil { // not a handler method
				continue
			}
			for _, pk := range spec.policies {
				policy := pk.policy
				if binder, ok := policy.(MethodBinder); ok {
					if binding, errBind := binder.NewMethodBinding(method, spec, pk.key); binding != nil {
						for _, observer := range observers {
							observer.BindingCreated(policy, descriptor, binding)
						}
						bindings.forPolicy(policy).insert(binding)
					} else if errBind != nil {
						invalid = multierror.Append(invalid, errBind)
					}
				}
			}
		} else {
			invalid = multierror.Append(invalid, err)
		}
	}
	if invalid != nil {
		return nil, &HandlerDescriptorError{s, invalid}
	}
	descriptor.bindings = bindings
	return descriptor, nil
}

func (s HandlerFuncSpec) Func() reflect.Value {
	return s.fun
}

func (s HandlerFuncSpec) String() string {
	return fmt.Sprintf("HandlerFuncSpec(%v)", s.fun)
}

func (s HandlerFuncSpec) PkgPath() string {
	return ""
}

func (s HandlerFuncSpec) key() any {
	return s.fun.Pointer()
}

func (s HandlerFuncSpec) suppress() bool {
	return false
}

func (s HandlerFuncSpec) newHandlerDescriptor(
	factory   bindingSpecFactory,
	observers []HandlerDescriptorObserver,
) (descriptor *HandlerDescriptor, invalid error) {
	funType    := s.fun.Type()
	bindings   := make(policyBindingsMap)
	descriptor  = &HandlerDescriptor{spec: s}

	if spec, err := factory.createSpec(funType, 1); err == nil {
		if spec == nil {
			invalid = fmt.Errorf("first argument is not a callback spec")
		} else {
			for _, pk := range spec.policies {
				policy := pk.policy
				if binder, ok := policy.(FuncBinder); ok {
					if binding, errBind := binder.NewFuncBinding(s.fun, spec, pk.key); binding != nil {
						for _, observer := range observers {
							observer.BindingCreated(policy, descriptor, binding)
						}
						bindings.forPolicy(policy).insert(binding)
					} else if errBind != nil {
						invalid = multierror.Append(invalid, errBind)
					}
				} else {
					invalid = multierror.Append(invalid, fmt.Errorf(
						"policy %T does not support function bindings", policy))
				}
			}
		}
	} else {
		invalid = multierror.Append(invalid, err)
	}
	if invalid != nil {
		return nil, &HandlerDescriptorError{s, invalid}
	}
	descriptor.bindings = bindings
	return descriptor, nil
}

func (e *HandlerDescriptorError) Error() string {
	return fmt.Sprintf("invalid handler: %v cause: %v", e.Spec, e.Cause)
}

func (e *HandlerDescriptorError) Unwrap() error {
	return e.Cause
}

func (d *HandlerDescriptor) HandlerSpec() HandlerSpec {
	return d.spec
}

func (d *HandlerDescriptor) Dispatch(
	policy   Policy,
	handler  any,
	callback Callback,
	greedy   bool,
	composer Handler,
	guard    CallbackGuard,
) (result HandleResult) {
	if pb, found := d.bindings[policy]; found {
		key := callback.Key()
		return pb.reduce(key, func (
			binding Binding,
			result  HandleResult,
		) (HandleResult, bool) {
			if result.stop || (result.handled && !greedy) {
				return result, true
			}
			if matches, _ := policy.MatchesKey(binding.Key(), key, false); matches {
				if guard != nil {
					reset, approve := guard.CanDispatch(handler, binding)
					defer func() {
						if reset != nil {
							reset()
						}
					}()
					if !approve { return result, false }
				}
				if guard, ok := callback.(CallbackGuard); ok {
					reset, approve := guard.CanDispatch(handler, binding)
					defer func() {
						if reset != nil {
							reset()
						}
					}()
					if !approve { return result, false }
				}
				var filters []providedFilter
				if check, ok := callback.(interface{
					CanFilter() bool
				}); !ok || check.CanFilter() {
					var tp []FilterProvider
					if tf, ok := handler.(Filter); ok {
						tp = []FilterProvider{
							&FilterInstanceProvider{[]Filter{tf}, true},
						}
					}
					if providedFilters, err := orderedFilters(
						composer, binding, callback, binding.Filters(),
						d.Filters(), policy.Filters(), tp);
						providedFilters != nil && err == nil {
						filters = providedFilters
					} else {
						return result, false
					}
				}
				var out  []any
				var pout *promise.Promise[[]any]
				var err  error
				ctx := HandleContext{
					handler,
					callback,
					binding,
					composer,
					greedy,
				}
				if len(filters) == 0 {
					out, pout, err = binding.Invoke(ctx)
				} else {
					out, pout, err = pipeline(ctx, filters,
						func(ctx HandleContext) ([]any, *promise.Promise[[]any], error) {
							return binding.Invoke(ctx)
					})
				}
				if err == nil {
					if pout != nil {
						out = []any{promise.Then(pout, func(oo []any) any {
							res, _ := policy.AcceptResults(oo)
							return res
						})}
					}
					res, accept := policy.AcceptResults(out)
					if res != nil {
						if accept.handled {
							strict := policy.Strict() || binding.Strict()
							accept = accept.And(callback.ReceiveResult(res, strict, composer))
						}
					}
					result = result.Or(accept)
				} else {
					switch err.(type) {
					case *RejectedError:
					case *NotHandledError:
					case *UnresolvedArgError:
						break
					default:
						result = result.WithError(err)
					}
				}
			}
			return result, result.stop || (result.handled && !greedy)
		})
	}
	return NotHandled
}

type (
	// HandlerDescriptorProvider returns HandlerDescriptor's.
	HandlerDescriptorProvider interface {
		Descriptor(spec any) *HandlerDescriptor
	}

	// HandlerDescriptorFactory registers HandlerDescriptor's.
	HandlerDescriptorFactory interface {
		HandlerDescriptorProvider
		NewSpec(spec any) HandlerSpec
		RegisterSpec(spec any) (*HandlerDescriptor, bool, error)
	}

	// HandlerDescriptorObserver observes HandlerDescriptor creation.
	HandlerDescriptorObserver interface {
		BindingCreated(
			policy     Policy,
			descriptor *HandlerDescriptor,
			binding    Binding,
		)

		DescriptorCreated(descriptor *HandlerDescriptor)
	}
	BindingObserverFunc func(Policy, *HandlerDescriptor, Binding)
)

func (f BindingObserverFunc) BindingCreated(
	policy     Policy,
	descriptor *HandlerDescriptor,
	binding    Binding,
) {
	f(policy, descriptor, binding)
}

// mutableDescriptorFactory creates HandlerDescriptor's on demand.
type mutableDescriptorFactory struct {
	sync.RWMutex
	bindingSpecFactory
	descriptors map[any]*HandlerDescriptor
	observers   []HandlerDescriptorObserver
}

func (f *mutableDescriptorFactory) NewSpec(
	spec any,
) HandlerSpec {
	if IsNil(spec) {
		panic("spec cannot be nil")
	}
	var hs HandlerSpec
	switch h := spec.(type) {
	case HandlerSpec:
		hs = h
	case reflect.Type:
		hs = HandlerTypeSpec{h}
	default:
		typ := reflect.TypeOf(spec)
		if typ.Kind() == reflect.Func {
			hs = HandlerFuncSpec{reflect.ValueOf(spec)}
		} else {
			hs = HandlerTypeSpec{typ}
		}
	}
	if hs.suppress() {
		return nil
	}
	return hs
}

func (f *mutableDescriptorFactory) Descriptor(
	spec any,
) *HandlerDescriptor {
	handler := f.NewSpec(spec)
	if handler == nil {
		return nil
	}
	f.RLock()
	defer f.RUnlock()
	return f.descriptors[handler.key()]
}

func (f *mutableDescriptorFactory) RegisterSpec(
	spec any,
) (*HandlerDescriptor, bool, error) {
	handler := f.NewSpec(spec)
	if handler == nil {
		return nil, false, nil
	}

	f.Lock()
	defer f.Unlock()

	key := handler.key()
	if descriptor := f.descriptors[key]; descriptor != nil {
		return descriptor, false, nil
	}
	if descriptor, err := handler.newHandlerDescriptor(f.bindingSpecFactory, f.observers); err == nil {
		for _, observer := range f.observers {
			observer.DescriptorCreated(descriptor)
		}
		f.descriptors[key] = descriptor
		return descriptor, true, nil
	} else {
		return nil, false, err
	}
}

// HandlerDescriptorFactoryBuilder build the HandlerDescriptorFactory.
type HandlerDescriptorFactoryBuilder struct {
	parsers   []BindingParser
	observers []HandlerDescriptorObserver
}

func (b *HandlerDescriptorFactoryBuilder) Parsers(
	parsers ...BindingParser,
) *HandlerDescriptorFactoryBuilder {
	b.parsers = append(b.parsers, parsers...)
	return b
}

func (b *HandlerDescriptorFactoryBuilder) Observers(
	observers ...HandlerDescriptorObserver,
) *HandlerDescriptorFactoryBuilder {
	b.observers = append(b.observers, observers...)
	return b
}

func (b *HandlerDescriptorFactoryBuilder) Build() HandlerDescriptorFactory {
	factory := &mutableDescriptorFactory{
		descriptors: make(map[any]*HandlerDescriptor),
		observers:   b.observers,
	}
	parsers := make([]BindingParser, len(b.parsers)+4)
	parsers[0] = &factory.bindingSpecFactory
	parsers[1] = BindingParserFunc(parseOptions)
	parsers[2] = BindingParserFunc(parseFilters)
	parsers[3] = BindingParserFunc(parseConstraints)
	for i, binding := range b.parsers {
		parsers[i+4] = binding
	}
	factory.bindingSpecFactory.parsers = parsers
	return factory
}

func CurrentHandlerDescriptorFactory(
	handler Handler,
) HandlerDescriptorFactory {
	if IsNil(handler) {
		panic("handler cannot be nil")
	}
	get := &currentHandlerDescriptorFactory{}
	handler.Handle(get, false, handler)
	return get.factory
}

// currentHandlerDescriptorFactory resolves the current HandlerDescriptorFactory
type currentHandlerDescriptorFactory struct {
	factory HandlerDescriptorFactory
}

func (g *currentHandlerDescriptorFactory) Handle(
	callback any,
	greedy   bool,
	composer Handler,
) HandleResult {
	if comp, ok := callback.(*Composition); ok {
		callback = comp.callback
	}
	if getFactory, ok := callback.(*currentHandlerDescriptorFactory); ok {
		getFactory.factory = g.factory
		return Handled
	}
	return NotHandled
}

func (g *currentHandlerDescriptorFactory) SuppressDispatch() {}

func (g *currentHandlerDescriptorFactory) CabBatch() bool {
	return false
}

var suppressDispatchType = TypeOf[suppressDispatch]()
