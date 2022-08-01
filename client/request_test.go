package client

import (
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
)

func TestParamsSetParamsWithStruct(t *testing.T) {
	t.Parallel()

	type args struct {
		TInt      int
		TString   string
		TFloat    float64
		TBool     bool
		TSlice    []string
		TIntSlice []int `param:"int_slice"`
	}

	t.Run("the struct should be applied", func(t *testing.T) {
		p := &Params{
			Values: make(url.Values),
		}
		p.SetParamsWithStruct(args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		utils.AssertEqual(t, "5", p.Get("TInt"))
		utils.AssertEqual(t, "string", p.Get("TString"))
		utils.AssertEqual(t, "3.1", p.Get("TFloat"))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["TSlice"] {
				if v == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["TSlice"] {
				if v == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["int_slice"] {
				if v == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["int_slice"] {
				if v == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the pointer of a struct should be applied", func(t *testing.T) {
		p := &Params{
			Values: make(url.Values),
		}
		p.SetParamsWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		utils.AssertEqual(t, "5", p.Get("TInt"))
		utils.AssertEqual(t, "string", p.Get("TString"))
		utils.AssertEqual(t, "3.1", p.Get("TFloat"))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["TSlice"] {
				if v == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["TSlice"] {
				if v == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["int_slice"] {
				if v == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.Values["int_slice"] {
				if v == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the zero val should be ignore", func(t *testing.T) {
		p := &Params{
			Values: make(url.Values),
		}
		p.SetParamsWithStruct(&args{
			TInt:    0,
			TString: "",
			TFloat:  0.0,
		})

		utils.AssertEqual(t, "", p.Get("TInt"))
		utils.AssertEqual(t, "", p.Get("TString"))
		utils.AssertEqual(t, "", p.Get("TFloat"))
		utils.AssertEqual(t, 0, len(p.Values["TSlice"]))
		utils.AssertEqual(t, 0, len(p.Values["int_slice"]))
	})
}
