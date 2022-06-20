package main

import (
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
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
	log.Printf("client subscribed to tempest, listening...")

	i := 0
	for obs := range obsCh {
		i += 1
		log.Printf("client received tempest message %d: %+v", i, obs)

		// Save to sqlite db
		err = wx.SaveTempestDataToDb(deviceId, obs)
		switch {
		case err == nil:
			log.Println("saved tempest data to sqlite db")
		case strings.HasPrefix(err.Error(), "UNIQUE constraint failed"):
			log.Println("observation already in sqlite db")
		default:
			panic(err)
		}

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
		log.Printf("sending to windy: %+v", observation)

		err = windy.SendToWindy(
			windyApiKey,
			[]windy.Station{station},
			[]windy.Observation{observation},
		)
		if err != nil {
			if _, ok := err.(windy.WindyError); ok {
				log.Println(err)
				continue
			}
			panic(err)
		}
		log.Println("windy updated successfully")
		lastTimestamp = obs.Timestamp
	}

	close(obsCh)
	log.Println("client tempest channel closed")
}
