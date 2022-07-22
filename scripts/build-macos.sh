#!/bin/bash
set -exu

go build
mkdir Wea.app/Contents/MacOS
cp gnome-wea Wea.app/Contents/MacOS/wea

brew install create-dmg
create-dmg Wea.dmg Wea.app
