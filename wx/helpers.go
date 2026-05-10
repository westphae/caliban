package wx

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/westphae/caliban/tempest"
)

// columns is the canonical list of observation columns in the order they
// appear in the table and in INSERT/SELECT statements. Update both this list
// and the matching field bindings in Save / Get if the schema changes.
var columns = []string{
	"deviceId",
	"timestamp",
	"windLull",
	"windAvg",
	"windGust",
	"windDirection",
	"windSampleInterval",
	"pressure",
	"airTemperature",
	"relativeHumidity",
	"illuminance",
	"uv",
	"solarRadiation",
	"rainAccumulation",
	"precipitationType",
	"averageStrikeDistance",
	"strikeCount",
	"batteryVolts",
	"reportInterval",
	"localDayRainAccumulation",
	"nCRainAccumulation",
	"localDayNCRainAccumulation",
	"precipitationAnalysisType",
}

const createObs = `
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

// Store wraps the sqlite database. The zero value is not usable; construct
// with Open.
type Store struct {
	db        *sql.DB
	insertSQL string
	selectSQL string
}

// DefaultPath returns the platform-default location for the observations
// database, following the XDG Base Directory specification: $XDG_DATA_HOME (if
// set) or $HOME/.local/share, with caliban/tempest.db appended. XDG_DATA_HOME
// is unset by default on macOS and most Linux setups, so this typically
// resolves to ~/.local/share/caliban/tempest.db.
func DefaultPath() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(dir, "caliban", "tempest.db")
}

// Open initialises the observations table at path, creating the parent
// directory if needed.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("wx: creating db directory: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(createObs); err != nil {
		db.Close()
		return nil, err
	}

	placeholders := ""
	for i := range columns {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
	}
	colList := ""
	for i, c := range columns {
		if i > 0 {
			colList += ", "
		}
		colList += c
	}

	return &Store{
		db: db,
		// INSERT OR IGNORE silently drops duplicate (deviceId, timestamp) rows
		// instead of returning a UNIQUE-constraint error we'd have to detect by
		// string-matching.
		insertSQL: fmt.Sprintf("INSERT OR IGNORE INTO observations (%s) VALUES (%s);", colList, placeholders),
		selectSQL: fmt.Sprintf("SELECT %s FROM observations WHERE deviceId = ? AND timestamp >= ? AND timestamp < ? ORDER BY timestamp;", colList),
	}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// Dewpoint computes dewpoint in Celsius from relative humidity (%) and dry-
// bulb temperature (°C) using the Magnus-Tetens approximation.
func Dewpoint(rh, t float64) (td float64) {
	rr := (17.625 * t) / (243.04 + t)
	lrh := math.Log(rh / 100)
	return 243.04 * (lrh + rr) / (17.625 - lrh - rr)
}

// Save inserts obs for deviceId, ignoring duplicate primary keys.
func (s *Store) Save(deviceId int, obs tempest.Observation) error {
	_, err := s.db.Exec(s.insertSQL,
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
	return err
}

// Get returns observations for deviceId in [tsStart, tsEnd), ordered by
// timestamp.
func (s *Store) Get(deviceId int, tsStart, tsEnd int64) ([]tempest.Observation, error) {
	rows, err := s.db.Query(s.selectSQL, deviceId, tsStart, tsEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []tempest.Observation
	for rows.Next() {
		var (
			d int
			o tempest.Observation
		)
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
