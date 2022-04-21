package miruken

import "reflect"

type inferenceHandler struct {
	descriptor *HandlerDescriptor
}

type inference struct {
	callback any
	resolved map[reflect.Type]struct{}
}

func (h *inferenceHandler) Handle(
	callback any,
	greedy   bool,
	composer Handler,
) HandleResult {
	return DispatchCallback(h, callback, greedy, composer)
}

func (h *inferenceHandler) DispatchPolicy(
	policy      Policy,
	callback    any,
	rawCallback Callback,
	greedy      bool,
	composer    Handler,
) HandleResult {
	if test, ok := rawCallback.(interface{CanInfer() bool}); ok && !test.CanInfer() {
		return NotHandled
	}
	infer := &inference{callback: callback}
	return h.descriptor.Dispatch(policy, h, infer, rawCallback, greedy, composer)
}

func (h *inferenceHandler) suppressDispatch() {}

// bindingIntercept intercepts Binding invocations to handler inference.
type bindingIntercept struct {
	Binding
	handlerType reflect.Type
	skipFilters bool
}

func (b *bindingIntercept) Filters() []FilterProvider {
	if b.skipFilters {
		return nil
	}
	return b.Binding.Filters()
}

func (b *bindingIntercept) SkipFilters() bool {
	return b.skipFilters
}

func (b *bindingIntercept) Invoke(
	context      HandleContext,
	explicitArgs ... any,
) ([]any, error) {
	if ctor, ok := b.Binding.(*constructorBinding); ok {
		return ctor.Invoke(context)
	}
	handlerType := b.handlerType
	if infer, ok := context.Callback().(*inference); ok {
		if visited := infer.resolved; visited == nil {
			infer.resolved = map[reflect.Type]struct{} {
				handlerType: {},
			}
		} else if _, ok := visited[handlerType]; ok {
			return nil, nil
		} else {
			visited[handlerType] = struct{}{}
		}
	}
	var builder ResolvingBuilder
	builder.
		WithCallback(context.RawCallback()).
		WithKey(b.handlerType)
	resolving := builder.NewResolving()
	if result := context.Composer().Handle(resolving, true, nil); result.IsError() {
		return nil, result.Error()
	} else {
		return []any{result}, nil
	}
}

func newInferenceHandler(
	factory HandlerDescriptorFactory,
	types   []reflect.Type,
) *inferenceHandler {
	if factory == nil {
		panic("factory cannot be nil")
	}
	bindings := make(policyBindingsMap)
	for _, typ := range types {
		if descriptor, added, err := factory.RegisterHandlerType(typ); err != nil {
			panic(err)
		} else if added {
			for policy, bs := range descriptor.bindings {
				pb := bindings.forPolicy(policy)
				// Us bs.index vs.variant since inference ONLY needs a
				// single binding to infer the handler type for a
				// specific key.
				for _, elem := range bs.index {
					binding := elem.Value.(Binding)
					_, ctorBinding := binding.(*constructorBinding)
					pb.insert(&bindingIntercept{
						binding,
						descriptor.handlerType,
						!ctorBinding,
					})
				}
				for _, bs := range bs.invariant {
					if len(bs) > 0 {
						b := bs[0]  // only need first
						_, ctorBinding := b.(*constructorBinding)
						pb.insert(&bindingIntercept{
							b,
							descriptor.handlerType,
							!ctorBinding,
						})
					}
				}
			}
		}
	}
	return &inferenceHandler {
		&HandlerDescriptor{
			handlerType: _inferenceHandlerType,
			bindings:    bindings,
		},
	}
}

var _inferenceHandlerType = TypeOf[inferenceHandler]()