package test

import (
	"errors"
	"fmt"
	"github.com/miruken-go/miruken"
	"github.com/stretchr/testify/suite"
	"reflect"
	"strings"
	"testing"
)

//go:generate $GOPATH/bin/miruken -tests

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

type (
	Foo struct { Counted }
	Bar struct { Counted }
	Baz struct { Counted }
	Bam struct { Counted }
	Boo struct { Counted }
)

// FooHandler
type FooHandler struct{}

func (h *FooHandler) Handle(
	callback any,
	greedy   bool,
	composer miruken.Handler,
) miruken.HandleResult {
	switch foo := callback.(type) {
	case *Foo:
		foo.Inc()
		return composer.Handle(Bar{}, false, nil)
	default:
		return miruken.NotHandled
	}
}

// BarHandler
type BarHandler struct {}

func (h *BarHandler) HandleBar(
	_*miruken.Handles, _ Bar,
) {
}

// CounterHandler
type CounterHandler struct {}

func (h *CounterHandler) HandleCounted(
	_*miruken.Handles, counter Counter,
) (Counter, miruken.HandleResult) {
	switch c := counter.Inc(); {
	case c > 0 && c % 3 == 0:
		err := fmt.Errorf("%v is divisible by 3", c)
		return nil, miruken.NotHandled.WithError(err)
	case c % 2 == 0:
		return nil, miruken.NotHandled
	default: return counter, miruken.Handled
	}
}

// CountByOneHandler
type CountByTwoHandler struct {}

func (h *CountByTwoHandler) HandleCounted(
	_*miruken.Handles, counter Counter,
) (Counter, miruken.HandleResult) {
	counter.Inc()
	counter.Inc()
	fmt.Println("CountByTwoHandler")
	return counter, miruken.Handled
}

// MultiHandler
type MultiHandler struct {
	foo Foo
	bar Bar
}

func (h *MultiHandler) HandleFoo(
	_*miruken.Handles, foo *Foo,
	composer miruken.Handler,
) error {
	h.foo.Inc()
	if foo.Inc() == 5 {
		return errors.New("count reached 5")
	}
	composer.Handle(new(Bar), false, nil)
	return nil
}

func (h *MultiHandler) HandleBar(
	_*miruken.Handles, bar *Bar,
) miruken.HandleResult {
	h.bar.Inc()
	if bar.Inc() % 2 == 0 {
		return miruken.Handled
	}
	return miruken.NotHandled
}

// EverythingHandler
type EverythingHandler struct{}

func (h *EverythingHandler) HandleEverything(
	_*miruken.Handles, callback any,
) miruken.HandleResult {
	switch cb := callback.(type) {
	case *Foo:
		cb.Inc()
		return miruken.Handled
	case Counter:
		cb.Inc()
		cb.Inc()
		return miruken.Handled
	default:
		return miruken.NotHandled
	}
}

// EverythingImplicitHandler
type EverythingImplicitHandler struct{}

func (h *EverythingImplicitHandler) HandleEverything(
	handles *miruken.Handles,
) miruken.HandleResult {
	switch cb := handles.Source().(type) {
	case *Bar:
		cb.Inc()
		cb.Inc()
		return miruken.Handled
	case Counter:
		cb.Inc()
		cb.Inc()
		cb.Inc()
		return miruken.Handled
	default:
		return miruken.NotHandled
	}
}

// EverythingSpecHandler
type EverythingSpecHandler struct{}

func (h *EverythingSpecHandler) HandleEverything(
	_*struct { miruken.Handles }, callback any,
) miruken.HandleResult {
	switch cb := callback.(type) {
	case *Baz:
		cb.Inc()
		return miruken.Handled
	case Counter:
		cb.Inc()
		cb.Inc()
		return miruken.Handled
	default:
		return miruken.NotHandled
	}
}

// SpecificationHandler
type SpecificationHandler struct{}

func (h *SpecificationHandler) HandleFoo(
	_*struct{ miruken.Handles; miruken.Strict }, foo *Foo,
) miruken.HandleResult {
	foo.Inc()
	fmt.Println("SpecificationHandler")
	return miruken.Handled
}

// DependencyHandler
type DependencyHandler struct{}

func (h *DependencyHandler) RequiredDependency(
	_*miruken.Handles, foo *Foo,
	bar *Bar,
) {
	if bar == nil {
		panic("bar cannot be nil")
	}
	foo.Inc()
}

func (h *DependencyHandler) RequiredSliceDependency(
	_*miruken.Handles, boo *Boo,
	bars []*Bar,
) {
	boo.Inc()
	for _, bar := range bars {
		bar.Inc()
	}
}

func (h *DependencyHandler) OptionalDependency(
	_*miruken.Handles, bar *Bar,
	_*struct{ miruken.Optional }, foo *Foo,
) {
	bar.Inc()
	if foo != nil {
		foo.Inc()
	}
}

func (h *DependencyHandler) OptionalSliceDependency(
	_*miruken.Handles, baz *Baz,
	_*struct{ miruken.Optional }, bars []*Bar,
) {
	baz.Inc()
	for _, bar := range bars {
		bar.Inc()
	}
}

func (h *DependencyHandler) StrictDependency(
	_*miruken.Handles, bam *Bam,
	_*struct{ miruken.Strict }, bars []*Bar,
) {
	bam.Inc()
	for _, bar := range bars {
		bar.Inc()
	}
}

type UnmanagedHandler struct {}

func (u *UnmanagedHandler) NoConstructor() {}

type Config struct {
	baseUrl string
	timeout int
}

type Configuration struct {
	config *Config
}

func (c Configuration) Validate(
	typ reflect.Type,
	_   miruken.DependencyArg,
) error {
	if !reflect.TypeOf(c.config).AssignableTo(typ) {
		return fmt.Errorf("the Configuration resolver expects a %T field", c.config)
	}
	return nil
}

func (c Configuration) Resolve(
	typ      reflect.Type,
	callback miruken.Callback,
	dep      miruken.DependencyArg,
	handler  miruken.Handler,
) (reflect.Value, error) {
	if c.config == nil {
		c.config = &Config{
			baseUrl: "https://server/api",
			timeout: 30000,
		}
	}
	return reflect.ValueOf(c.config), nil
}

// DependencyResolverHandler
type DependencyResolverHandler struct{}

func (h *DependencyResolverHandler) UseDependencyResolver(
	_*miruken.Handles, foo *Foo,
	_*struct{ Configuration }, config *Config,
) *Config {
	foo.Inc()
	return config
}

// MixedHandler
type MixedHandler struct {}

func (m *MixedHandler) Mix(
	_*struct{
		miruken.Handles
		miruken.Maps
	  }, callback miruken.Callback,
) string {
	switch cb := callback.(type) {
	case *miruken.Handles:
		return fmt.Sprintf("Handles %T", cb.Source())
	case *miruken.Maps:
		return fmt.Sprintf("Maps %T", cb.Source())
	default:
		return ""
	}
}

// InvalidHandler
type InvalidHandler struct {}

func (h *InvalidHandler) Constructor() {}

func (h *InvalidHandler) NoConstructor() {}

func (h *InvalidHandler) MissingDependency(
	_*miruken.Handles, _ *Bar,
	_*struct{ },
) {
}

func (h *InvalidHandler) TooManyReturnValues(
	_*miruken.Handles, _ *Bar,
) (int, string, Counter) {
	return 0, "bad", nil
}

func (h *InvalidHandler) SecondReturnMustBeErrorOrHandleResult(
	_*miruken.Handles, _ *Counter,
) (Foo, string) {
	return Foo{}, "bad"
}

func (h *InvalidHandler) UntypedInterfaceDependency(
	_*miruken.Handles, _ *Bar,
	any any,
) miruken.HandleResult {
	return miruken.Handled
}

func (h *InvalidHandler) CallbackInterfaceArgument(
	cb miruken.Callback,
) miruken.HandleResult {
	return miruken.Handled
}

func (h *InvalidHandler) CallbackInterfaceSpec(
	*struct{ miruken.Callback },
) miruken.HandleResult {
	return miruken.Handled
}

func (h *InvalidHandler) MissingCallbackArgument(
	*struct{ miruken.Handles },
) miruken.HandleResult {
	return miruken.Handled
}

func HandleFoo(
	_*miruken.Handles, foo *Foo,
) miruken.HandleResult {
	foo.Inc()
	return miruken.Handled
}

func HandleCounted(
	_*struct{ miruken.Handles }, counter Counter,
) {
	counter.Inc()
	counter.Inc()
}

type HandlerTestSuite struct {
	suite.Suite
}

func (suite *HandlerTestSuite) Setup() miruken.Handler {
	return miruken.Setup(TestFeature, miruken.ExcludeHandlerSpecs(
		func (spec miruken.HandlerSpec) bool {
			switch ts := spec.(type) {
			case miruken.HandlerTypeSpec:
				return strings.Contains(ts.Name(), "Invalid")
			default:
				return false
			}
		}))
}

func (suite *HandlerTestSuite) SetupWith(features ... miruken.Feature) miruken.Handler {
	return miruken.Setup(features...)
}

func (suite *HandlerTestSuite) TestHandles() {
	suite.Run("Invariant", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(
				&FooHandler{},
				&BarHandler{}),
			miruken.WithHandlers(new(FooHandler), new(BarHandler)))
		foo     := new(Foo)
		result  := handler.Handle(foo, false, nil)
		suite.False(result.IsError())
		suite.Equal(miruken.Handled, result)
		suite.Equal(1, foo.Count())
	})

	suite.Run("Contravariant", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&CounterHandler{}),
			miruken.WithHandlers(new(CounterHandler)))
		foo    := new(Foo)
		result := handler.Handle(foo, false, nil)
		suite.False(result.IsError())
		suite.Equal(miruken.Handled, result)
		suite.Equal(1, foo.Count())
	})

	suite.Run("HandleResult", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&CounterHandler{}))
		suite.Run("Handled", func() {
			foo := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
		})

		suite.Run("NotHandled", func() {
			foo := new(Foo)
			foo.Inc()
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
		})

		suite.Run("NotHandled With Error", func() {
			foo := new(Foo)
			foo.Inc()
			foo.Inc()
			result := handler.Handle(foo, false, nil)
			suite.True(result.IsError())
			suite.Equal(miruken.NotHandledAndStop, result.WithoutError())
			suite.Equal("3 is divisible by 3", result.Error().Error())
		})
	})

	suite.Run("Multiple", func () {
		multi   := new(MultiHandler)
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&MultiHandler{}),
			miruken.WithHandlers(multi))

		foo := new(Foo)
		for i := 0; i < 4; i++ {
			result := handler.Handle(foo, false, nil)
			suite.Equal(miruken.Handled, result)
			suite.Equal(i + 1, foo.Count())
		}

		suite.Equal(4, multi.foo.Count())
		suite.Equal(8, multi.bar.Count())

		result := handler.Handle(foo, false, nil)
		suite.True(result.IsError())
		suite.Equal("count reached 5", result.Error().Error())

		suite.Equal(5, multi.foo.Count())
		suite.Equal(8, multi.bar.Count())
	})

	suite.Run("Everything", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&EverythingHandler{}),
			miruken.WithHandlers(new(EverythingHandler)))

		suite.Run("Invariant", func () {
			foo    := new(Foo)
			result := handler.Handle(foo, false, nil)

			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Contravariant", func () {
			bar    := new(Bar)
			result := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(2, bar.Count())
		})
	})

	suite.Run("EverythingImplicit", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&EverythingImplicitHandler{}),
			miruken.WithHandlers(new(EverythingImplicitHandler)))

		suite.Run("Invariant", func () {
			bar    := new(Bar)
			result := handler.Handle(bar, false, nil)

			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(2, bar.Count())
		})

		suite.Run("Contravariant", func () {
			foo    := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(3, foo.Count())
		})
	})

	suite.Run("EverythingSpec", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&EverythingSpecHandler{}),
			miruken.WithHandlers(new(EverythingSpecHandler)))

		suite.Run("Invariant", func () {
			baz    := new(Baz)
			result := handler.Handle(baz, false, nil)

			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, baz.Count())
		})

		suite.Run("Contravariant", func () {
			bar    := new(Bar)
			result := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(2, bar.Count())
		})
	})

	suite.Run("Specification", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&SpecificationHandler{}),
			miruken.WithHandlers(new(SpecificationHandler)))
		suite.Run("Strict", func() {
			foo    := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, foo.Count())
		})
	})

	suite.Run("Dependencies", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&DependencyHandler{}),
			miruken.WithHandlers(new(DependencyHandler)))
		suite.Run("Required", func () {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(miruken.MethodBindingError); ok {
						suite.Equal("RequiredDependency", err.Method().Name)
					} else {
						suite.Fail("Expected MethodBindingError")
					}
				}
			}()
			handler.Handle(new(Foo), false, nil)
		})

		suite.Run("RequiredSlice", func () {
			boo    := new(Boo)
			bars := []any{new(Bar), new(Bar)}
			result := miruken.Build(handler, miruken.With(bars...)).Handle(boo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, boo.Count())
			for _, bar := range bars {
				suite.Equal(1, bar.(*Bar).Count())
			}
		})

		suite.Run("Optional", func () {
			bar    := new(Bar)
			result := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, bar.Count())
		})

		suite.Run("OptionalWithValue", func () {
			bar    := new(Bar)
			foo    := new(Foo)
			result := miruken.Build(handler, miruken.With(foo)).Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, bar.Count())
			suite.Equal(1, foo.Count())
		})

		suite.Run("OptionalSlice", func () {
			baz    := new(Baz)
			bars   := []any{new(Bar), new(Bar)}
			result := handler.Handle(baz, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, baz.Count())
			result = miruken.Build(handler, miruken.With(bars...)).Handle(baz, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(2, baz.Count())
			for _, bar := range bars {
				suite.Equal(1, bar.(*Bar).Count())
			}
		})

		suite.Run("StrictSlice", func () {
			bam    := new(Bam)
			bars1  := []any{new(Bar), new(Bar)}
			result := miruken.Build(handler, miruken.With(bars1...)).Handle(bam, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
			bars2  := []*Bar{new(Bar), new(Bar)}
			result  = miruken.Build(handler, miruken.With(bars2)).Handle(bam, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, bam.Count())
			for _, bar := range bars2 {
				suite.Equal(1, bar.Count())
			}
		})

		suite.Run("CustomResolver", func() {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(&DependencyResolverHandler{}),
				miruken.WithHandlers(new(DependencyResolverHandler)))
			if config, err := miruken.Invoke[*Config](handler, new(Foo)); err == nil {
				suite.NotNil(*config)
				suite.Equal("https://server/api", config.baseUrl)
				suite.Equal(30000, config.timeout)
			} else {
				suite.Fail("unexpected error", err.Error())
			}
		})
	})

	suite.Run("CallSemantics", func () {
		suite.Run("BestEffort", func () {
			handler := miruken.Build(
				suite.SetupWith(miruken.WithHandlers(new(BarHandler))),
				miruken.WithBestEffort)
			foo     := new(Foo)
			result  := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(0, foo.Count())
		})

		suite.Run("Broadcast", func () {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(
					&FooHandler{},
					&BarHandler{}),
				miruken.WithHandlers(new(FooHandler), new(FooHandler), new(BarHandler)))
			foo     := new(Foo)
			result  := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, foo.Count())

			result = miruken.Build(handler, miruken.WithBroadcast).Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(3, foo.Count())
		})
	})

	suite.Run("Invoke", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&CounterHandler{}),
			miruken.WithHandlers(new(CounterHandler)))
		suite.Run("Invoke", func () {
			suite.Run("Invariant", func() {
				if foo, err := miruken.Invoke[*Foo](handler, new(Foo)); err == nil {
					suite.NotNil(foo)
					suite.Equal(1, foo.Count())
				} else {
					suite.Fail("unexpected error", err.Error())
				}
			})

			suite.Run("Contravariant", func() {
				if foo, err := miruken.Invoke[any](handler, new(Foo)); err == nil {
					suite.NotNil(foo)
					suite.IsType(&Foo{}, foo)
					suite.Equal(1, foo.(*Foo).Count())
				} else {
					suite.Fail("unexpected error", err.Error())
				}
			})

			suite.Run("BestEffort", func () {
				handler := miruken.Build(
					suite.SetupWith(miruken.WithHandlers(new(BarHandler))),
						miruken.WithBestEffort)
				if foo, err := miruken.Invoke[*Foo](handler, new(Foo)); err == nil {
					suite.Nil(foo)
				} else {
					suite.Fail("unexpected error", err.Error())
				}
			})

			suite.Run("Mixed", func() {
				handler := suite.SetupWith(miruken.WithHandlerSpecs(&MixedHandler{}))
				if ret, err := miruken.Invoke[any](handler, new(Foo)); err == nil {
					suite.Equal("Handles *test.Foo", ret)
				} else {
					suite.Fail("unexpected error", err.Error())
				}
				if ret, err := miruken.Map[any](handler, new(Foo)); err == nil {
					suite.Equal("Maps *test.Foo", ret)
				} else {
					suite.Fail("unexpected error", err.Error())
				}
			})
		})

		suite.Run("InvokeAll", func () {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(
					&CountByTwoHandler{},
					&SpecificationHandler{}),
				miruken.WithHandlers(&CountByTwoHandler{}, &SpecificationHandler{}))

			suite.Run("Invariant", func () {
				if foo, err := miruken.InvokeAll[*Foo](handler, &Foo{Counted{1}}); err == nil {
					suite.NotNil(foo)
					// 1 from explicit return of *CountByTwoHandler
					// 2 from inference of *CountByTwoHandler (1) which includes explicit instance (1)
					suite.Len(foo, 3)
					// 3 from explicit *CountByTwoHandler (2) and *SpecificationHandler (1)
					// 4 for inference of *CountByTwoHandler (2) which includes explicit instance (2)
					// 2 for inference of *SpecificationHandler (1) which includes explicit instance (1)
					// 9 + 1 = 10 total
					suite.Equal(10, foo[0].Count())
				} else {
					suite.Fail("unexpected error", err.Error())
				}
			})

			suite.Run("Invariant Error", func () {
				handler := suite.SetupWith(
					miruken.WithHandlerSpecs(&CounterHandler{}))
				foo := new(Foo)
				foo.Inc()
				foo.Inc()
				if _, err := miruken.InvokeAll[*Foo](handler, foo); err != nil {
					suite.NotNil(err)
					// *CounterHandler returns error based on rule
					suite.Equal("3 is divisible by 3", err.Error())
				} else {
					suite.Fail("expected error")
				}
			})
		})
	})

	suite.Run("Invalid", func () {
		failures := 0
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(*miruken.HandlerDescriptorError); ok {
					var errMethod miruken.MethodBindingError
					for reason := errors.Unwrap(err.Reason);
						errors.As(reason, &errMethod); reason = errors.Unwrap(reason) {
						failures++
					}
					suite.Equal(8, failures)
				} else {
					suite.Fail("Expected HandlerDescriptorError")
				}
			}
		}()
		suite.SetupWith(
			miruken.WithHandlerSpecs(&InvalidHandler{}),
			miruken.WithHandlers(new(InvalidHandler)))
		suite.Fail("should cause panic")
	})

	suite.Run("Function Binding", func () {
		suite.Run("Invariant", func() {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(HandleFoo))
			foo := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Contravariant", func() {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(HandleCounted))
			bar := new(Bar)
			bar.Inc()
			result := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(3, bar.Count())
		})

		suite.Run("Invariant Explicit", func() {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(HandleFoo),
				miruken.WithHandlers(HandleFoo))
			foo := new(Foo)
			result := handler.Handle(foo, true, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			// 1 for explicit instance
			// 1 for inferred call
			suite.Equal(2, foo.Count())
		})

		suite.Run("Contravariant Explicit", func() {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(HandleCounted),
				miruken.WithHandlers(HandleCounted))
			bar := new(Bar)
			bar.Inc()
			result := handler.Handle(bar, true, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			// Started as 1
			// 2 for explicit instance
			// 2 for inferred instance
			suite.Equal(5, bar.Count())
		})
	})
}

// FooProvider
type FooProvider struct {
	foo Foo
}

func (f *FooProvider) ProvideFoo(*miruken.Provides) *Foo {
	f.foo.Inc()
	return &f.foo
}

// ListProvider
type ListProvider struct{}

func (f *ListProvider) ProvideFooSlice(*miruken.Provides) []*Foo {
	return []*Foo{{Counted{1}}, {Counted{2}}}
}

func (f *ListProvider) ProvideFooArray(*miruken.Provides) [2]*Bar {
	return [2]*Bar{{Counted{3}}, {Counted{4}}}
}

// MultiProvider
type MultiProvider struct {
	foo Foo
	bar Bar
}

func (p *MultiProvider) Constructor(*miruken.Creates) {
	p.foo.Inc()
}

func (p *MultiProvider) ProvideFoo(*miruken.Provides) *Foo {
	p.foo.Inc()
	return &p.foo
}

func (p *MultiProvider) ProvideBar(*miruken.Provides) (*Bar, miruken.HandleResult) {
	count := p.bar.Inc()
	if count % 3 == 0 {
		return &p.bar, miruken.NotHandled.WithError(
			fmt.Errorf("%v is divisible by 3", p.bar.Count()))
	}
	if count % 2 == 0 {
		return &p.bar, miruken.NotHandled
	}
	return &p.bar, miruken.Handled
}

// SpecificationProvider
type SpecificationProvider struct {
	foo Foo
	bar Bar
}

func (p *SpecificationProvider) Constructor(baz Baz) {
	p.foo.count = baz.Count()
}

func (p *SpecificationProvider) ProvideFoo(
	_*struct{
		miruken.Provides
		miruken.Creates
	  },
) *Foo {
	p.foo.Inc()
	return &p.foo
}

func (p *SpecificationProvider) ProvideBar(
	_*struct{ miruken.Provides; miruken.Strict },
) []*Bar {
	p.bar.Inc()
	return []*Bar{&p.bar, {}}
}

type OpenProvider struct{}

func (p *OpenProvider) Provide(
	provides *miruken.Provides,
) any {
	if key := provides.Key(); key == miruken.TypeOf[*Foo]() {
		return &Foo{}
	} else if key == miruken.TypeOf[*Bar]() {
		return &Bar{}
	}
	return nil
}

// InvalidProvider
type InvalidProvider struct {}

func (p *InvalidProvider) MissingReturnValue(*miruken.Provides) {
}

func (p *InvalidProvider) TooManyReturnValues(
	*miruken.Provides,
) (*Foo, string, Counter) {
	return nil, "bad", nil
}

func (p *InvalidProvider) InvalidHandleResultReturnValue(
	*miruken.Provides,
) miruken.HandleResult {
	return miruken.Handled
}

func (p *InvalidProvider) InvalidErrorReturnValue(
	*miruken.Provides,
) error {
	return errors.New("not good")
}

func (p *InvalidProvider) SecondReturnMustBeErrorOrHandleResult(
	*miruken.Provides,
) (*Foo, string) {
	return &Foo{}, "bad"
}

func (p *InvalidProvider) UntypedInterfaceDependency(
	_*miruken.Provides,
	any any,
) *Foo {
	return &Foo{}
}

func ProvideBar(*miruken.Provides) (*Bar, miruken.HandleResult) {
	bar := &Bar{}
	bar.Inc()
	bar.Inc()
	return bar, miruken.Handled
}

func (suite *HandlerTestSuite) TestProvides() {
	suite.Run("Implied", func () {
		handler := suite.SetupWith(miruken.WithHandlers(new(FooProvider)))
		fooProvider, err := miruken.Resolve[*FooProvider](handler)
		suite.Nil(err)
		suite.NotNil(fooProvider)
	})

	suite.Run("Invariant", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&FooProvider{}),
			miruken.WithHandlers(new(FooProvider)))
		foo, err := miruken.Resolve[*Foo](handler)
		suite.Nil(err)
		suite.Equal(1, foo.Count())
	})

	suite.Run("Covariant", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&FooProvider{}),
			miruken.WithHandlers(new(FooProvider)))
		counter, err := miruken.Resolve[Counter](handler)
		suite.Nil(err)
		suite.Equal(1, counter.Count())
		if foo, ok := counter.(*Foo); !ok {
			suite.Fail(fmt.Sprintf("expected *Foo, but found %T", foo))
		}
	})

	suite.Run("NotHandledReturnNil", func () {
		handler := suite.SetupWith()
		foo, err := miruken.Resolve[*Foo](handler)
		suite.Nil(err)
		suite.Nil(foo)
	})

	suite.Run("Open", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&OpenProvider{}),
			miruken.WithHandlers(new(OpenProvider)))
		foo, err := miruken.Resolve[*Foo](handler)
		suite.Nil(err)
		suite.Equal(0, foo.Count())
		bar, err := miruken.Resolve[*Bar](handler)
		suite.Nil(err)
		suite.Equal(0, bar.Count())
	})

	suite.Run("Multiple", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&MultiProvider{}),
			miruken.WithHandlers(new(MultiProvider)))
		foo, err := miruken.Resolve[*Foo](handler)
		suite.Nil(err)
		suite.Equal(1, foo.Count())

		bar, err := miruken.Resolve[*Bar](handler)
		suite.Nil(err)
		suite.Equal(1, bar.Count())

		bar, err = miruken.Resolve[*Bar](handler)
		suite.NotNil(err)
		suite.Nil(bar)
	})

	suite.Run("Specification", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&SpecificationProvider{}),
			miruken.WithHandlers(new(SpecificationProvider)))

		suite.Run("Invariant", func () {
			foo, err := miruken.Resolve[*Foo](handler)
			suite.Nil(err)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Strict", func () {
			bar, err := miruken.Resolve[*Bar](handler)
			suite.Nil(err)
			suite.Nil(bar)

			bars, err := miruken.Resolve[[]*Bar](handler)
			suite.Nil(err)
			suite.NotNil(bars)
			suite.Equal(2, len(bars))
		})
	})

	suite.Run("Lists", func () {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&ListProvider{}),
			miruken.WithHandlers(new(ListProvider)))

		suite.Run("Slice", func () {
			foo, err := miruken.Resolve[*Foo](handler)
			suite.Nil(err)
			suite.NotNil(foo)
		})

		suite.Run("Array", func () {
			bar, err := miruken.Resolve[*Bar](handler)
			suite.Nil(err)
			suite.NotNil(bar)
		})
	})

	suite.Run("Constructor", func () {
		handler := suite.Setup()

		suite.Run("NoInit", func () {
			fooProvider, err := miruken.Resolve[*FooProvider](handler)
			suite.NotNil(fooProvider)
			suite.Nil(err)
		})

		suite.Run("Constructor", func () {
			multiProvider, err := miruken.Resolve[*MultiProvider](handler)
			suite.NotNil(multiProvider)
			suite.Equal(1, multiProvider.foo.Count())
			suite.Nil(err)
		})

		suite.Run("ConstructorDependencies", func () {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(&SpecificationProvider{}))
			specProvider, err := miruken.Resolve[*SpecificationProvider](
				miruken.Build(handler, miruken.With(Baz{Counted{2}})))
			suite.NotNil(specProvider)
			suite.Equal(2, specProvider.foo.Count())
			suite.Equal(0, specProvider.bar.Count())
			suite.Nil(err)
		})

		suite.Run("NoConstructor", func () {
			unmanaged, err := miruken.Resolve[*UnmanagedHandler](handler)
			suite.Nil(err)
			suite.Nil(unmanaged)
		})
	})

	suite.Run("Infer", func () {
		handler := suite.Setup()

		suite.Run("Invariant", func() {
			foo := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(1, foo.Count())
		})

		suite.Run("Open", func () {
			handler := suite.Setup()
			foo, err := miruken.ResolveAll[*Foo](handler)
			suite.Nil(err)
			// 1 from FooProvider.ProvideFoo
			// 2 from ListProvider.ProvideFooSlice
			// 1 from MultiProvider.ProvideFoo
			// 1 from OpenProvider.Provides
			// None from SpecificationProvider.ProvideFoo since it
			//   depends on an unsatisfied Baz
			// 5 total
			suite.Equal(5, len(foo))
		})

		suite.Run("Disable", func() {
			handler := suite.SetupWith(
				miruken.WithHandlers(new(FooProvider)),
				miruken.WithoutInference)
			foo := new(Foo)
			result := handler.Handle(foo, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
		})
	})

	suite.Run("ResolveAll", func () {
		suite.Run("Invariant", func () {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(
					&FooProvider{},
					&MultiProvider{},
					&SpecificationProvider{}),
				miruken.WithHandlers(
					new(FooProvider), new(MultiProvider), new (SpecificationProvider)))

			if foo, err := miruken.ResolveAll[*Foo](handler); err == nil {
				suite.NotNil(foo)
				// 3 from each of the 3 explicit instances (1)
				// 2 for inference of *FooProvider (1) which includes explicit instance (1)
				// 2 for inference of *MultiProvider (1) which includes explicit instance (1)
				// 1 for inference of *SpecificationProvider (1) which excludes constructed
				//   instance since it has an unsatisfied dependency on Baz
				// 8 total
				suite.Len(foo, 8)
				suite.True(foo[0] != foo[1])
			} else {
				suite.Fail("unexpected error", err.Error())
			}
		})

		suite.Run("Covariant", func () {
			handler := suite.SetupWith(
				miruken.WithHandlerSpecs(&ListProvider{}),
				miruken.WithHandlers(new(ListProvider)))
			if counted, err := miruken.ResolveAll[Counter](handler); err == nil {
				suite.NotNil(counted)
				// 4 from 2 methods on explicit *ListProvider
				// 8 for inference of *ListProvider (4) which includes explicit instance (4)
				// 12 total
				suite.Len(counted, 12)
			} else {
				suite.Fail("unexpected error", err.Error())
			}
		})

		suite.Run("Empty", func () {
			handler := suite.SetupWith(miruken.WithHandlers(new(FooProvider)))
			bars, err := miruken.ResolveAll[*Bar](handler)
			suite.Nil(err)
			suite.NotNil(bars)
		})
	})

	suite.Run("With", func () {
		handler := miruken.Setup()
		fooProvider, err := miruken.Resolve[*FooProvider](handler)
		suite.Nil(err)
		suite.Nil(fooProvider)
		fooProvider, err = miruken.Resolve[*FooProvider](
			miruken.Build(handler, miruken.With(new(FooProvider))))
		suite.Nil(err)
		suite.NotNil(fooProvider)
	})

	suite.Run("Invalid", func () {
		failures := 0
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(*miruken.HandlerDescriptorError); ok {
					var errMethod miruken.MethodBindingError
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
		suite.SetupWith(
			miruken.WithHandlerSpecs(&InvalidProvider{}),
			miruken.WithHandlers(new(InvalidProvider)))
		suite.Fail("should cause panic")
	})

	suite.Run("Function Binding", func () {
		suite.Run("Implied", func() {
			handler := suite.SetupWith(miruken.WithHandlerSpecs(ProvideBar))
			bar, err := miruken.Resolve[*Bar](handler)
			suite.Nil(err)
			suite.NotNil(bar)
			suite.Equal(2, bar.Count())
		})
	})
}

func (suite *HandlerTestSuite) TestCreates() {
	suite.Run("Invariant", func() {
		handler := suite.SetupWith(
			miruken.WithHandlerSpecs(&SpecificationProvider{}),
			miruken.WithHandlers(&SpecificationProvider{foo: Foo{Counted{10}}}))
		var foo *Foo
		err := miruken.Create(handler, &foo)
		suite.Nil(err)
		suite.Equal(11, foo.Count())
	})

	suite.Run("Infer", func() {
		handler := suite.Setup()
		var multiProvider *MultiProvider
		err := miruken.Create(handler, &multiProvider)
		suite.NotNil(multiProvider)
		suite.Nil(err)
	})
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}