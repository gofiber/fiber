package idempotency

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
)

// errBudgetWriter is an io.Writer that accepts up to n bytes total and then
// fails. It is used to drive the error-handling branches of EncodeMsg by
// failing the underlying writer at every possible offset.
type errBudgetWriter struct {
	n int
}

var errWriteBudget = errors.New("write budget exhausted")

func (w *errBudgetWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, errWriteBudget
	}
	if len(p) > w.n {
		k := w.n
		w.n = 0
		return k, errWriteBudget
	}
	w.n -= len(p)
	return len(p), nil
}

func populatedResponse() response {
	return response{
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Multi":      {"a", "b", "c"},
		},
		Body:       []byte("hello world"),
		StatusCode: 200,
	}
}

// Test_response_MarshalUnmarshal_Populated exercises the headers-map, body and
// status-code paths of MarshalMsg/UnmarshalMsg with non-empty data, including
// the clear()/reuse branch when unmarshaling into an already-populated value.
func Test_response_MarshalUnmarshal_Populated(t *testing.T) {
	t.Parallel()

	v := populatedResponse()
	bts, err := v.MarshalMsg(nil)
	require.NoError(t, err)

	// Unmarshal into a value that already has a populated Headers map and Body
	// to exercise the clear()/capacity-reuse branches.
	out := response{
		Headers: map[string][]string{"stale": {"value"}},
		Body:    make([]byte, 0, 64),
	}
	left, err := out.UnmarshalMsg(bts)
	require.NoError(t, err)
	require.Empty(t, left)
	require.Equal(t, v.Headers, out.Headers)
	require.Equal(t, v.Body, out.Body)
	require.Equal(t, v.StatusCode, out.StatusCode)
}

// Test_response_EncodeDecode_Populated exercises the streaming EncodeMsg /
// DecodeMsg paths with non-empty data.
func Test_response_EncodeDecode_Populated(t *testing.T) {
	t.Parallel()

	v := populatedResponse()

	var buf bytes.Buffer
	w := msgp.NewWriter(&buf)
	require.NoError(t, v.EncodeMsg(w))
	require.NoError(t, w.Flush())

	// Decode into a value with a pre-existing Headers map to hit the clear() branch.
	out := response{Headers: map[string][]string{"stale": {"x"}}}
	r := msgp.NewReader(&buf)
	require.NoError(t, out.DecodeMsg(r))

	require.Equal(t, v.Headers, out.Headers)
	require.Equal(t, v.Body, out.Body)
	require.Equal(t, v.StatusCode, out.StatusCode)
}

// Test_response_UnmarshalMsg_UnknownField verifies the default/skip branch of
// both UnmarshalMsg and DecodeMsg when an unrecognized map key is present.
func Test_response_UnmarshalMsg_UnknownField(t *testing.T) {
	t.Parallel()

	// Build a map with one known field ("sc") and one unknown field ("zz").
	var raw []byte
	raw = msgp.AppendMapHeader(raw, 2)
	raw = msgp.AppendString(raw, "zz")
	raw = msgp.AppendString(raw, "ignored")
	raw = msgp.AppendString(raw, "sc")
	raw = msgp.AppendInt(raw, 418)

	var out response
	left, err := out.UnmarshalMsg(raw)
	require.NoError(t, err)
	require.Empty(t, left)
	require.Equal(t, 418, out.StatusCode)

	// Same for the streaming decoder.
	var dec response
	r := msgp.NewReader(bytes.NewReader(raw))
	require.NoError(t, dec.DecodeMsg(r))
	require.Equal(t, 418, dec.StatusCode)
}

// Test_response_Decode_Truncated feeds every truncated prefix of a valid
// encoding to UnmarshalMsg and DecodeMsg, exercising the error-return branch of
// each individual read in the generated code.
func Test_response_Decode_Truncated(t *testing.T) {
	t.Parallel()

	v0 := populatedResponse()
	full, err := v0.MarshalMsg(nil)
	require.NoError(t, err)

	for i := 0; i < len(full); i++ {
		prefix := full[:i]

		var out response
		_, uerr := out.UnmarshalMsg(prefix)
		require.Error(t, uerr, "UnmarshalMsg should fail on prefix len %d", i)

		var dec response
		r := msgp.NewReader(bytes.NewReader(prefix))
		require.Error(t, dec.DecodeMsg(r), "DecodeMsg should fail on prefix len %d", i)
	}
}

// Test_response_Decode_LimitExceeded covers the ErrLimitExceeded branches for
// both the outer headers map and the per-header value array.
func Test_response_Decode_LimitExceeded(t *testing.T) {
	t.Parallel()

	// Outer headers map header exceeds the 1024 limit.
	var bigMap []byte
	bigMap = msgp.AppendMapHeader(bigMap, 1)
	bigMap = msgp.AppendString(bigMap, "hs")
	bigMap = msgp.AppendMapHeader(bigMap, 1025)

	var out response
	_, err := out.UnmarshalMsg(bigMap)
	require.ErrorIs(t, err, msgp.ErrLimitExceeded)

	var dec response
	require.ErrorIs(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(bigMap))), msgp.ErrLimitExceeded)

	// Per-header value array header exceeds the 1024 limit.
	var bigArr []byte
	bigArr = msgp.AppendMapHeader(bigArr, 1)
	bigArr = msgp.AppendString(bigArr, "hs")
	bigArr = msgp.AppendMapHeader(bigArr, 1)
	bigArr = msgp.AppendString(bigArr, "key")
	bigArr = msgp.AppendArrayHeader(bigArr, 1025)

	var out2 response
	_, err = out2.UnmarshalMsg(bigArr)
	require.ErrorIs(t, err, msgp.ErrLimitExceeded)

	var dec2 response
	require.ErrorIs(t, dec2.DecodeMsg(msgp.NewReader(bytes.NewReader(bigArr))), msgp.ErrLimitExceeded)
}

// Test_response_EncodeMsg_WriterErrors drives the error-return branches of
// EncodeMsg by failing the underlying writer at every byte offset.
func Test_response_EncodeMsg_WriterErrors(t *testing.T) {
	t.Parallel()

	v0 := populatedResponse()
	full, err := v0.MarshalMsg(nil)
	require.NoError(t, err)

	sawErr := false
	for budget := 0; budget < len(full); budget++ {
		v := populatedResponse()
		w := msgp.NewWriterSize(&errBudgetWriter{n: budget}, 8)
		encErr := v.EncodeMsg(w)
		if encErr == nil {
			encErr = w.Flush()
		}
		if encErr != nil {
			sawErr = true
		}
	}
	require.True(t, sawErr, "expected at least one writer error across budgets")
}
