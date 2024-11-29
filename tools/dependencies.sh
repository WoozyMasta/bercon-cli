#!/usr/bin/env bash
set -eu

: "${UPX_VERSION:=4.2.4}"

sudo apt update
sudo apt install -y xz-utils curl

curl -#Lo upx.tar.xz \
  "https://github.com/upx/upx/releases/download/v$UPX_VERSION/upx-$UPX_VERSION-amd64_linux.tar.xz"
tar -xvf upx.tar.xz --strip-components=1 "upx-$UPX_VERSION-amd64_linux/upx"
chmod +x upx
sudo mv upx /usr/local/bin/
