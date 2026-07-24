package cmd

import (
	"strings"
	"testing"
)

// kernel cleanup removes packages by default as of v1.3.0. The confirmation prompt is what
// stands between an operator who typed the old preview command and an unintended removal, so
// only an explicit yes may pass.
func TestConfirmKernelCleanupRequiresExplicitYes(t *testing.T) {
	accepted := []string{"y\n", "Y\n", "yes\n", "YES\n", " yes \n", "Yes\n"}
	for _, answer := range accepted {
		if !confirmKernelCleanup(strings.NewReader(answer), 3) {
			t.Errorf("answer %q should confirm", answer)
		}
	}

	rejected := []string{
		"n\n", "N\n", "no\n",
		"\n", // bare Enter
		"",   // EOF with no input at all
		"maybe\n", "ye\n", "yess\n", "1\n", "true\n",
		" \n", // whitespace only
	}
	for _, answer := range rejected {
		if confirmKernelCleanup(strings.NewReader(answer), 3) {
			t.Errorf("answer %q must NOT confirm a package removal", answer)
		}
	}
}

// A closed or unreadable stdin must abort rather than fall through to removal.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errReadFailed }

type readError string

func (e readError) Error() string { return string(e) }

const errReadFailed = readError("stdin unavailable")

func TestConfirmKernelCleanupAbortsOnReadError(t *testing.T) {
	if confirmKernelCleanup(failingReader{}, 2) {
		t.Fatal("an unreadable stdin confirmed a package removal")
	}
}
