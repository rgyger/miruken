package miruken

import "github.com/hashicorp/go-multierror"

var (
	Handled           = HandleResult{true,  false, nil}
	HandledAndStop    = HandleResult{true,  true,  nil}
	NotHandled        = HandleResult{false, false, nil}
	NotHandledAndStop = HandleResult{false, true,  nil}
)


type (
	// HandleResult describes the result of an operation.
	HandleResult struct {
		handled bool
		stop    bool
		err     error
	}

	// HandleResultBlock provides another HandleResult.
	HandleResultBlock func() HandleResult
)


func (r HandleResult) Handled() bool {
	return r.handled
}

func (r HandleResult) Stop() bool {
	return r.stop
}

func (r HandleResult) IsError() bool {
	return r.err != nil
}

func (r HandleResult) Error() error {
	return r.err
}

func  (r HandleResult) WithError(err error) HandleResult {
	if err == nil {
		return r
	}
	return HandleResult{r.handled, true, err}
}

func (r HandleResult) WithoutError() HandleResult {
	if r.IsError() {
		return HandleResult{r.handled, r.stop, nil}
	}
	return r
}

func (r HandleResult) Then(
	block HandleResultBlock,
) HandleResult {
	if block == nil {
		panic("block cannot be nil")
	}
	if r.stop {
		return r
	} else {
		return r.Or(block())
	}
}

func (r HandleResult) ThenIf(
	condition bool,
	block     HandleResultBlock,
) HandleResult {
	if block == nil {
		panic("block cannot be nil")
	}

	if r.stop || !condition {
		return r
	} else {
		return r.Or(block())
	}
}

func (r HandleResult) Otherwise(
	block HandleResultBlock,
) HandleResult {
	if block == nil {
		panic("block cannot be nil")
	}

	if r.handled || r.stop {
		return r
	} else {
		return block()
	}
}

func (r HandleResult) OtherwiseIf(
	condition bool,
	block     HandleResultBlock,
) HandleResult {
	if block == nil {
		panic("block cannot be nil")
	}

	if r.stop || (r.handled && !condition) {
		return r
	} else {
		return r.Or(block())
	}
}

func (r HandleResult) OtherwiseHandledIf(
	handled bool,
) HandleResult {
	if handled || r.handled {
		if r.stop {
			return r.Or(HandledAndStop)
		} else {
			return r.Or(Handled)
		}
	} else {
		if r.stop {
			return r.Or(NotHandledAndStop)
		} else {
			return r.Or(NotHandled)
		}
	}
}

func (r HandleResult) Or(other HandleResult) HandleResult {
	err := combineErrors(r, other)
	if r.handled || other.handled {
		if r.stop || other.stop {
			return HandledAndStop.WithError(err)
		} else {
			return Handled.WithError(err)
		}
	} else {
		if r.stop || other.stop {
			return NotHandledAndStop.WithError(err)
		} else {
			return NotHandled.WithError(err)
		}
	}
}

func (r HandleResult) OrBlock(block HandleResultBlock) HandleResult {
	if r.handled {
		if r.stop {
			return HandledAndStop
		} else {
			return Handled
		}
	} else {
		other := block()
		err   := combineErrors(r, other)
		if r.stop || other.stop {
			return NotHandledAndStop.WithError(err)
		} else {
			return NotHandled.WithError(err)
		}
	}
}

func (r HandleResult) And(other HandleResult) HandleResult {
	err := combineErrors(r, other)
	if r.handled && other.handled {
		if r.stop || other.stop {
			return HandledAndStop.WithError(err)
		} else {
			return Handled.WithError(err)
		}
	} else {
		if r.stop || other.stop {
			return NotHandledAndStop.WithError(err)
		} else {
			return NotHandled.WithError(err)
		}
	}
}

func (r HandleResult) AndBlock(other HandleResult) HandleResult {
	err := combineErrors(r, other)
	if r.handled && other.handled {
		if r.stop || other.stop {
			return HandledAndStop.WithError(err)
		} else {
			return Handled.WithError(err)
		}
	} else {
		if r.stop || other.stop {
			return NotHandledAndStop.WithError(err)
		} else {
			return NotHandled.WithError(err)
		}
	}
}


func combineErrors(r1 HandleResult, r2 HandleResult) error {
	if e1, e2 := r1.err, r2.err; e1 != nil && e2 != nil {
		return multierror.Append(e1, e2)
	} else if e1 != nil {
		return e1
	} else if e2 != nil {
		return e2
	}
	return nil
}