package observe

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/pmeier/redgiant"
	"github.com/rs/zerolog"
)

type SummaryQuantity uint8

const (
	GridPower SummaryQuantity = iota
	BatteryPower
	PVPower
	LoadPower
	BatteryLevel
)

func (q SummaryQuantity) Name() string {
	return map[SummaryQuantity]string{
		GridPower:    "grid_power",
		BatteryPower: "battery_power",
		PVPower:      "pv_power",
		LoadPower:    "load_power",
		BatteryLevel: "battery_level",
	}[q]
}

func (q SummaryQuantity) Unit() string {
	return map[SummaryQuantity]string{
		GridPower:    "watts",
		BatteryPower: "watts",
		PVPower:      "watts",
		LoadPower:    "watts",
		BatteryLevel: "ratio",
	}[q]
}

func (q SummaryQuantity) String() string {
	return q.Name()
}

type timestampedValue struct {
	T time.Time
	V float32
}

type timestampedSummary struct {
	lastTick time.Time
	tvs      map[SummaryQuantity]timestampedValue
}

type SummaryTickHandler struct {
	Log      zerolog.Logger
	QTHS     map[SummaryQuantity]float64
	TW       ThresholdWeighter
	rg       *redgiant.Redgiant
	qids     map[SummaryQuantity]uint
	deviceID int
	ts       timestampedSummary
}

func (th *SummaryTickHandler) Setup(rg *redgiant.Redgiant) ([]*Quantity, []*Data, error) {
	th.rg = rg

	sqs := []SummaryQuantity{
		GridPower,
		BatteryPower,
		PVPower,
		LoadPower,
		BatteryLevel,
	}
	qids := make(map[SummaryQuantity]uint, len(sqs))
	qs := make([]*Quantity, 0, len(sqs))
	for i, sq := range sqs {
		id := uint(i) + 1
		qids[sq] = id

		qs = append(qs, &Quantity{ID: id, Name: sq.Name(), Unit: sq.Unit()})

	}
	th.qids = qids

	deviceID, err := getSummaryDeviceID(rg)
	if err != nil {
		return nil, nil, err
	}
	th.deviceID = deviceID

	t := time.Now()
	s, err := computeSummary(rg, deviceID)
	if err != nil {
		return nil, nil, err
	}
	tvs := make(map[SummaryQuantity]timestampedValue, len(s))
	ds := make([]*Data, 0, len(s))
	for q, v := range s {
		tvs[q] = timestampedValue{T: t, V: v}
		ds = append(ds, &Data{Timestamp: t, QuantityID: qids[q], Value: v})
	}
	th.ts = timestampedSummary{lastTick: t, tvs: tvs}

	return qs, ds, nil
}

func (th *SummaryTickHandler) Tick(t time.Time) ([]*Data, error) {
	s, err := computeSummary(th.rg, th.deviceID)
	if err != nil {
		return nil, err
	}
	th.Log.Info().Float32("BatteryLevel", s[BatteryLevel]).Send()

	ds := []*Data{}
	for q, v := range s {
		tv := th.ts.tvs[q]
		if math.Abs(float64(tv.V-v)) <= th.QTHS[q]*th.TW.Weight(t.Sub(tv.T)) {
			continue
		}

		qid := th.qids[q]
		ds = append(ds, &Data{Timestamp: th.ts.lastTick, QuantityID: qid, Value: tv.V})
		ds = append(ds, &Data{Timestamp: t, QuantityID: qid, Value: v})
		th.ts.tvs[q] = timestampedValue{T: t, V: v}
	}

	th.ts.lastTick = t

	return ds, nil
}

func getSummaryDeviceID(rg *redgiant.Redgiant) (int, error) {
	var deviceID int

	ds, err := rg.Devices()
	if err != nil {
		return deviceID, err
	}

	for _, d := range ds {
		if d.Type == 35 {
			deviceID = d.ID
			break
		}
	}
	if deviceID == 0 {
		return deviceID, errors.New("no summary device available")
	}

	return deviceID, nil
}

func computeSummary(rg *redgiant.Redgiant, deviceID int) (map[SummaryQuantity]float32, error) {
	ms, err := rg.RealData(deviceID, redgiant.NoLanguage, "real", "real_battery")
	if err != nil {
		return nil, err
	}

	vs := map[string]float32{}
	for _, m := range ms {
		v, err := strconv.ParseFloat(m.Value, 32)
		if err != nil {
			continue
		}
		vs[m.I18NCode] = float32(v)
	}

	return map[SummaryQuantity]float32{
		GridPower:    (vs["I18N_CONFIG_KEY_4060"] - vs["I18N_COMMON_FEED_NETWORK_TOTAL_ACTIVE_POWER"]) * 1e3,
		BatteryPower: (vs["I18N_CONFIG_KEY_3921"] - vs["I18N_CONFIG_KEY_3907"]) * 1e3,
		PVPower:      vs["I18N_COMMON_TOTAL_DCPOWER"] * 1e3,
		LoadPower:    vs["I18N_COMMON_LOAD_TOTAL_ACTIVE_POWER"] * 1e3,
		BatteryLevel: vs["I18N_COMMON_BATTERY_SOC"] * 1e-2,
	}, nil
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
