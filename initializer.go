package miruken

import "math"

// initializer is a Filter that invokes a 'constructor'
// method on the current result of the pipeline.
type initializer struct {
	constructor methodInvoke
}

func (i *initializer) Order() int {
	return math.MaxInt32
}

func (i *initializer) Next(
	next     Next,
	context  HandleContext,
	provider FilterProvider,
)  ([]interface{}, error) {
	instance, err := next.Filter()
	if err == nil && len(instance) > 0 {
		var results []interface{}
		results, err = i.constructor.Invoke(context, instance[0])
		if len(results) > 0 {
			if e, ok := results[len(results)-1].(error); ok {
				err = e
			}
		}
	}
	return instance, err
}

// initializerProvider is a FilterProvider for initializer.
type initializerProvider struct {
	filters []Filter
}

func (i *initializerProvider) Required() bool {
	return true
}

func (i *initializerProvider) AppliesTo(
	callback interface{},
) bool {
	switch callback.(type) {
	case *Inquiry, *Creates: return true
	default: return false
	}
}

func (i *initializerProvider) Filters(
	binding  Binding,
	callback interface{},
	composer Handler,
) ([]Filter, error) {
	return i.filters, nil
}