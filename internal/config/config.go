package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

type LoggingFormat uint8

const (
	AutoLoggingFormat LoggingFormat = iota
	ConsoleLoggingFormat
	JSONLoggingFormat
)

func (f LoggingFormat) String() string {
	switch f {
	case AutoLoggingFormat:
		return "auto"
	case ConsoleLoggingFormat:
		return "console"
	case JSONLoggingFormat:
		return "json"
	default:
		return strconv.Itoa(int(f))
	}
}

func ParseLoggingFormat(formatStr string) (LoggingFormat, error) {
	for _, lang := range []LoggingFormat{
		AutoLoggingFormat,
		ConsoleLoggingFormat,
		JSONLoggingFormat,
	} {
		if strings.EqualFold(formatStr, lang.String()) {
			return lang, nil
		}
	}
	return AutoLoggingFormat, errors.New("unknown language")
}

func (f LoggingFormat) Writer() io.Writer {
	if f == AutoLoggingFormat {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			f = ConsoleLoggingFormat
		} else {
			f = JSONLoggingFormat
		}
	}

	switch f {
	case ConsoleLoggingFormat:
		return zerolog.ConsoleWriter{Out: os.Stdout}
	case JSONLoggingFormat:
		return os.Stdout
	default:
		panic("unknown logging format")
	}
}

type LoggingConfig struct {
	Level  zerolog.Level
	Format LoggingFormat
}

type RedgiantConfig struct {
	Host string
	Port uint
}

type DatabaseConfig struct {
	Username string
	Password string `validate:"required"`
	Host     string
	Port     uint
	Name     string
}

type ThresholdsConfig struct {
	GridPower    float64
	BatteryPower float64
	PVPower      float64
	LoadPower    float64
	BatteryLevel float64
}

type ThresholdWeighterConfig struct {
	Start  time.Duration
	Factor float64
}

type StorageConfig struct {
	Database          DatabaseConfig
	Thresholds        ThresholdsConfig
	ThresholdWeighter ThresholdWeighterConfig
}

type UIConfig struct {
	Host string
	Port uint
}

type ObserveConfig struct {
	SampleInterval time.Duration
	Storage        StorageConfig
	UI             UIConfig
}

type Config struct {
	Logging  LoggingConfig
	Redgiant RedgiantConfig
	Observe  ObserveConfig
}

func Load() (*Config, error) {
	v := viper.New()

	if err := loadDefaults(v); err != nil {
		return nil, err
	}

	if err := loadFromFiles(v, "telescope",
		"/etc/telescope",
		"$HOME/.config/telescope",
		".",
	); err != nil {
		return nil, err
	}

	enableLoadFromEnvVars(v, "TELESCOPE")

	c := &Config{}
	if err := v.Unmarshal(c, func(dc *mapstructure.DecoderConfig) {

		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			stringTemplatingHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
			stringToZerologLevelHookFunc(),
			stringToLoggingFormatHookFunc(),
		)
	}); err != nil {
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(c); err != nil {
		return nil, err
	}

	return c, nil
}

func loadDefaults(v *viper.Viper) error {
	dc := Config{
		Logging: LoggingConfig{
			Level:  zerolog.InfoLevel,
			Format: AutoLoggingFormat,
		},
		Redgiant: RedgiantConfig{
			Host: "127.0.0.1",
			Port: 8000,
		},
		Observe: ObserveConfig{
			SampleInterval: time.Second * 5,
			Storage: StorageConfig{
				Database: DatabaseConfig{
					Username: "postgres",
					Host:     "127.0.0.1",
					Port:     5432,
					Name:     "postgres",
				},
				Thresholds: ThresholdsConfig{
					GridPower:    50,
					BatteryPower: 50,
					PVPower:      50,
					LoadPower:    50,
					BatteryLevel: 0.5e-2,
				},
				ThresholdWeighter: ThresholdWeighterConfig{
					Start:  time.Minute * 5,
					Factor: 2,
				},
			},
			UI: UIConfig{
				Host: "127.0.0.1",
				Port: 8001,
			},
		},
	}

	b, err := json.Marshal(dc)
	if err != nil {
		return err
	}

	r := bytes.NewReader(b)
	vv := viper.New()
	vv.SetConfigType("json")
	if err := vv.MergeConfig(r); err != nil {
		return err
	}

	v.MergeConfigMap(vv.AllSettings())
	return nil
}

func loadFromFiles(v *viper.Viper, configName string, paths ...string) error {
	for _, in := range paths {
		vv := viper.New()
		vv.SetConfigName(configName)
		vv.AddConfigPath(in)
		if err := vv.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return err
			}
		}

		v.MergeConfigMap(vv.AllSettings())
	}

	return nil
}

func enableLoadFromEnvVars(v *viper.Viper, prefix string) error {
	v.AutomaticEnv()
	v.SetEnvPrefix(prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return nil
}

func stringTemplatingHookFunc() mapstructure.DecodeHookFuncType {
	e := map[string]string{}
	for _, kv := range os.Environ() {
		s := strings.SplitN(kv, "=", 2)
		e[s[0]] = s[1]
	}

	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		switch v := data.(type) {
		case string:
			tpl, err := template.New("").Funcs(sprig.FuncMap()).Parse(v)
			if err != nil {
				return nil, err
			}

			var b bytes.Buffer
			if err := tpl.Execute(&b, e); err != nil {
				return nil, err
			}

			return b.String(), nil
		default:
			return v, nil
		}
	}
}

func stringToZerologLevelHookFunc() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f.Kind() != reflect.String || t != reflect.TypeOf(zerolog.NoLevel) {
			return data, nil
		}

		return zerolog.ParseLevel(data.(string))
	}
}

func stringToLoggingFormatHookFunc() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f.Kind() != reflect.String || t != reflect.TypeOf(AutoLoggingFormat) {
			return data, nil
		}

		return ParseLoggingFormat(data.(string))
	}
}
