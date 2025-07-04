package observe

import (
	"time"

	"github.com/pmeier/redgiant"
	"github.com/pmeier/telescope/internal/config"
	"github.com/pmeier/telescope/internal/observe/storage"
	"github.com/pmeier/telescope/internal/observe/ui"
	"github.com/pmeier/telescope/internal/summary"

	rghttp "github.com/pmeier/redgiant/http"

	"github.com/rs/zerolog"
)

type SummaryHandler interface {
	Setup(config.ObserveConfig, zerolog.Logger, summary.Summary) error
	Handle(summary.Summary) error
}

func summaryHandlers() []SummaryHandler {
	return []SummaryHandler{
		&storage.StorageSummaryHandler{},
		&ui.UISummaryHandler{},
	}
}

func Run(c config.Config) error {
	log := zerolog.New(c.Logging.Format.Writer()).With().Timestamp().Logger().Level(c.Logging.Level)

	rg := rghttp.NewRedgiant(c.Redgiant.Host, c.Redgiant.Port, redgiant.WithLogger(log))

	deviceID, err := summary.GetDeviceID(rg)
	if err != nil {
		return err
	}

	ths := summaryHandlers()

	s, err := summary.Compute(rg, deviceID)
	if err != nil {
		return err
	}
	for _, th := range ths {
		if err := th.Setup(c.Observe, log, s); err != nil {
			return err
		}
	}

	for range ticks(c.Observe.SampleInterval) {
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
