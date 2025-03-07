#!/bin/sh
# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux
MD_CIDATA_GUEST_INSTALL_PREFIX=/usr/local
if [ "${MD_CIDATA_MOUNTTYPE}" = "reverse-sshfs" ]; then
	# Create mount points
	# NOTE: Busybox sh does not support `for ((i=0;i<$N;i++))` form
	for f in $(seq 0 $((MD_CIDATA_MOUNTS - 1))); do
		mountpointvar="MD_CIDATA_MOUNTS_${f}_MOUNTPOINT"
		mountpoint="$(eval echo \$"$mountpointvar")"
		mkdir -p "${mountpoint}"
		gid=$(id -g "${MD_CIDATA_USER}")
		chown "${MD_CIDATA_UID}:${gid}" "${mountpoint}"
	done
fi

# Install or update the guestagent binary
install -m 755 "${MD_CIDATA_MNT}"/md-guest "${MD_CIDATA_GUEST_INSTALL_PREFIX}"/bin/md-guest

# Launch the guestagent service
if [ -f /sbin/openrc-run ]; then
	# Install the openrc md-guest service script
	cat >/etc/init.d/md-guest <<'EOF'
#!/sbin/openrc-run
supervisor=supervise-daemon

log_file="${log_file:-/var/log/${RC_SVCNAME}.log}"
err_file="${err_file:-${log_file}}"
log_mode="${log_mode:-0644}"
log_owner="${log_owner:-root:root}"

supervise_daemon_args="${supervise_daemon_opts:---stderr \"${err_file}\" --stdout \"${log_file}\"}"

name="md-guest"
description="Forward ports to the md-hostagent"

command=${MD_CIDATA_GUEST_INSTALL_PREFIX}/bin/md-guest
command_args="guest serve"
command_background=true
pidfile="/run/md-guest.pid"
EOF
	chmod 755 /etc/init.d/md-guest

	rc-update add md-guest default
	rc-service md-guest start
else
        if [ -f /etc/systemd/system/md-guest.service ];
        then
                echo "md-guest agent is already installed" ; return
        fi
        cat >/etc/systemd/system/md-guest.service <<EOF
[Unit]
Description=md-guest

[Service]
ExecStart=${MD_CIDATA_GUEST_INSTALL_PREFIX}/bin/md-guest guest serve
Type=simple
Environment="HOME=${MD_CIDATA_HOME}"
Restart=on-failure
OOMPolicy=continue
OOMScoreAdjust=-500

[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload || true
	systemctl enable md-guest.service
	systemctl restart md-guest || true
fi

