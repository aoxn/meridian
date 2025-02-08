#!/bin/sh
set -eux

# /etc/environment must be written after 04-persistent-data-volume.sh has run to
# make sure the changes on a restart are applied to the persisted version.

orig=$(test ! -f /etc/environment || cat /etc/environment)
if [ -e /etc/environment ]; then
	sed -i '/#MD-START/,/#MD-END/d' /etc/environment
fi
cat "${MD_CIDATA_MNT}/etc_environment" >>/etc/environment

# Signal that provisioning is done. The instance-id in the meta-data file changes on every boot,
# so any copy from a previous boot cycle will have different content.
cp "${MD_CIDATA_MNT}"/meta-data /run/md-ssh-ready

