package wx

import (
	"database/sql"
	"log"
	"math"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/westphae/caliban/tempest"
)

const (
	dbFile    string = "tempest.db"
	createObs string = `
CREATE TABLE IF NOT EXISTS observations (
deviceId INTEGER NOT NULL,
timestamp INTEGER NOT NULL,
windLull REAL,
windAvg REAL,
windGust REAL,
windDirection INTEGER,
windSampleInterval INTEGER,
pressure REAL,
airTemperature REAL,
relativeHumidity INTEGER,
illuminance INTEGER,
uv REAL,
solarRadiation INTEGER,
rainAccumulation INTEGER,
precipitationType INTEGER,
averageStrikeDistance INTEGER,
strikeCount INTEGER,
batteryVolts REAL,
reportInterval INTEGER,
localDayRainAccumulation INTEGER,
nCRainAccumulation INTEGER,
localDayNCRainAccumulation INTEGER,
precipitationAnalysisType INTEGER,
PRIMARY KEY (deviceId, timestamp)
);`
	insertObs string = `
INSERT INTO observations VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);`
	getObs string = `SELECT * FROM observations WHERE deviceId = ? AND timestamp>= ? AND timestamp < ? ORDER BY timestamp;`
)

var (
	db *sql.DB
)

func Dewpoint(rh, t float64) (td float64) {
	rr := (17.625 * t) / (243.04 + t)
	lrh := math.Log(rh / 100)
	return 243.04 * (lrh + rr) / (17.625 - lrh - rr)
}

func init() {
	// Set up database
	var err error
	if db, err = sql.Open("sqlite3", dbFile); err != nil {
		panic(err)
	}

	if _, err := db.Exec(createObs); err != nil {
		panic(err)
	}
}

func SaveTempestDataToDb(deviceId int, obs tempest.Observation) (err error) {
	res, err := db.Exec(insertObs,
		deviceId,
		obs.Timestamp,
		obs.WindLull,
		obs.WindAvg,
		obs.WindGust,
		obs.WindDirection,
		obs.WindSampleInterval,
		obs.Pressure,
		obs.AirTemperature,
		obs.RelativeHumidity,
		obs.Illuminance,
		obs.UV,
		obs.SolarRadiation,
		obs.RainAccumulation,
		obs.PrecipitationType,
		obs.AverageStrikeDistance,
		obs.StrikeCount,
		obs.BatteryVolts,
		obs.ReportInterval,
		obs.LocalDayRainAccumulation,
		obs.NCRainAccumulation,
		obs.LocalDayNCRainAccumulation,
		obs.PrecipitationAnalysisType,
	)
	switch {
	case err == nil:
		log.Println("saved tempest data to sqlite db")
	case strings.HasPrefix(err.Error(), "UNIQUE constraint failed"):
		log.Println("observation already in sqlite db")
		return nil
	default:
		return err
	}

	if _, err = res.LastInsertId(); err != nil {
		return err
	}
	return nil
}

func GetTempestDataFromDb(deviceId int, tsStart, tsEnd int64) (obs []tempest.Observation, err error) {
	var (
		d int
	)
	rows, err := db.Query(getObs, deviceId, tsStart, tsEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	o := tempest.Observation{}
	for rows.Next() {
		err = rows.Scan(
			&d,
			&o.Timestamp,
			&o.WindLull,
			&o.WindAvg,
			&o.WindGust,
			&o.WindDirection,
			&o.WindSampleInterval,
			&o.Pressure,
			&o.AirTemperature,
			&o.RelativeHumidity,
			&o.Illuminance,
			&o.UV,
			&o.SolarRadiation,
			&o.RainAccumulation,
			&o.PrecipitationType,
			&o.AverageStrikeDistance,
			&o.StrikeCount,
			&o.BatteryVolts,
			&o.ReportInterval,
			&o.LocalDayRainAccumulation,
			&o.NCRainAccumulation,
			&o.LocalDayNCRainAccumulation,
			&o.PrecipitationAnalysisType,
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}

	return obs, nil
}
