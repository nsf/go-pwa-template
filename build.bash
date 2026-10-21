#!/bin/bash

set -e

if [ ! -d "static/icons" ]; then
  mkdir -p static/icons
  sizes=(48 72 96 128 144 152 192 256 384 512)
  for size in "${sizes[@]}"; do
    rsvg-convert -w "$size" -h "$size" "static-src/icon.svg" -o "static/icons/icon_${size}x${size}.png"
  done
fi

if [ ! -d "static/screenshots" ]; then
  mkdir -p static/screenshots
  magick convert static-src/desktop.svg static/screenshots/desktop.png
  magick convert static-src/mobile.svg static/screenshots/mobile.png
fi

tailwindcss --input style/style.css --output static/style.css --minify
templ generate
go build