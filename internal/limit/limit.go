// Package limit defines the shared marker for configured limit failures.
package limit

import "errors"

// ErrExceeded identifies a configured resource or processing limit failure.
var ErrExceeded = errors.New("limit exceeded")
