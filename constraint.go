package miruken

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/miruken-go/miruken/promise"
	"reflect"
	"strings"
)

type (
	// Constraint manages BindingMetadata assertions.
	Constraint interface {
		Required() bool
		Satisfies(required Constraint, callback Callback) bool
	}

	// ConstraintSource returns one or more Constraint.
	ConstraintSource interface {
		Constraints() []Constraint
	}
)


// Named matches against a name.
type Named string

func (n *Named) Name() string {
	return string(*n)
}

func (n *Named) Required() bool {
	return false
}

func (n *Named) InitWithTag(tag reflect.StructTag) error {
	if name, ok := tag.Lookup("name"); ok && len(strings.TrimSpace(name)) > 0 {
		*n = Named(name)
		return nil
	}
	return ErrConstraintNameMissing
}

func (n *Named) Satisfies(required Constraint, _ Callback) bool {
	rn, ok := required.(*Named)
	return ok && *n == *rn
}


type (
	// Metadata matches against kev/value pairs.
	Metadata map[any]any

	metadataOwner interface {
		metadata() *Metadata
	}
)

func (m *Metadata) Required() bool {
	return false
}

func (m *Metadata) InitWithTag(
	tag reflect.StructTag,
) (err error) {
	if *m != nil {
		panic("Metadata already initialized")
	}
	*m = make(map[any]any)
	if tag, ok := tag.Lookup("metadata"); ok {
		if tag == "" {
			return nil
		}
		for _, metadata := range strings.Split(tag, ",") {
			var meta = strings.SplitN(metadata, "=", 2)
			switch len(meta) {
			case 1:
				(*m)[meta[0]] = nil
			case 2:
				(*m)[meta[0]] = meta[1]
			default:
				err = multierror.Append(err,
					fmt.Errorf("invalid metadata [%v]", metadata))
			}
		}
	}
	return err
}

func (m *Metadata) Satisfies(required Constraint, _ Callback) bool {
	rm, ok := required.(metadataOwner)
	return ok && reflect.DeepEqual(rm.metadata(), m.metadata())
}

func (m *Metadata) metadata() *Metadata {
	return m
}


type (
	// Qualifier matches against a type.
	Qualifier[T any] struct {}

	qualifierOwner[T any] interface {
		qualifier() Qualifier[T]
	}
)

func (q Qualifier[T]) Required() bool {
	return false
}

func (q Qualifier[T]) Satisfies(required Constraint, _ Callback) bool {
	_, ok := required.(qualifierOwner[T])
	return ok
}

func (q Qualifier[T]) qualifier() Qualifier[T] {
	return q
}


// constraintFilter enforces constraints.
type constraintFilter struct{}

func (c *constraintFilter) Order() int {
	return FilterStage
}

func (c *constraintFilter) Next(
	next     Next,
	ctx      HandleContext,
	provider FilterProvider,
)  ([]any, *promise.Promise[[]any], error) {
	if cp, ok := provider.(ConstraintSource); ok {
		constraints := cp.Constraints()
		required    := ctx.Callback().Constraints()
		if len(required) == 0 {
			for _, c := range constraints {
				if c.Required() {
					return next.Abort()
				}
			}
		} else if len(constraints) == 0 {
			return next.Abort()
		} else {
			callback := ctx.Callback()
			var matched map[Constraint]struct{}
			Loop:
			for _, rc := range required {
				for _, c := range constraints {
					if c.Satisfies(rc, callback) {
						if c.Required() {
							if matched == nil {
								matched = make(map[Constraint]struct{})
							}
							matched[c] = struct{}{}
						}
						continue Loop
					}
				}
				return next.Abort()
			}
			for _, c := range constraints {
				if c.Required() {
					if _, ok := matched[c]; !ok {
						return next.Abort()
					}
				}
			}
		}
	}
	return next.Pipe()
}

var _constraintFilter = []Filter{&constraintFilter{}}

// ConstraintProvider is a FilterProvider for constraints.
type ConstraintProvider struct {
	constraints []Constraint
}

func (c *ConstraintProvider) Constraints() []Constraint {
	return c.constraints
}

func (c *ConstraintProvider) Required() bool {
	return true
}

func (c *ConstraintProvider) Filters(
	binding  Binding,
	callback any,
	composer Handler,
) ([]Filter, error) {
	return _constraintFilter, nil
}

var ErrConstraintNameMissing = errors.New("the Named constraint requires a non-empty `name:[name]` tag")