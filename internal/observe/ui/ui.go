package ui

import (
	"fmt"
	"time"

	"github.com/pmeier/telescope/internal/config"
	"github.com/pmeier/telescope/internal/health"
	"github.com/pmeier/telescope/internal/summary"
	"github.com/rs/zerolog"
)

type UISummaryHandler struct {
	s *Server
}

func (sh *UISummaryHandler) Setup(c config.ObserveConfig, log zerolog.Logger, s summary.Summary) error {
	uc := c.UI
	sh.s = NewServer(log)

	host := uc.Host
	port := uc.Port
	log = log.With().Str("host", host).Uint("port", port).Logger()
	log.Info().Msg("starting")

	go func() {
		sh.s.Start(fmt.Sprintf("%s:%d", host, port))
	}()

	if err := health.WaitForHealthy(host, port, time.Second*10); err != nil {
		return err
	}

	log.Info().Msg("started")
	return nil
}

func (sh *UISummaryHandler) Handle(s summary.Summary) error {
	sh.s.UpdateData(&s)
	return nil
}
