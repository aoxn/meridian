#!/bin/bash

set -e

function setup::env() {
    export version=0.1.0

    uid=$(id -u)

    os=$(uname|tr '[:upper:]' '[:lower:]')
    arch=$(uname -m|tr '[:upper:]' '[:lower:]')
    case $arch in
    "x86_64")
            arch=amd64
            ;;
    "aarch64")
            arch=arm64
            ;;
    *)
            echo "set arch: ${arch} for ${os}"
            ;;
    esac

    server=http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com

    export os arch server uid

    if [ "$HOME" == "" ];then
        echo "HOME env not found, cannot determine home dir"; exit 1
    fi

    if [ "$workDir" == "" ];
    then
            homeDir=$HOME/.meridian
            workDir=$homeDir/_daemon
    fi

    mkdir -p "$homeDir"
}

function setup::uninstall() {
    launchctl bootout gui/"$uid" ~/Library/LaunchAgents/cn.xdpin.meridian.plist || true
    sudo rm -rf ~/Library/LaunchAgents/cn.xdpin.meridian.plist || true
    pgrep meridian|xargs -I '{}' kill -9 {} ||true
    sudo rm -rf ~/.meridian || true
    sudo rm -rf /usr/local/bin/meridian
    sudo rm -rf ~/Library/Caches/meridian || true
}

function setup::install_meridian() {

    need_install=0
    if [[ -f /usr/local/bin/meridian ]];
    then
            wget -q -O /tmp/meridian."${os}".${arch}.tar.gz.sum \
                    $server/bin/"${os}"/${arch}/${version}/meridian."${os}".${arch}.tar.gz.sum
            # shellcheck disable=SC2002
            m1=$(cat /tmp/meridian."${os}".${arch}.tar.gz.sum |awk '{print $1}')
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
            wget -q -O /tmp/meridian."${os}".${arch}.tar.gz \
                    $server/bin/"${os}"/${arch}/${version}/meridian."${os}".${arch}.tar.gz

            wget -q -O /tmp/meridian."${os}".${arch}.tar.gz.sum \
                    $server/bin/"${os}"/${arch}/${version}/meridian."${os}".${arch}.tar.gz.sum
            #md5sum -c /tmp/meridian.${os}.${arch}.tar.gz.sum
            tar xf /tmp/meridian."${os}".${arch}.tar.gz -C /tmp
            sudo mv -f /tmp/bin/meridian."${os}".${arch} /usr/local/bin/meridian
            sudo mv -f /tmp/bin/meridiand."${os}".${arch} /usr/local/bin/meridiand
            sudo mv -f /tmp/bin/meridian-vm."${os}".${arch} /usr/local/bin/meridian-vm
            rm -rf /tmp/meridian."${os}".${arch}.tar.gz /tmp/meridian."${os}".${arch}.tar.gz.sum
    fi

    sudo rm -rf /tmp/meridian.sock || true

    case ${os} in
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
                <string>/usr/local/bin/meridiand</string>
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
ExecStart=/usr/local/bin/meridiand serve
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
            echo "unknown os type: [${os}]"
            return
    esac

}

function setup::install_meridian_node() {
    wget -q -O /tmp/meridian-node."${os}".${arch}.tar.gz \
            $server/bin/"${os}"/${arch}/${version}/meridian-node."${os}".${arch}.tar.gz

    tar xf /tmp/meridian-node."${os}".${arch}.tar.gz -C /tmp
    sudo mv -f /tmp/bin/meridian-node."${os}".${arch} /usr/local/bin/meridian-node
    rm -rf /tmp/meridian-node."${os}".${arch}.tar.gz /tmp/meridian-node."${os}".${arch}.tar.gz.sum
}

setup::env

action=$1
if [[ "$action" == "" ]];
then
        action=install
fi

case $action in
"uninstall")
        setup::uninstall
        ;;
"install")
        resource=$2
        case $resource in
        "meridian-node")
            setup::install_meridian_node
            ;;
        *)
            setup::install_meridian
            setup::install_meridian_node
        esac
        ;;
esac


