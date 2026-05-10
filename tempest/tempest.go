package tempest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	RESTRootURL           = "https://swd.weatherflow.com/swd/rest"
	stationsURL           = "/stations"
	stationURL            = "/stations/%d"
	deviceObservationsURL = "/observations/device/%d"
	WSURL                 = "wss://ws.weatherflow.com/swd/data"
	maxId                 int64 // mutated via atomic.AddInt64; keeps the package goroutine-safe

	// HTTPClient is the client used for REST calls. Exported so tests can override.
	HTTPClient = &http.Client{Timeout: 10 * time.Second}

	wsHandshakeTimeout = 10 * time.Second
	wsReadTimeout      = 90 * time.Second
	wsPingInterval     = 30 * time.Second
)

type Status struct {
	Code    int    `json:"status_code"`
	Message string `json:"status_message"`
}

type StationMeta struct {
	ShareWithWf bool    `json:"share_with_wf"`
	ShareWithWU bool    `json:"share_with_wu"`
	Elevation   float64 `json:"elevation"`
}

type DeviceSettings struct {
	ShowPrecipFinal bool `json:"show_precip_final"`
}

type DeviceMeta struct {
	AGL             float64 `json:"agl"`
	Name            string  `json:"name"`
	Environment     string  `json:"environment"`
	WiFiNetworkName string  `json:"wifi_network_name"`
}

type Device struct {
	DeviceId         int            `json:"device_id"`
	SerialNumber     string         `json:"serial_number"`
	LocationId       int            `json:"location_id"`
	DeviceMeta       DeviceMeta     `json:"device_meta"`
	DeviceType       string         `json:"device_type"`
	HardwareRevision string         `json:"hardware_revision"`
	FirmwareRevision string         `json:"firmware_revision"`
	DeviceSettings   DeviceSettings `json:"device_settings"`
	Notes            string         `json:"notes"`
}

type Item struct {
	LocationItemId int    `json:"location_item_id"`
	LocationId     int    `json:"location_id"`
	DeviceId       int    `json:"device_id"`
	Item           string `json:"item"`
	Sort           int    `json:"sort"`
	StationId      int    `json:"station_id"`
	StationItemId  int    `json:"station_item_id"`
}

type Station struct {
	LocationId            int         `json:"location_id"`
	StationId             int         `json:"station_id"`
	Name                  string      `json:"name"`
	PublicName            string      `json:"public_name"`
	Latitude              float64     `json:"latitude"`
	Longitude             float64     `json:"longitude"`
	TimeZone              string      `json:"timezone"`
	TimeZoneOffsetMinutes int         `json:"timezone_offset_minutes"`
	StationMeta           StationMeta `json:"station_meta"`
	Devices               []Device    `json:"devices"`
	StationItems          []Item      `json:"station_items"`
	IsLocalMode           bool        `json:"is_local_mode"`
}

type StationsResult struct {
	Status   Status    `json:"status"`
	Stations []Station `json:"stations"`
}

type ObservationsResult struct {
	Status            Status      `json:"status"`
	DeviceId          int         `json:"device_id"`
	Type              string      `json:"type"`
	BucketStepMinutes int         `json:"bucket_step_minutes"`
	Source            string      `json:"source"`
	ObservationsRaw   [][]float64 `json:"obs"`
	Observations      []Observation
}

type WSReqMessage struct {
	Type     string `json:"type"`
	DeviceId int    `json:"device_id"`
	Id       string `json:"id"`
}

type Observation struct {
	Timestamp                  int64
	WindLull                   float64
	WindAvg                    float64
	WindGust                   float64
	WindDirection              int
	WindSampleInterval         int64
	Pressure                   float64
	AirTemperature             float64
	RelativeHumidity           int
	Illuminance                int
	UV                         float64
	SolarRadiation             int
	RainAccumulation           int
	PrecipitationType          int
	AverageStrikeDistance      int
	StrikeCount                int
	BatteryVolts               float64
	ReportInterval             int64
	LocalDayRainAccumulation   int
	NCRainAccumulation         int
	LocalDayNCRainAccumulation int
	PrecipitationAnalysisType  int
}

type WSRespMessage struct {
	Type            string      `json:"type"`
	Id              string      `json:"id"`
	DeviceId        int         `json:"device_id"`
	StationId       int         `json:"station_id"`
	Event           []int64     `json:"evt"`
	ObservationsRaw [][]float64 `json:"obs"`
	Observations    []Observation
}

func RawToObs(raw []float64) (obs Observation) {
	return Observation{
		int64(raw[0]),
		raw[1],
		raw[2],
		raw[3],
		int(raw[4]),
		int64(raw[5]),
		raw[6],
		raw[7],
		int(raw[8]),
		int(raw[9]),
		raw[10],
		int(raw[11]),
		int(raw[12]),
		int(raw[13]),
		int(raw[14]),
		int(raw[15]),
		raw[16],
		int64(raw[17]),
		int(raw[18]),
		int(raw[19]),
		int(raw[20]),
		int(raw[21]),
	}
}

func GetStations(token string) (stationList []Station, err error) {
	u, err := url.Parse(RESTRootURL)
	if err != nil {
		return nil, err
	}

	u.Path += stationsURL
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	resp, err := HTTPClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var stations StationsResult
	if err := json.Unmarshal(body, &stations); err != nil {
		return nil, err
	}

	if stations.Status.Code != 0 {
		return nil, fmt.Errorf("tempest return error code %d: %s", stations.Status.Code, stations.Status.Message)
	}

	return stations.Stations, nil
}
func GetStation(token string, stationId int) (station *Station, err error) {
	u, err := url.Parse(RESTRootURL)
	if err != nil {
		return nil, err
	}

	u.Path += fmt.Sprintf(stationURL, stationId)
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	resp, err := HTTPClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var stations StationsResult
	if err := json.Unmarshal(body, &stations); err != nil {
		return nil, err
	}

	if stations.Status.Code != 0 {
		return nil, fmt.Errorf("tempest return error code %d: %s", stations.Status.Code, stations.Status.Message)
	}

	return &stations.Stations[0], nil
}

func GetDeviceObservations(token string, deviceId int, timeStart, timeEnd int64) (obs []Observation, err error) {
	u, err := url.Parse(RESTRootURL)
	if err != nil {
		return nil, err
	}

	u.Path += fmt.Sprintf(deviceObservationsURL, deviceId)
	q := u.Query()
	q.Set("token", token)
	if timeStart > 0 && timeEnd > 0 {
		q.Set("time_start", fmt.Sprintf("%d", timeStart))
		q.Set("time_end", fmt.Sprintf("%d", timeEnd))
	}
	u.RawQuery = q.Encode()

	resp, err := HTTPClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var obsResult ObservationsResult
	if err := json.Unmarshal(body, &obsResult); err != nil {
		return nil, err
	}

	if obsResult.Status.Code != 0 {
		return nil, fmt.Errorf("tempest return error code %d: %s", obsResult.Status.Code, obsResult.Status.Message)
	}

	if obsResult.DeviceId != deviceId {
		return nil, fmt.Errorf("received deviceId %d, requested %d", obsResult.DeviceId, deviceId)
	}

	if obsResult.Type != "obs_st" {
		log.Printf("received observation type %s, expected obs_st", obsResult.Type)
	}
	if obsResult.BucketStepMinutes != 0 {
		log.Printf("received bucket_step_minutes %d, expecting 1", obsResult.BucketStepMinutes)
	}
	if obsResult.Source != "db" && obsResult.Source != "cache" {
		log.Printf("received source %s, expecting db", obsResult.Source)
	}

	obs = make([]Observation, len(obsResult.ObservationsRaw))
	for i, v := range obsResult.ObservationsRaw {
		obs[i] = RawToObs(v)
	}
	return obs, nil
}

// SubscribeObservations opens a Tempest WebSocket subscription for the given
// device. Observations are delivered on the returned channel. The channel is
// closed when the connection ends, either because ctx was cancelled or the
// underlying socket failed; callers should range over it and reconnect on
// close. A read deadline plus periodic client-side pings detect half-open
// connections that would otherwise hang silently.
func SubscribeObservations(ctx context.Context, token string, deviceId int) (<-chan Observation, error) {
	u, err := url.Parse(WSURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = wsHandshakeTimeout
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resetReadDeadline := func() { _ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout)) }
	resetReadDeadline()
	conn.SetPongHandler(func(string) error { resetReadDeadline(); return nil })

	// Read connection_opened.
	var msg WSRespMessage
	if err = conn.ReadJSON(&msg); err != nil {
		conn.Close()
		return nil, err
	}
	if msg.Type != "connection_opened" {
		conn.Close()
		return nil, fmt.Errorf("received message type %s, expecting connection_opened", msg.Type)
	}
	log.Printf("connected to tempest ws: %+v", msg)

	// Subscribe.
	subId := atomic.AddInt64(&maxId, 1)
	startReq := WSReqMessage{Type: "listen_start", DeviceId: deviceId, Id: fmt.Sprintf("%d", subId)}
	startJSON, err := json.Marshal(startReq)
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(wsHandshakeTimeout))
	if err = conn.WriteMessage(websocket.TextMessage, startJSON); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetWriteDeadline(time.Time{}) // clear write deadline; pings set their own
	if err = conn.ReadJSON(&msg); err != nil {
		conn.Close()
		return nil, err
	}
	if msg.Type != "ack" {
		conn.Close()
		return nil, fmt.Errorf("received message type %s, expecting ack", msg.Type)
	}
	log.Printf("subscribed tempest: %+v", msg)

	ch := make(chan Observation)
	done := make(chan struct{})

	// ctx-watcher: closing conn unblocks any in-flight read with an error.
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		conn.Close()
	}()

	// pinger: keeps the server-side idle timer happy and exercises the round
	// trip, so a half-open connection trips our read deadline within ~90s.
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				deadline := time.Now().Add(wsHandshakeTimeout)
				if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	go func() {
		defer close(ch)
		defer close(done)
		for {
			var rxMsg WSRespMessage
			if err := conn.ReadJSON(&rxMsg); err != nil {
				if ctx.Err() != nil {
					log.Printf("tempest ws: shutting down (%s)", ctx.Err())
				} else {
					log.Printf("tempest ws read error: %s", err)
				}
				return
			}
			resetReadDeadline()
			if rxMsg.Type != "obs_st" {
				log.Printf("tempest ws: ignoring %s message", rxMsg.Type)
				continue
			}
			for _, v := range rxMsg.ObservationsRaw {
				select {
				case ch <- RawToObs(v):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}
