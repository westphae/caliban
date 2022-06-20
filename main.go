package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"github.com/westphae/caliban/tempest"
	"github.com/westphae/caliban/windy"
	"github.com/westphae/caliban/wx"
)

var (
	token          string
	stationId      int
	deviceId       int
	windyApiKey    string
	windyStationId string
)

func init() {
	viper.SetConfigName("caliban")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error in config file: %w", err))
	}

	token = viper.GetString("tempest-token")
	stationId = viper.GetInt("tempest-stationId")
	deviceId = viper.GetInt("tempest-deviceId")
	windyApiKey = viper.GetString("windy-apiKey")
	windyStationId = viper.GetString("windy-stationId")
}

func main() {
	var (
		err           error
		s             *tempest.Station
		station       windy.Station
		observation   windy.Observation
		lastTimestamp int64
	)

	if s, err = tempest.GetStation(token, stationId); err != nil {
		panic(err)
	}

	station = windy.Station{
		Name:        s.PublicName,
		ShareOption: "Open",
		Latitude:    s.Latitude,
		Longitude:   s.Longitude,
		Elevation:   s.StationMeta.Elevation,
		TempHeight:  s.StationMeta.Elevation,
		WindHeight:  s.StationMeta.Elevation,
	}

	obsCh, err := tempest.SubscribeObservations(token, deviceId)
	if err != nil {
		panic(err)
	}
	log.Printf("client subscribed, listening...")

	i := 0
	for obs := range obsCh {
		i += 1
		log.Printf("client received message %d: %+v", i, obs)

		// Windy only wants data every 5 minutes
		dts := obs.Timestamp - lastTimestamp
		if dts < 300 {
			log.Printf("not updating windy, time diff is only %d sec", dts)
			continue
		}

		observation = windy.Observation{
			TS:       obs.Timestamp,
			Temp:     obs.AirTemperature,
			Wind:     obs.WindAvg,
			WindDir:  obs.WindDirection,
			Gust:     obs.WindGust,
			RH:       obs.RelativeHumidity,
			Dewpoint: wx.Dewpoint(float64(obs.RelativeHumidity), obs.AirTemperature),
			Pressure: obs.Pressure,
			Precip:   float64(obs.RainAccumulation),
			UV:       obs.UV,
		}
		log.Printf("sending %+v", observation)

		if err = windy.SendToWindy(
			windyApiKey,
			[]windy.Station{station},
			[]windy.Observation{observation},
		); err != nil {
			panic(err)
		}

		lastTimestamp = obs.Timestamp
	}

	close(obsCh)
	log.Println("client channel closed")

}
