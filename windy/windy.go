package windy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

var (
	PWSURL string = "https://stations.windy.com/pws/update/%s"
)

type Station struct {
	Station     int     `json:"station"`
	ShareOption string  `json:"shareOption,omitempty"`
	Name        string  `json:"name,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Elevation   float64 `json:"elevation,omitempty"`
	TempHeight  float64 `json:"tempheight,omitempty"`
	WindHeight  float64 `json:"windheight,omitempty"`
}

type Observation struct {
	Station      int     `json:"station"`
	Time         string  `json:"time,omitempty"`
	DateUTC      string  `json:"dateuts,omitempty"`
	TS           int64   `json:"ts,omitempty"`
	Temp         float64 `json:"temp,omitempty"`
	TempF        float64 `json:"tempf,omitempty"`
	Wind         float64 `json:"wind,omitempty"`
	WindSpeedMPH float64 `json:"windspeedmph,omitempty"`
	WindDir      int     `json:"winddir"`
	Gust         float64 `json:"gust,omitempty"`
	WindGustMPH  float64 `json:"windgustmph,omitempty"`
	RH           int     `json:"rh,omitempty"`
	Dewpoint     float64 `json:"dewpoint,omitempty"`
	Pressure     float64 `json:"pressure,omitempty"`
	MBar         float64 `json:"mbar,omitempty"`
	BaromIn      float64 `json:"baromin,omitempty"`
	Precip       float64 `json:"precip,omitempty"`
	RainIn       float64 `json:"rainin,omitempty"`
	UV           float64 `json:"uv,omitempty"`
}

func SendToWindy(apiKey string, stations []Station, observations []Observation) (err error) {
	var (
		jsonData []byte
		resp     *http.Response
	)

	url := fmt.Sprintf(PWSURL, apiKey)

	if jsonData, err = json.Marshal(map[string]interface{}{
		"stations":     stations,
		"observations": observations,
	}); err != nil {
		return err
	}

	if resp, err = http.Post(url, "application/json; charset=UTF-8", bytes.NewBuffer(jsonData)); err != nil {
		return err
	}

	log.Printf("response status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Printf("response body: %s", string(body))

	return nil
}
