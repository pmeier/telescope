package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type TelescopeConfig struct {
	Host string
	Port uint
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

type Config struct {
	Telescope TelescopeConfig
	Redgiant  RedgiantConfig
	Database  DatabaseConfig
}

func New() *Config {
	return &Config{
		Telescope: TelescopeConfig{
			Host: "127.0.0.1",
			Port: 8001,
		},
		Redgiant: RedgiantConfig{
			Host: "127.0.0.1",
			Port: 8000,
		},
		Database: DatabaseConfig{
			Username: "postgres",
			Host:     "127.0.0.1",
			Port:     5432,
			Name:     "postgres",
		},
	}
}

type Viper struct {
	*viper.Viper
}

func NewViper() *Viper {
	v := Viper{Viper: viper.New()}
	v.SetConfigName("telescope")
	return &v
}

func (v *Viper) ReadAndMergeInConfigs() error {
	for _, in := range []string{
		"/etc/telescope",
		"$HOME/.config/telescope",
		".",
	} {
		vv := NewViper()
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

func (v *Viper) Unmarshal(rawVal any) error {
	return v.Viper.Unmarshal(rawVal, func(dc *mapstructure.DecoderConfig) { dc.DecodeHook = envVarTemplating() })
}

func envVarTemplating() mapstructure.DecodeHookFuncType {
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
