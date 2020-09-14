package json

import (
	"reflect"
	"testing"
)

type token struct {
	delim Delim
	value RawValue
	err   error
	depth int
	index int
	isKey bool
}

func delim(s string, depth, index int) token {
	return token{
		delim: Delim(s[0]),
		value: RawValue(s),
		depth: depth,
		index: index,
	}
}

func key(v string, depth, index int) token {
	return token{
		value: RawValue(v),
		depth: depth,
		index: index,
		isKey: true,
	}
}

func value(v string, depth, index int) token {
	return token{
		value: RawValue(v),
		depth: depth,
		index: index,
	}
}

func tokenize(b []byte) (tokens []token) {
	t := NewTokenizer(b)

	for t.Next() {
		tokens = append(tokens, token{
			delim: t.Delim,
			value: t.Value,
			err:   t.Err,
			depth: t.Depth,
			index: t.Index,
			isKey: t.IsKey,
		})
	}

	if t.Err != nil {
		panic(t.Err)
	}

	return
}

func TestTokenizer(t *testing.T) {
	tests := []struct {
		input  []byte
		tokens []token
	}{
		{
			input: []byte(`null`),
			tokens: []token{
				value(`null`, 0, 0),
			},
		},

		{
			input: []byte(`true`),
			tokens: []token{
				value(`true`, 0, 0),
			},
		},

		{
			input: []byte(`false`),
			tokens: []token{
				value(`false`, 0, 0),
			},
		},

		{
			input: []byte(`""`),
			tokens: []token{
				value(`""`, 0, 0),
			},
		},

		{
			input: []byte(`"Hello World!"`),
			tokens: []token{
				value(`"Hello World!"`, 0, 0),
			},
		},

		{
			input: []byte(`-0.1234`),
			tokens: []token{
				value(`-0.1234`, 0, 0),
			},
		},

		{
			input: []byte(` { } `),
			tokens: []token{
				delim(`{`, 0, 0),
				delim(`}`, 0, 0),
			},
		},

		{
			input: []byte(`{ "answer": 42 }`),
			tokens: []token{
				delim(`{`, 0, 0),
				key(`"answer"`, 1, 0),
				delim(`:`, 1, 0),
				value(`42`, 1, 0),
				delim(`}`, 0, 0),
			},
		},

		{
			input: []byte(`{ "sub": { "key-A": 1, "key-B": 2, "key-C": 3 } }`),
			tokens: []token{
				delim(`{`, 0, 0),
				key(`"sub"`, 1, 0),
				delim(`:`, 1, 0),
				delim(`{`, 1, 0),
				key(`"key-A"`, 2, 0),
				delim(`:`, 2, 0),
				value(`1`, 2, 0),
				delim(`,`, 2, 0),
				key(`"key-B"`, 2, 1),
				delim(`:`, 2, 1),
				value(`2`, 2, 1),
				delim(`,`, 2, 1),
				key(`"key-C"`, 2, 2),
				delim(`:`, 2, 2),
				value(`3`, 2, 2),
				delim(`}`, 1, 0),
				delim(`}`, 0, 0),
			},
		},

		{
			input: []byte(` [ ] `),
			tokens: []token{
				delim(`[`, 0, 0),
				delim(`]`, 0, 0),
			},
		},

		{
			input: []byte(`[1, 2, 3]`),
			tokens: []token{
				delim(`[`, 0, 0),
				value(`1`, 1, 0),
				delim(`,`, 1, 0),
				value(`2`, 1, 1),
				delim(`,`, 1, 1),
				value(`3`, 1, 2),
				delim(`]`, 0, 0),
			},
		},
	}

	for _, test := range tests {
		t.Run(string(test.input), func(t *testing.T) {
			tokens := tokenize(test.input)

			if !reflect.DeepEqual(tokens, test.tokens) {
				t.Error("tokens mismatch")
				t.Logf("expected: %+v", test.tokens)
				t.Logf("found:    %+v", tokens)
			}
		})
	}
}

// func BenchmarkTokenizer(b *testing.B) {
// 	values := []struct {
// 		scenario string
// 		payload  []byte
// 	}{
// 		{
// 			scenario: "null",
// 			payload:  []byte(`null`),
// 		},

// 		{
// 			scenario: "true",
// 			payload:  []byte(`true`),
// 		},

// 		{
// 			scenario: "false",
// 			payload:  []byte(`false`),
// 		},

// 		{
// 			scenario: "number",
// 			payload:  []byte(`-1.23456789`),
// 		},

// 		{
// 			scenario: "string",
// 			payload:  []byte(`"1234567890"`),
// 		},

// 		{
// 			scenario: "object",
// 			payload: []byte(`{
//     "timestamp": "2019-01-09T18:59:57.456Z",
//     "channel": "server",
//     "type": "track",
//     "event": "Test",
//     "userId": "test-user-whatever",
//     "messageId": "test-message-whatever",
//     "integrations": {
//         "whatever": {
//             "debugMode": false
//         },
//         "myIntegration": {
//             "debugMode": true
//         }
//     },
//     "properties": {
//         "trait1": 1,
//         "trait2": "test",
//         "trait3": true
//     },
//     "settings": {
//         "apiKey": "1234567890",
//         "debugMode": false,
//         "directChannels": [
//             "server",
//             "client"
//         ],
//         "endpoint": "https://somewhere.com/v1/integrations/segment"
//     }
// }`),
// 		},
// 	}

// 	benchmarks := []struct {
// 		scenario string
// 		function func(*testing.B, []byte)
// 	}{
// 		{
// 			scenario: "github.com/segmentio/encoding/json",
// 			function: func(b *testing.B, json []byte) {
// 				t := NewTokenizer(nil)

// 				for i := 0; i < b.N; i++ {
// 					t.Reset(json)

// 					for t.Next() {
// 						// Does nothing other than iterating over each token to measure the
// 						// CPU and memory footprint.
// 					}

// 					if t.Err != nil {
// 						b.Error(t.Err)
// 					}
// 				}
// 			},
// 		},
// 	}

// 	for _, bechmark := range benchmarks {
// 		b.Run(bechmark.scenario, func(b *testing.B) {
// 			for _, value := range values {
// 				b.Run(value.scenario, func(b *testing.B) {
// 					bechmark.function(b, value.payload)
// 				})
// 			}
// 		})
// 	}
// }
