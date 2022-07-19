# gnome-background-changer

Change background randomly via app indicator

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
    "pictures_dir": "/home/you/Pictures/Backgrounds",
    "interval": "15m"
}
```
