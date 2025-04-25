package observe

import (
	"os"
	"time"

	rghttp "github.com/pmeier/redgiant/http"

	"github.com/pmeier/telescope/internal/config"
	"github.com/pmeier/telescope/internal/summary"

	"github.com/rs/zerolog"
)

func Run(c config.Config) error {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	var th TickHandler = &SummaryTickHandler{
		Log: log,
		QuantityThresholds: map[summary.Quantity]float64{
			summary.GridPower:    50,
			summary.BatteryPower: 50,
			summary.PVPower:      50,
			summary.LoadPower:    50,
			summary.BatteryLevel: 0.5e-2},
		TW: ExponentialCutoffThresholdWeighter{D: time.Minute * 5, C: 2},
	}

	rg := rghttp.NewClient(c.Redgiant.Host)

	db := NewDB(c.Database.Host, c.Database.Port, c.Database.Username, c.Database.Password, c.Database.Name)

	if qs, ds, err := th.Setup(rg); err != nil {
		return err
	} else {
		db.Save(qs)
		if len(ds) > 0 {
			db.Create(ds)
		}
	}

	for t := range ticks(time.Second * 5) {
		if ds, err := th.Tick(t); err != nil {
			return err
		} else if len(ds) > 0 {
			db.Create(ds)
		}
	}

	return nil
}

func ticks(d time.Duration) <-chan time.Time {
	t := make(chan time.Time)

	go func() {
		t <- time.Now()
		for now := range time.NewTicker(d).C {
			t <- now
		}
	}()

	return t
}
