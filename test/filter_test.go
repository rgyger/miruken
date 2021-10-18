package test

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/suite"
	"math"
	"miruken.com/miruken"
	"reflect"
	"testing"
)

type Captured interface {
	Handled() int
	IncHandled(howMany int)
	Composer() miruken.Handler
	SetComposer(composer miruken.Handler)
	Filters()  []miruken.Filter
	AddFilters(filters ...miruken.Filter)
}

type Capture struct {
	handled  int
	composer miruken.Handler
	filters  []miruken.Filter
}

func (c *Capture) Handled() int {
	return c.handled
}

func (c *Capture) IncHandled(howMany int) {
	c.handled += howMany
}

func (c *Capture) Composer() miruken.Handler {
	return c.composer
}

func (c *Capture) SetComposer(composer miruken.Handler) {
	c.composer = composer
}

func (c *Capture) Filters() []miruken.Filter {
	return c.filters
}

func (c *Capture) AddFilters(filters ...miruken.Filter) {
	c.filters = append(c.filters, filters...)
}

type (
	FooC struct { Capture }
	SpecialFooC struct { FooC }
	BarC struct { Capture }
	SpecialBarC struct { BarC }
	BooC struct { Capture }
	BazC struct { Capture }
	SpecialBazC struct {BazC }
	BeeC struct { Capture }
)

type Logging interface {
	Log(msg string)
}

type ConsoleLogger struct{}
func (c *ConsoleLogger) Log(msg string) {
	fmt.Println(msg)
}

// NullFilter test filter
type NullFilter struct {}

func (n NullFilter) Order() int {
	return math.MaxInt32
}

func (n NullFilter) Next(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
)  ([]interface{}, error) {
	if captured := extractCaptured(context.Callback); captured != nil {
		captured.AddFilters(n)
	}
	return next.Filter()
}

// LogFilter test filter
type LogFilter struct {}

func (l *LogFilter) Order() int {
	return 1
}

func (l *LogFilter) Next(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
)  ([]interface{}, error) {
	return miruken.DynNext(l, next, context, provider)
}

func (l *LogFilter) DynNext(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
	logging Logging,
)  ([]interface{}, error) {
	captured := extractCaptured(context.Callback)
	logging.Log(
		fmt.Sprintf("Log callback %#v", captured))
	if captured != nil {
		captured.AddFilters(l)
	}
	return next.Filter()
}

// ExceptionFilter test filter
type ExceptionFilter struct {}

func (e *ExceptionFilter) Order() int {
	return 2
}

func (e *ExceptionFilter) Next(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
)  ([]interface{}, error) {
	captured := extractCaptured(context.Callback)
	if captured != nil {
		captured.AddFilters(e)
	}
	if result, err := next.Filter(); err != nil {
		return result, err
	} else if _, ok := captured.(*BooC); ok {
		return result, errors.New("system shutdown")
	} else {
		return result, err
	}
}

// AbortFilter test filter
type AbortFilter struct {}

func (a *AbortFilter) Order() int {
	return 0
}

func (a *AbortFilter) Next(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
)  ([]interface{}, error) {
	if captured := extractCaptured(context.Callback);
		captured == nil || captured.Handled() > 99 {
		return next.Abort()
	}
	return next.Filter()
}

func extractCaptured(callback interface{}) Captured {
	switch cb := callback.(type) {
	case Captured: return cb
	case *miruken.Command:
		if captured, ok := cb.Callback().(Captured); ok {
			return captured
		}
	}
	return nil
}

// FilteringHandler test handler
type FilteringHandler struct {}

func (f FilteringHandler) Order() int {
	return 10
}

func (f FilteringHandler) HandleBar(
	_ *struct{
		miruken.Handles
		NullFilter
		LogFilter
		ExceptionFilter `filter:"required"`
		AbortFilter
	  },
	bar *BarC,
) {
	bar.IncHandled(1)
}

func (f FilteringHandler) HandleBee(
	_ *struct{
		miruken.Handles
		miruken.SkipFilters
		LogFilter
	  },
	bee *BeeC,
) {
	bee.IncHandled(3)
}

func (f FilteringHandler) HandleStuff(
	_ miruken.Handles,
	callback interface{},
) {
	if bar, ok := callback.(*BarC); ok {
		bar.IncHandled(-999)
	}
}

func (f FilteringHandler) Next(
	next     miruken.Next,
	context  miruken.HandleContext,
	provider miruken.FilterProvider,
)  ([]interface{}, error) {
	if bar, ok := context.Callback.(*BarC); ok {
		bar.AddFilters(f)
		bar.IncHandled(1)
	}
	return next.Filter()
}

// SpecialFilteringHandler test handler
type SpecialFilteringHandler struct {}

func (s SpecialFilteringHandler) HandleFoo(
	_ *struct{
		miruken.Handles
		LogFilter
		ExceptionFilter
	  },
	foo *FooC,
) *SpecialFooC {
	return new(SpecialFooC)
}

func (s SpecialFilteringHandler) RemoveBoo(
	_ *struct{
		miruken.Handles
		ExceptionFilter
	  },
	boo *BooC,
) {
}

// SingletonHandler test handler

type SingletonHandler struct{}

func (s *SingletonHandler) Constructor(
	_ *struct{
		miruken.Provides
		miruken.Singleton
	  },
) {
}

func (s *SingletonHandler) HandleBar(
	_ *struct{
		miruken.Handles
		LogFilter
	  },
	bar *BarC,
) {
	bar.IncHandled(3)
}

// SingletonErrorHandler test handler

var errorCount = 0

type SingletonErrorHandler struct {
	count int
}

func (s *SingletonErrorHandler) Constructor(
	_ *struct{
		miruken.Provides
		miruken.Singleton
	  },
) error {
	errorCount++
	switch errorCount {
	case 1: return errors.New("something bad")
	case 2: panic("something bad")
	default:
		errorCount = 0
		return nil
	}
}

func (s *SingletonErrorHandler) HandleBee(
	_ miruken.Handles,
	bee *BeeC,
) {
	bee.IncHandled(3)
}

// BadHandler test handler

type BadHandler struct{}

func (b BadHandler) HandleBar(
	_ *struct{
		miruken.Handles
		LogFilter
      },
	bar *BarC,
) {
}

type FilterTestSuite struct {
	suite.Suite
	HandleTypes []reflect.Type
}

func (suite *FilterTestSuite) SetupTest() {
	handleTypes := []reflect.Type{
		reflect.TypeOf((*FilteringHandler)(nil)),
		reflect.TypeOf((*SpecialFilteringHandler)(nil)),
		reflect.TypeOf((*SingletonHandler)(nil)),
		reflect.TypeOf((*LogFilter)(nil)),
		reflect.TypeOf((*ConsoleLogger)(nil)),
		reflect.TypeOf((*ExceptionFilter)(nil)),
		reflect.TypeOf((*AbortFilter)(nil)),
		reflect.TypeOf((*NullFilter)(nil)).Elem(),
	}
	suite.HandleTypes = handleTypes
}

func (suite *FilterTestSuite) InferenceRoot() miruken.Handler {
	return miruken.NewRootHandler(miruken.WithHandlerTypes(suite.HandleTypes...))
}

func (suite *FilterTestSuite) InferenceRootWith(
	handlerTypes ... reflect.Type) miruken.Handler {
	return miruken.NewRootHandler(miruken.WithHandlerTypes(handlerTypes...))
}

func (suite *FilterTestSuite) TestFilters() {
	suite.Run("FilterOptions", func () {
		suite.Run("Merges", func () {
			filters  := []miruken.Filter{NullFilter{}}
			provider := miruken.NewFilterInstanceProvider(false, filters...)
			options  := miruken.FilterOptions{
				Providers:   []miruken.FilterProvider{provider},
				SkipFilters: miruken.OptionTrue,
			}
			other    := miruken.FilterOptions{}
			other2   := miruken.FilterOptions{
				Providers:   []miruken.FilterProvider{provider},
				SkipFilters: miruken.OptionFalse,
			}
			miruken.MergeOptions(options, &other)
			suite.True(other.SkipFilters.Bool())
			suite.ElementsMatch([]miruken.FilterProvider{provider}, options.Providers)
			miruken.MergeOptions(options, &other2)
			suite.False(other2.SkipFilters.Bool())
			suite.ElementsMatch([]miruken.FilterProvider{provider, provider}, other2.Providers)
		})
	})

	suite.Run("Create Pipeline", func () {
		handler := suite.InferenceRoot()
		bar     := new(BarC)
		result  := handler.Handle(bar, false, nil)
		suite.False(result.IsError())
		suite.Equal(miruken.Handled, result)
		suite.Equal(2, bar.Handled())
		suite.Equal(4, len(bar.Filters()))
		suite.IsType(&LogFilter{}, bar.Filters()[0])
		suite.IsType(&ExceptionFilter{}, bar.Filters()[1])
		suite.IsType(FilteringHandler{}, bar.Filters()[2])
		suite.IsType(NullFilter{}, bar.Filters()[3])
	})

	suite.Run("Abort Pipeline", func () {
		handler := suite.InferenceRoot()
		bar := new(BarC)
		bar.IncHandled(100)
		result := handler.Handle(bar, false, nil)
		suite.False(result.IsError())
		suite.Equal(miruken.Handled, result)
		suite.Equal(-898, bar.Handled())
	})

	suite.Run("Skip Pipeline", func () {
		suite.Run("Implicit", func() {
			handler := suite.InferenceRoot()
			bee := new(BeeC)
			result := handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(3, bee.Handled())
			suite.Equal(0, len(bee.Filters()))
		})

		suite.Run("Explicit", func() {
			handler := miruken.Build(suite.InferenceRoot(), miruken.DisableFilters)
			bar     := new(BarC)
			result  := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(2, bar.Handled())
			suite.Equal(2, len(bar.Filters()))
			suite.IsType(&ExceptionFilter{}, bar.Filters()[0])
			suite.IsType(FilteringHandler{}, bar.Filters()[1])
		})
	})

	suite.Run("Singleton", func () {
		suite.Run("Implicit", func() {
			handler := suite.InferenceRoot()
			var singletonHandler *SingletonHandler
			err := miruken.Resolve(handler, &singletonHandler)
			suite.Nil(err)
			suite.NotNil(singletonHandler)
			var singletonHandler2 *SingletonHandler
			err = miruken.Resolve(handler, &singletonHandler2)
			suite.Nil(err)
			suite.Same(singletonHandler, singletonHandler2)
		})

		suite.Run("Infer", func() {
			handler := suite.InferenceRootWith(
				reflect.TypeOf((*SingletonHandler)(nil)),
				reflect.TypeOf((*ConsoleLogger)(nil)),
				reflect.TypeOf((*LogFilter)(nil)),
			)
			bar := new(BarC)
			bar.IncHandled(10)
			result := handler.Handle(bar, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
			suite.Equal(13, bar.Handled())
		})

		suite.Run("Error", func() {
			handler := suite.InferenceRootWith(
				reflect.TypeOf((*SingletonErrorHandler)(nil)),
			)
			bee := new(BeeC)
			result := handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
			result = handler.Handle(bee, false, nil)
			result = handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
		})

		suite.Run("Panic", func() {
			handler := suite.InferenceRootWith(
				reflect.TypeOf((*SingletonErrorHandler)(nil)),
			)
			bee := new(BeeC)
			result := handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
			result = handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.NotHandled, result)
			result = handler.Handle(bee, false, nil)
			suite.False(result.IsError())
			suite.Equal(miruken.Handled, result)
		})
	})

	suite.Run("Missing Dependencies", func () {
		handler := suite.InferenceRootWith(
			reflect.TypeOf((*BadHandler)(nil)),
			reflect.TypeOf((*LogFilter)(nil)),
		)
		bar   := new(BarC)
		result := handler.Handle(bar, false, nil)
		suite.False(result.IsError())
		suite.Equal(miruken.NotHandled, result)
	})
}

func TestFilterTestSuite(t *testing.T) {
	suite.Run(t, new(FilterTestSuite))
}
