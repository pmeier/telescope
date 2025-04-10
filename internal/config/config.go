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
	Host string `validate:"required"`
	Port uint   `validate:"required"`
}

type SungrowConfig struct {
	Host     string `validate:"required"`
	Username string `validate:"required"`
	Password string `validate:"required"`
}

type DatabaseConfig struct {
	Username string
	Password string
	Host     string
	Port     uint
	Name     string
}

type Config struct {
	Telescope TelescopeConfig
	Sungrow   SungrowConfig
	Database  DatabaseConfig
}

type Viper struct {
	*viper.Viper
}

func NewViper() *Viper {
	v := Viper{Viper: viper.New()}
	v.SetConfigName("telescope")
	v.AddConfigPath("/etc/telescope")
	return &v
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

		// if f.Kind() != reflect.String {
		// 	return data, nil
		// }

		// tpl, err := template.New("").Parse(data.(string))
		// if err != nil {
		// 	return nil, err
		// }

		// var b bytes.Buffer
		// if err := tpl.Execute(&b, e); err != nil {
		// 	return nil, err
		// }

		// return b.String(), nil
	}
}
