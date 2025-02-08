#!/bin/bash

set -e

version=0.1.0
OS=$(uname|tr '[:upper:]' '[:lower:]')
arch=$(uname -m|tr '[:upper:]' '[:lower:]')
case $arch in
"amd64")
        arch=x86_64
        ;;
"arm64")
        arch=aarch64
        ;;
"x86_64")
        ;;
*)
        echo "unknown arch: ${arch} for ${OS}"; exit 1
        ;;
esac

if [ "$HOME" == "" ];then
	echo "HOME env not found, cannot determine home dir"; exit 1
fi

if [ "$workDir" == "" ];
then
        homeDir=$HOME/.meridian
        workDir=$homeDir/_daemon
fi

mkdir -p "$homeDir"

action=$1
if [[ "$action" == "" ]];
then
        action=install
fi

case $action in
"uninstall")
        launchctl bootout gui/"$uid" ~/Library/LaunchAgents/cn.xdpin.meridian.plist || true
        sudo rm -rf ~/Library/LaunchAgents/cn.xdpin.meridian.plist || true
        pgrep meridian|xargs -I '{}' kill -9 {} ||true
        sudo rm -rf ~/.meridian || true
        sudo rm -rf /usr/local/bin/meridian
        sudo rm -rf ~/Library/Caches/meridian || true
        exit 0
;;
esac

server=http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com

need_install=0
if [[ -f /usr/local/bin/meridian ]];
then
        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz.sum
        m1=$(cat /tmp/meridian.${OS}.${arch}.tar.gz.sum |awk '{print $1}')
        m2=$(md5sum /usr/local/bin/meridian |awk '{print $1}')
        if [[ "$m1" == "$m2" ]];
        then
                need_install=0
        else
                need_install=1
        fi
else
        need_install=1
fi

if [[ "$need_install" == "1" ]];
then
        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz

        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz.sum
        #md5sum -c /tmp/meridian.${OS}.${arch}.tar.gz.sum
        tar xf /tmp/meridian.${OS}.${arch}.tar.gz -C /tmp
        sudo mv -f /tmp/bin/meridian.${OS}.${arch} /usr/local/bin/meridian
        rm -rf /tmp/meridian.${OS}.${arch}.tar.gz /tmp/meridian.${OS}.${arch}.tar.gz.sum
fi

uid=$(id -u)

sudo rm -rf /tmp/meridian.sock || true

case $OS in
"darwin")
        cat > /tmp/meridian.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>Label</key>
        <string>cn.xdpin.meridian</string>
        <key>ProgramArguments</key>
        <array>
                <string>/usr/local/bin/meridian</string>
                <string>serve</string>
                <string>-v</string>
                <string>6</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>StandardErrorPath</key>
        <string>launchd.stderr.log</string>
        <key>StandardOutPath</key>
        <string>launchd.stdout.log</string>
        <key>WorkingDirectory</key>
        <string>$workDir</string>
</dict>
</plist>
EOF
        mv /tmp/meridian.plist ~/Library/LaunchAgents/cn.xdpin.meridian.plist
        # shellcheck disable=SC2046
        launchctl bootout gui/"$uid" ~/Library/LaunchAgents/cn.xdpin.meridian.plist || true
        launchctl bootstrap gui/"$uid" ~/Library/LaunchAgents/cn.xdpin.meridian.plist
        ;;
"linux")
        sudo mkdir -p /etc/systemd/meridian/
        sudo cat > /etc/systemd/meridian/meridiand.service << EOF
[Unit]
Description=Meridian daemon.
Documentation=meridian document

[Service]
ExecStart=/usr/local/bin/meridiand
WorkingDirectory=${workDir}
Type=simple
TimeoutSec=10
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
        sudo systemctl enable meridiand
        sudo systemctl start meridiand
        ;;
"")
        echo "unknonw os type"
        return
esac

