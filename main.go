package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FlowingSPDG/streamdeck"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type Settings struct {
	PrometheusEndpoint string `json:"prometheusEndpoint,omitempty"`
	PrometheusUsername string `json:"prometheusUsername,omitempty"`
	PrometheusPassword string `json:"prometheusPassword,omitempty"`
	PrometheusQuery    string `json:"prometheusQuery,omitempty"`
	Threshold          string `json:"threshold,omitempty"`
}

func coordsKey(c streamdeck.Coordinates) string {
	return fmt.Sprintf("%d:%d", c.Row, c.Column)
}

const (
	defaultPrometheusInterval = 60 // seconds
	defaultPrometheusTimeout  = 10 // seconds
	imgWidth                  = 72
	imgHeight                 = 72
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exeDir := filepath.Dir(exePath)
	f, err := os.Create(path.Join(exeDir, "streamdeck-grafana.log"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	log.SetOutput(f)
	//log.SetOutput(os.Stdout)

	log.Println("Initializing streamdeck-grafana plugin")

	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("%v\n", err)
	}
}

func run(ctx context.Context) error {
	params, err := streamdeck.ParseRegistrationParams(os.Args)
	if err != nil {
		return err
	}

	client := streamdeck.NewClient(ctx, params)
	setup(client)

	return client.Run(ctx)
}

func setup(client *streamdeck.Client) {
	actionName := "dev.mtanda.streamdeck.grafana.stat"
	action := client.Action(actionName)

	type monitor struct {
		cancel      context.CancelFunc
		settingsChan chan Settings
	}
	monitors := make(map[string]*monitor)
	var mu sync.Mutex

	streamdeck.RegisterTypedHandler(action, streamdeck.WillAppear, func(ctx context.Context, client *streamdeck.Client, p streamdeck.WillAppearPayload[Settings]) error {
		log.Println("WillAppear", p.Coordinates)

		key := coordsKey(p.Coordinates)

		mu.Lock()
		if m, ok := monitors[key]; ok {
			m.cancel()
			close(m.settingsChan)
			delete(monitors, key)
		}

		settingsChan := make(chan Settings, 10)
		// send initial settings (non-blocking in case monitor not started yet)
		select {
		case settingsChan <- p.Settings:
		default:
		}
		monitorCtx, cancel := context.WithCancel(ctx)
		monitors[key] = &monitor{cancel: cancel, settingsChan: settingsChan}
		mu.Unlock()

		go statMonitor(monitorCtx, client, settingsChan)

		return nil
	})

	streamdeck.RegisterTypedHandler(action, streamdeck.DidReceiveSettings, func(ctx context.Context, client *streamdeck.Client, p streamdeck.DidReceiveSettingsPayload[Settings]) error {
	log.Println("DidReceiveSettings", p.Coordinates)
	key := coordsKey(p.Coordinates)
		mu.Lock()
		if m, ok := monitors[key]; ok {
			select {
			case m.settingsChan <- p.Settings:
			default:
			}
		} else {
			log.Println("DidReceiveSettings: no monitor for", key)
		}
		mu.Unlock()
		return nil
	})

	streamdeck.RegisterTypedHandler(action, streamdeck.WillDisappear, func(ctx context.Context, client *streamdeck.Client, p streamdeck.WillDisappearPayload[Settings]) error {
	log.Println("WillDisappear", p.Coordinates)
	key := coordsKey(p.Coordinates)
		mu.Lock()
		if m, ok := monitors[key]; ok {
			m.cancel()
			close(m.settingsChan)
			delete(monitors, key)
		}
		mu.Unlock()

		return nil
	})
}

func statMonitor(ctx context.Context, client *streamdeck.Client, settingsChan <-chan Settings) {
	var settings Settings
	ticker := time.NewTicker(time.Second * defaultPrometheusInterval)
	defer ticker.Stop()

	for {
		select {
		case s := <-settingsChan:
			log.Println("Updated settings")
			settings = s
			go func() {
				if err := updateDisplay(ctx, client, settings); err != nil {
					log.Printf("error updating display: %v\n", err)
				}
			}()
		case <-ticker.C:
			log.Println("Update value")
			if err := updateDisplay(ctx, client, settings); err != nil {
				log.Printf("error updating display: %v\n", err)
			}
		case <-ctx.Done():
			log.Println("Stop monitor")
			return
		}
	}
}

func updateDisplay(ctx context.Context, client *streamdeck.Client, settings Settings) error {
	result, err := queryPrometheus(
		ctx,
		settings.PrometheusEndpoint,
		settings.PrometheusUsername,
		settings.PrometheusPassword,
		settings.PrometheusQuery,
	)
	if err != nil {
		return fmt.Errorf("error querying prometheus: %w", err)
	}

	backgroundColor := getBackgroundColor(float64(result.Value), settings)
	img, err := streamdeck.Image(background(backgroundColor))
	if err != nil {
		return fmt.Errorf("error creating image: %w", err)
	}

	if err := client.SetImage(ctx, img, streamdeck.HardwareAndSoftware); err != nil {
		return fmt.Errorf("error setting image: %w", err)
	}

	title := fmt.Sprintf("%.1f", result.Value)
	if err := client.SetTitle(ctx, title, streamdeck.HardwareAndSoftware); err != nil {
		return fmt.Errorf("error setting title: %w", err)
	}

	return nil
}

func getBackgroundColor(value float64, settings Settings) color.RGBA {
	greenColor := color.RGBA{0x6a, 0xb7, 0x4b, 0xff}
	orangeColor := color.RGBA{0xf4, 0x81, 0x18, 0xff}
	redColor := color.RGBA{0xe5, 0x5a, 0x4e, 0xff}

	if settings.Threshold != "" {
		thresholds := strings.Split(settings.Threshold, ",")
		if len(thresholds) >= 2 {
			warnStr := strings.TrimSpace(thresholds[0])
			if warnThreshold, err := strconv.ParseFloat(warnStr, 64); err == nil {
				critStr := strings.TrimSpace(thresholds[1])
				if critThreshold, err := strconv.ParseFloat(critStr, 64); err == nil {
					if value >= critThreshold {
						return redColor
					} else if value >= warnThreshold {
						return orangeColor
					}
				}
			}
		} else if len(thresholds) == 1 {
			warnStr := strings.TrimSpace(thresholds[0])
			if warnThreshold, err := strconv.ParseFloat(warnStr, 64); err == nil {
				if value >= warnThreshold {
					return orangeColor
				}
			}
		}
	}

	return greenColor
}

func queryPrometheus(ctx context.Context, endpoint, username, password, query string) (*model.Sample, error) {
	if endpoint == "" || username == "" || password == "" || query == "" {
		return nil, fmt.Errorf("prometheus endpoint, username, password, and query must be set")
	}

	client, err := api.NewClient(api.Config{
		Address: endpoint,
		RoundTripper: config.NewBasicAuthRoundTripper(
			config.NewInlineSecret(username),
			config.NewInlineSecret(password),
			api.DefaultRoundTripper,
		),
	})
	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	result, warnings, err := v1api.Query(ctx, query, time.Now(), v1.WithTimeout(time.Second*defaultPrometheusTimeout))
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	if result == nil {
		return nil, fmt.Errorf("no result")
	}
	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("expected vector result, got %s", result.Type().String())
	}

	vector := result.(model.Vector)
	// return the last sample
	return vector[len(vector)-1], nil
}

func background(color color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	for x := 0; x < imgWidth; x++ {
		for y := 0; y < imgHeight; y++ {
			img.Set(x, y, color)
		}
	}
	return img
}
