package golden

import (
	"io"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Setup keeps the old golden-test call site for consistency.
// Call this at the start of every golden file test.
func Setup(t *testing.T) {
	t.Helper()
}

func StripANSI(s string) string { return ansi.Strip(s) }

// ReadBts reads all bytes from r, failing the test on error.
func ReadBts(tb testing.TB, r io.Reader) []byte {
	tb.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		tb.Fatalf("ReadBts: %v", err)
	}
	return data
}
