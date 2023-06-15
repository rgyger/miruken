package authorizes

import (
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/handles"
	"github.com/miruken-go/miruken/promise"
	"github.com/miruken-go/miruken/security"
	"reflect"
)

type (
	// Access is a FilterProvider for authorization.
	Access struct {
		policy any
	}

	// filter controls access to actions using policies
	// satisfied by the privileges of a security.Subject.
	filter struct {}

	accessDenied struct{}
)

var ErrAccessDenied = accessDenied{}


// Action

func (a *Access) InitWithTag(tag reflect.StructTag) error {
	if policy, ok := tag.Lookup("policy"); ok {
		a.policy = policy
	}
	return nil
}

func (a *Access) Policy() any {
	return a.policy
}

func (a *Access) Required() bool {
	return true
}

func (a *Access) AppliesTo(
	callback miruken.Callback,
) bool {
	_, ok := callback.(*handles.It)
	return ok
}

func (a *Access) Filters(
	binding  miruken.Binding,
	callback any,
	composer miruken.Handler,
) ([]miruken.Filter, error) {
	return filters, nil
}


// accessDenied

func (e accessDenied) Error() string {
	return "access has been denied"
}


// filter

func (f filter) Order() int {
	return miruken.FilterStageAuthorization
}

func (f filter) Next(
	next     miruken.Next,
	ctx      miruken.HandleContext,
	provider miruken.FilterProvider,
)  (out []any, pout *promise.Promise[[]any], err error) {
	return miruken.DynNext(f, next, ctx, provider)
}

func (f filter) DynNext(
	next     miruken.Next,
	ctx      miruken.HandleContext,
	provider miruken.FilterProvider,
	subject  security.Subject,
)  (out []any, pout *promise.Promise[[]any], err error) {
	if ap, ok := provider.(*Access); ok {
		callback := ctx.Callback()
		composer := ctx.Composer()
		// perform authorization check
		g, pg, err := Action(composer, callback.Source(), ap.policy)
		if err != nil {
			// error performing authorization
			return nil, nil, err
		}
		if pg == nil {
			// if denied return ErrAccessDenied.
			if !g {
				return nil, nil, ErrAccessDenied
			}
			// perform the next step in the pipeline
			return next.Pipe()
		}
		// asynchronous authorization check
		return nil, promise.Then(pg, func(g bool) []any {
			// if denied return ErrAccessDenied.
			if !g {
				panic(ErrAccessDenied)
			}
			return next.PipeAwait()
		}), nil
	}
	return next.Abort()
}

var filters = []miruken.Filter{filter{}}
