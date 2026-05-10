package tempest

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/spf13/viper"
)

var (
	token     string
	stationId int
	deviceId  int
)

func init() {
	viper.SetConfigName("caliban")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error in config file: %w", err))
	}

	token = viper.GetString("tempest-token")
	stationId = viper.GetInt("tempest-stationId")
	deviceId = viper.GetInt("tempest-deviceId")
}

// TestRawToObsIndexMapping locks in the wire-format index → struct field
// mapping for obs_st payloads. Reordering Observation fields without updating
// RawToObs will fail this test instead of silently corrupting historical data
// in the sqlite store.
func TestRawToObsIndexMapping(t *testing.T) {
	raw := make([]float64, 22)
	for i := range raw {
		raw[i] = float64(i + 1) // 1, 2, 3, ... so each field carries its own index
	}
	o := RawToObs(raw)

	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"Timestamp", float64(o.Timestamp), 1},
		{"WindLull", o.WindLull, 2},
		{"WindAvg", o.WindAvg, 3},
		{"WindGust", o.WindGust, 4},
		{"WindDirection", float64(o.WindDirection), 5},
		{"WindSampleInterval", float64(o.WindSampleInterval), 6},
		{"Pressure", o.Pressure, 7},
		{"AirTemperature", o.AirTemperature, 8},
		{"RelativeHumidity", float64(o.RelativeHumidity), 9},
		{"Illuminance", float64(o.Illuminance), 10},
		{"UV", o.UV, 11},
		{"SolarRadiation", float64(o.SolarRadiation), 12},
		{"RainAccumulation", float64(o.RainAccumulation), 13},
		{"PrecipitationType", float64(o.PrecipitationType), 14},
		{"AverageStrikeDistance", float64(o.AverageStrikeDistance), 15},
		{"StrikeCount", float64(o.StrikeCount), 16},
		{"BatteryVolts", o.BatteryVolts, 17},
		{"ReportInterval", float64(o.ReportInterval), 18},
		{"LocalDayRainAccumulation", float64(o.LocalDayRainAccumulation), 19},
		{"NCRainAccumulation", float64(o.NCRainAccumulation), 20},
		{"LocalDayNCRainAccumulation", float64(o.LocalDayNCRainAccumulation), 21},
		{"PrecipitationAnalysisType", float64(o.PrecipitationAnalysisType), 22},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestGetStations(t *testing.T) {
	s, err := GetStations(token)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%+v", s)
}
func TestGetStation(t *testing.T) {
	s, err := GetStation(token, stationId)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%+v", *s)
}

func TestGetLatestDeviceObservation(t *testing.T) {
	obs, err := GetDeviceObservations(token, deviceId, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%+v", obs)
}

func TestGetManyDeviceObservations(t *testing.T) {
	timeNow := time.Now().Unix()
	timeBefore := timeNow - 300
	log.Printf("Retreiving from %d to %d", timeBefore, timeNow)
	obs, err := GetDeviceObservations(token, deviceId, timeBefore, timeNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) < 2 || len(obs) > 4 {
		t.Errorf("Expected 2-4 observations, received %d", len(obs))
	}

	log.Printf("%+v", obs)
}

func TestSubscribeObservations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	obsCh, err := SubscribeObservations(ctx, token, deviceId)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("client subscribed, listening...")

	i := 0
	for obs := range obsCh {
		i++
		log.Printf("client received message %d: %+v", i, obs)
		if i >= 3 {
			cancel()
		}
	}
	log.Println("client channel closed")
	// Give the goroutine a beat to release the conn so we don't trip leak
	// detection when this is the final test.
	time.Sleep(100 * time.Millisecond)
}
