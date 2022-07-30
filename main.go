package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
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
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	updateSignal := make(chan struct{}, 1)
	go func(us chan<- struct{}) {
		for {
			sig := <-signals
			switch sig {
			case syscall.SIGUSR1:
				fmt.Println("Received SIGUSR1, updating weather")
				us <- struct{}{}
			}
		}
	}(updateSignal)

	configFile, cfg, err := loadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file '%s' does not exist, creating one", configFile)
			if err := editConfigFile(cfg, configFile); err != nil {
				log.Fatalf("Failed to create config file: %v", err)
			}
			log.Printf("Please reload this app after creating a suitable configuration file")
			os.Exit(0)
		} else {
			log.Fatalf("Failed to open config file: %v", err)
		}
	}
	systray.Run(
		func() { onReady(configFile, cfg, updateSignal) },
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
	ShowGraph            bool           `json:"show_graph"`
	Debug                bool           `json:"debug"`
	Editor               string         `json:"editor"`
	EditorArgs           []string       `json:"editor_args"`
}

func loadConfig() (string, *Config, error) {
	cfg := Config{}

	configPath := configdir.LocalConfig(progname)
	configFile := path.Join(configPath, "config.json")
	log.Printf("Trying to load config file %s", configFile)
	if err := configdir.MakePath(configPath); err != nil {
		return configFile, nil, err
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return configFile, nil, err
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
	if cfg.Debug {
		log.Printf("GMaps Geocoding response: %+v", resp)
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

func updateCurrentLocation(cfg *Config, g *Graph) {
	tempUnit := openweathermap.TempUnits[openweathermap.Units(cfg.Units)]
	curLocName, err := getCurrentLocation(cfg)
	if err != nil {
		log.Printf("Cannot get current location: %v", err)
		return
	}
	curLoc, err := getLocation(cfg, curLocName)
	if err != nil {
		log.Fatalf("Failed to get location '%s': %v", curLocName, err)
	}
	curLocWea, err := getWeather(cfg, curLoc)
	if err != nil {
		systray.SetTitle("failed to get weather")
		log.Printf("failed to get weather for '%s': %v", curLoc.name, err)
		// try the other locations without stopping
	} else {
		systray.SetTitle(fmt.Sprintf("%s: %.01f%s %s", curLoc.name, curLocWea.Current.Temp, tempUnit, curLocWea.Current.Weather[0].Description))
		if cfg.ShowGraph {
			g.SetNext(int(curLocWea.Current.Temp))
			icon, err := g.ToIcon()
			if err != nil {
				log.Printf("Failed to convert to icon, skipping: %v", err)
			} else {
				systray.SetIcon(icon)
			}
		} else {
			systray.SetIcon(icons.Icons[curLocWea.Current.Weather[0].Icon])
		}
	}
}

func updateWeather(cfg *Config, items []weatherItem, lastUpdateItem *systray.MenuItem, doCurrentLocation bool, g *Graph) {
	tempUnit := openweathermap.TempUnits[openweathermap.Units(cfg.Units)]

	if doCurrentLocation {
		updateCurrentLocation(cfg, g)
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
		item.menuitem.SetIcon(icons.Icons[wea.Current.Weather[0].Icon])
	}
	lastUpdateItem.SetTitle(fmt.Sprintf("Last update: %s", time.Now().Format("Mon Jan 2 15:04:05 MST")))
}

func getCurrentLocation(cfg *Config) (string, error) {
	resp, err := ipapi.Get(nil, nil)
	if err != nil {
		return "", fmt.Errorf("ipapi.Get failed: %w", err)
	}
	if cfg.Debug {
		log.Printf("IP cfgAPI response: %+v", resp)
	}
	return fmt.Sprintf("%s, %s", resp.City, resp.CountryCode), nil
}

func onReady(configFile string, cfg *Config, updateSignal <-chan struct{}) {
	var g *Graph
	if cfg.ShowGraph {
		g = NewGraph(100, 100, &darkGreen, &gray, graphStyleBar)
		g.Blank()
		icon, err := g.ToIcon()
		if err != nil {
			log.Fatalf("Failed to convert to icon: %v", err)
		}
		systray.SetIcon(icon)
	}

	// use the weather icon if the user is not requesting the temperature graph
	if !cfg.ShowGraph {
		systray.SetIcon(icons.Icon01d)
	}
	systray.SetTitle("Weather")
	systray.SetTooltip("Weather app")

	mUpdate := systray.AddMenuItem("Update weather now", "Force an update of the weather information for all the locations")
	var mInterval *systray.MenuItem
	if cfg.Interval == 0 {
		mInterval = systray.AddMenuItem("Weather will not update automatically", "No interval is defined in the config file, or zero is set")
	} else {
		mInterval = systray.AddMenuItem(fmt.Sprintf("Weather will update every %s", cfg.Interval), "The weather information will automatically update at the configured interval")
	}
	mLastUpdate := systray.AddMenuItem("Last updated: never", "Show the last time weather was updated")
	mLastUpdate.Disable()
	mInterval.Disable()
	mEdit := systray.AddMenuItem("Edit config", "Open configuration file for editing")
	systray.AddSeparator()

	// Sets the icon of a menu item. Only available on Mac and Windows.

	var items []weatherItem

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

	updateWeather(cfg, items, mLastUpdate, true, g)
	go func() {
		timer := time.NewTicker(time.Duration(cfg.Interval))
		log.Printf("Updating weather every %s", cfg.Interval)
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
			case <-mEdit.ClickedCh:
				if err := editConfigFile(cfg, configFile); err != nil {
					log.Printf("Failed to edit config file: %v", err)
				}
			case <-mUpdate.ClickedCh:
				updateWeather(cfg, items, mLastUpdate, true, g)
			case <-timer.C:
				updateWeather(cfg, items, mLastUpdate, true, g)
			case <-updateSignal:
				updateWeather(cfg, items, mLastUpdate, true, g)
			}
		}
	}()
}

func editConfigFile(cfg *Config, configFile string) error {
	var (
		editorPath = DefaultEditorPath
		editorArgs = DefaultEditorArgs
	)
	if cfg != nil {
		if cfg.Editor != "" {
			editorPath = cfg.Editor
		}
		if cfg.EditorArgs != nil {
			editorArgs = cfg.EditorArgs
		}
	}

	cmd := exec.Command(editorPath, append(editorArgs, configFile)...)
	log.Printf("Executing %v", cmd)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func onExit() {
	// clean up here
}
