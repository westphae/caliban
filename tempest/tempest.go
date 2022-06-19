package tempest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var (
	RESTRootURL = "https://swd.weatherflow.com/swd/rest"
	stationURL  = "%s/stations?token=%s"
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

type Stations struct {
	Status   Status    `json:"status"`
	Stations []Station `json:"stations"`
}

func GetStations(token string) (station *Station, err error) {
	url := fmt.Sprintf(stationURL, RESTRootURL, token)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var stations Stations
	if err := json.Unmarshal(body, &stations); err != nil {
		return nil, err
	}

	if stations.Status.Code != 0 {
		return nil, fmt.Errorf("Tempest return error code %d: %s", stations.Status.Code, stations.Status.Message)
	}

	return &stations.Stations[0], nil
}
