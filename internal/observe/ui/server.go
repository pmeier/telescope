package ui

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pmeier/telescope/internal/health"
	"github.com/pmeier/telescope/internal/summary"
	"github.com/rs/zerolog"

	"github.com/google/uuid"
)

type Server struct {
	log  zerolog.Logger
	tg   *TemplateGroup
	data map[string]any
	*echo.Echo
	wss map[uuid.UUID]*websocket.Conn
	mu  sync.Mutex
}

type routeFunc = func(*Server) (string, string, echo.HandlerFunc)

//go:embed static/*
var staticFS embed.FS

//go:embed templates
var templatesFS embed.FS

func (tg *TemplateGroup) Render(wr io.Writer, name string, data any, c echo.Context) error {
	return tg.ExecuteTemplate(wr, name, data)
}

func NewServer(log zerolog.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Debug = true

	e.StaticFS("/static", echo.MustSubFS(staticFS, "static"))

	tg := NewTemplateGroup()
	tg.ParseFS(templatesFS, "templates")
	e.Renderer = tg

	s := &Server{log: log, tg: tg, data: map[string]any{}, Echo: e, wss: map[uuid.UUID]*websocket.Conn{}}

	routeFuncs := []routeFunc{
		wrapBasicRouteFunc(health.HealthRouteFunc),
		index,
		ws,
	}
	for _, routeFunc := range routeFuncs {
		method, path, handler := routeFunc(s)
		e.Add(method, path, handler)
	}

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogRemoteIP: true,
		LogURI:      true,
		LogStatus:   true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().
				Str("origin", v.RemoteIP).
				Str("path", v.URI).
				Int("status_code", v.Status).
				Msg("request")

			return nil
		},
	}))

	return s
}

func wrapBasicRouteFunc(basicRouteFunc func() (string, string, echo.HandlerFunc)) routeFunc {
	return func(*Server) (string, string, echo.HandlerFunc) {
		return basicRouteFunc()
	}
}

func index(s *Server) (string, string, echo.HandlerFunc) {
	return http.MethodGet, "/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "views/index.html", s.data)
	}
}

func ws(s *Server) (string, string, echo.HandlerFunc) {
	upgrader := websocket.Upgrader{}

	return http.MethodGet, "/ws", func(c echo.Context) error {
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return nil
		}

		id := uuid.New()
		s.mu.Lock()
		s.wss[id] = ws
		s.mu.Unlock()

		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
					s.log.Error().Err(err).Send()
				}
				break
			}
			s.log.Warn().Bytes("message", msg).Msg("ignoring received message")
		}

		s.mu.Lock()
		delete(s.wss, id)
		s.mu.Unlock()
		ws.Close()

		return nil
	}
}

func (s *Server) UpdateData(sm *summary.Summary) {
	s.data["TimeStamp"] = sm.Timestamp
	s.data["GridPower"] = sm.Values[summary.GridPower]
	s.data["BatteryPower"] = sm.Values[summary.BatteryPower]
	s.data["PVPower"] = sm.Values[summary.PVPower]
	s.data["LoadPower"] = sm.Values[summary.LoadPower]
	s.data["BatteryLevel"] = sm.Values[summary.BatteryLevel]

	var b bytes.Buffer
	s.tg.ExecuteTemplate(&b, "components/summary.html", &s.data)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ws := range s.wss {
		err := ws.WriteMessage(websocket.TextMessage, b.Bytes())
		if err != nil && err != websocket.ErrCloseSent {
			s.log.Error().Err(err).Send()
		}
	}
}
