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

	"github.com/spf13/viper"
	"github.com/westphae/caliban/tempest"
	"github.com/westphae/caliban/windy"
	"github.com/westphae/caliban/wx"
)

var (
	token          string
	deviceId       int
	windyApiKey    string
	windyStationID string
	windyV2        bool
	dbPath         string
)

func init() {
	viper.SetConfigName("caliban")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error in config file: %w", err))
	}

	token = viper.GetString("tempest-token")
	deviceId = viper.GetInt("tempest-deviceId")
	windyApiKey = viper.GetString("windy-apiKey")
	windyStationID = viper.GetString("windy-stationId")
	windyV2 = viper.GetBool("windy-v2")
	dbPath = viper.GetString("db-path")
	if dbPath == "" {
		dbPath = wx.DefaultPath()
	}
}

// reconnect backoff
const (
	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, err := wx.Open(dbPath)
	if err != nil {
		log.Fatalf("fatal: opening sqlite at %s: %s", dbPath, err)
	}
	defer store.Close()
	log.Printf("observations db: %s", dbPath)

	sender := windy.NewSender(windyApiKey)
	if windyV2 {
		if windyStationID == "" {
			log.Fatalf("fatal: windy-v2 enabled but windy-stationId is unset")
		}
		sender.EnableV2(windyStationID)
		log.Printf("using windy v2 endpoint with station id %s", windyStationID)
	} else {
		log.Println("using windy legacy PWS endpoint (sunsets end of 2026)")
	}

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

			if err := store.Save(deviceId, obs); err != nil {
				// Don't kill the daemon on a single failed write — log and move on.
				log.Printf("sqlite save failed: %s", err)
			}

			// Precip semantics differ across endpoints. v2 wants mm-since-local-
			// midnight (LocalDayRainAccumulation); the legacy endpoint historically
			// wanted mm-in-the-last-15-minutes — we send the per-minute bucket as
			// an approximation, matching long-standing behavior.
			precip := float64(obs.RainAccumulation)
			if windyV2 {
				precip = float64(obs.LocalDayRainAccumulation)
			}
			wObs := windy.Observation{
				TS:       obs.Timestamp,
				Temp:     obs.AirTemperature,
				Wind:     obs.WindAvg,
				WindDir:  obs.WindDirection,
				Gust:     obs.WindGust,
				RH:       obs.RelativeHumidity,
				Dewpoint: wx.Dewpoint(float64(obs.RelativeHumidity), obs.AirTemperature),
				Pressure: obs.Pressure,
				Precip:   precip,
				UV:       obs.UV,
			}
			switch err := sender.Send(wObs); {
			case err == nil:
				log.Printf("windy updated successfully (ts=%d)", wObs.TS)
			case errors.Is(err, windy.ErrThrottled):
				log.Printf("skipping windy upload: %s", err)
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
