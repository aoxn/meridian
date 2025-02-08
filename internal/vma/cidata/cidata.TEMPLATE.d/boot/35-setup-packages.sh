#!/bin/sh
set -ex

update_fuse_conf() {
	# Modify /etc/fuse.conf (/etc/fuse3.conf) to allow "-o allow_root"
	if [ "${MD_CIDATA_MOUNTS}" -gt 0 ]; then
		fuse_conf="/etc/fuse.conf"
		if [ -e /etc/fuse3.conf ]; then
			fuse_conf="/etc/fuse3.conf"
		fi
		if ! grep -q "^user_allow_other" "${fuse_conf}"; then
			echo "user_allow_other" >>"${fuse_conf}"
		fi
	fi
}

SETUP_DNS=0
if [ -n "${MD_CIDATA_UDP_DNS_LOCAL_PORT}" ] && [ "${MD_CIDATA_UDP_DNS_LOCAL_PORT}" -ne 0 ]; then
	SETUP_DNS=1
fi
if [ -n "${MD_CIDATA_TCP_DNS_LOCAL_PORT}" ] && [ "${MD_CIDATA_TCP_DNS_LOCAL_PORT}" -ne 0 ]; then
	SETUP_DNS=1
fi
if [ "${SETUP_DNS}" = 1 ]; then
	# Try to setup iptables rule again, in case we just installed iptables
	"${MD_CIDATA_MNT}/boot/09-host-dns-setup.sh"
fi

# update_fuse_conf has to be called after installing all the packages,
# otherwise apt-get fails with conflict
if [ "${MD_CIDATA_MOUNTTYPE}" = "reverse-sshfs" ]; then
	update_fuse_conf
fi

