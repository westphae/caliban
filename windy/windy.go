package windy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	PWSURL string = "https://stations.windy.com/pws/update/%s"

	// HTTPClient is used for all windy uploads. Exported so tests / callers can
	// override (e.g. to inject a transport).
	HTTPClient = &http.Client{Timeout: 10 * time.Second}
)

// ErrThrottled is returned by Windy when an update arrives less than five
// minutes after the previous one for the same station. Callers should treat
// this as a no-op, not a failure. Use errors.Is to match.
var ErrThrottled = errors.New("windy: rejected update (throttled, <5 min since last)")

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

func SendToWindy(apiKey string, stations []Station, observations []Observation) error {
	jsonData, err := json.Marshal(map[string]interface{}{
		"stations":     stations,
		"observations": observations,
	})
	if err != nil {
		return err
	}

	resp, err := HTTPClient.Post(fmt.Sprintf(PWSURL, apiKey), "application/json; charset=UTF-8", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("windy: reading response body: %w", err)
	}
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("windy: HTTP %d: %s", resp.StatusCode, bodyStr)
	}
	if bodyStr == "SUCCESS" {
		return nil
	}
	// Legacy endpoint returns 200 with a non-SUCCESS body when it rejects an
	// update for being too soon. Anything else with a non-SUCCESS body we
	// surface as a generic error including the body so the cause is visible.
	if strings.Contains(strings.ToLower(bodyStr), "minute") {
		return ErrThrottled
	}
	return fmt.Errorf("windy: unexpected response: %s", bodyStr)
}
