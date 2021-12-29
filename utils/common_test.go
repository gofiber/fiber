// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"testing"
)

func Test_FunctionName(t *testing.T) {
	t.Parallel()
	AssertEqual(t, "github.com/gofiber/fiber/v2/utils.Test_UUID", FunctionName(Test_UUID))

	AssertEqual(t, "github.com/gofiber/fiber/v2/utils.Test_FunctionName.func1", FunctionName(func() {}))

	dummyint := 20
	AssertEqual(t, "int", FunctionName(dummyint))
}

func Test_UUID(t *testing.T) {
	t.Parallel()
	res := UUID()
	AssertEqual(t, 36, len(res))
	AssertEqual(t, true, res != "00000000-0000-0000-0000-000000000000")
}

func Test_UUID_Concurrency(t *testing.T) {
	t.Parallel()
	iterations := 1000
	var res string
	ch := make(chan string, iterations)
	results := make(map[string]string)
	for i := 0; i < iterations; i++ {
		go func() {
			ch <- UUID()
		}()
	}
	for i := 0; i < iterations; i++ {
		res = <-ch
		results[res] = res
	}
	AssertEqual(t, iterations, len(results))
}

func Test_UUIDv4(t *testing.T) {
	t.Parallel()
	res := UUIDv4()
	AssertEqual(t, 36, len(res))
	AssertEqual(t, true, res != "00000000-0000-0000-0000-000000000000")
}

func Test_UUIDv4_Concurrency(t *testing.T) {
	t.Parallel()
	iterations := 1000
	var res string
	ch := make(chan string, iterations)
	results := make(map[string]string)
	for i := 0; i < iterations; i++ {
		go func() {
			ch <- UUIDv4()
		}()
	}
	for i := 0; i < iterations; i++ {
		res = <-ch
		results[res] = res
	}
	AssertEqual(t, iterations, len(results))
}

// go test -v -run=^$ -bench=Benchmark_UUID -benchmem -count=2

func Benchmark_UUID(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = UUID()
		}
		AssertEqual(b, 36, len(res))
	})
	b.Run("default", func(b *testing.B) {
		rnd := make([]byte, 16)
		_, _ = rand.Read(rnd)
		for n := 0; n < b.N; n++ {
			res = fmt.Sprintf("%x-%x-%x-%x-%x", rnd[0:4], rnd[4:6], rnd[6:8], rnd[8:10], rnd[10:])
		}
		AssertEqual(b, 36, len(res))
	})
}


var DefaultWriter io.Writer = os.Stdout
var DefaultErrorWriter io.Writer = os.Stderr
// captureOutput will capture the output in the command line.
func captureOutput(t *testing.T, f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	defaultWriter := DefaultWriter
	defaultErrorWriter := DefaultErrorWriter
	defer func() {
		DefaultWriter = defaultWriter
		DefaultErrorWriter = defaultErrorWriter
		log.SetOutput(os.Stderr)
	}()
	DefaultWriter = writer
	DefaultErrorWriter = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		_, err := io.Copy(&buf, reader)
		AssertEqual(t, err, nil)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func Test_print_routes(t *testing.T) {

}