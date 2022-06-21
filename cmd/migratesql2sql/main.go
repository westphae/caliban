/*
One-time-use script to merge and tidy up early sqlite databases
*/
package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbFile0   string = "tempest0.db"
	dbFile1   string = "tempest1.db"
	dbFile2   string = "tempest2.db"
	dbFile3   string = "tempest3.db"
	dbFileOut string = "tempest.db"
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
	getObs0 string = `
SELECT 
timestamp,
windLull,
windAvg,
windGust,
windDirection,
windSampleInterval,
pressure,
airTemperature,
relativeHumidity,
illuminance,
uv,
solarRadiation,
rainAccumulation,
precipitationType,
averageStrikeDistance,
strikeCount,
batteryVolts,
reportInterval,
localDayRainAccumulation,
nCRainAccumulation,
localDayNCRainAccumulation,
precipitationAnalysisType
FROM observations ORDER BY timestamp;`
	getObs string = `
SELECT 
deviceId,
windLull,
windAvg,
windGust,
windDirection,
windSampleInterval,
pressure,
airTemperature,
relativeHumidity,
illuminance,
uv,
solarRadiation,
rainAccumulation,
precipitationType,
averageStrikeDistance,
strikeCount,
batteryVolts,
reportInterval,
localDayRainAccumulation,
nCRainAccumulation,
localDayNCRainAccumulation,
precipitationAnalysisType
FROM observations ORDER BY timestamp;`
	myDeviceId int = 204604
)

var (
	dbOut *sql.DB
)

func main() {
	var (
		dbIn *sql.DB
		err  error
		rows *sql.Rows

		timestamp                  int
		windLull                   float64
		windAvg                    float64
		windGust                   float64
		windDirection              int
		windSampleInterval         int
		pressure                   float64
		airTemperature             float64
		relativeHumidity           int
		illuminance                int
		uv                         float64
		solarRadiation             int
		rainAccumulation           int
		precipitationType          int
		averageStrikeDistance      int
		strikeCount                int
		batteryVolts               float64
		reportInterval             int
		localDayRainAccumulation   int
		nCRainAccumulation         int
		localDayNCRainAccumulation int
		precipitationAnalysisType  int
	)

	// Set up output database
	if dbOut, err = sql.Open("sqlite3", dbFileOut); err != nil {
		panic(err)
	}

	if _, err := dbOut.Exec(createObs); err != nil {
		panic(err)
	}

	// Transfer from dbs
	for i, dbFile := range []string{dbFile0, dbFile1, dbFile2, dbFile3} {
		log.Printf("reading from %s", dbFile)
		if dbIn, err = sql.Open("sqlite3", dbFile); err != nil {
			panic(err)
		}

		q := getObs
		if i == 0 {
			q = getObs0
		}
		rows, err = dbIn.Query(q)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			err = rows.Scan(
				&timestamp,
				&windLull,
				&windAvg,
				&windGust,
				&windDirection,
				&windSampleInterval,
				&pressure,
				&airTemperature,
				&relativeHumidity,
				&illuminance,
				&uv,
				&solarRadiation,
				&rainAccumulation,
				&precipitationType,
				&averageStrikeDistance,
				&strikeCount,
				&batteryVolts,
				&reportInterval,
				&localDayRainAccumulation,
				&nCRainAccumulation,
				&localDayNCRainAccumulation,
				&precipitationAnalysisType,
			)
			if err != nil {
				panic(err)
			}
			obs := []interface{}{
				myDeviceId,
				timestamp,
				windLull,
				windAvg,
				windGust,
				windDirection,
				windSampleInterval,
				pressure,
				airTemperature,
				relativeHumidity,
				illuminance,
				uv,
				solarRadiation,
				rainAccumulation,
				precipitationType,
				averageStrikeDistance,
				strikeCount,
				batteryVolts,
				reportInterval,
				localDayRainAccumulation,
				nCRainAccumulation,
				localDayNCRainAccumulation,
				precipitationAnalysisType,
			}
			fmt.Println(obs)
			if err = writeDbOut(obs); err != nil {
				panic(err)
			}
		}
	}
}

func writeDbOut(obs []interface{}) (err error) {
	// Write to output database
	res, err := dbOut.Exec(insertObs, obs...)
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
