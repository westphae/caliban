package windy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// PWSURL is the legacy ("PWS") update endpoint. Windy will retire this at
	// the end of 2026 — see https://stations.windy.com/api-reference.
	PWSURL string = "https://stations.windy.com/pws/update/%s"

	// V2URL is the current ("v2") observation update endpoint. Takes a GET with
	// query parameters and an Authorization: Bearer header.
	V2URL = "https://stations.windy.com/api/v2/observation/update"

	// HTTPClient is used for all windy uploads. Exported so tests / callers can
	// override (e.g. to inject a transport).
	HTTPClient = &http.Client{Timeout: 10 * time.Second}

	// SoftwareType identifies the client in v2 uploads.
	SoftwareType = "caliban"
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
	Pressure     float64 `json:"pressure,omitempty"` // pascals
	MBar         float64 `json:"mbar,omitempty"`
	BaromIn      float64 `json:"baromin,omitempty"`
	Precip       float64 `json:"precip,omitempty"`
	RainIn       float64 `json:"rainin,omitempty"`
	UV           float64 `json:"uv,omitempty"`
}

// minWindyInterval is the minimum allowed interval between windy uploads for a
// single station, per windy's PWS API rules.
const minWindyInterval = 5 * 60 // seconds

// Sender posts observations to Windy for a single station, enforcing the
// 5-minute minimum-interval rule before any HTTP call. Zero value is not
// usable; construct with NewSender.
//
// By default Sender targets the legacy PWS endpoint. Call EnableV2 to switch
// to Windy's v2 API (which the legacy one is being retired in favour of at
// the end of 2026). The two endpoints differ in transport (POST/JSON vs
// GET/query params), authentication (account-wide API key vs per-station
// password — separate credentials in Windy's model), and one semantic
// detail callers must handle: the v2 `precip` field is millimetres since
// local midnight, whereas legacy `precip` was millimetres in the last
// fifteen minutes. Sender does not transform observation values — the caller
// is responsible for putting the right rain accumulation in Observation.Precip.
type Sender struct {
	apiKey     string
	v2ID       string // non-empty enables v2
	v2Password string // per-station password used as v2 Bearer credential
	lastSent   int64  // unix seconds of the last successful (non-throttled) upload
}

// NewSender returns a Sender bound to apiKey. Station definition (lat/lon/
// name/elevation/etc.) is no longer transmitted on observation upload —
// Windy rejected that as deprecated in early 2026 — so the caller doesn't
// need to supply it. Stations are now configured once via the Windy
// dashboard or PUT /api/v2/pws/:id (out of scope for this client).
func NewSender(apiKey string) *Sender {
	return &Sender{apiKey: apiKey}
}

// EnableV2 switches this Sender to Windy's v2 endpoint. stationID is the
// per-station identifier from the Windy dashboard (e.g. "f07f453a"), passed
// in the `id` query parameter. password is the station's auto-generated
// password (visible on the My Stations page, distinct from the account API
// key) and is sent as the Bearer token. v2 will not accept the account API
// key here — the request fails with "Provided password is invalid".
func (s *Sender) EnableV2(stationID, password string) {
	s.v2ID = stationID
	s.v2Password = password
}

// Send uploads obs unless less than five minutes have elapsed since the
// previous successful send, in which case it returns ErrThrottled.
//
// On a server-side throttle response we also bump lastSent: the daemon's
// in-memory window resets to zero on every restart, so on cold start the
// server may reject the first observation as too soon. Treating that as a
// soft throttle (and aligning lastSent with the server's view) avoids
// firing the next 4–5 obs straight at a wall before reconverging.
func (s *Sender) Send(obs Observation) error {
	if obs.TS-s.lastSent < minWindyInterval {
		return ErrThrottled
	}
	var err error
	if s.v2ID != "" {
		err = postV2(s.v2Password, s.v2ID, obs)
	} else {
		err = postLegacy(s.apiKey, obs)
	}
	if err == nil || errors.Is(err, ErrThrottled) {
		s.lastSent = obs.TS
	}
	return err
}

// isThrottleBody returns true if Windy's response body indicates the upload
// was rejected for being too close to the previous one. Recognises both the
// historical "less than 5 minutes" wording and the post-2026 "Measurement
// sent too soon, update interval is 5 minutes" wording.
func isThrottleBody(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "too soon") || strings.Contains(lower, "5 minute")
}

func postLegacy(apiKey string, obs Observation) error {
	// As of early 2026 the legacy endpoint returns 410 if the request body
	// includes a "stations" property. Send observations only.
	jsonData, err := json.Marshal(map[string]interface{}{
		"observations": []Observation{obs},
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

	// Throttle responses can come back with either status 200 (legacy plain
	// text) or 400 (current JSON envelope), so check the body before the
	// status code.
	if isThrottleBody(bodyStr) {
		return ErrThrottled
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("windy: HTTP %d: %s", resp.StatusCode, bodyStr)
	}
	if bodyStr == "SUCCESS" {
		return nil
	}
	// As of early 2026 the legacy endpoint started replying with a JSON
	// envelope ({"update":{...,"errors":{"observations":[...]}}, ...}) instead
	// of the historical plain "SUCCESS". Empty errors.observations means
	// every field validated; non-empty means at least one field was rejected
	// (the rest may still have been saved server-side).
	if strings.HasPrefix(bodyStr, "{") {
		var rsp struct {
			Update struct {
				Errors struct {
					Observations []json.RawMessage `json:"observations"`
				} `json:"errors"`
			} `json:"update"`
		}
		if err := json.Unmarshal(body, &rsp); err == nil {
			if len(rsp.Update.Errors.Observations) == 0 {
				return nil
			}
			return fmt.Errorf("windy: validation errors: %s", string(rsp.Update.Errors.Observations[0]))
		}
	}
	return fmt.Errorf("windy: unexpected response: %s", bodyStr)
}

// postV2 uploads obs via the v2 GET endpoint. Auth is the per-station
// password sent as a Bearer token (NOT the account API key — Windy treats
// the two as separate credentials in v2). The station identifier is the
// `id` query parameter.
func postV2(password, stationID string, obs Observation) error {
	q := url.Values{}
	q.Set("id", stationID)
	q.Set("ts", strconv.FormatInt(obs.TS, 10))
	q.Set("temp", strconv.FormatFloat(obs.Temp, 'f', -1, 64))
	q.Set("dewpoint", strconv.FormatFloat(obs.Dewpoint, 'f', -1, 64))
	q.Set("humidity", strconv.Itoa(obs.RH))
	q.Set("wind", strconv.FormatFloat(obs.Wind, 'f', -1, 64))
	q.Set("gust", strconv.FormatFloat(obs.Gust, 'f', -1, 64))
	q.Set("winddir", strconv.Itoa(obs.WindDir))
	q.Set("pressure", strconv.FormatFloat(obs.Pressure, 'f', -1, 64))
	q.Set("precip", strconv.FormatFloat(obs.Precip, 'f', -1, 64))
	q.Set("uv", strconv.FormatFloat(obs.UV, 'f', -1, 64))
	q.Set("softwaretype", SoftwareType)

	req, err := http.NewRequest(http.MethodGet, V2URL+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+password)
	req.Header.Set("Accept", "application/json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("windy v2: reading response body: %w", err)
	}
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests || isThrottleBody(bodyStr) {
		return ErrThrottled
	}
	return fmt.Errorf("windy v2: HTTP %d: %s", resp.StatusCode, bodyStr)
}
