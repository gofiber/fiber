package client

import (
	"testing"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

func TestParamsSetParamsWithStruct(t *testing.T) {
	t.Parallel()

	type args struct {
		unexport  int
		TInt      int
		TString   string
		TFloat    float64
		TBool     bool
		TSlice    []string
		TIntSlice []int `param:"int_slice"`
	}

	t.Run("the struct should be applied", func(t *testing.T) {
		p := &Params{
			Args: fasthttp.AcquireArgs(),
		}
		p.SetParamsWithStruct(args{
			unexport:  5,
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     false,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		utils.AssertEqual(t, "", string(p.Peek("unexport")))
		utils.AssertEqual(t, []byte("5"), p.Peek("TInt"))
		utils.AssertEqual(t, []byte("string"), p.Peek("TString"))
		utils.AssertEqual(t, []byte("3.1"), p.Peek("TFloat"))
		utils.AssertEqual(t, "", string(p.Peek("TBool")))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the pointer of a struct should be applied", func(t *testing.T) {
		p := &Params{
			Args: fasthttp.AcquireArgs(),
		}
		p.SetParamsWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		utils.AssertEqual(t, []byte("5"), p.Peek("TInt"))
		utils.AssertEqual(t, []byte("string"), p.Peek("TString"))
		utils.AssertEqual(t, []byte("3.1"), p.Peek("TFloat"))
		utils.AssertEqual(t, "true", string(p.Peek("TBool")))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the zero val should be ignore", func(t *testing.T) {
		p := &Params{
			Args: fasthttp.AcquireArgs(),
		}
		p.SetParamsWithStruct(&args{
			TInt:    0,
			TString: "",
			TFloat:  0.0,
		})

		utils.AssertEqual(t, "", string(p.Peek("TInt")))
		utils.AssertEqual(t, "", string(p.Peek("TString")))
		utils.AssertEqual(t, "", string(p.Peek("TFloat")))
		utils.AssertEqual(t, 0, len(p.PeekMulti("TSlice")))
		utils.AssertEqual(t, 0, len(p.PeekMulti("int_slice")))
	})

	t.Run("error type should ignore", func(t *testing.T) {
		p := &Params{
			Args: fasthttp.AcquireArgs(),
		}
		p.SetParamsWithStruct(5)
		utils.AssertEqual(t, 0, p.Len())
	})
}
