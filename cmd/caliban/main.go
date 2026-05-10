package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"github.com/westphae/caliban/tempest"
	"github.com/westphae/caliban/windy"
	"github.com/westphae/caliban/wx"
)

var (
	token          string
	stationId      int
	deviceId       int
	windyApiKey    string
	windyStationId string
)

func init() {
	viper.SetConfigName("caliban")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error in config file: %w", err))
	}

	token = viper.GetString("tempest-token")
	stationId = viper.GetInt("tempest-stationId")
	deviceId = viper.GetInt("tempest-deviceId")
	windyApiKey = viper.GetString("windy-apiKey")
	windyStationId = viper.GetString("windy-stationId")
}

// reconnect backoff
const (
	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	s, err := tempest.GetStation(token, stationId)
	if err != nil {
		log.Fatalf("fatal: getting tempest station %d: %s", stationId, err)
	}

	station := windy.Station{
		Name:        s.PublicName,
		ShareOption: "Open",
		Latitude:    s.Latitude,
		Longitude:   s.Longitude,
		Elevation:   s.StationMeta.Elevation,
		TempHeight:  s.StationMeta.Elevation,
		WindHeight:  s.StationMeta.Elevation,
	}

	var lastTimestamp int64
	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			log.Printf("shutting down: %s", ctx.Err())
			return
		}

		obsCh, err := tempest.SubscribeObservations(ctx, token, deviceId)
		if err != nil {
			log.Printf("subscribe failed: %s; retrying in ~%s", err, backoff)
			if !sleepCtx(ctx, jitter(backoff)) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		log.Printf("client subscribed to tempest, listening...")
		backoff = minBackoff // reset on a healthy subscription

		i := 0
		for obs := range obsCh {
			i++
			log.Printf("client received tempest message %d: %+v", i, obs)

			if err := wx.SaveTempestDataToDb(deviceId, obs); err != nil {
				// Don't kill the daemon on a single failed write — log and move on.
				log.Printf("sqlite save failed: %s", err)
			}

			// Windy only wants data every 5 minutes
			if dts := obs.Timestamp - lastTimestamp; dts < 300 {
				log.Printf("not updating windy, time diff is only %d sec", dts)
				continue
			}

			observation := windy.Observation{
				TS:       obs.Timestamp,
				Temp:     obs.AirTemperature,
				Wind:     obs.WindAvg,
				WindDir:  obs.WindDirection,
				Gust:     obs.WindGust,
				RH:       obs.RelativeHumidity,
				Dewpoint: wx.Dewpoint(float64(obs.RelativeHumidity), obs.AirTemperature),
				Pressure: obs.Pressure,
				Precip:   float64(obs.RainAccumulation),
				UV:       obs.UV,
			}
			log.Printf("sending to windy: %+v", observation)

			err := windy.SendToWindy(windyApiKey, []windy.Station{station}, []windy.Observation{observation})
			switch {
			case err == nil:
				log.Println("windy updated successfully")
				lastTimestamp = obs.Timestamp
			case errors.Is(err, windy.ErrThrottled):
				log.Println(err)
			default:
				log.Printf("windy update failed: %s", err)
			}
		}

		log.Println("tempest subscription ended; reconnecting")
	}
}

// nextBackoff doubles cur, capped at maxBackoff.
func nextBackoff(cur time.Duration) time.Duration {
	cur *= 2
	if cur > maxBackoff {
		cur = maxBackoff
	}
	return cur
}

// jitter returns a duration uniformly distributed in [0, d) — full-jitter
// backoff per AWS architecture blog. Avoids a thundering herd if Tempest's
// load balancer evicts every connected client at once.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(d)))
}

// sleepCtx sleeps for d, returning false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
