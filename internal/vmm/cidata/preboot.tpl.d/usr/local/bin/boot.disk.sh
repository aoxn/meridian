#!/bin/bash

exec > /var/log/meridian.boot.disk.log 2>&1
set -x

CUR_DIR="$(cd "$(dirname "$0")" && pwd)"
ehoc "meridian: init boot disk running from: $CUR_DIR"


echo "meridian: [AutoSetup] running..."
sudo launchctl load -w /System/Library/LaunchDaemons/ssh.plist || true
echo "meridian: [AutoSetup] done"

# 1. 找到 Preboot 挂载点
PREBOOT=$(find /System/Volumes/Preboot \
		-mindepth 2 -maxdepth 2 -type d -name usr 2>/dev/null | head -n1 | xargs dirname)
if [[ -z "$PREBOOT" ]]; then
  echo "meridian: [bootstrap] PreBoot volume not found"; exit 1
fi

# 2. 复制文件
mkdir -p /usr/local/bin
rsync -a "$PREBOOT/usr/local/bin/" /usr/local/bin/

# 3. 可选：把 LaunchDaemon 本身也复制过去
mkdir -p /Library/LaunchDaemons
cp -rf "$PREBOOT/Library/LaunchDaemons/*" /Library/LaunchDaemons/

