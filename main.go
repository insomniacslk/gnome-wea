package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/getlantern/systray"
	"github.com/insomniacslk/ipapi"
	"github.com/insomniacslk/openweathermap"
	"github.com/insomniacslk/openweathermap/icons"
	"github.com/insomniacslk/xjson"
	"github.com/kirsle/configdir"
	"googlemaps.github.io/maps"
)

const progname = "wea"

func main() {
	configFile, cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	systray.Run(
		func() { onReady(configFile, cfg) },
		onExit,
	)
}

// Config contains the program's configuration.
type Config struct {
	Locations            []string       `json:"locations"`
	GoogleMapsAPIKey     string         `json:"googlemaps_api_key"`
	OpenweathermapAPIKey string         `json:"openweathermap_api_key"`
	Interval             xjson.Duration `json:"interval"`
	Language             string         `json:"language"`
	Units                string         `json:"units"`
	Debug                bool           `json:"debug"`
	Editor               string         `json:"editor"`
}

func loadConfig() (string, *Config, error) {
	cfg := Config{}

	configPath := configdir.LocalConfig(progname)
	configFile := path.Join(configPath, "config.json")
	log.Printf("Trying to load config file %s", configFile)
	if err := configdir.MakePath(configPath); err != nil {
		if os.IsNotExist(err) {
			return configFile, &cfg, nil
		}
		return configFile, nil, fmt.Errorf("failed to create config path '%s': %w", configPath, err)
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return configFile, &cfg, nil
		}
		return configFile, nil, fmt.Errorf("failed to open '%s': %w", configFile, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return configFile, nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	// sanity checks
	if len(cfg.Locations) == 0 {
		return configFile, nil, fmt.Errorf("no locations are configured")
	}
	if cfg.OpenweathermapAPIKey == "" {
		return configFile, nil, fmt.Errorf("openweathermap_api_key cannot be empty")
	}
	if cfg.GoogleMapsAPIKey == "" {
		return configFile, nil, fmt.Errorf("googlemaps_api_key cannot be empty")
	}

	return configFile, &cfg, nil
}

type location struct {
	name     string
	lat, lon float64
}

func getLocation(cfg *Config, locName string) (*location, error) {
	client, err := maps.NewClient(maps.WithAPIKey(cfg.GoogleMapsAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get Maps client: %w", err)
	}
	r := maps.GeocodingRequest{
		Address: locName,
	}
	resp, err := client.Geocode(context.Background(), &r)
	if err != nil {
		return nil, fmt.Errorf("failed to geocode location: %w", err)
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("location not found")
	}
	return &location{
		name: resp[0].AddressComponents[0].LongName,
		lat:  resp[0].Geometry.Location.Lat,
		lon:  resp[0].Geometry.Location.Lng,
	}, nil
}

func getWeather(cfg *Config, loc *location) (*openweathermap.Weather, error) {
	return openweathermap.Request(
		cfg.OpenweathermapAPIKey,
		loc.lat,
		loc.lon,
		[]openweathermap.Exclude{
			openweathermap.Minutely,
			openweathermap.Hourly,
			openweathermap.Daily,
			openweathermap.Alerts,
		},
		openweathermap.Units(cfg.Units),
		openweathermap.Lang(cfg.Language),
		cfg.Debug,
	)
}

type weatherItem struct {
	menuitem *systray.MenuItem
	loc      location
}

func refreshWeather(cfg *Config, curLoc *location, items []weatherItem) {
	tempUnit := openweathermap.TempUnits[openweathermap.Units(cfg.Units)]

	curLocWea, err := getWeather(cfg, curLoc)
	if err != nil {
		systray.SetTitle("failed to get weather")
		log.Printf("failed to get weather for '%s': %v", curLoc.name, err)
		// try the other locations without stopping
	} else {
		systray.SetTitle(fmt.Sprintf("%s: %.01f%s %s", curLoc.name, curLocWea.Current.Temp, tempUnit, curLocWea.Current.Weather[0].Description))
		systray.SetIcon(icons.Icons[curLocWea.Current.Weather[0].Icon])
	}
	for _, item := range items {
		wea, err := getWeather(cfg, &item.loc)
		if err != nil {
			log.Printf("failed to get weather for '%s': %v", item.loc.name, err)
			// try the other locations
			continue
		}
		text := fmt.Sprintf(
			"%s: %.02f%s %s",
			item.loc.name,
			wea.Current.Temp, tempUnit,
			wea.Current.Weather[0].Description,
		)
		item.menuitem.SetTitle(text)
	}
}

func getCurrentLocation() (string, error) {
	resp, err := ipapi.Get(nil, nil)
	if err != nil {
		return "", fmt.Errorf("ipapi.Get failed: %w", err)
	}
	return fmt.Sprintf("%s, %s", resp.City, resp.CountryCode), nil
}

func onReady(configFile string, cfg *Config) {
	curLocName, err := getCurrentLocation()
	if err != nil {
		log.Fatalf("Cannot get current location: %v", err)
	}

	systray.SetIcon(icons.Icon01d)
	systray.SetTitle("Weather")
	systray.SetTooltip("Weather app")

	mRefresh := systray.AddMenuItem("Refresh weather now", "Force a refresh of the weather information for all the locations")
	var mInterval *systray.MenuItem
	if cfg.Interval == 0 {
	} else {
		mInterval = systray.AddMenuItem(fmt.Sprintf("Weather will refresh every %s", cfg.Interval), "The weather information will automatically refresh at the configured interval")
	}
	mInterval.Disable()
	mEdit := systray.AddMenuItem("Edit config", "Open configuration file for editing")
	systray.AddSeparator()

	// Sets the icon of a menu item. Only available on Mac and Windows.

	var items []weatherItem
	curLoc, err := getLocation(cfg, curLocName)
	if err != nil {
		log.Fatalf("Failed to get location '%s': %v", curLocName, err)
	}

	for _, locName := range cfg.Locations {
		loc, err := getLocation(cfg, locName)
		if err != nil {
			log.Fatalf("Failed to get location '%s': %v", locName, err)
		}
		items = append(items, weatherItem{
			loc: location{
				name: loc.name,
				lat:  loc.lat,
				lon:  loc.lon,
			},
			menuitem: systray.AddMenuItem(
				fmt.Sprintf("%s: not loaded yet", loc.name),
				fmt.Sprintf("Weather for %s", loc.name),
			),
		},
		)
	}
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Terminate the app")
	mQuit.SetIcon(Icon)

	// sets the editor
	editor := "xdg-open"
	if cfg.Editor != "" {
		editor = cfg.Editor
	}

	refreshWeather(cfg, curLoc, items)
	go func() {
		timer := time.NewTimer(time.Duration(cfg.Interval))
		log.Printf("Refreshing weather every %s", cfg.Interval)
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
			case <-mEdit.ClickedCh:
				cmd := exec.Command(editor, configFile)
				log.Printf("Executing %v", cmd)
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				if err := cmd.Run(); err != nil {
					log.Printf("Error when opening editor: %v", err)
				}
			case <-mRefresh.ClickedCh:
				refreshWeather(cfg, curLoc, items)
			case <-timer.C:
				refreshWeather(cfg, curLoc, items)
			}
		}
	}()
}

func onExit() {
	// clean up here
}
