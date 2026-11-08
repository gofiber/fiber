package sse

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func Benchmark_SSE_StreamEvent(b *testing.B) {
	for _, tc := range []struct {
		name  string
		event Event
	}{
		{name: "data", event: Event{Data: "hello"}},
		{
			name:  "multiline",
			event: Event{ID: "42", Name: "update", Retry: 2500 * time.Millisecond, Data: "first line\nsecond line\n"},
		},
		{name: "json", event: Event{Data: struct{ Status string }{Status: "ready"}}},
		{name: "buffer_boundary", event: Event{Data: strings.Repeat("x", 4<<10)}},
		{name: "large", event: Event{Data: strings.Repeat("x", 8<<10)}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			stream := newStream(context.Background(), bufio.NewWriter(io.Discard), "")
			b.Cleanup(stream.closeStream)
			b.ReportAllocs()
			for b.Loop() {
				if err := stream.Event(tc.event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
