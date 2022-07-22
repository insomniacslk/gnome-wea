# gnome-wea

Monitor weather in your system tray

Build with
```
go build
```

On systems without the new `ayatana` appindicator (e.g. Fedora), use
```
go build -tags=legacy_appindicator
```

Create a configfile at `~/.config/bgchanger/config.json` with content similar to
the following:
```
{
    "locations": ["dublin", "san francisco"],
    "openweathermap_api_key": "your api key",
    "googlemaps_api_key": "your api key",
    "interval": "15m",
    "language": "en",
    "units": "metric",
    "debug": false,
    "editor": "gedit"
}
```

## Create DMG for macOS

```
./scripts/build-macos.sh
```

