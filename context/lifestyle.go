package context

import (
	"errors"
	"fmt"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/promise"
	"github.com/miruken-go/miruken/provides"
	"maps"
	"reflect"
	"sync"
	"sync/atomic"
)

type (
	// Lifestyle is a BindingGroup for requesting a scoped lifestyle.
	// Lifestyle associates both the scoped lifestyle and fromScope constraint.
	Lifestyle struct {
		miruken.BindingGroup
		scopedProvider
		miruken.Qualifier[scopedProvider]
	}

	// Rooted is a BindingGroup for requesting a rooted scoped lifestyle
	// with all resolutions assigned to the root Context.
	Rooted struct {
		miruken.BindingGroup
		scopedProvider `scoped:"rooted"`
		miruken.Qualifier[scopedProvider]
	}

	// scopedProvider LifestyleProvider provides instances per Context.
	scopedProvider struct {
		miruken.LifestyleProvider
		rooted bool
	}

	// scopedEntry stores a lazy instance.
	scopedEntry struct {
		instance []any
		once     *sync.Once
	}

	// scopedCache maintains a cache of scopedEntry's.
	scopedCache map[any]*scopedEntry

	// scoped is a Filter that caches a known instance per Context.
	scoped struct {
		miruken.Lifestyle
		cache  atomic.Pointer[map[*Context]*scopedEntry]
		lock   sync.Mutex
	}

	// scopedUnk is a miruken.Filter that caches unknown instances
	// per Context.  When a Handler provides any results, a map of
	// key to instance is maintained using copy-on-write idiom.
	scopedUnk struct {
		miruken.Lifestyle
		cache  map[*Context]scopedCache
		lock   sync.RWMutex
	}
)


// scopedProvider

func (s *scopedProvider) InitWithTag(tag reflect.StructTag) error {
	if scoped, ok := tag.Lookup("scoped"); ok {
		s.rooted = scoped == "rooted"
	}
	return nil
}

func (s *scopedProvider)InitLifestyle(binding miruken.Binding) error {
	if !s.FiltersAssigned() {
		if typ, ok := binding.Key().(reflect.Type); ok && internal.IsAny(typ) {
			s.SetFilters(&scopedUnk{})
		} else {
			s.SetFilters(&scoped{})
		}
	}
	return nil
}


// scoped

func (s *scoped) Next(
	self     miruken.Filter,
	next     miruken.Next,
	ctx      miruken.HandleContext,
	provider miruken.FilterProvider,
)  (out []any, po *promise.Promise[[]any], err error) {
	key := ctx.Callback.(*provides.It).Key()
	context, abort, err := getContext(key, ctx, provider)
	if err != nil {
		return nil, nil, err
	} else if abort {
		return next.Abort()
	} else if context == nil {
		return nil, nil, nil
	}

	var entry *scopedEntry
	if cache := s.cache.Load(); cache != nil {
		if e, ok := (*cache)[context]; ok {
			entry = e
		}
	}

	// Use copy-on-write idiom since reads should be more frequent than writes.
	if entry == nil {
		s.lock.Lock()
		if cache := s.cache.Load(); cache != nil {
			if entry = (*cache)[context]; entry == nil {
				cc := maps.Clone(*cache)
				if entry == nil {
					entry = &scopedEntry{once: new(sync.Once)}
					cc[context] = entry
				}
				s.cache.Store(&cc)
			}
		} else {
			entry = &scopedEntry{once: new(sync.Once)}
			s.cache.Store(&map[*Context]*scopedEntry{context: entry})
		}
		s.lock.Unlock()
	}

	return entry.get(context, s, s.removeContext, next)
}

func (s *scoped) ContextChanging(
	contextual Contextual,
	oldCtx     *Context,
	newCtx     **Context,
) {
	if oldCtx == *newCtx {
		return
	}
	if *newCtx != nil {
		panic("managed instances cannot change context")
	}
	if cache := s.cache.Load(); cache != nil {
		if entry := (*cache)[oldCtx]; entry != nil {
			s.lock.Lock()
			defer s.lock.Unlock()
			cc := make(map[*Context]*scopedEntry, len(*cache)+1)
			for k, v := range *cache {
				if k != oldCtx {
					cc[k] = v
				}
			}
			tryDispose(contextual)
			s.cache.Store(&cc)
		}
	}
}

func (s *scoped) removeContext(context *Context) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if cache := s.cache.Load(); cache != nil {
		cc := make(map[*Context]*scopedEntry, len(*cache)+1)
		for k, v := range *cache {
			if k != context {
				cc[k] = v
			}
		}
		s.cache.Store(&cc)
	}
}


// scopedUnk

func (s *scopedUnk) Next(
	self     miruken.Filter,
	next     miruken.Next,
	ctx      miruken.HandleContext,
	provider miruken.FilterProvider,
)  (out []any, po *promise.Promise[[]any], err error) {
	key := ctx.Callback.(*provides.It).Key()
	context, abort, err := getContext(key, ctx, provider)
	if err != nil {
		return nil, nil, err
	} else if abort {
		return next.Abort()
	} else if context == nil {
		return nil, nil, nil
	}

	var entry *scopedEntry
	s.lock.RLock()
	if cache := s.cache; cache != nil {
		if keys := cache[context]; keys != nil {
			entry = keys[key]
		}
	}
	s.lock.RUnlock()

	if entry == nil {
		s.lock.Lock()
		if cache := s.cache; cache != nil {
			if keys := cache[context]; keys != nil {
				if entry = keys[key]; entry == nil {
					// If the key is not found, check if any existing instances
					// can satisfy the key before a new instance is provided.
					if typ, ok := key.(reflect.Type); ok {
						for _,v := range keys {
							if instance := v.instance; len(instance) > 0 {
								if o := instance[0]; o != nil && reflect.TypeOf(o).AssignableTo(typ) {
									entry = v
									keys[key] = v
									break
								}
							}
						}
					}
					if entry == nil {
						entry = &scopedEntry{once: new(sync.Once)}
						keys[key] = entry
					}
				}
			} else {
				entry = &scopedEntry{once: new(sync.Once)}
				cache[context] = scopedCache{key: entry}
			}
		} else {
			entry = &scopedEntry{once: new(sync.Once)}
			s.cache = map[*Context]scopedCache{context: {key: entry}}
		}
		s.lock.Unlock()
	}

	return entry.get(context, s, s.removeContext, next)
}

func (s *scopedUnk) ContextChanging(
	contextual Contextual,
	oldCtx     *Context,
	newCtx     **Context,
) {
	if oldCtx == *newCtx {
		return
	}
	if *newCtx != nil {
		panic("managed instances cannot change context")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if cache := s.cache; cache == nil {
		return
	} else if keys := cache[oldCtx]; keys == nil {
		return
	} else {
		for key, entry := range keys {
			if entry.instance[0] == contextual {
				delete(keys, key)
				tryDispose(contextual)
				break
			}
		}
	}
}

func (s *scopedUnk) removeContext(context *Context) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.cache, context)
}


// scopedEntry

func (s *scopedEntry) get(
	context       *Context,
	observer      Observer,
	removeContext func(*Context),
	next          miruken.Next,
) (out []any, po *promise.Promise[[]any], err error) {
	s.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("scoped: panic: %v", r)
				}
				s.once = new(sync.Once)
			}
		}()
		if out, po, err = next.Pipe(); err == nil && po != nil {
			out, err = po.Await()
		}
		if err != nil || len(out) == 0 {
			s.once = new(sync.Once)
		} else {
			s.instance = out
			if contextual, ok := out[0].(Contextual); ok {
				contextual.SetContext(context)
				unsubscribe := contextual.Observe(observer)
				context.Observe(EndedObserverFunc(func(*Context, any) {
					removeContext(context)
					unsubscribe.Dispose()
					tryDispose(out[0])
					contextual.SetContext(nil)
				}))
			} else {
				context.Observe(EndedObserverFunc(func(*Context, any) {
					removeContext(context)
					tryDispose(out[0])
				}))
			}
		}
	})
	return s.instance, nil, nil
}


func getContext(
	key      any,
	ctx      miruken.HandleContext,
	provider miruken.FilterProvider,
) (*Context, bool, error) {
	if key == contextType {
		// can't resolve a context contextually
		return nil, false, nil
	}

	rooted := false
	if scp, ok := provider.(*scopedProvider); ok {
		rooted = scp.rooted
	}

	if !isCompatibleWithParent(ctx, rooted) {
		return nil, false, nil
	}
	context, _, err := provides.Type[*Context](ctx.Composer)
	if err != nil {
		return nil, false, err
	} else if context == nil {
		return nil, true, nil
	} else if context.State() != StateActive {
		return nil, false, errors.New("scoped: cannot scope instances to an inactive context")
	} else if rooted {
		context = context.Root()
	}

	return context, false, nil
}

func isCompatibleWithParent(
	ctx    miruken.HandleContext,
	rooted bool,
) bool {
	if parent := ctx.Callback.(*provides.It).Parent(); parent != nil {
		if pb := parent.Binding(); pb != nil {
			for _, filter := range pb.Filters() {
				if scoped, ok := filter.(*scopedProvider); !ok || (!rooted && scoped.rooted) {
					return false
				}
			}
		}
	}
	return true
}

func tryDispose(instance any) {
	if disposable, ok := instance.(miruken.Disposable); ok {
		disposable.Dispose()
	}
}


var (
	contextType = internal.TypeOf[*Context]()

	// From constrains resolution to a handler with scoped lifestyle.
	// This is used to suppress resolving implied values available through a Context.
	From miruken.Qualifier[scopedProvider]
)
