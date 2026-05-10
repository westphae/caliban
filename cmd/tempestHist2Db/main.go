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
	dbPath   string
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
	dbPath = viper.GetString("db-path")
	if dbPath == "" {
		dbPath = wx.DefaultPath()
	}
}

func main() {
	store, err := wx.Open(dbPath)
	if err != nil {
		log.Fatalf("opening sqlite at %s: %s", dbPath, err)
	}
	defer store.Close()

	timeNow := time.Now().Unix()
	timeBefore := timeNow - 60*60*24*5
	log.Printf("retrieving device %d from %d to %d", deviceId, timeBefore, timeNow)

	obs, err := tempest.GetDeviceObservations(token, deviceId, timeBefore, timeNow)
	if err != nil {
		log.Fatalf("fetching device observations: %s", err)
	}
	log.Printf("received %d observations", len(obs))

	saved := 0
	for _, o := range obs {
		if err := store.Save(deviceId, o); err != nil {
			log.Printf("save ts=%d failed: %s", o.Timestamp, err)
			continue
		}
		saved++
	}
	log.Printf("finished processing %d/%d observations", saved, len(obs))
}
