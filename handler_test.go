package miruken

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/suite"
	"reflect"
	"testing"
)

type Counter interface {
	Count() int
	Inc() int
}

type Counted struct {
	count int
}

func (c *Counted) Count() int {
	return c.count
}

func (c *Counted) Inc() int {
	c.count++
	return c.count
}

type Foo struct {
	Counted
}

type Bar struct {
	Counted
}

// FooHandler

type FooHandler struct {}

func (h *FooHandler) Handle(
	callback interface{},
	greedy   bool,
	composer Handler,
) HandleResult {
	switch foo := callback.(type) {
	case *Foo:
		foo.Inc()
		return composer.Handle(Bar{}, false, nil)
	default:
		return NotHandled
	}
}

func (h *FooHandler) doSomething() {

}

// BarHandler

type BarHandler struct {}

func (h *BarHandler) HandleBar(
	policy Handles,
	bar    Bar,
) {
}

// CounterHandler

type CounterHandler struct {}

func (h *CounterHandler) HandleCounted(
	policy  Handles,
	counter Counter,
) (Counter, HandleResult) {
	switch c := counter.Inc(); {
	case c % 3 == 0:
		err := fmt.Errorf("%v is divisible by 3", c)
		return nil, NotHandled.WithError(err)
	case c % 2 == 0: return nil, NotHandled
	default: return counter, Handled
	}
}

// MultiHandler

type MultiHandler struct {
	foo Foo
	bar Bar
}

func (h *MultiHandler) HandleFoo(
	policy    Handles,
	foo      *Foo,
	composer  Handler,
) error {
	h.foo.Inc()
	if foo.Inc() == 5 {
		return errors.New("count reached 5")
	}
	composer.Handle(new(Bar), false, nil)
	return nil
}

func (h *MultiHandler) HandleBar(
	policy Handles,
	bar    *Bar,
) HandleResult {
	h.bar.Inc()
	if bar.Inc() % 2 == 0 {
		return Handled
	}
	return NotHandled
}

// EverythingHandler

type EverythingHandler struct{}

func (h *EverythingHandler) HandleEverything(
	policy   Handles,
	callback interface{},
) HandleResult {
	switch f := callback.(type) {
	case *Foo:
		f.Inc()
		return Handled
	case Counter:
		f.Inc()
		f.Inc()
		return Handled
	default:
		return NotHandled
	}
}

// SpecificationHandler

type SpecificationHandler struct{}

func (h *SpecificationHandler) HandleFoo(
	binding *struct {
		Handles  `strict:"true"`
	},
	foo *Foo,
) HandleResult {
	foo.Inc()
	return Handled
}

// InvalidHandler

type InvalidHandler struct {}

func (h *InvalidHandler) MissingCallback(
	policy Handles,
) {
}

func (h *InvalidHandler) TooManyReturnValues(
	policy Handles,
	bar    *Bar,
) (int, string, Counter) {
	return 0, "bad", nil
}

func (h *InvalidHandler) SecondReturnMustBeErrorOrHandleResult(
	policy   Handles,
	counter *Counter,
) (Foo, string) {
	return Foo{}, "bad"
}

func (h *InvalidHandler) UntypedInterfaceDependency(
	policy Handles,
	bar    *Bar,
	any     interface{},
) HandleResult {
	return Handled
}

type HandlerTestSuite struct {
	suite.Suite
}

func (suite *HandlerTestSuite) SetupTest() {
}

func (suite *HandlerTestSuite) TestHandles() {
	suite.Run("Invariant", func () {
		handler := NewHandleContext(WithHandlers(new(FooHandler), new(BarHandler)))
		foo     := new(Foo)
		result  := handler.Handle(foo, false, nil)
		suite.False(result.IsError())
		suite.Equal(Handled, result)
		suite.Equal(1, foo.Count())
	})

	suite.Run("Covariant", func () {
		handler := NewHandleContext(WithHandlers(new(CounterHandler)))
		foo    := new(Foo)
		result := handler.Handle(foo, false, nil)
		suite.False(result.IsError())
		suite.Equal(Handled, result)
		suite.Equal(1, foo.Count())
	})

	suite.Run("HandleResult", func () {
		handler := NewHandleContext(WithHandlers(new(CounterHandler)))

		suite.Run("Handled", func() {
			foo := new(Foo)
			foo.Inc()
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(NotHandled, result)
		})

		suite.Run("NotHandled", func() {
			foo := new(Foo)
			foo.Inc()
			foo.Inc()
			result := handler.Handle(foo, false, nil)
			suite.True(result.IsError())
			suite.Equal("3 is divisible by 3", result.Error().Error())
		})
	})

	suite.Run("Multiple", func () {
		multi   := new(MultiHandler)
		handler := NewHandleContext(WithHandlers(multi))
		foo     := new(Foo)

		for i := 0; i < 4; i++ {
			result := handler.Handle(foo, false, nil)
			suite.Equal(Handled, result)
			suite.Equal(i + 1, foo.Count())
		}

		suite.Equal(4, multi.foo.Count())
		suite.Equal(4, multi.bar.Count())

		result := handler.Handle(foo, false, nil)
		suite.True(result.IsError())
		suite.Equal("count reached 5", result.Error().Error())

		suite.Equal(5, multi.foo.Count())
		suite.Equal(4, multi.bar.Count())
	})

	suite.Run("Everything", func () {
		handler := NewHandleContext(WithHandlers(new(EverythingHandler)))

		suite.Run("Invariant", func () {
			foo    := new(Foo)
			result := handler.Handle(foo, false, nil)

			suite.False(result.IsError())
			suite.Equal(Handled, result)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Contravariant", func () {
			bar    := new(Bar)
			result := handler.Handle(bar, false, nil)

			suite.False(result.IsError())
			suite.Equal(Handled, result)
			suite.Equal(2, bar.Count())
		})
	})

	suite.Run("Specification", func () {
		handler := NewHandleContext(WithHandlers(new(SpecificationHandler)))

		suite.Run("Strict", func() {
			foo    := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(Handled, result)
			suite.Equal(1, foo.Count())
		})
	})

	suite.Run("Command", func () {
		handler := NewHandleContext(WithHandlers(new(CounterHandler)))

		suite.Run("Invoke", func () {
			suite.Run("Invariant", func() {
				var foo *Foo
				if err := Invoke(handler, new(Foo), &foo); err == nil {
					suite.NotNil(*foo)
					suite.Equal(1, foo.Count())
				} else {
					suite.Failf("unexpected error: %v", err.Error())
				}
			})

			suite.Run("Contravariant", func() {
				var foo interface{}
				if err := Invoke(handler, new(Foo), &foo); err == nil {
					suite.NotNil(foo)
					suite.IsType(&Foo{}, foo)
					suite.Equal(1, foo.(*Foo).Count())
				} else {
					suite.Failf("unexpected error: %v", err.Error())
				}
			})
		})

		suite.Run("InvokeAll", func () {
			handler := NewHandleContext(WithHandlers(
				new(CounterHandler), new(SpecificationHandler)))

			suite.Run("Invariant", func () {
				var foo []*Foo
				if err := InvokeAll(handler, new(Foo), &foo); err == nil {
					suite.NotNil(foo)
					suite.Len(foo, 1)
					suite.Equal(2, foo[0].Count())
				} else {
					suite.Failf("unexpected error: %v", err.Error())
				}
			})
		})
	})

	suite.Run("Invalid", func () {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(*HandlerDescriptorError); ok {
					failures := 0
					var errMethod *MethodBindingError
					for reason := errors.Unwrap(err.Reason);
						errors.As(reason, &errMethod); reason = errors.Unwrap(reason) {
						failures++
					}
					suite.Equal(4, failures)
				} else {
					suite.Fail("Expected HandlerDescriptorError")
				}
			}
		}()

		NewHandleContext(WithHandlers(new(InvalidHandler)))
	})
}

// FooProvider
type FooProvider struct {
	foo Foo
}

func (f *FooProvider) ProvideFoo(policy Provides) *Foo {
	f.foo.Inc()
	return &f.foo
}

// ListProvider

type ListProvider struct {}

func (f *ListProvider) ProvideFooSlice(
	policy Provides,
) []*Foo {
	return []*Foo{{Counted{1}}, {Counted{2}}}
}

func (f *ListProvider) ProvideFooArray(
	policy Provides,
) [2]*Bar {
	return [2]*Bar{{Counted{3}}, {Counted{4}}}
}

// MultiProvider

type MultiProvider struct {
	foo Foo
	bar Bar
}

func (p *MultiProvider) ProvideFoo(
	policy Provides,
) *Foo {
	p.foo.Inc()
	return &p.foo
}

func (p *MultiProvider) ProvideBar(
	policy Provides,
) (*Bar, HandleResult) {
	if p.bar.Inc() % 3 == 0 {
		return &p.bar, NotHandled.WithError(
			fmt.Errorf("%v is divisible by 3", p.bar.Count()))
	}
	if p.bar.Inc() % 2 == 0 {
		return &p.bar, NotHandled
	}
	return &p.bar, Handled
}

// SpecificationProvider

type SpecificationProvider struct{
	foo Foo
	bar Bar
}

func (p *SpecificationProvider) ProvidesFoo(
	binding *struct {
		Provides
	},
) *Foo {
	p.foo.Inc()
	return &p.foo
}

func (p *SpecificationProvider) ProvidesBar(
	binding *struct {
		Provides  `strict:"true"`
    },
) []*Bar {
	p.bar.Inc()
	return []*Bar{&p.bar, {}}
}

type GenericProvider struct{}

func (p *GenericProvider) Provide(
	policy   Provides,
	inquiry *Inquiry,
) interface{} {
	if inquiry.Key() == reflect.TypeOf((*Foo)(nil)) {
		return &Foo{}
	}
	if inquiry.Key() == reflect.TypeOf((*Bar)(nil)) {
		return &Bar{}
	}
	return nil
}

// InvalidProvider

type InvalidProvider struct {}

func (p *InvalidProvider) MissingReturnValue(
	policy Provides,
) {
}

func (p *InvalidProvider) TooManyReturnValues(
	policy Provides,
) (*Foo, string, Counter) {
	return nil, "bad", nil
}

func (p *InvalidProvider) InvalidHandleResultReturnValue(
	policy Provides,
) HandleResult {
	return Handled
}

func (p *InvalidProvider) InvalidErrorReturnValue(
	policy Provides,
) error {
	return errors.New("not good")
}

func (p *InvalidProvider) SecondReturnMustBeErrorOrHandleResult(
	policy Provides,
) (*Foo, string) {
	return &Foo{}, "bad"
}

func (p *InvalidProvider) UntypedInterfaceDependency(
	policy Provides,
	any    interface{},
) *Foo {
	return &Foo{}
}

func (suite *HandlerTestSuite) TestProvides() {
	suite.Run("Implied", func () {
		handler := NewHandleContext(WithHandlers(new(FooProvider)))
		var fooProvider *FooProvider
		err := Resolve(handler, &fooProvider)
		suite.Nil(err)
		suite.NotNil(fooProvider)
	})

	suite.Run("Invariant", func () {
		handler := NewHandleContext(WithHandlers(new(FooProvider)))
		var foo *Foo
		err := Resolve(handler, &foo)
		suite.Nil(err)
		suite.Equal(1, foo.Count())
	})

	suite.Run("Covariant", func () {
		handler := NewHandleContext(WithHandlers(new(FooProvider)))
		var counter Counter
		err := Resolve(handler, &counter)
		suite.Nil(err)
		suite.Equal(1, counter.Count())
		if foo, ok := counter.(*Foo); !ok {
			suite.Fail(fmt.Sprintf("expected *Foo, but found %T", foo))
		}
	})

	suite.Run("NotHandledReturnNil", func () {
		handler := NewHandleContext()
		var foo *Foo
		err := Resolve(handler, &foo)
		suite.Nil(err)
		suite.Nil(foo)
	})

	suite.Run("Generic", func () {
		handler := NewHandleContext(WithHandlers(new(GenericProvider)))
		var foo *Foo
		err := Resolve(handler, &foo)
		suite.Nil(err)
		suite.Equal(0, foo.Count())
		var bar *Bar
		err = Resolve(handler, &bar)
		suite.Nil(err)
		suite.Equal(0, bar.Count())
	})

	suite.Run("Multiple", func () {
		handler := NewHandleContext(WithHandlers(new(MultiProvider)))
		var foo *Foo
		err := Resolve(handler, &foo)
		suite.Nil(err)
		suite.Equal(1, foo.Count())

		var bar *Bar
		err = Resolve(handler, &bar)
		suite.Nil(err)
		suite.Nil(bar)

		err = Resolve(handler, &bar)
		suite.NotNil(err)
		suite.Equal("3 is divisible by 3", err.Error())
		suite.Nil(bar)
	})

	suite.Run("Specification", func () {
		handler := NewHandleContext(WithHandlers(new(SpecificationProvider)))

		suite.Run("Invariant", func () {
			var foo *Foo
			err := Resolve(handler, &foo)
			suite.Nil(err)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Strict", func () {
			var bar *Bar
			err := Resolve(handler, &bar)
			suite.Nil(err)
			suite.Nil(bar)

			var bars []*Bar
			err = Resolve(handler, &bars)
			suite.Nil(err)
			suite.NotNil(bars)
			suite.Equal(2, len(bars))
		})
	})

	suite.Run("Lists", func () {
		handler := NewHandleContext(WithHandlers(new(ListProvider)))

		suite.Run("Slice", func () {
			var foo *Foo
			err := Resolve(handler, &foo)
			suite.Nil(err)
			suite.NotNil(foo)
		})

		suite.Run("Array", func () {
			var bar *Bar
			err := Resolve(handler, &bar)
			suite.Nil(err)
			suite.NotNil(bar)
		})
	})

	suite.Run("ResolveAll", func () {
		suite.Run("Invariant", func () {
			handler := NewHandleContext(WithHandlers(
				new(FooProvider), new(MultiProvider), new (SpecificationProvider)))
			var foo []*Foo
			if err := ResolveAll(handler, &foo); err == nil {
				suite.NotNil(foo)
				suite.Len(foo, 3)
				suite.True(foo[0] != foo[1])
			} else {
				suite.Failf("unexpected error: %v", err.Error())
			}
		})

		suite.Run("Covariant", func () {
			handler := NewHandleContext(WithHandlers(new(ListProvider)))
			var counted []Counter
			if err := ResolveAll(handler, &counted); err == nil {
				suite.NotNil(counted)
				suite.Len(counted, 4)
			} else {
				suite.Failf("unexpected error: %v", err.Error())
			}
		})

		suite.Run("Empty", func () {
			handler := NewHandleContext(WithHandlers(new(FooProvider)))
			var bars []*Bar
			err := ResolveAll(handler, &bars)
			suite.Nil(err)
			suite.NotNil(bars)
		})
	})

	suite.Run("With", func () {
		handler := NewHandleContext()
		var fooProvider *FooProvider
		err := Resolve(handler, &fooProvider)
		suite.Nil(err)
		suite.Nil(fooProvider)
		err = Resolve(With(handler, new(FooProvider)), &fooProvider)
		suite.Nil(err)
		suite.NotNil(fooProvider)
	})

	suite.Run("Invalid", func () {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(*HandlerDescriptorError); ok {
					failures := 0
					var errMethod *MethodBindingError
					for reason := errors.Unwrap(err.Reason);
						errors.As(reason, &errMethod); reason = errors.Unwrap(reason) {
						failures++
					}
					suite.Equal(6, failures)
				} else {
					suite.Fail("Expected HandlerDescriptorError")
				}
			}
		}()

		NewHandleContext(WithHandlers(new(InvalidProvider)))
	})
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}