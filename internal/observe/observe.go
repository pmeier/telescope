package observe

import (
	"math"
	"time"

	rghttp "github.com/pmeier/redgiant/http"
	"github.com/pmeier/telescope/internal/summary"
	"github.com/rs/zerolog"
)

type timestampedValue struct {
	T time.Time
	V float32
}

type timestampedSummary struct {
	lastTick time.Time
	tvs      map[summary.Quantity]timestampedValue
}

type SummaryTickHandler struct {
	Log                zerolog.Logger
	QuantityThresholds map[summary.Quantity]float64
	TW                 ThresholdWeighter
	rg                 *rghttp.Client
	quantityIDS        map[summary.Quantity]uint
	deviceID           int
	ts                 timestampedSummary
}

func (th *SummaryTickHandler) Setup(rg *rghttp.Client) ([]*Quantity, []*Data, error) {
	th.rg = rg

	qids := map[summary.Quantity]uint{}
	qs := []*Quantity{}
	for i, sq := range summary.Quantities() {
		id := uint(i) + 1
		qids[sq] = id

		qs = append(qs, &Quantity{ID: id, Name: sq.Name(), Unit: sq.Unit()})

	}
	th.quantityIDS = qids

	deviceID, err := summary.GetDeviceID(rg)
	if err != nil {
		return nil, nil, err
	}
	th.deviceID = deviceID

	t := time.Now()
	s, err := summary.Compute(rg, deviceID)
	if err != nil {
		return nil, nil, err
	}
	tvs := make(map[summary.Quantity]timestampedValue, len(s))
	ds := make([]*Data, 0, len(s))
	for q, v := range s {
		tvs[q] = timestampedValue{T: t, V: v}
		ds = append(ds, &Data{Timestamp: t, QuantityID: qids[q], Value: v})
	}
	th.ts = timestampedSummary{lastTick: t, tvs: tvs}

	return qs, ds, nil
}

func (th *SummaryTickHandler) Tick(t time.Time) ([]*Data, error) {
	s, err := summary.Compute(th.rg, th.deviceID)
	if err != nil {
		return nil, err
	}

	ds := []*Data{}
	for q, v := range s {
		tv := th.ts.tvs[q]
		if math.Abs(float64(tv.V-v)) <= th.QuantityThresholds[q]*th.TW.Weight(t.Sub(tv.T)) {
			continue
		}

		qid := th.quantityIDS[q]
		ds = append(ds, &Data{Timestamp: th.ts.lastTick, QuantityID: qid, Value: tv.V})
		ds = append(ds, &Data{Timestamp: t, QuantityID: qid, Value: v})
		th.ts.tvs[q] = timestampedValue{T: t, V: v}
	}

	th.ts.lastTick = t

	return ds, nil
}

type ThresholdWeighter interface {
	Weight(d time.Duration) float64
}

type ExponentialCutoffThresholdWeighter struct {
	D time.Duration
	C float64
}

func (w ExponentialCutoffThresholdWeighter) Weight(d time.Duration) float64 {
	// w(d <= D) = 1
	// w(d = 2*D) = 1 / C
	// w(d -> oo) -> 0
	if d <= w.D {
		return 1.0
	}
	return math.Pow(w.C, 1.0-(d.Seconds()/w.D.Seconds()))
}
