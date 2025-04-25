package summary

import (
	"errors"
	"strconv"

	rghttp "github.com/pmeier/redgiant/http"

	"github.com/pmeier/redgiant"
)

type Quantity uint8

const (
	GridPower Quantity = iota
	BatteryPower
	PVPower
	LoadPower
	BatteryLevel
)

func (q Quantity) Name() string {
	return map[Quantity]string{
		GridPower:    "grid_power",
		BatteryPower: "battery_power",
		PVPower:      "pv_power",
		LoadPower:    "load_power",
		BatteryLevel: "battery_level",
	}[q]
}

func (q Quantity) Unit() string {
	return map[Quantity]string{
		GridPower:    "watts",
		BatteryPower: "watts",
		PVPower:      "watts",
		LoadPower:    "watts",
		BatteryLevel: "ratio",
	}[q]
}

func (q Quantity) String() string {
	return q.Name()
}

func Quantities() []Quantity {
	return []Quantity{
		GridPower,
		BatteryPower,
		PVPower,
		LoadPower,
		BatteryLevel,
	}
}

func GetDeviceID(rg *rghttp.Client) (int, error) {
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

func Compute(rg *rghttp.Client, deviceID int) (map[Quantity]float32, error) {
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

	return map[Quantity]float32{
		GridPower:    (vs["I18N_CONFIG_KEY_4060"] - vs["I18N_COMMON_FEED_NETWORK_TOTAL_ACTIVE_POWER"]) * 1e3,
		BatteryPower: (vs["I18N_CONFIG_KEY_3921"] - vs["I18N_CONFIG_KEY_3907"]) * 1e3,
		PVPower:      vs["I18N_COMMON_TOTAL_DCPOWER"] * 1e3,
		LoadPower:    vs["I18N_COMMON_LOAD_TOTAL_ACTIVE_POWER"] * 1e3,
		BatteryLevel: vs["I18N_COMMON_BATTERY_SOC"] * 1e-2,
	}, nil
}
