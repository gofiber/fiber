package binder

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_CookieBinder_Bind_FirstWins pins that a repeated cookie name binds its first value, the one Ctx.Cookies returns.
func Test_CookieBinder_Bind_FirstWins(t *testing.T) {
	t.Parallel()
	req := acquireRequest(t)
	req.Header.Set(fasthttp.HeaderCookie, "tok=a; ids=1,2; tok=b; ids=3")
	b := &CookieBinding{EnableSplitting: true}

	var out struct {
		Tok string   `cookie:"tok"`
		IDs []string `cookie:"ids"`
	}
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, "a", out.Tok)
	require.Equal(t, []string{"1", "2"}, out.IDs)

	last := map[string]string{}
	require.NoError(t, b.Bind(req, &last))
	require.Equal(t, map[string]string{"tok": "a", "ids": "1,2"}, last)

	all := map[string][]string{}
	require.NoError(t, b.Bind(req, &all))
	require.Equal(t, map[string][]string{"tok": {"a"}, "ids": {"1", "2"}}, all)

	// Past the names firstOf scans, the map takes over and keeps the first value as well.
	req = acquireRequest(t)
	req.Header.Set(fasthttp.HeaderCookie, "c1=1; c2=2; c3=3; c4=4; c5=5; c6=6; c7=7; c8=8; c9=9; c2=x; c10=10; c9=y")
	many := map[string]string{}
	require.NoError(t, b.Bind(req, &many))
	require.Equal(t, map[string]string{
		"c1": "1", "c2": "2", "c3": "3", "c4": "4", "c5": "5", "c6": "6", "c7": "7", "c8": "8", "c9": "9", "c10": "10",
	}, many)
}

// Test_CookieBinder_Bind_ExactName pins that a tagged field takes the cookie c.Cookies returns for its tag: a name
// that matches only after folding case, or reaches a promoted field through its embedded struct, is not bound.
func Test_CookieBinder_Bind_ExactName(t *testing.T) {
	t.Parallel()
	type CookieAuth struct {
		Host string `cookie:"__Host-token"`
	}
	type CookieInner struct {
		Deep string `cookie:"deep"`
	}
	// Auth is shadowed by the field tagged auth and must not block that field's nested names.
	type Auth struct {
		Other string `cookie:"other"`
	}
	type CookieMid struct {
		CookieInner
		Auth
	}
	type session struct {
		Meta map[string]string `cookie:"meta"`
		CookieAuth
		CookieMid
		Auth  CookieInner `cookie:"auth"`
		Token string      `cookie:"access_token"`
		Name  string
		IDs   []string      `cookie:"ids"`
		Items []CookieInner `cookie:"items"`
	}
	host, deep := session{CookieAuth: CookieAuth{Host: "a"}}, session{CookieMid: CookieMid{CookieInner: CookieInner{"a"}}}
	nested, item := session{Auth: CookieInner{"a"}}, session{Items: []CookieInner{{"a"}}}
	for _, tc := range []struct {
		name, cookie string
		want         session
	}{
		{name: "exact first", cookie: "access_token=mallory; Access_Token=alice", want: session{Token: "mallory"}},
		{name: "folded first", cookie: "Access_Token=alice; access_token=mallory", want: session{Token: "mallory"}},
		{name: "folded alone", cookie: "ACCESS_TOKEN=alice"},
		{name: "path forms", cookie: ".access_token=a; Access_Token.0=a; access_token[0]=a; [access_token]=a"},
		{name: "promoted, exact first", cookie: "__Host-token=a; __host-token=b", want: host},
		{name: "promoted, folded first", cookie: "__host-token=b; __Host-token=a", want: host},
		{name: "promoted through its struct", cookie: "CookieAuth.__Host-token=a; cookieauth[__Host-token]=a"},
		{name: "two levels down", cookie: "deep=a; CookieInner.deep=b", want: deep},
		{name: "nested, exact first", cookie: "auth.deep=a; Auth.deep=b; auth.DEEP=b", want: nested},
		{name: "nested, folded first", cookie: "Auth.deep=b; auth.DEEP=b; auth.deep=a", want: nested},
		{name: "nested, brackets", cookie: "auth[DEEP]=b; auth.deep=a; auth[deep]=b", want: nested},
		{name: "nested in a slice", cookie: "items.0.deep=a; Items.0.deep=b; items.0.DEEP=b", want: item},
		{name: "index spellings", cookie: "items.0.deep=a; items.00.deep=b; items.+0.deep=b", want: item},
		{name: "map field", cookie: "meta.k=a; META.k=b; Meta.k=b", want: session{Meta: map[string]string{"k": "a"}}},
		{name: "slice", cookie: "IDS=1; ids=2", want: session{IDs: []string{"2"}}},
		{name: "untagged field folds", cookie: "name=john", want: session{Name: "john"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := acquireRequest(t)
			req.Header.Set(fasthttp.HeaderCookie, tc.cookie)
			var out session
			require.NoError(t, (&CookieBinding{}).Bind(req, &out))
			require.Equal(t, tc.want, out)
		})
	}
}

// Test_CookieBinder_Bind_SameAsWalk pins that the names a bind looks up instead of walking them bind as a walk of
// every name, keeping the first of each, binds them.
func Test_CookieBinder_Bind_SameAsWalk(t *testing.T) {
	t.Parallel()
	type inner struct {
		Deep string `cookie:"deep"`
	}
	type closed struct {
		Token string   `cookie:"access_token"`
		IDs   []string `cookie:"ids"`
		N     int      `cookie:"n"`
	}
	type open struct {
		Token string `cookie:"access_token"`
		Name  string
	}
	type nested struct {
		Meta map[string]string `cookie:"meta"`
		inner
		Auth  inner   `cookie:"auth"`
		Items []inner `cookie:"items"`
	}
	wide := make([]reflect.StructField, 70)
	for i := range wide {
		wide[i] = reflect.StructField{
			Name: fmt.Sprintf("F%d", i), Type: reflect.TypeFor[string](), Tag: reflect.StructTag(fmt.Sprintf(`cookie:"f%d"`, i)),
		}
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[closed](), reflect.TypeFor[open](), reflect.TypeFor[nested](), reflect.StructOf(wide),
	} {
		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()
			names := cookieFieldCache.cookieNames(typ)
			var all []string
			for _, name := range append(slices.Clone(names.exact), "inner", "x", "a.b") {
				for _, suffix := range []string{"", ".0", ".00", ".x", ".0.deep", ".0.DEEP", ".k"} {
					v := name + suffix
					// Another name of the same nameBit, and the name in other cases.
					twin := v[:len(v)-1] + "_"
					variants := []string{v, strings.ToUpper(v), strings.ToUpper(v[:1]) + v[1:], twin}
					all = append(all, variants...)
					for _, a := range variants {
						for _, b := range variants {
							requireSameAsWalk(t, typ, a+"=1; "+b+"=2")
						}
					}
				}
			}
			requireSameAsWalk(t, typ, strings.Join(all, "=1; ")+"=1")
		})
	}
}

// requireSameAsWalk binds the cookie header into a new value of typ and requires the result of a walk of every name.
func requireSameAsWalk(t *testing.T, typ reflect.Type, header string) {
	t.Helper()
	req := acquireRequest(t)
	req.Header.Set(fasthttp.HeaderCookie, header)
	got, want := reflect.New(typ), reflect.New(typ)
	err := (&CookieBinding{}).Bind(req, got.Interface())

	data := acquireBindDataMode(bindPairs)
	defer releaseBindData(data)
	info, ok := cookieFieldCache.get(typ, bindingCookie)
	require.True(t, ok)
	for key, val := range req.Header.Cookies() {
		if cookieFieldCache.takesCookie(info, string(key)) && data.firstOf(string(key)) {
			require.NoError(t, data.bind(bindingCookie, want.Interface(), string(key), string(val), false, false))
		}
	}
	require.Equal(t, data.parse(bindingCookie, want.Interface()) == nil, err == nil, header)
	require.Equal(t, want.Interface(), got.Interface(), header)
}

func Benchmark_CookieBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &CookieBinding{EnableSplitting: true}

	var user benchUser

	req := acquireRequest(b)
	req.Header.SetCookie("name", "john")
	req.Header.SetCookie("age", "42")
	req.Header.SetCookie("posts", "post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
