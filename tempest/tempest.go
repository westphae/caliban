package tempest

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

var (
	RESTRootURL           = "https://swd.weatherflow.com/swd/rest"
	stationsURL           = "/stations"
	stationURL            = "/stations/%d"
	deviceObservationsURL = "/observations/device/%d"
	WSURL                 = "wss://ws.weatherflow.com/swd/data"
	maxId                 = 0
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
	Event           []string    `json:"evt"`
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

	resp, err := http.Get(u.String())
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

	resp, err := http.Get(u.String())
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
		q.Set("timeStart", fmt.Sprintf("%d", timeStart))
		q.Set("time_end", fmt.Sprintf("%d", timeEnd))
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
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
		return nil, fmt.Errorf("received observation type %s, expected obs_st", obsResult.Type)
	}
	if obsResult.BucketStepMinutes != 0 {
		return nil, fmt.Errorf("received bucket_step_minutes %d, expecting 1", obsResult.BucketStepMinutes)
	}
	if obsResult.Source != "db" && obsResult.Source != "cache" {
		return nil, fmt.Errorf("received source %s, expecting db", obsResult.Source)
	}

	obs = make([]Observation, len(obsResult.ObservationsRaw))
	for i, v := range obsResult.ObservationsRaw {
		obs[i] = RawToObs(v)
	}
	return obs, nil
}

func SubscribeObservations(token string, deviceId int) (ch chan Observation, err error) {
	var (
		conn    *websocket.Conn
		reqJson []byte
		msg     WSRespMessage
		req     WSReqMessage
	)

	// Connect
	u, err := url.Parse(WSURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	if conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil); err != nil {
		return nil, err
	}
	log.Println("connected to tempest ws")
	if err = conn.ReadJSON(&msg); err != nil {
		conn.Close()
		return nil, err
	}
	if msg.Type != "connection_opened" {
		log.Printf("%+v", msg)
		return nil, fmt.Errorf("received message type %s, expecting connection_opened", msg.Type)
	}
	log.Printf("received connection_opened from tempest: %+v", msg)

	// Subscribe
	req = WSReqMessage{
		Type:     "listen_start",
		DeviceId: deviceId,
		Id:       fmt.Sprintf("%d", maxId),
	}
	maxId += 1
	if reqJson, err = json.Marshal(req); err != nil {
		return nil, err
	}
	if err = conn.WriteMessage(websocket.TextMessage, reqJson); err != nil {
		return nil, err
	}
	log.Printf("sent listen_start message to tempest %+v", req)
	if err = conn.ReadJSON(&msg); err != nil {
		conn.Close()
		return nil, err
	}
	if msg.Type != "ack" {
		log.Printf("%+v", msg)
		return nil, fmt.Errorf("received message type %s, expecting ack", msg.Type)
	}
	log.Printf("subscribed tempest: %+v", msg)

	ch = make(chan Observation)

	go func() {
		for {
			log.Println("waiting for tempest message")
			if err = conn.ReadJSON(&msg); err != nil {
				close(ch)
				break
			}
			log.Printf("received tempest message %+v", msg)
			if msg.Type != "obs_st" {
				log.Printf("Unexpected msg type received from tempest: %+v", msg)
				continue
			}
			for _, v := range msg.ObservationsRaw {
				ch <- RawToObs(v)
			}
			log.Printf("tempest message sent to client")
		}
		log.Println("closing tempest ws connection and channel")
		req = WSReqMessage{
			Type:     "listen_stop",
			DeviceId: deviceId,
			Id:       fmt.Sprintf("%d", maxId),
		}
		if reqJson, err = json.Marshal(req); err != nil {
			log.Println(err)
		}
		if err = conn.WriteMessage(websocket.TextMessage, reqJson); err != nil {
			log.Println(err)
		}
		conn.Close()
		log.Println("goodbye tempest!")
	}()

	return ch, nil
}
