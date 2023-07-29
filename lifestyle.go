package miruken

import (
	"fmt"
	"github.com/miruken-go/miruken/promise"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
)

type (
	// LifestyleProvider is a FilterProvider of lifestyles.
	LifestyleProvider struct {
		filters []Filter
	}

	// Lifestyle provides common lifestyle functionality.
	Lifestyle struct{}
)


func (l *Lifestyle) Order() int {
	return math.MaxInt32 - 1000
}


func (l *LifestyleProvider) Required() bool {
	return true
}

func (l *LifestyleProvider) AppliesTo(
	callback Callback,
) bool {
	_, ok := callback.(*Provides)
	return ok
}

func (l *LifestyleProvider) Filters(
	binding Binding,
	callback any,
	composer Handler,
) ([]Filter, error) {
	return l.filters, nil
}

func (l *LifestyleProvider) SetFilters(filters ...Filter) {
	if len(filters) == 0 {
		panic("filters cannot be empty")
	}
	if l.filters != nil {
		panic("lifestyle can only be set once")
	}
	l.filters = filters
}


// Single

type (
	// Single LifestyleProvider providing same instance.
	Single struct {
		LifestyleProvider
	}

	// single is a Filter that caches an instance.
	// Since a Single Handler can provide results polymorphically,
	// a map of key to instance is maintained.
	single struct {
		Lifestyle
		keys atomic.Pointer[singleCache]
		lock sync.Mutex
	}

	// singleEntry stores a lazy instance.
	singleEntry struct {
		instance []any
		once     *sync.Once
	}

	// singleCache maintains a cache of singleEntry's.
	singleCache map[any]*singleEntry
)


func (s *Single) Init() error {
	s.SetFilters(&single{})
	return nil
}

func (s *single) Next(
	self     Filter,
	next     Next,
	ctx      HandleContext,
	provider FilterProvider,
)  (out []any, po *promise.Promise[[]any], err error) {
	key := ctx.Callback().(*Provides).Key()

	var entry *singleEntry
	if keys := s.keys.Load(); keys != nil {
		if e, ok := (*keys)[key]; ok {
			entry = e
		}
	}

	// Use copy-on-write idiom since reads should be more frequent than writes.
	if entry == nil {
		s.lock.Lock()
		if keys := s.keys.Load(); keys != nil {
			if entry = (*keys)[key]; entry == nil {
				kc := make(singleCache)
				typ, assignable := key.(reflect.Type)
				// If the key is not found, check if any existing instances
				// can satisfy the key before a new instance is provided.
				for k, v := range *keys {
					kc[k] = v
					if assignable {
						if instance := v.instance; len(instance) > 0 {
							if o := instance[0]; o != nil {
								if ot := reflect.TypeOf(o); ot.AssignableTo(typ) {
									entry   = v
									kc[key] = v
									break
								}
							}
						}
					}
				}
				if entry == nil {
					entry = &singleEntry{once: new(sync.Once)}
					kc[key] = entry
				}
				s.keys.Store(&kc)
			}
		} else {
			entry = &singleEntry{once: new(sync.Once)}
			s.keys.Store(&singleCache{key: entry})
		}
		s.lock.Unlock()
	}

	entry.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("single: panic: %v", r)
				}
				entry.once = new(sync.Once)
			}
		}()
		if out, po, err = next.Pipe(); err == nil && po != nil {
			out, err = po.Await()
		}
		if err != nil || len(out) == 0 {
			entry.once = new(sync.Once)
		} else {
			entry.instance = out
		}
	})

	return entry.instance, nil, err
}
