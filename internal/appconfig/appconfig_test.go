package appconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// uninstalled is what Of answers before anything installs a reader. Captured
// here rather than inside the test because package initialization runs once
// however many times the tests are run, and the first installation below is
// permanent. Package fiber is not linked into this binary, so this is the only
// place that answer is observable at all.
var uninstalled = Of("no reader installed yet")

// Test_Reader covers the reader's contract in the order it depends on: what Of
// answers before an installation, the one that wins, and every call after it.
//
// One function, and not parallel, because SetReader is once-only and the
// assertions mean nothing out of order.
func Test_Reader(t *testing.T) {
	require.Equal(t, Hot{}, uninstalled)

	// A nil reader must not spend the single installation, which the real one
	// taking effect below is what proves.
	SetReader(nil)

	installed := Hot{Immutable: true, UnescapePath: true}
	SetReader(func(app any) Hot {
		if app == nil {
			return Hot{}
		}
		return installed
	})
	require.Equal(t, installed, Of("an app"))
	require.Equal(t, Hot{}, Of(nil), "the installed reader decides what it answers for")

	SetReader(func(any) Hot { return Hot{DisableHeaderNormalizing: true} })
	require.Equal(t, installed, Of("an app"), "the first reader keeps answering")
}
