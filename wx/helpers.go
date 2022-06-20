package wx

import (
	"database/sql"
	"github.com/westphae/caliban/tempest"
	"log"
	"math"
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
		obs.Timestamp,
		deviceId,
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
	if err != nil {
		log.Printf("%T: %s", err, err)
		return err
	}

	if _, err = res.LastInsertId(); err != nil {
		return err
	}
	return nil
}
