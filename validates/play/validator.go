package playvalidator

import (
	"errors"
	"fmt"
	ut "github.com/go-playground/universal-translator"
	play "github.com/go-playground/validator/v10"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/args"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/validates"
	"reflect"
	"strings"
)

type (
	// Validator provides core validation behavior.
	Validator struct {
		validate   *play.Validate
		translator ut.Translator
	}

	// ValidatorT handles validation for a specific type.
	ValidatorT[T any] struct {
		Validator
	}

	// TypeRules express the validation constraints for a type
	// without depending on validation struct tags.
	TypeRules struct{
		Type        any
		Constraints map[string]string
	}

	// Rules express the validation constraints for a set of types.
	Rules []TypeRules

	// validator performs default tag based validation.
	validator struct { Validator }
)


// Validator

func (v *Validator) Constructor(
	validate *play.Validate,
	_*struct{args.Optional}, translator ut.Translator,
) {
	v.validate   = validate
	v.translator = translator
}

func (v *Validator) WithRules(
	rules      Rules,
	configure  func(*play.Validate) error,
	translator ut.Translator,
) error {
	if v.validate != nil {
		panic("validator already initialized")
	}

	validate := play.New()
	if configure != nil {
		if err := configure(validate); err != nil {
			return err
		}
	}

	for _, rule := range rules {
		validate.RegisterStructValidationMapRules(rule.Constraints, rule.Type)
	}

	v.validate   = validate
	v.translator = translator
	return nil
}

func (v *Validator) Validate(
	target  any,
	outcome *validates.Outcome,
) miruken.HandleResult {
	if !internal.IsStruct(target) {
		return miruken.NotHandled
	}
	if err := v.validate.Struct(target); err != nil {
		switch e := err.(type) {
		case *play.InvalidValidationError:
			return miruken.NotHandled.WithError(err)
		case play.ValidationErrors:
			if v.translator == nil {
				v.addErrors(outcome, e)
			} else {
				v.translateErrors(outcome, e)
			}
			return miruken.HandledAndStop
		default:
			panic(fmt.Errorf("unexpected validation error: %w", err))
		}
	}
	return miruken.Handled
}

func (v *Validator) ValidateAndStop(
	target  any,
	outcome *validates.Outcome,
) miruken.HandleResult {
	result := v.Validate(target, outcome)
	if result.Handled() {
		// Stop the generic validator from validating tags
		return result.Or(miruken.HandledAndStop)
	}
	return result
}

func (v *Validator) addErrors(
	outcome     *validates.Outcome,
	fieldErrors play.ValidationErrors,
) {
	for _, err := range fieldErrors {
		var path string
		ns    := err.StructNamespace()
		parts := strings.SplitN(ns, ".", 2)
		if len(parts) > 1 { path = parts[1] }
		outcome.AddError(path, err)
	}
}

func (v *Validator) translateErrors(
	outcome     *validates.Outcome,
	fieldErrors play.ValidationErrors,
) {
	for field, msg := range fieldErrors.Translate(v.translator) {
		var path string
		parts := strings.SplitN(field, ".", 2)
		if len(parts) > 1 { path = parts[1] }
		outcome.AddError(path, errors.New(msg))
	}
}


// Type is a helper function to define the constraints for a type.
func Type[T any](constraints map[string]string) TypeRules {
	typ := reflect.Zero(internal.TypeOf[T]()).Interface()
	return TypeRules{Type: typ, Constraints: constraints}
}


// ValidatorT

func (v *ValidatorT[T]) Validate(
	validate *validates.It, t T,
) miruken.HandleResult {
	return v.ValidateAndStop(t, validate.Outcome())
}


// validator

func (v *validator) Validate(
	validate *validates.It, target any,
) miruken.HandleResult {
	return v.Validator.Validate(target, validate.Outcome())
}
