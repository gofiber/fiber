package load

import (
	"encoding/json"
)

// var invoke common.Invoker = common.Invoke{}
type AvgStat struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

func (l AvgStat) String() string {
	s, _ := json.Marshal(l)
	return string(s)
}

type MiscStat struct {
	ProcsTotal   int64 `json:"procsTotal"`
	ProcsCreated int64 `json:"procsCreated"`
	ProcsRunning int64 `json:"procsRunning"`
	ProcsBlocked int64 `json:"procsBlocked"`
	Ctxt         int64 `json:"ctxt"`
}

func (m MiscStat) String() string {
	s, _ := json.Marshal(m)
	return string(s)
}
