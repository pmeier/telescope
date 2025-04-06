package observe

import (
	"os"
	"time"

	"github.com/pmeier/redgiant"
	"github.com/pmeier/telescope/internal/config"

	"github.com/rs/zerolog"
)

func Run(c config.Config) error {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	var th TickHandler = &SummaryTickHandler{
		Log: log,
		QTHS: map[SummaryQuantity]float64{
			GridPower:    50,
			BatteryPower: 50,
			PVPower:      50,
			LoadPower:    50,
			BatteryLevel: 0.5e-2},
		TW: ExponentialCutoffThresholdWeighter{D: time.Minute * 5, C: 4},
	}

	sg := redgiant.NewSungrow(c.Sungrow.Host, c.Sungrow.Username, c.Sungrow.Password, redgiant.WithLogger(log))
	rg := redgiant.NewRedgiant(sg, redgiant.WithLogger(log))
	if err := rg.Connect(); err != nil {
		return err
	}
	defer rg.Close()

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
