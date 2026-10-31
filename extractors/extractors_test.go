package extractors

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// extractorsTestCase is a request and the value an extractor must read from it.
// The request starts blank and carries only what the fields below put on it.
type extractorsTestCase struct {
	headers   map[string]string // set with Header.Set
	cookies   map[string]string // set with Header.SetCookie
	name      string
	route     string // route pattern; the extractor runs in its handler, where route parameters exist
	uri       string // request URI, query string included; with route, the GET target and the only field sent
	form      string // urlencoded body; makes the request a form POST
	want      string // the value to extract; empty means ErrNotFound
	extractor Extractor
	source    Source // the Source Resolve must report with the result; only extractorsTestRunResolve reads it
}

// extractorsTestCtx returns a context from a fresh app whose request carries what
// tc describes, route aside.
func extractorsTestCtx(t *testing.T, tc *extractorsTestCase) fiber.Ctx {
	t.Helper()

	ctx := apptest.Ctx(t, fiber.New())
	req := ctx.Request()
	if tc.uri != "" {
		req.SetRequestURI(tc.uri)
	}
	if tc.form != "" {
		req.Header.SetContentType(fiber.MIMEApplicationForm)
		req.Header.SetMethod(fiber.MethodPost)
		req.SetBodyString(tc.form)
	}
	for name, value := range tc.headers {
		req.Header.Set(name, value)
	}
	for name, value := range tc.cookies {
		req.Header.SetCookie(name, value)
	}
	return ctx
}

// extractorsTestQueryCtx returns a context from a fresh app whose request URI carries token=from-query.
func extractorsTestQueryCtx(t *testing.T) fiber.Ctx {
	t.Helper()

	ctx := apptest.Ctx(t, fiber.New())
	ctx.Request().SetRequestURI("/?token=from-query")
	return ctx
}

// extractorsTestRun runs one parallel subtest per case. Each builds the case's
// request on a fresh context, or sends it to the case's route, and requires the
// case's extractor to yield want, or ErrNotFound when want is empty.
func extractorsTestRun(t *testing.T, cases []extractorsTestCase) {
	t.Helper()

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var (
				got string
				err error
			)
			if tc.route != "" {
				got, err = extractorsTestParam(t, fiber.New(), tc.route, tc.uri, tc.extractor)
			} else {
				got, err = tc.extractor.Extract(extractorsTestCtx(t, tc))
			}

			if tc.want == "" {
				require.Empty(t, got)
				require.ErrorIs(t, err, ErrNotFound)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// extractorsTestRunResolve is extractorsTestRun for Resolve: each case runs on a
// fresh context, since a walk writes the chain state into the one it is given,
// and must report want from source, or fail with ErrNotFound and report source
// when want is empty.
func extractorsTestRunResolve(t *testing.T, cases []extractorsTestCase) {
	t.Helper()

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := extractorsTestCtx(t, tc)
			if tc.want == "" {
				extractorsTestExpectResolveErr(t, tc.extractor, ctx, tc.source, ErrNotFound)
				return
			}
			extractorsTestExpectResolved(t, tc.extractor, ctx, tc.want, tc.source)
		})
	}
}

// extractorsTestAuthCase is an Authorization header value and the credential
// FromAuthHeader must take from it.
type extractorsTestAuthCase struct {
	name        string
	header      string // the Authorization value; empty leaves the field off the request
	want        string // the credential; empty means ErrNotFound
	description string // what the case pins down, quoted when it fails
}

// extractorsTestRunAuth runs one parallel subtest per case against
// FromAuthHeader(scheme), each on a fresh context.
func extractorsTestRunAuth(t *testing.T, scheme string, cases []extractorsTestAuthCase) {
	t.Helper()

	extractor := FromAuthHeader(scheme)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := apptest.Ctx(t, fiber.New())
			if tc.header != "" {
				ctx.Request().Header.Set(fiber.HeaderAuthorization, tc.header)
			}

			if tc.want == "" {
				extractorsTestExpectErr(t, extractor, ctx, ErrNotFound, "Expected error for %s", tc.description)
				return
			}
			extractorsTestExpect(t, extractor, ctx, tc.want, "Expected no error for %s", tc.description)
		})
	}
}

// extractorsTestExpect requires e to yield want, with no error, on c.
func extractorsTestExpect(t *testing.T, e Extractor, c fiber.Ctx, want string, msgAndArgs ...any) {
	t.Helper()

	got, err := e.Extract(c)
	require.NoError(t, err, msgAndArgs...)
	require.Equal(t, want, got, msgAndArgs...)
}

// extractorsTestExpectErr requires e to yield no value and an error matching
// wantErr on c.
func extractorsTestExpectErr(t *testing.T, e Extractor, c fiber.Ctx, wantErr error, msgAndArgs ...any) {
	t.Helper()

	got, err := e.Extract(c)
	require.Empty(t, got, msgAndArgs...)
	require.ErrorIs(t, err, wantErr, msgAndArgs...)
}

// extractorsTestExpectFiberError requires e to yield no value and a *fiber.Error
// with the given code and a message containing msg on c.
func extractorsTestExpectFiberError(t *testing.T, e Extractor, c fiber.Ctx, code int, msg string) {
	t.Helper()

	got, err := e.Extract(c)
	require.Empty(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), msg)
	var fe *fiber.Error
	require.ErrorAs(t, err, &fe)
	require.Equal(t, code, fe.Code)
}

// resolveSource adapts Resolve to the (value, Source, error) shape most Resolve
// tests assert on. Test_Resolve_Winner covers the rest of Result.
func resolveSource(e Extractor, c fiber.Ctx) (string, Source, error) {
	r, err := Resolve(e, c)
	return r.Value, r.Source, err
}

// extractorsTestExpectResolved requires Resolve to report value from source for
// e on c. msgAndArgs describe the source check.
func extractorsTestExpectResolved(t *testing.T, e Extractor, c fiber.Ctx, value string, source Source, msgAndArgs ...any) {
	t.Helper()

	gotValue, gotSource, err := resolveSource(e, c)
	require.NoError(t, err)
	require.Equal(t, value, gotValue)
	require.Equal(t, source, gotSource, msgAndArgs...)
}

// extractorsTestExpectResolveErr requires Resolve to fail with wantErr and no
// value for e on c, reporting source.
func extractorsTestExpectResolveErr(t *testing.T, e Extractor, c fiber.Ctx, source Source, wantErr error) {
	t.Helper()

	gotValue, gotSource, err := resolveSource(e, c)
	require.Empty(t, gotValue)
	require.Equal(t, source, gotSource)
	require.ErrorIs(t, err, wantErr)
}

// extractorsTestExpectResult requires Resolve to return want for e on c, Key and
// Resolved included.
func extractorsTestExpectResult(t *testing.T, e Extractor, c fiber.Ctx, want Result) {
	t.Helper()

	res, err := Resolve(e, c)
	require.NoError(t, err)
	require.Equal(t, want, res)
}

// extractorsTestFixed returns a custom extractor named key that always yields
// value and err.
func extractorsTestFixed(key, value string, err error) Extractor {
	return Extractor{
		Extract: func(fiber.Ctx) (string, error) { return value, err },
		Key:     key,
		Source:  SourceCustom,
	}
}

// extractorsTestDecorate returns e with an Extract that runs e's own and hands its value to after, as a
// caller decorating an extractor for validation or normalization does. An error of e's own is returned as is.
func extractorsTestDecorate(e Extractor, after func(c fiber.Ctx, value string) (string, error)) Extractor {
	base := e.Extract
	e.Extract = func(c fiber.Ctx) (string, error) {
		v, err := base(c)
		if err != nil {
			return "", err
		}
		return after(c, v)
	}
	return e
}

// extractorsTestCyclic returns a chain whose only child extracts through the
// chain again, a cycle that Extract must refuse with ErrChainCycle.
func extractorsTestCyclic() Extractor {
	var cyclic Extractor
	cyclic = Chain(FromCustom("cycle", func(c fiber.Ctx) (string, error) { return cyclic.Extract(c) }))
	return cyclic
}

// extractorsTestHeaderCtx returns a context from an app configured with cfg. When
// cfg disables header normalizing, the request keeps its header names byte for
// byte too, as it does for a request read off the wire.
func extractorsTestHeaderCtx(t *testing.T, cfg ...fiber.Config) fiber.Ctx {
	t.Helper()

	ctx := apptest.Ctx(t, fiber.New(cfg...))
	if len(cfg) > 0 && cfg[0].DisableHeaderNormalizing {
		ctx.Request().Header.DisableNormalizing()
	}
	return ctx
}

// extractorsTestParam sends a GET for target to app, which routes pattern to a
// handler that runs e, and returns what e extracted there. A route parameter
// exists only inside a matched route, so the test fails if the route did not
// match.
func extractorsTestParam(t *testing.T, app *fiber.App, pattern, target string, e Extractor) (string, error) {
	t.Helper()

	var (
		value string
		err   error
		ran   bool
	)
	app.Get(pattern, func(c fiber.Ctx) error {
		ran = true
		value, err = e.Extract(c)
		return nil
	})
	apptest.Expect(t, app, target, fiber.StatusOK)
	require.True(t, ran, "the route must have matched")
	return value, err
}

func Test_Extractors_Missing(t *testing.T) {
	t.Parallel()
	extractorsTestRun(t, []extractorsTestCase{
		// The route has no :token segment, so there is nothing to read.
		{name: "FromParam", extractor: FromParam("token"), route: "/test", uri: "/test"},
		{name: "FromForm", extractor: FromForm("token")},
		{name: "FromQuery", extractor: FromQuery("token")},
		{name: "FromHeader", extractor: FromHeader("X-Token")},
		{name: "FromAuthHeader", extractor: FromAuthHeader("Bearer")},
		{name: "FromCookie", extractor: FromCookie("token")},
	})
}

func Test_Extractors(t *testing.T) {
	t.Parallel()
	extractorsTestRun(t, []extractorsTestCase{
		{
			name:      "FromParam",
			extractor: FromParam("token"),
			route:     "/test/:token",
			uri:       "/test/token_from_param",
			want:      "token_from_param",
		},
		{name: "FromForm", extractor: FromForm("token"), form: "token=token_from_form", want: "token_from_form"},
		{name: "FromQuery", extractor: FromQuery("token"), uri: "/?token=token_from_query", want: "token_from_query"},
		{
			name:      "FromHeader",
			extractor: FromHeader("X-Token"),
			headers:   map[string]string{"X-Token": "token_from_header"},
			want:      "token_from_header",
		},
		{
			name:      "FromAuthHeader",
			extractor: FromAuthHeader("Bearer"),
			headers:   map[string]string{fiber.HeaderAuthorization: "Bearer token_from_auth_header"},
			want:      "token_from_auth_header",
		},
		{
			name:      "FromCookie",
			extractor: FromCookie("token"),
			cookies:   map[string]string{"token": "token_from_cookie"},
			want:      "token_from_cookie",
		},
	})
}

func Test_Extractor_Chain(t *testing.T) {
	t.Parallel()
	headerThenQuery := Chain(FromHeader("X-Token"), FromQuery("token"))

	extractorsTestRun(t, []extractorsTestCase{
		{name: "no_extractors", extractor: Chain()},
		{
			name:      "first_extractor_succeeds",
			extractor: headerThenQuery,
			headers:   map[string]string{"X-Token": "token_from_header"},
			uri:       "/?token=token_from_query",
			want:      "token_from_header",
		},
		{name: "second_extractor_succeeds", extractor: headerThenQuery, uri: "/?token=token_from_query", want: "token_from_query"},
		{name: "all_extractors_fail", extractor: headerThenQuery},
		// This extractor will return "", nil
		{name: "empty_extractor_returns_not_found", extractor: Chain(extractorsTestFixed("token", "", nil))},
	})
}

// Test_Extractor_FromHeader_CombinesRepeatedLines covers a field carried on
// several lines. FromHeader is given its name by the application, and a name it
// is given may be a list field a peer may legally send twice, so the lines are
// combined the way RFC 9110 §5.3 says a recipient may rather than refused as an
// ambiguous single value.
func Test_Extractor_FromHeader_CombinesRepeatedLines(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			ctx := extractorsTestHeaderCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})

			ctx.Request().Header.Add("Accept", "text/html")
			// The second spelling is the one HTTP/2 and 3 put on the wire.
			ctx.Request().Header.Add("accept", "application/json")

			extractorsTestExpect(t, FromHeader("Accept"), ctx, "text/html, application/json")
		})
	}
}

// Test_Extractor_FromHeader_CookieKeepsItsOwnSeparator covers FromHeader named
// with Cookie, whose crumbs are joined with "; " (RFC 6265 §5.4) rather than
// with the comma a list field takes.
//
// fasthttp keeps cookies in a store of their own that PeekAll enumerates one at
// a time, so the generic join produced "a=1, b=2" for a request that said
// "a=1; b=2" — a value no cookie parser downstream would read back.
func Test_Extractor_FromHeader_CookieKeepsItsOwnSeparator(t *testing.T) {
	t.Parallel()
	ctx := apptest.Ctx(t, fiber.New())

	// Read from the wire rather than Set: a request carrying Cookie twice, which
	// is what an HTTP/1 gateway writes for the split field HTTP/2 permits. Only
	// then does fasthttp hand the crumbs back one at a time.
	raw := "GET / HTTP/1.1\r\nHost: example.com\r\nCookie: a=1\r\nCookie: b=2\r\n\r\n"
	require.NoError(t, ctx.Request().Header.Read(bufio.NewReader(strings.NewReader(raw))))

	extractorsTestExpect(t, FromHeader(fiber.HeaderCookie), ctx, "a=1; b=2")
}

// Test_Extractor_FromHeader_SingleLineIsUnchanged is the control for the test
// above: one line is returned as it arrived, with no separator introduced.
func Test_Extractor_FromHeader_SingleLineIsUnchanged(t *testing.T) {
	t.Parallel()
	ctx := apptest.Ctx(t, fiber.New())
	ctx.Request().Header.Set("X-API-Key", "abc123")

	extractorsTestExpect(t, FromHeader("X-API-Key"), ctx, "abc123")
}

// Test_Extractor_FromAuthHeader_RepeatedLineIsNotACredential covers a message
// carrying Authorization twice, which is what a middleware clearing the field
// leaves behind when the client sent another spelling of it.
//
// Under DisableHeaderNormalizing a byte-exact Set writes the canonical name and
// leaves the client's lower-case line — the spelling HTTP/2 and 3 put on the
// wire — in place beside it. Reading either one lets whoever wrote the other
// decide: the first line is the cleared value only where fasthttp happens to
// store it first, and the first non-empty one is always the client's. Neither
// is an answer, so no credential is extracted and the caller refuses.
func Test_Extractor_FromAuthHeader_RepeatedLineIsNotACredential(t *testing.T) {
	t.Parallel()
	cfg := fiber.Config{DisableHeaderNormalizing: true}
	extractor := FromAuthHeader("Bearer")

	// Both orders fasthttp stores, depending on whether the client's line was
	// already under the canonical name when the middleware cleared it.
	for _, tc := range []struct {
		name  string
		lines [2][2]string
	}{
		{name: "client_line_first", lines: [2][2]string{{"authorization", "Bearer client"}, {"Authorization", ""}}},
		{name: "cleared_line_first", lines: [2][2]string{{"Authorization", ""}, {"authorization", "Bearer client"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := extractorsTestHeaderCtx(t, cfg)
			for _, line := range tc.lines {
				ctx.Request().Header.Add(line[0], line[1])
			}

			extractorsTestExpectErr(t, extractor, ctx, ErrNotFound,
				"a second Authorization line makes the credential ambiguous, whichever order it is stored in")
		})
	}

	// A single line is still extracted, so the assertions above cannot pass by
	// refusing everything.
	t.Run("single_line_is_extracted", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestHeaderCtx(t, cfg)
		ctx.Request().Header.Set("authorization", "Bearer client")

		extractorsTestExpect(t, extractor, ctx, "client")
	})
}

func Test_Extractor_FromAuthHeader_EdgeCases(t *testing.T) {
	t.Parallel()
	extractorsTestRunAuth(t, "Bearer", []extractorsTestAuthCase{
		{name: "wrong_scheme", header: "Basic dXNlcjpwYXNz", description: "Basic auth instead of Bearer"},
		{name: "missing_space_after_scheme", header: "Bearertoken", description: "missing space after Bearer"},
		{name: "case_insensitive_scheme_matching", header: "bearer token", want: "token", description: "lowercase bearer"},
		// A token containing whitespace is rejected per the RFC 7235 token68 spec.
		{name: "token_with_whitespace", header: "Bearer token with spaces and\ttabs", description: "token with spaces and a tab"},
	})

	extractorsTestRunAuth(t, "CustomScheme", []extractorsTestAuthCase{
		{name: "custom_scheme", header: "CustomScheme my-token", want: "my-token", description: "custom auth scheme"},
	})
}

func Test_Extractor_Chain_Introspection(t *testing.T) {
	t.Parallel()
	chainExtractor := Chain(FromHeader("X-Token"), FromQuery("token"), FromCookie("auth"))

	require.Equal(t, SourceHeader, chainExtractor.Source)
	require.Equal(t, "X-Token", chainExtractor.Key)
	require.Len(t, chainExtractor.Chain, 3)

	require.Equal(t, SourceHeader, chainExtractor.Chain[0].Source)
	require.Equal(t, "X-Token", chainExtractor.Chain[0].Key)
	require.Equal(t, SourceQuery, chainExtractor.Chain[1].Source)
	require.Equal(t, "token", chainExtractor.Chain[1].Key)
	require.Equal(t, SourceCookie, chainExtractor.Chain[2].Source)
	require.Equal(t, "auth", chainExtractor.Chain[2].Key)
}

func Test_Extractor_Contains(t *testing.T) {
	t.Parallel()
	isCSRFCookie := func(e Extractor) bool { return e.Source == SourceCookie && e.Key == "CSRF" }

	cyclic := FromHeader("X-Token")
	cyclic.Chain = make([]Extractor, 1)
	cyclic.Chain[0] = cyclic

	for _, tc := range []struct {
		pred      func(Extractor) bool
		name      string
		extractor Extractor
		want      bool
	}{
		{name: "nil_predicate", extractor: FromHeader("X-Token"), pred: nil, want: false},
		{name: "single_extractor_match", extractor: FromCookie("CSRF"), pred: isCSRFCookie, want: true},
		{
			name:      "nested_chain_match",
			extractor: Chain(FromQuery("token"), Chain(FromHeader("X-Token"), FromCookie("CSRF"))),
			pred:      isCSRFCookie,
			want:      true,
		},
		{
			name:      "nested_chain_no_match",
			extractor: Chain(FromQuery("token"), Chain(FromHeader("X-Token"), FromCookie("session"))),
			pred:      isCSRFCookie,
			want:      false,
		},
		{name: "cyclic_chain_no_match", extractor: cyclic, pred: isCSRFCookie, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.extractor.Contains(tc.pred))
		})
	}
}

func Test_Extractor_FromCustom(t *testing.T) {
	t.Parallel()
	t.Run("successful_extraction", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.Set("X-Custom", "custom-value")

		customExtractor := FromCustom("X-Custom", func(c fiber.Ctx) (string, error) {
			value := c.Get("X-Custom")
			if value == "" {
				return "", ErrNotFound
			}
			return strings.ToUpper(value), nil
		})

		extractorsTestExpect(t, customExtractor, ctx, "CUSTOM-VALUE")
		require.Equal(t, SourceCustom, customExtractor.Source)
		require.Equal(t, "X-Custom", customExtractor.Key)
		require.Empty(t, customExtractor.AuthScheme)
	})

	t.Run("extraction_with_error", func(t *testing.T) {
		t.Parallel()
		errorExtractor := FromCustom("test", func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusBadRequest, "Custom error")
		})

		extractorsTestExpectFiberError(t, errorExtractor, apptest.Ctx(t, fiber.New()), fiber.StatusBadRequest, "Custom error")
	})

	t.Run("extraction_returning_empty_string", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		emptyExtractor := FromCustom("empty", func(_ fiber.Ctx) (string, error) { return "", nil })

		token, err := emptyExtractor.Extract(ctx)
		require.Empty(t, token)
		require.NoError(t, err)
	})

	t.Run("nil_function", func(t *testing.T) {
		t.Parallel()
		extractorsTestExpectErr(t, FromCustom("nil", nil), apptest.Ctx(t, fiber.New()), ErrNotFound)
	})
}

func Test_Extractor_Chain_Error_Propagation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		msg   string
		chain Extractor
		code  int
	}{
		{
			name: "last_error_wins",
			chain: Chain(
				extractorsTestFixed("error1", "", fiber.NewError(fiber.StatusBadRequest, "First error")),
				extractorsTestFixed("error2", "", fiber.NewError(fiber.StatusUnauthorized, "Second error")),
			),
			msg:  "Second error",
			code: fiber.StatusUnauthorized,
		},
		{
			name: "all_errors",
			chain: Chain(
				extractorsTestFixed("error1", "", fiber.NewError(fiber.StatusUnauthorized, "First auth error")),
				extractorsTestFixed("error2", "", fiber.NewError(fiber.StatusForbidden, "Second auth error")),
			),
			msg:  "Second auth error",
			code: fiber.StatusForbidden,
		},
		{
			name: "fail_then_error",
			chain: Chain(
				extractorsTestFixed("fail", "", ErrNotFound),
				extractorsTestFixed("error", "", fiber.NewError(fiber.StatusBadRequest, "Bad request")),
			),
			msg:  "Bad request",
			code: fiber.StatusBadRequest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extractorsTestExpectFiberError(t, tc.chain, apptest.Ctx(t, fiber.New()), tc.code, tc.msg)
		})
	}
}

func Test_Extractor_FromAuthHeader_RFC_Compliance(t *testing.T) {
	t.Parallel()
	extractorsTestRunAuth(t, "Bearer", []extractorsTestAuthCase{
		{
			name:        "single_space_after_scheme",
			header:      "Bearer token",
			want:        "token",
			description: "single space after scheme should be accepted - standard format",
		},
		{
			name:        "multiple_spaces_after_scheme",
			header:      "Bearer  token",
			description: "multiple spaces after scheme rejected for simplicity - single space is standard",
		},
		{
			name:        "mixed_whitespace_after_scheme",
			header:      "Bearer \t \ttoken",
			description: "mixed whitespace after scheme should be rejected - RFC specifies 1*SP, not tabs",
		},
		{
			name:        "only_whitespace_after_scheme",
			header:      "Bearer   \t  ",
			description: "only whitespace after scheme should fail",
		},
		{
			name:        "case_insensitive_scheme",
			header:      "BEARER token",
			want:        "token",
			description: "case-insensitive scheme matching should work",
		},
	})

	// Special case for case-insensitive scheme matching with different extractor scheme
	extractorsTestRunAuth(t, "bearer", []extractorsTestAuthCase{
		{
			name:        "case_insensitive_extractor_scheme",
			header:      "BEARER token",
			want:        "token",
			description: "lowercase extractor scheme matches an uppercase header scheme",
		},
	})
}

func Test_Extractor_FromAuthHeader_Token68_Validation(t *testing.T) {
	t.Parallel()
	const allToken68Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/="

	extractorsTestRunAuth(t, "Bearer", []extractorsTestAuthCase{
		{name: "all_token68_characters", header: "Bearer " + allToken68Chars, want: allToken68Chars, description: "valid token68 characters"},
		{name: "space_in_token", header: "Bearer abc def", description: "space in token"},
		{name: "space_after_scheme", header: "Bearer  abc", description: "multiple spaces after scheme"},
		{name: "only_scheme", header: "Bearer", description: "only scheme, too short for scheme + space + token"},
		{name: "tab_after_scheme", header: "Bearer\ttoken", description: "tab after scheme, RFC specifies 1*SP"},
		{name: "tab_in_token", header: "Bearer abc\tdef", description: "tab in token"},
		{name: "newline_in_token", header: "Bearer abc\ndef", description: "newline in token"},
		{name: "trailing_space_in_token", header: "Bearer abc ", description: "trailing space in token"},
		{name: "comma_in_token", header: "Bearer abc,def", description: "comma in token"},
		{name: "semicolon_in_token", header: "Bearer abc;def", description: "semicolon in token"},
		{name: "quote_in_token", header: "Bearer abc\"def", description: "quote in token"},
		{name: "bracket_in_token", header: "Bearer abc[def", description: "bracket in token"},
		{name: "equals_at_start", header: "Bearer =abc", description: "equals at start of token"},
		{name: "equals_in_middle", header: "Bearer ab=cd", description: "equals in middle of token"},
		{name: "valid_equals_at_end", header: "Bearer abc=", want: "abc=", description: "valid equals at end"},
		{name: "valid_double_equals", header: "Bearer abc==", want: "abc==", description: "valid double equals at end"},
	})
}

func Test_Extractor_FromAuthHeader_NoScheme(t *testing.T) {
	t.Parallel()
	extractorsTestRunAuth(t, "", []extractorsTestAuthCase{
		{name: "returns_header_value", header: "some-token-value", want: "some-token-value", description: "no scheme"},
		{name: "empty_header_returns_not_found", description: "no Authorization header set"},
	})
}

func Test_Extractor_FromAuthHeader_Metadata(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, scheme string }{
		{name: "custom_scheme", scheme: "CustomScheme"},
		{name: "bearer", scheme: "Bearer"},
		{name: "no_scheme", scheme: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extractor := FromAuthHeader(tc.scheme)
			require.Equal(t, SourceAuthHeader, extractor.Source)
			require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
			require.Equal(t, tc.scheme, extractor.AuthScheme)
		})
	}
}

func Test_Extractor_Chain_MixedScenarios(t *testing.T) {
	t.Parallel()
	failingExtractor := extractorsTestFixed("fail", "", ErrNotFound)
	errorExtractor := extractorsTestFixed("error", "", fiber.NewError(fiber.StatusBadRequest, "Bad request"))
	successExtractor := extractorsTestFixed("success", "success", nil)

	extractorsTestRun(t, []extractorsTestCase{
		{name: "error_then_success", extractor: Chain(errorExtractor, successExtractor), want: "success"},
		{name: "fail_then_error_then_success", extractor: Chain(failingExtractor, errorExtractor, successExtractor), want: "success"},
		{
			name:      "fail_then_success",
			extractor: Chain(failingExtractor, extractorsTestFixed("success", "success-token", nil)),
			want:      "success-token",
		},
		{
			name:      "nil_function_then_success",
			extractor: Chain(Extractor{Extract: nil, Key: "nil", Source: SourceCustom}, extractorsTestFixed("valid", "valid-token", nil)),
			want:      "valid-token",
		},
	})
}

func Test_Extractor_Chain_Cycle_Prevention(t *testing.T) {
	t.Parallel()
	t.Run("direct_cycle_returns_err_chain_cycle", func(t *testing.T) {
		t.Parallel()
		extractorsTestExpectErr(t, extractorsTestCyclic(), apptest.Ctx(t, fiber.New()), ErrChainCycle)
	})

	t.Run("indirect_cycle_returns_err_chain_cycle", func(t *testing.T) {
		t.Parallel()
		var chainA, chainB Extractor
		chainA = Chain(FromCustom("to-b", func(c fiber.Ctx) (string, error) { return chainB.Extract(c) }))
		chainB = Chain(FromCustom("to-a", func(c fiber.Ctx) (string, error) { return chainA.Extract(c) }))

		extractorsTestExpectErr(t, chainA, apptest.Ctx(t, fiber.New()), ErrChainCycle)
	})

	t.Run("nested_non_cyclic_chain_still_works", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.Set("X-Token", "token_from_header")

		inner := Chain(FromHeader("X-Token"), FromQuery("token"))
		outer := Chain(inner, FromCookie("token"))

		extractorsTestExpect(t, outer, ctx, "token_from_header")
	})
}

func Test_Extractor_Chain_ReusedAcrossMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	shared := Chain(FromHeader("X-Token"), FromQuery("token"))

	app.Use(func(c fiber.Ctx) error {
		extractorsTestExpect(t, shared, c, "token_from_header")
		return c.Next()
	})

	app.Get("/test", func(c fiber.Ctx) error {
		extractorsTestExpect(t, shared, c, "token_from_header")
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("X-Token", "token_from_header"))

	apptest.Expect(t, app, req, fiber.StatusNoContent)
}

func Test_Extractor_SourceTypes(t *testing.T) {
	t.Parallel()
	t.Run("individual_extractor_sources", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, SourceHeader, FromHeader("test").Source)
		require.Equal(t, SourceAuthHeader, FromAuthHeader("Bearer").Source)
		require.Equal(t, SourceAuthHeader, FromAuthHeader("").Source) // Empty scheme should still be SourceAuthHeader
		require.Equal(t, SourceForm, FromForm("test").Source)
		require.Equal(t, SourceQuery, FromQuery("test").Source)
		require.Equal(t, SourceParam, FromParam("test").Source)
		require.Equal(t, SourceCookie, FromCookie("test").Source)
		require.Equal(t, SourceCustom, FromCustom("test", func(_ fiber.Ctx) (string, error) { return "test", nil }).Source)
	})

	t.Run("chain_source_metadata", func(t *testing.T) {
		t.Parallel()
		chain := Chain(FromHeader("X-Test"), FromQuery("test"))
		require.Equal(t, SourceHeader, chain.Source)
		require.Equal(t, "X-Test", chain.Key)
	})
}

func Test_Extractor_URL_Encoded(t *testing.T) {
	t.Parallel()
	extractorsTestRun(t, []extractorsTestCase{
		// Should be URL-decoded automatically by fasthttp
		{name: "FromQuery_with_spaces", extractor: FromQuery("token"), uri: "/?token=token%20with%20spaces", want: "token with spaces"},
		// URL-decoded
		{name: "FromForm_with_plus", extractor: FromForm("token"), form: "token=token%2Bwith%2Bplus", want: "token+with+plus"},
		// URL-encoded base64 "cGFzc3dvcmQ=", which should be URL-decoded
		{name: "FromQuery_base64_encoded", extractor: FromQuery("token"), uri: "/?token=cGFzc3dvcmQ%3D", want: "cGFzc3dvcmQ="},
		{
			name:      "FromParam_with_slashes",
			extractor: FromParam("token"),
			route:     "/test/:token",
			uri:       "/test/token%2Fwith%2Fslashes",
			want:      "token/with/slashes",
		},
	})
}

func Test_isValidToken68(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "empty string", token: "", want: false},
		{name: "single uppercase", token: "A", want: true},
		{name: "single lowercase", token: "a", want: true},
		{name: "single digit", token: "0", want: true},
		{name: "all allowed symbols except =", token: "-._~+/", want: true},
		{name: "letters and digits", token: "token68", want: true},
		{name: "equals at end", token: "token=", want: true},
		{name: "multiple equals", token: "token==", want: true},
		{name: "equals at start", token: "=token", want: false},
		{name: "equals in middle", token: "tok=en", want: false},
		{name: "equals not at end with other chars", token: "token=extra", want: false},
		{name: "space in token", token: "token space", want: false},
		{name: "tab character in token", token: "token\ttab", want: false},
		{name: "invalid symbol", token: "token@", want: false},
		{token: "token68=", want: true, name: "valid token68 with equals at end"},
		{token: "token68==", want: true, name: "multiple equals at end"},
		{token: "token68=extra", want: false, name: "equals followed by extra chars"},
		{token: "T0ken-._~+/=", want: true, name: "all allowed chars with equals at end"},
		{token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P", want: true, name: "jwt-like long token"},
		{token: "abcdefgh", want: true, name: "8-byte token accepted"},
		{token: "abcdefg,", want: false, name: "trailing comma rejected"},
		{token: "abcdefghijklmno\x00", want: false, name: "NUL rejected"},
		{token: "abcdefgh\xc3\xa9", want: false, name: "non-ascii rejected"},
		{token: "abcdefghijklmnop====", want: true, name: "long token with padding run"},
		{token: "abcdefghijklmnop==x=", want: false, name: "base char inside padding run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isValidToken68(tc.token)
			if got != tc.want {
				t.Errorf("isValidToken68(%q) = %v, want %v", tc.token, got, tc.want)
			}
		})
	}
}

func Benchmark_isValidToken68(b *testing.B) {
	inputs := []string{
		benchToken,             // JWT-sized credential
		"dXNlcjpwYXNzd29yZA==", // short base64 credential
		"token@invalid",        // early reject
	}
	var got bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = isValidToken68(in)
		}
	}
	_ = got
}

// Test_Extractor_IgnoresHeaderNameCase pins that a value is found under the name
// it actually arrived as, for both header readers.
//
// FromHeader: Ctx.Get compares the stored key byte for byte, so under
// DisableHeaderNormalizing a token sent under the lower-case name that HTTP/2
// and HTTP/3 put on the wire was not found, and the request refused for
// carrying no token when it carried one.
//
// FromAuthHeader, which reaches every keyauth and bearer-token caller, was
// missed when FromHeader was fixed first: a lower-case "authorization:" read as
// absent, so the scheme check never ran and the request was refused for
// carrying no credential when it carried one.
func Test_Extractor_IgnoresHeaderNameCase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		value     string
		want      string
		sent      []string
		extractor Extractor
	}{
		{
			name:      "FromHeader",
			extractor: FromHeader("X-Csrf-Token"),
			sent:      []string{"X-Csrf-Token", "x-csrf-token", "X-CSRF-TOKEN"},
			value:     "the-token",
			want:      "the-token",
		},
		{
			name:      "FromAuthHeader",
			extractor: FromAuthHeader("Bearer"),
			sent:      []string{"Authorization", "authorization", "AUTHORIZATION"},
			value:     "Bearer the-token",
			want:      "the-token",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, normalize := range []bool{true, false} {
				t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
					t.Parallel()
					for _, sent := range tc.sent {
						t.Run(sent, func(t *testing.T) {
							t.Parallel()
							ctx := extractorsTestHeaderCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
							ctx.Request().Header.Set(sent, tc.value)

							extractorsTestExpect(t, tc.extractor, ctx, tc.want, "sent as %q", sent)
						})
					}
				})
			}
		})
	}
}

func Test_Resolve(t *testing.T) {
	t.Parallel()
	handRolled := extractorsTestFixed("X-Legacy", "legacy-value", nil)
	handRolled.Source = SourceHeader

	extractorsTestRunResolve(t, []extractorsTestCase{
		{
			name:      "builtin_cookie_source",
			extractor: FromCookie("session"),
			cookies:   map[string]string{"session": "cookie-value"},
			want:      "cookie-value",
			source:    SourceCookie,
		},
		{name: "falls_back_to_Extract_with_static_Source", extractor: handRolled, want: "legacy-value", source: SourceHeader},
		{name: "nil_both_returns_ErrNotFound", extractor: Extractor{Source: SourceCustom, Key: "empty"}, source: SourceCustom},
		// An Extract that succeeds and hands back nothing is a miss rather than a
		// value, and the miss reports the declared source.
		{
			name: "empty_value_is_a_miss",
			extractor: Extractor{
				Extract: func(fiber.Ctx) (string, error) { return "", nil },
				Source:  SourceForm,
				Key:     "empty",
			},
			source: SourceForm,
		},
	})

	t.Run("builtins_report_static_source_via_Resolve", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.Set("X-Token", "h")
		ctx.Request().Header.SetCookie("token", "c")
		ctx.Request().SetRequestURI("/?token=q")

		extractorsTestExpectResolved(t, FromHeader("X-Token"), ctx, "h", SourceHeader)
		extractorsTestExpectResolved(t, FromCookie("token"), ctx, "c", SourceCookie)
		extractorsTestExpectResolved(t, FromQuery("token"), ctx, "q", SourceQuery)
		require.NotNil(t, Chain(FromHeader("X-Token")).Extract)
		require.NotNil(t, Chain().Extract)
	})

	t.Run("unkeyed_literal_still_compiles_without_extra_fields", func(t *testing.T) {
		t.Parallel()
		// Positional form must match the public field set exactly. Keeping
		// source-aware extraction field-free preserves this shape.
		fn := func(_ fiber.Ctx) (string, error) { return "v", nil }
		e := Extractor{fn, "k", "", nil, SourceCustom}
		require.NotNil(t, e.Extract)
		require.Equal(t, "k", e.Key)
		require.Equal(t, SourceCustom, e.Source)
	})

	t.Run("honors_legacy_Extract_override", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.Set("X-Token", "raw-header")

		// Callers may decorate Extract for validation/normalization.
		// Resolve always uses Extract for leaves, so the override wins.
		e := extractorsTestDecorate(FromHeader("X-Token"),
			func(_ fiber.Ctx, v string) (string, error) { return "normalized:" + v, nil })

		extractorsTestExpectResolved(t, e, ctx, "normalized:raw-header", SourceHeader)
	})

	t.Run("honors_chain_level_Extract_override", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		// Only query is present. An undecorated chain would accept it; a
		// chain-level Extract override that rejects query-sourced values must
		// still win under Resolve.

		rejectQuery := errors.New("query tokens not allowed")
		chain := extractorsTestDecorate(Chain(FromHeader("X-Token"), FromQuery("token")),
			func(c fiber.Ctx, v string) (string, error) {
				// Simulate a decorator that only permits header-sourced credentials.
				// The value is present via query; reject it.
				if c.Query("token") == v {
					return "", rejectQuery
				}
				return v, nil
			})

		extractorsTestExpectErr(t, chain, ctx, rejectQuery)

		// SourceHeader is the static first-child metadata on error.
		extractorsTestExpectResolveErr(t, chain, ctx, SourceHeader, rejectQuery)
	})
}

func Test_Extractor_Chain_ExtractSource(t *testing.T) {
	t.Parallel()
	handRolled := extractorsTestFixed("form-key", "hand-rolled", nil)
	handRolled.Source = SourceForm

	extractorsTestRunResolve(t, []extractorsTestCase{
		{
			name:      "first_child_source_when_first_wins",
			extractor: Chain(FromHeader("X-Token"), FromQuery("token")),
			headers:   map[string]string{"X-Token": "from-header"},
			uri:       "/?token=from-query",
			want:      "from-header",
			source:    SourceHeader,
		},
		{name: "hand_rolled_in_chain", extractor: Chain(FromHeader("X-Token"), handRolled), want: "hand-rolled", source: SourceForm},
		{name: "empty_chain_source_path", extractor: Chain(), source: SourceCustom},
		{
			// Outer chain has one child: an inner chain whose first child misses
			// and second (query) wins. Must report SourceQuery, not the inner
			// chain's static first-child SourceCookie.
			name:      "nested_chain_reports_inner_winning_source",
			extractor: Chain(Chain(FromCookie("token"), FromQuery("token"))),
			uri:       "/?token=from-query",
			want:      "from-query",
			source:    SourceQuery,
		},
	})

	t.Run("reports_winning_child_source", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		// Header missing; query wins. Static chain Source stays SourceHeader,
		// but ExtractSource must report SourceQuery.

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		require.Equal(t, SourceHeader, chain.Source)

		extractorsTestExpectResolved(t, chain, ctx, "from-query", SourceQuery)
	})

	t.Run("skips_empty_children_preserves_prior_error", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		customErr := errors.New("custom failure")
		// Zero-value trailing child must not rewrite the chain error to ErrNotFound
		// under Resolve (Extract already skips nil Extract).
		chain := Chain(extractorsTestFixed("fail", "", customErr), Extractor{})

		extractorsTestExpectErr(t, chain, ctx, customErr)
		extractorsTestExpectResolveErr(t, chain, ctx, SourceCustom, customErr)
	})

	t.Run("shared_guard_allows_sequential_Extract_then_Resolve", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.SetCookie("token", "cookie-token")

		chain := Chain(FromHeader("X-Token"), FromCookie("token"))

		// Guard is cleared on return, so a later entry point on the same
		// request is not treated as a cycle.
		extractorsTestExpect(t, chain, ctx, "cookie-token")
		extractorsTestExpectResolved(t, chain, ctx, "cookie-token", SourceCookie)
	})

	t.Run("source_path_detects_cycle", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		var chainExtractor Extractor
		chainExtractor = Chain(FromCustom("cycle", func(c fiber.Ctx) (string, error) {
			// Re-enter via Resolve while Extract is already active.
			_, _, err := resolveSource(chainExtractor, c)
			return "", err
		}))

		// Shared guard must treat the cross-API re-entry as a cycle.
		extractorsTestExpectResolveErr(t, chainExtractor, ctx, SourceCustom, ErrChainCycle)
	})

	t.Run("cross_api_reentry_via_Extract_detects_cycle", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		// Enter via Extract while a nested child hits Resolve.
		var chainB Extractor
		chainB = Chain(FromCustom("to-source", func(c fiber.Ctx) (string, error) {
			_, _, err := resolveSource(chainB, c)
			return "", err
		}))
		extractorsTestExpectErr(t, chainB, ctx, ErrChainCycle)
	})

	t.Run("public_Chain_slice_mutation_does_not_affect_Extract", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		ctx.Request().Header.Set("X-Token", "from-header")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		require.Len(t, chain.Chain, 2)

		// Mutating the public introspection slice must not change which
		// children Extract runs (private execution list).
		chain.Chain[0] = FromQuery("token")
		chain.Chain[1] = FromHeader("X-Missing")

		extractorsTestExpect(t, chain, ctx, "from-header")

		// Value and source come from private kids via Extract capture, not
		// a second walk of the mutated public Chain.
		extractorsTestExpectResolved(t, chain, ctx, "from-header", SourceHeader)
	})

	t.Run("recursive_public_Chain_metadata_uses_captured_source", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		ctx.Request().Header.Set("X-Token", "from-header")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		// Corrupt introspection metadata after construction. Extract still
		// succeeds via the private kids list and records the winner so
		// Resolve does not need to walk the recursive public slice.
		chain.Chain[0] = chain

		extractorsTestExpect(t, chain, ctx, "from-header")
		extractorsTestExpectResolved(t, chain, ctx, "from-header", SourceHeader)
	})

	t.Run("replaced_Extract_without_base_uses_static_Source", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		ctx.Request().Header.SetCookie("session", "cookie-value")

		// Full Extract replacement (does not call base): must not peek Chain
		// children and mis-tag the replacement value as SourceQuery.
		chain := Chain(FromHeader("X-Missing"), FromQuery("token"))
		chain.Extract = func(c fiber.Ctx) (string, error) { return FromCookie("session").Extract(c) }

		// SourceHeader is the static first-child metadata.
		extractorsTestExpectResolved(t, chain, ctx, "cookie-value", SourceHeader)
	})

	t.Run("shared_guard_blocks_cross_entry_during_source_path", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)

		// First child re-enters the outer chain via Extract while the chain's
		// Extract is active; second child would succeed if the cycle were ignored.
		var chain Extractor
		chain = Chain(
			FromCustom("reenter", func(c fiber.Ctx) (string, error) { return chain.Extract(c) }),
			FromQuery("token"),
		)

		// Must not mis-attribute the query value to the custom re-entry child.
		extractorsTestExpectResolved(t, chain, ctx, "from-query", SourceQuery)
	})

	t.Run("no_double_extract_of_stateful_custom_child", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		calls := 0
		chain := Chain(
			FromCustom("once", func(_ fiber.Ctx) (string, error) {
				calls++
				if calls == 1 {
					return "only-once", nil
				}
				return "", ErrNotFound
			}),
			FromQuery("token"),
		)

		extractorsTestExpectResolved(t, chain, ctx, "only-once", SourceCustom)
		require.Equal(t, 1, calls, "stateful child must not run again during source resolution")
	})

	t.Run("clearing_public_Chain_makes_provenance_unresolved", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		// The public Chain is how a record is tied back to the chain that made
		// it. Clearing it loses that identity, so the value still resolves but
		// its provenance cannot be vouched for. Unresolved is the safe
		// degradation; honoring any record in the frame is what let an
		// unrelated chain's winner be credited to this one.
		chain.Chain = nil

		res, err := Resolve(chain, ctx)
		require.NoError(t, err)
		require.Equal(t, "from-query", res.Value)
		require.False(t, res.Resolved, "a record that cannot be tied to this chain is not provenance")
	})

	t.Run("bare_Extract_does_not_pollute_later_Resolve", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		ctx.Request().Header.SetCookie("session", "cookie-value")

		// Legacy path: Extract alone must not leave a winner that a later
		// source-aware call on an unrelated leaf would mis-attribute.
		chain := Chain(FromHeader("X-Missing"), FromQuery("token"))
		extractorsTestExpect(t, chain, ctx, "from-query")

		extractorsTestExpectResolved(t, FromCookie("session"), ctx, "cookie-value", SourceCookie)
	})

	t.Run("rejected_nested_chain_does_not_poison_fallback_source", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)
		ctx.Request().Header.SetCookie("session", "cookie-value")

		// Nested chain would win via cookie, but a decorator rejects after
		// base Extract. Outer fallback is query — source must be SourceQuery,
		// not the rejected nested cookie capture.
		inner := extractorsTestDecorate(Chain(FromCookie("session"), FromHeader("X-Missing")),
			func(fiber.Ctx, string) (string, error) { return "", errors.New("cookie rejected") })
		outer := Chain(inner, FromQuery("token"))

		extractorsTestExpectResolved(t, outer, ctx, "from-query", SourceQuery)
	})

	t.Run("chain_override_success_still_reports_winning_source", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)

		chain := extractorsTestDecorate(Chain(FromHeader("X-Token"), FromQuery("token")),
			func(_ fiber.Ctx, v string) (string, error) { return "normalized:" + v, nil })

		extractorsTestExpectResolved(t, chain, ctx, "normalized:from-query", SourceQuery)
	})
}

// Test_FromParam_DecodesOnce pins that a parameter is percent-decoded exactly
// once whatever UnescapePath is set to. With UnescapePath enabled the router
// already decoded the path, so a second decode here turned "%2520" into a
// space instead of the "%20" the client sent.
func Test_FromParam_DecodesOnce(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		unescapePath bool
	}{
		{name: "UnescapePath disabled", unescapePath: false},
		{name: "UnescapePath enabled", unescapePath: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{UnescapePath: tc.unescapePath})
			token, err := extractorsTestParam(t, app, "/test/:token", "/test/%2520", FromParam("token"))
			require.NoError(t, err)
			// The client sent "%2520", which stands for the literal
			// four-character string "%20".
			require.Equal(t, "%20", token)
		})
	}
}

// Test_Resolve_BareChain covers an Extractor built with only its
// Chain set and no Extract func, the one shape that reaches
// resolveChain directly: the winner's own source is reported, the
// cycle guard is released when the walk ends so the same ctx can be walked
// again, and a chain that contains itself is still refused.
func Test_Resolve_BareChain(t *testing.T) {
	t.Parallel()
	// Children with nothing to run are stepped over, and a chain that ran
	// nothing reports ErrNotFound rather than a child's error.
	headers := map[string]string{"X-Token": "from-header"}
	extractorsTestRunResolve(t, []extractorsTestCase{
		{
			name:      "every child is skipped",
			extractor: Extractor{Chain: []Extractor{{}, {Key: "no extract, no chain"}}, Source: SourceCookie},
			headers:   headers,
			source:    SourceCookie,
		},
		{
			name:      "a skipped child does not stop the walk",
			extractor: Extractor{Chain: []Extractor{{}, FromHeader("X-Token")}, Source: SourceCookie},
			headers:   headers,
			want:      "from-header",
			source:    SourceHeader,
		},
	})

	t.Run("winner reports its own source", func(t *testing.T) {
		t.Parallel()
		ctx := extractorsTestQueryCtx(t)

		bare := Extractor{Chain: []Extractor{FromHeader("X-Token"), FromQuery("token")}}

		extractorsTestExpectResolved(t, bare, ctx, "from-query", SourceQuery)

		// The guard set for the walk must be cleared afterwards, or the next
		// walk on this ctx would be mistaken for a cycle.
		require.NotContains(t, chainStateFor(ctx).active, chainGuard(bare.Chain))

		extractorsTestExpectResolved(t, bare, ctx, "from-query", SourceQuery)
	})

	t.Run("self-referencing chain is refused", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		bare := Extractor{Chain: make([]Extractor, 1)}
		bare.Chain[0] = bare

		_, _, err := resolveSource(bare, ctx)
		require.ErrorIs(t, err, ErrChainCycle)

		require.NotContains(t, chainStateFor(ctx).active, chainGuard(bare.Chain), "the guard is released even after a refused walk")
	})
}

// The state is reclaimed by the request store calling Close on it.
var _ io.Closer = (*chainState)(nil)

// closeRecorder reports whether the request store closed it. The contract is
// pinned here rather than through the chain state, whose pointer is back in
// the pool by the time a test could look at it.
type closeRecorder struct {
	closed *atomic.Bool
}

// Close records the call fasthttp makes when it resets the request.
func (r closeRecorder) Close() error {
	r.closed.Store(true)
	return nil
}

// Test_Chain_StateIsRecycledOnRequestReset pins the recycling: fasthttp closes
// request-local io.Closers on reset, the state is stored where that happens,
// and one handed out afterwards is clean.
func Test_Chain_StateIsRecycledOnRequestReset(t *testing.T) {
	t.Parallel()
	t.Run("request store closes values on reset", func(t *testing.T) {
		t.Parallel()
		var closed atomic.Bool
		fctx := &fasthttp.RequestCtx{}
		fctx.SetUserValue("recorder", closeRecorder{closed: &closed})
		require.False(t, closed.Load())

		fctx.Request.Reset()
		require.True(t, closed.Load(), "fasthttp must close request-local values that implement io.Closer")
	})

	t.Run("chain state lives in the request store and is dropped on reset", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		app.Get("/t", func(c fiber.Ctx) error {
			extractorsTestExpect(t, chain, c, "tok")
			// Dirty it, so a stale state would show up in the next holder.
			st := chainStateFor(c)
			st.rec.win = &Extractor{Source: SourceQuery}
			st.rec.guard = new(byte)
			return c.SendStatus(fiber.StatusNoContent)
		})

		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/t")
		fctx.Request.Header.Set("X-Token", "tok")
		app.Handler()(fctx)
		require.Equal(t, fiber.StatusNoContent, fctx.Response.StatusCode())

		_, ok := fctx.UserValue(chainStateKey{}).(*chainState)
		require.True(t, ok, "the state must be stored in the request, which is what gets closed")

		fctx.Request.Reset()
		require.Nil(t, fctx.UserValue(chainStateKey{}), "the reset must drop the state")
	})

	t.Run("a state handed out is clean", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())

		// Whether this state is new or recycled, it must carry nothing from
		// the request that used it before.
		st := chainStateFor(ctx)
		require.Empty(t, st.active)
		require.Zero(t, st.depth)
		require.Nil(t, st.rec.win)
		require.Same(t, st, chainStateFor(ctx), "one state per request")
	})
}

// Test_Chain_NoAllocations guards the pooled state: the source-aware path
// allocated a []Source per winning child before it existed.
//
// Not parallel: AllocsPerRun counts allocations process-wide, and Go pauses
// parallel tests while a sequential one runs.
func Test_Chain_NoAllocations(t *testing.T) {
	ctx := extractorsTestQueryCtx(t)
	ctx.Request().Header.Set("X-Token", "from-header")

	flat := Chain(FromCookie("token"), FromQuery("token"))
	nested := Chain(FromHeader("X-Missing"), Chain(FromCookie("token"), FromQuery("token")))

	cases := []struct {
		run  func()
		name string
	}{
		{name: "Extract", run: func() {
			if _, err := flat.Extract(ctx); err != nil {
				t.Error(err)
			}
		}},
		{name: "Resolve", run: func() {
			if _, _, err := resolveSource(flat, ctx); err != nil {
				t.Error(err)
			}
		}},
		{name: "Resolve_nested", run: func() {
			if _, _, err := resolveSource(nested, ctx); err != nil {
				t.Error(err)
			}
		}},
		{name: "Resolve_bare_chain", run: func() {
			bare := Extractor{Chain: flat.Chain}
			if _, _, err := resolveSource(bare, ctx); err != nil {
				t.Error(err)
			}
		}},
	}

	for _, tc := range cases {
		tc.run() // warm the state into the request store
		require.Zero(t, testing.AllocsPerRun(100, tc.run), "%s must not allocate", tc.name)
	}
}

// countingCtx makes the Locals override observable.
type countingCtx struct {
	fiber.DefaultCtx
	calls atomic.Int64
}

// Locals counts each call before forwarding it.
func (c *countingCtx) Locals(key any, value ...any) any {
	c.calls.Add(1)
	return c.DefaultCtx.Locals(key, value...)
}

// Test_Chain_CustomCtx keeps the chain state reachable through a context that
// overrides Locals. Such a store may never close the state, which costs an
// allocation but must not change any answer.
func Test_Chain_CustomCtx(t *testing.T) {
	t.Parallel()
	ctx := apptest.Ctx(t, fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &countingCtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
	}))
	ctx.Request().SetRequestURI("/?token=from-query")

	custom, ok := ctx.(*countingCtx)
	require.True(t, ok, "the app must hand out the custom context")

	chain := Chain(FromHeader("X-Token"), FromQuery("token"))

	extractorsTestExpect(t, chain, ctx, "from-query")

	// Sequential reuse: the guard is released, so the second walk is not a cycle.
	extractorsTestExpectResolved(t, chain, ctx, "from-query", SourceQuery)

	extractorsTestExpectErr(t, extractorsTestCyclic(), ctx, ErrChainCycle)

	require.Positive(t, custom.calls.Load(), "the overridden Locals must be the one that ran")
}

// Test_Chain_Concurrent runs chains on many requests at once, so the race
// detector sees the pooled state cross goroutines.
func Test_Chain_Concurrent(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	chain := Chain(FromHeader("X-Token"), FromQuery("token"))
	cyclic := extractorsTestCyclic()

	const workers = 64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range 50 {
				fctx := &fasthttp.RequestCtx{}
				fctx.Request.SetRequestURI("/?token=from-query")
				c := app.AcquireCtx(fctx)

				v, src, err := resolveSource(chain, c)
				if assert.NoError(t, err) {
					assert.Equal(t, "from-query", v)
					assert.Equal(t, SourceQuery, src)
				}

				_, err = cyclic.Extract(c)
				assert.ErrorIs(t, err, ErrChainCycle) //nolint:testifylint // we can't use require in goroutines

				app.ReleaseCtx(c)
				// What the server does between requests.
				fctx.Request.Reset()
			}
		})
	}
	wg.Wait()
}

// token68Samples are the shapes the table and the scan it replaced could most
// easily disagree about. The fuzz target seeds its corpus with them.
var token68Samples = []string{
	"", "=", "==", "===", "=a", "a", "a=", "a==", "a===", "a=b", "a==b",
	"dXNlcjpwYXNzd29yZA==", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0",
	benchToken, "-._~+/", "a b", "a\tb", "a\nb", "token@invalid",
}

// isValidToken68Reference is the range-compare scan the table replaced, kept
// as the definition it is checked against.
func isValidToken68Reference(token string) bool {
	if token == "" {
		return false
	}
	paddingStarted := false
	for i := 0; i < len(token); i++ {
		c := token[i]
		switch {
		case (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' || c == '+' || c == '/':
			if paddingStarted {
				return false // No characters allowed after padding starts
			}
		case c == '=':
			if i == 0 {
				return false // Cannot start with padding
			}
			paddingStarted = true
		default:
			return false // Invalid character
		}
	}
	return true
}

// Test_isValidToken68_MatchesReference checks the table against the scan it
// replaced, on every byte in every position that could distinguish them.
func Test_isValidToken68_MatchesReference(t *testing.T) {
	t.Parallel()
	t.Run("every byte in every position", func(t *testing.T) {
		t.Parallel()
		for c := range 256 {
			// From the byte, not rune(c): the high half must be one invalid
			// byte, not two UTF-8 ones.
			b := string([]byte{byte(c)})
			for _, in := range []string{
				b,             // alone
				b + "aA",      // at the start
				"a" + b + "A", // in the middle
				"aA" + b,      // at the end
				"aA" + b + "==",
				"aA==" + b,
			} {
				require.Equal(t, isValidToken68Reference(in), isValidToken68(in), "input %q", in)
			}
		}
	})

	t.Run("padding shapes", func(t *testing.T) {
		t.Parallel()
		for _, in := range token68Samples {
			require.Equal(t, isValidToken68Reference(in), isValidToken68(in), "input %q", in)
		}
	})
}

// Test_Extractor_FromAuthHeader_SchemeWithoutCredential covers a header that is
// the scheme and its separating space and nothing else, which is a scheme
// naming no credential rather than a credential that happens to be empty.
func Test_Extractor_FromAuthHeader_SchemeWithoutCredential(t *testing.T) {
	t.Parallel()
	ctx := apptest.Ctx(t, fiber.New())
	// Set rather than read from the wire: parsing strips the trailing space,
	// so only a field written in process carries one. Middleware rewriting
	// Authorization writes this shape.
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer ")
	require.Equal(t, "Bearer ", string(ctx.Request().Header.Peek(fiber.HeaderAuthorization)))

	extractorsTestExpectErr(t, FromAuthHeader("Bearer"), ctx, ErrNotFound)
}

// Test_FromParam_RefusesUndecodableValue covers a parameter the router left
// raw and that percent-decoding cannot make sense of.
func Test_FromParam_RefusesUndecodableValue(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	extractor := FromParam("id")
	ran := false
	app.Get("/users/:id", func(c fiber.Ctx) error {
		ran = true
		extractorsTestExpectErr(t, extractor, c, ErrNotFound)
		return c.SendStatus(fiber.StatusNoContent)
	})

	// Driven through the handler rather than app.Test, whose URL parsing
	// refuses the escape before the router ever sees it.
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/users/%zz")
	app.Handler()(fctx)

	require.Equal(t, fiber.StatusNoContent, fctx.Response.StatusCode())
	require.True(t, ran, "the route must have matched")
}

// Test_ChainState_PoolHoldsSomethingElse covers the guard on what the pool
// hands back. sync.Pool is typed as any, so a state is rebuilt rather than
// trusted; this drives that branch by putting sentinels in front of it.
//
// Not parallel: it borrows the shared pool, and it takes back out everything
// it puts in.
func Test_ChainState_PoolHoldsSomethingElse(t *testing.T) {
	app := fiber.New()

	const sentinels = 8
	for range sentinels {
		// Pointer-like, so parking it in the pool does not allocate.
		chainStatePool.Put(new(struct{}))
	}

	for range sentinels {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		st := chainStateFor(ctx)
		require.NotNil(t, st, "a usable state whatever the pool held")
		require.Empty(t, st.active)
		require.Zero(t, st.depth)
		require.Nil(t, st.rec.win)
		app.ReleaseCtx(ctx)
	}
}

// Test_Resolve_NestedCapturePreservesWinner covers a decorated chain
// that calls its base and then makes another source-aware call before
// returning the base's value. The nested frame must not consume the winner the
// base recorded, or the value is reported against the decorator's declared
// source instead of the child that supplied it.
func Test_Resolve_NestedCapturePreservesWinner(t *testing.T) {
	t.Parallel()
	ctx := extractorsTestQueryCtx(t)
	ctx.Request().Header.SetCookie("other", "cookie-value")

	chain := extractorsTestDecorate(Chain(FromHeader("X-Missing"), FromQuery("token")),
		func(c fiber.Ctx, v string) (string, error) {
			// An unrelated source-aware lookup, after the winner was recorded.
			if _, _, err := resolveSource(FromCookie("other"), c); err != nil {
				return "", err
			}
			return v, nil
		})

	extractorsTestExpectResolved(t, chain, ctx, "from-query", SourceQuery, "the query child supplied the value")
}

// Test_ChainState_CloseClearsGuards covers the guards a finished request leaves
// in the buffer past its length: a retained one keeps a chain's Extractor array
// alive for as long as the pool holds the state.
func Test_ChainState_CloseClearsGuards(t *testing.T) {
	t.Parallel()
	st := &chainState{active: make([]*byte, 0, 4)}
	first := Chain(FromHeader("X-One")).Chain
	second := Chain(FromHeader("X-Two")).Chain
	require.True(t, st.enter(chainGuard(first)))
	require.True(t, st.enter(chainGuard(second)))
	st.leave()
	st.leave()
	require.Empty(t, st.active, "both are released")

	buffer := st.active[:cap(st.active)]
	require.NotNil(t, buffer[0], "released guards are still in the buffer")

	// reset rather than Close: Close hands the state to the pool, where another
	// request may take it and write the very buffer this reads.
	st.reset()
	for i, guard := range buffer {
		require.Nil(t, guard, "guard %d must not outlive the request", i)
	}
}

// Test_Resolve_Winner covers what Result adds over a Source alone: the Key of
// the extractor that supplied the value, which a caller writing it back needs.
func Test_Resolve_Winner(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	newCtx := func(t *testing.T, build func(*fasthttp.RequestCtx)) fiber.Ctx {
		t.Helper()

		fctx := &fasthttp.RequestCtx{}
		if build != nil {
			build(fctx)
		}
		return apptest.CtxFor(t, app, fctx)
	}

	t.Run("leaf reports its own key and source", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, func(f *fasthttp.RequestCtx) { f.Request.Header.SetCookie("sid", "v") })

		extractorsTestExpectResult(t, FromCookie("sid"), ctx,
			Result{Value: "v", Key: "sid", Source: SourceCookie, Resolved: true})
	})

	t.Run("chain reports the winning child, not the first", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, func(f *fasthttp.RequestCtx) { f.Request.Header.Set("X-Sid", "v") })

		extractorsTestExpectResult(t, Chain(FromCookie("sid"), FromHeader("X-Sid")), ctx,
			Result{Value: "v", Key: "X-Sid", Source: SourceHeader, Resolved: true})
	})

	t.Run("nested chain reports the innermost winner", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, func(f *fasthttp.RequestCtx) { f.Request.SetRequestURI("/p?tok=v") })

		inner := Chain(FromHeader("X-Nope"), FromQuery("tok"))
		extractorsTestExpectResult(t, Chain(FromCookie("sid"), inner), ctx,
			Result{Value: "v", Key: "tok", Source: SourceQuery, Resolved: true})
	})

	t.Run("a decorated chain reports the child that answered", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, func(f *fasthttp.RequestCtx) { f.Request.Header.Set("X-Sid", "v") })

		base := Chain(FromCookie("sid"), FromHeader("X-Sid"))
		decorated := base
		decorated.Extract = func(c fiber.Ctx) (string, error) { return base.Extract(c) }

		extractorsTestExpectResult(t, decorated, ctx,
			Result{Value: "v", Key: "X-Sid", Source: SourceHeader, Resolved: true})
	})

	t.Run("an Extract that replaces the chain reports the declared metadata", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, nil)

		replaced := Chain(FromCookie("sid"), FromHeader("X-Sid"))
		replaced.Extract = func(fiber.Ctx) (string, error) { return "made-up", nil }

		res, err := Resolve(replaced, ctx)
		require.NoError(t, err)
		require.Equal(t, "made-up", res.Value)
		// No child won, so the declared metadata stands rather than crediting
		// whichever child would answer on a second walk — and it is unresolved,
		// because it names the first child rather than wherever "made-up" came
		// from.
		require.False(t, res.Resolved)
		require.Equal(t, "sid", res.Key)
		require.Equal(t, SourceCookie, res.Source)
	})

	t.Run("a failed resolve reports no value", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, nil)

		res, err := Resolve(Chain(FromCookie("sid"), FromHeader("X-Sid")), ctx)
		require.Error(t, err)
		require.Empty(t, res.Value)
	})

	t.Run("a child that runs a foreign chain is not credited to it", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, func(f *fasthttp.RequestCtx) {
			f.Request.SetRequestURI("/p?sid=planted")
			f.Request.Header.SetCookie("tenant", "acme")
		})

		tenant := Chain(FromHeader("X-Tenant"), FromCookie("tenant"))
		child := FromCustom("sid", func(c fiber.Ctx) (string, error) {
			//nolint:errcheck // the lookup's outcome is irrelevant; it runs only so a foreign chain records
			tenant.Extract(c)
			return fiber.Query[string](c, "sid"), nil
		})

		res, err := Resolve(Chain(child, FromCookie("sid_cookie")), ctx)
		require.NoError(t, err)
		require.Equal(t, "planted", res.Value)
		require.NotEqual(t, "tenant", res.Key,
			"a record made by a chain the child merely ran must not become this chain's provenance")
		require.False(t, res.Resolved, "with no origin we can vouch for, the provenance must be unresolved")
	})

	t.Run("a chain that re-enters itself is reported as a cycle", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx(t, nil)

		res, err := Resolve(extractorsTestCyclic(), ctx)
		require.ErrorIs(t, err, ErrChainCycle)
		require.Empty(t, res.Value)
	})
}

// Test_Extractor_Walk covers the traversal Contains and both middleware share:
// chain order, descent into nested chains, early stop, and cycle safety.
func Test_Extractor_Walk(t *testing.T) {
	t.Parallel()
	keys := func(e Extractor) []string {
		var seen []string
		e.Walk(func(candidate Extractor) bool {
			seen = append(seen, candidate.Key)
			return true
		})
		return seen
	}

	t.Run("visits the root then children in chain order", func(t *testing.T) {
		t.Parallel()
		e := Chain(FromHeader("a"), FromQuery("b"), FromCookie("c"))
		// The root reports its first child's Key as declared metadata.
		require.Equal(t, []string{"a", "a", "b", "c"}, keys(e))
	})

	t.Run("descends into a nested chain", func(t *testing.T) {
		t.Parallel()
		inner := Chain(FromQuery("q"), FromCookie("deep"))
		e := Chain(FromHeader("h"), inner)
		require.Contains(t, keys(e), "deep", "a sink one level down must be reachable")
	})

	t.Run("stops early when fn returns false", func(t *testing.T) {
		t.Parallel()
		e := Chain(FromHeader("a"), FromQuery("b"), FromCookie("c"))
		visits := 0
		e.Walk(func(Extractor) bool {
			visits++
			return visits < 2
		})
		require.Equal(t, 2, visits)
	})

	t.Run("a self-referential chain terminates", func(t *testing.T) {
		t.Parallel()
		e := Chain(FromHeader("a"), FromQuery("b"))
		// Point a child's chain back at the parent's children.
		e.Chain[1].Chain = e.Chain

		visits := 0
		e.Walk(func(Extractor) bool {
			visits++
			return true
		})
		require.Less(t, visits, 16, "the cycle guard must stop the walk")
	})

	t.Run("a nil fn is a no-op", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { Chain(FromHeader("a")).Walk(nil) })
	})
}
