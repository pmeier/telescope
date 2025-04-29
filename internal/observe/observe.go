package observe

import (
	"net/http"
	"os"
	"time"

	"github.com/pmeier/telescope/internal/config"
	"github.com/pmeier/telescope/internal/observe/store"
	"github.com/pmeier/telescope/internal/observe/ui"
	"github.com/pmeier/telescope/internal/summary"

	rghttp "github.com/pmeier/redgiant/http"

	"github.com/rs/zerolog"
)

type SummaryHandler interface {
	Setup(config.Config, zerolog.Logger, summary.Summary) error
	Handle(summary.Summary) error
}

func summaryHandlers(c config.Config) []SummaryHandler {
	// FIXME: make this configurable
	return []SummaryHandler{
		&store.StoreSummaryHandler{
			QuantityThresholds: map[summary.Quantity]float64{
				summary.GridPower:    50,
				summary.BatteryPower: 50,
				summary.PVPower:      50,
				summary.LoadPower:    50,
				summary.BatteryLevel: 0.5e-2},
			TW: store.ExponentialCutoffThresholdWeighter{D: time.Minute * 5, C: 2},
		},
		&ui.UISummaryHandler{},
	}
}

func Run(c config.Config) error {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	// FIXME: allow passing logger into HTTP client
	rg := rghttp.NewRedgiant(&http.Client{Timeout: time.Second * 30}, c.Redgiant.Host, c.Redgiant.Port)

	deviceID, err := summary.GetDeviceID(rg)
	if err != nil {
		return err
	}

	ths := summaryHandlers(c)

	s, err := summary.Compute(rg, deviceID)
	if err != nil {
		return err
	}
	for _, th := range ths {
		if err := th.Setup(c, log, s); err != nil {
			return err
		}
	}

	// FIXME: make this configurable
	for range ticks(time.Second * 5) {
		s, err := summary.Compute(rg, deviceID)
		if err != nil {
			return err
		}
		// FIXME: check if all values are 0 and continue if so

		for _, th := range ths {
			// FIXME goroutine?
			if err := th.Handle(s); err != nil {
				return err
			}
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
