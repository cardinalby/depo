package errr

import "errors"

// As is a wrapper for errors.As that returns target error together with errors.As bool result
func As[Target error](err error) (Target, bool) {
	var target Target
	ok := errors.As(err, &target)
	return target, ok
}
