package storage

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

type StorageSummaryHandler struct {
	Log         zerolog.Logger
	thresholds  map[summary.Quantity]float64
	tw          ThresholdWeighter
	db          *DB
	quantityIDS map[summary.Quantity]uint
	ts          timestampedSummary
}

func (sh *StorageSummaryHandler) Setup(c config.ObserveConfig, log zerolog.Logger, s summary.Summary) error {
	sc := c.Storage
	db := NewDB(sc.Database.Host, sc.Database.Port, sc.Database.Username, sc.Database.Password, sc.Database.Name)
	sh.db = db

	sh.thresholds = map[summary.Quantity]float64{
		summary.GridPower:    sc.Thresholds.GridPower,
		summary.BatteryPower: sc.Thresholds.BatteryPower,
		summary.PVPower:      sc.Thresholds.PVPower,
		summary.LoadPower:    sc.Thresholds.LoadPower,
		summary.BatteryLevel: sc.Thresholds.BatteryLevel,
	}

	sh.tw = ExponentialCutoffThresholdWeighter{
		Start:  sc.ThresholdWeighter.Start,
		Factor: sc.ThresholdWeighter.Factor,
	}

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

func (sh *StorageSummaryHandler) Handle(s summary.Summary) error {
	ds := []*Data{}
	for q, v := range s.Values {
		tv := sh.ts.tvs[q]
		if math.Abs(float64(tv.V-v)) <= sh.thresholds[q]*sh.tw.Weight(s.Timestamp.Sub(tv.T)) {
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
	Start  time.Duration
	Factor float64
}

func (w ExponentialCutoffThresholdWeighter) Weight(d time.Duration) float64 {
	// w(d <= Start) = 1
	// w(d = 2*Start) = 1 / Factor
	// w(d -> oo) -> 0
	if d <= w.Start {
		return 1.0
	}
	return math.Pow(w.Factor, 1.0-(d.Seconds()/w.Start.Seconds()))
}
