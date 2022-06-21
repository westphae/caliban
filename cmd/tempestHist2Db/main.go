package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
	"github.com/westphae/caliban/tempest"
	"github.com/westphae/caliban/wx"
)

var (
	token    string
	deviceId int
)

func init() {
	viper.SetConfigName("caliban")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error in config file: %w", err))
	}

	token = viper.GetString("tempest-token")
	deviceId = viper.GetInt("tempest-deviceId")
}

func main() {
	var (
		err        error
		i          int
		o          tempest.Observation
		timeNow    = time.Now().Unix()
		timeBefore = timeNow - 60*60*24*5
	)
	log.Printf("Retreiving %d data from %d to %d", deviceId, timeBefore, timeNow)

	obs, err := tempest.GetDeviceObservations(token, deviceId, timeBefore, timeNow)
	if err != nil {
		panic(err)
	}

	log.Printf("received %d observations", len(obs))

	for i, o = range obs {
		if err = wx.SaveTempestDataToDb(deviceId, o); err != nil {
			panic(err)
		}
	}
	log.Printf("finished processing %d observations", i)
}
