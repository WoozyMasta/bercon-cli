#!/usr/bin/env bash
set -eu

path=pkg/beparser
db=GeoLite2-Country.mmdb

echo "Download $db to $path/"
curl -#SfLo "$path/$db" "https://git.io/$db"
echo "Done"
