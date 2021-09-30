package load

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

func skipIfNotImplementedErr(t testing.TB, err error) {
	if err == common.ErrNotImplementedError {
		t.Skip("not implemented")
	}
}

func TestLoad(t *testing.T) {
	v, err := Avg()
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}

	empty := &AvgStat{}
	if v == empty {
		t.Errorf("error load: %v", v)
	}
	t.Log(v)
}

func TestLoadAvgStat_String(t *testing.T) {
	v := AvgStat{
		Load1:  10.1,
		Load5:  20.1,
		Load15: 30.1,
	}
	e := `{"load1":10.1,"load5":20.1,"load15":30.1}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("LoadAvgStat string is invalid: %v", v)
	}
	t.Log(e)
}

func TestMisc(t *testing.T) {
	v, err := Misc()
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}

	empty := &MiscStat{}
	if v == empty {
		t.Errorf("error load: %v", v)
	}
	t.Log(v)
}

func TestMiscStatString(t *testing.T) {
	v := MiscStat{
		ProcsTotal:   4,
		ProcsCreated: 5,
		ProcsRunning: 1,
		ProcsBlocked: 2,
		Ctxt:         3,
	}
	e := `{"procsTotal":4,"procsCreated":5,"procsRunning":1,"procsBlocked":2,"ctxt":3}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("TestMiscString string is invalid: %v", v)
	}
	t.Log(e)
}

func BenchmarkLoad(b *testing.B) {

	loadAvg := func(t testing.TB) {
		v, err := Avg()
		skipIfNotImplementedErr(t, err)
		if err != nil {
			t.Errorf("error %v", err)
		}
		empty := &AvgStat{}
		if v == empty {
			t.Errorf("error load: %v", v)
		}
	}

	b.Run("FirstCall", func(b *testing.B) {
		loadAvg(b)
	})

	b.Run("SubsequentCalls", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			loadAvg(b)
		}
	})
}
