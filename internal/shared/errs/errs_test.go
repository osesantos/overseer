package errs

import (
	"errors"
	"testing"
)

func TestSentinelsAreDistinct(t *testing.T) {
	if ErrNotFound == nil || ErrAlreadyExists == nil || ErrInvalidInput == nil || ErrCorruptedData == nil {
		t.Fatal("expected all sentinels to be non-nil")
	}
	if ErrNotFound == ErrAlreadyExists || ErrNotFound == ErrInvalidInput || ErrNotFound == ErrCorruptedData {
		t.Fatal("expected sentinels to be distinct")
	}
}

func TestWrapPreservesErrorForIs(t *testing.T) {
	wrapped := Wrap(ErrNotFound, "load config")
	if !Is(wrapped, ErrNotFound) {
		t.Fatal("expected wrapped error to match ErrNotFound")
	}
	if errors.Is(wrapped, ErrAlreadyExists) {
		t.Fatal("wrapped error should not match ErrAlreadyExists")
	}
}

func TestIsDelegatesToErrorsIs(t *testing.T) {
	wrapped := Wrap(ErrCorruptedData, "parse file")
	if !Is(wrapped, ErrCorruptedData) {
		t.Fatal("expected Is() to match ErrCorruptedData")
	}
	if Is(wrapped, ErrInvalidInput) {
		t.Fatal("expected Is() to reject non-matching sentinel")
	}
}
