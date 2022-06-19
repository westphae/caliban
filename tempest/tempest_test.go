package tempest

import (
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
	obsCh, err := SubscribeObservations(token, deviceId)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("client subscribed, listening...")
	log.Printf("chan len is %d", len(obsCh))

	i := 0
	for obs := range obsCh {
		i += 1
		log.Printf("client received message %d: %+v", i, obs)
		if i >= 3 {
			break
		}
	}
	close(obsCh)
	log.Println("client channel closed")
	time.Sleep(1)
}
