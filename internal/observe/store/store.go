package store

import (
	"math"
	"time"

	"github.com/pmeier/telescope/internal/config"
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

type StoreSummaryHandler struct {
	Log                zerolog.Logger
	QuantityThresholds map[summary.Quantity]float64
	TW                 ThresholdWeighter
	db                 *DB
	quantityIDS        map[summary.Quantity]uint
	ts                 timestampedSummary
}

func (sh *StoreSummaryHandler) Setup(c config.Config, log zerolog.Logger, s summary.Summary) error {
	db := NewDB(c.Database.Host, c.Database.Port, c.Database.Username, c.Database.Password, c.Database.Name)
	sh.db = db

	qids := map[summary.Quantity]uint{}
	qs := []*Quantity{}
	for i, sq := range summary.Quantities() {
		id := uint(i) + 1
		qids[sq] = id

		qs = append(qs, &Quantity{ID: id, Name: sq.Name(), Unit: sq.Unit()})
	}
	sh.quantityIDS = qids
	db.Save(qs)

	tvs := make(map[summary.Quantity]timestampedValue, len(s.Values))
	ds := make([]*Data, 0, len(s.Values))
	for q, v := range s.Values {
		tvs[q] = timestampedValue{T: s.Timestamp, V: v}

		ds = append(ds, &Data{Timestamp: s.Timestamp, QuantityID: qids[q], Value: v})
	}
	sh.ts = timestampedSummary{lastTick: s.Timestamp, tvs: tvs}
	db.Create(ds)

	return nil
}

func (sh *StoreSummaryHandler) Handle(s summary.Summary) error {
	ds := []*Data{}
	for q, v := range s.Values {
		tv := sh.ts.tvs[q]
		if math.Abs(float64(tv.V-v)) <= sh.QuantityThresholds[q]*sh.TW.Weight(s.Timestamp.Sub(tv.T)) {
			continue
		}

		qid := sh.quantityIDS[q]
		ds = append(ds, &Data{Timestamp: sh.ts.lastTick, QuantityID: qid, Value: tv.V})
		ds = append(ds, &Data{Timestamp: s.Timestamp, QuantityID: qid, Value: v})
		sh.ts.tvs[q] = timestampedValue{T: s.Timestamp, V: v}
	}
	sh.ts.lastTick = s.Timestamp

	if len(ds) > 0 {
		sh.db.Create(ds)
	}

	return nil
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
