package tempest

import (
	"fmt"
	"log"
	"testing"

	"github.com/spf13/viper"
)

var (
	token     string
	stationId int
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
