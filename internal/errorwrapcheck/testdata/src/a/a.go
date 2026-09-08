package a

import (
	"errors"
	"fmt"
)

func checks() {
	err := errors.New("cause")
	_ = fmt.Errorf("context: %v", err) // want `fmt.Errorf formats an error without %w`
	_ = fmt.Errorf("context: %s", err) // want `fmt.Errorf formats an error without %w`
	_ = fmt.Errorf("context: %w", err)
	_ = fmt.Errorf("context: %[1]w", err)
	_ = fmt.Errorf("literal %%w: %v", err) // want `fmt.Errorf formats an error without %w`
	_ = fmt.Errorf("value: %v", 42)
	_ = fmt.Errorf("leaf error")
}
