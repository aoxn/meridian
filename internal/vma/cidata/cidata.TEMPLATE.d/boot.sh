#!/bin/sh
# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0
set -eu

INFO() {
	echo "MD $(date -Iseconds)| $*"
}

WARNING() {
	echo "MD $(date -Iseconds)| WARNING: $*"
}

# shellcheck disable=SC2163
while read -r line; do export "$line"; done <"${MD_CIDATA_MNT}"/md.env

# shellcheck disable=SC2163
while read -r line; do
	# pam_env implementation:
	# - '#' is treated the same as newline; terminates value
	# - skip leading tabs and spaces
	# - skip leading "export " prefix (only single space)
	# - skip leading quote ('\'' or '"') on the value side
	# - skip trailing quote only if leading quote has been skipped;
	#   quotes don't need to match; trailing quote may be omitted
	line="$(echo "$line" | sed -E "s/^[ \\t]*(export )?//; s/#.*//; s/(^[^=]+=)[\"'](.*[^\"'])?[\"']?$/\1\2/")"
	[ -n "$line" ] && export "$line"
done <"${MD_CIDATA_MNT}"/etc_environment

PATH="${MD_CIDATA_MNT}"/util:"${PATH}"
export PATH

CODE=0

# Don't make any changes to /etc or /var/lib until boot/04-persistent-data-volume.sh
# has run because it might move the directories to /mnt/data on first boot. In that
# case changes made on restart would be lost.
for f in "${MD_CIDATA_MNT}"/boot/*; do
        INFO "Executing $f"
        if ! "$f"; then
                WARNING "Failed to execute $f"
                CODE=1
        fi
done

if [ -d "${MD_CIDATA_MNT}"/provision.system ]; then
	for f in "${MD_CIDATA_MNT}"/provision.system/*; do
		INFO "Executing $f"
		if ! "$f"; then
			WARNING "Failed to execute $f"
			CODE=1
		fi
	done
fi

USER_SCRIPT="${MD_CIDATA_HOME}/.md-user-script"
if [ -d "${MD_CIDATA_MNT}"/provision.user ]; then
	if [ ! -f /sbin/openrc-run ]; then
		until [ -e "/run/user/${MD_CIDATA_UID}/systemd/private" ]; do sleep 3; done
	fi
	for f in "${MD_CIDATA_MNT}"/provision.user/*; do
		INFO "Executing $f (as user ${MD_CIDATA_USER})"
		cp "$f" "${USER_SCRIPT}"
		chown "${MD_CIDATA_USER}" "${USER_SCRIPT}"
		chmod 755 "${USER_SCRIPT}"
		if ! sudo -iu "${MD_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${MD_CIDATA_UID}" "${USER_SCRIPT}"; then
			WARNING "Failed to execute $f (as user ${MD_CIDATA_USER})"
			CODE=1
		fi
		rm "${USER_SCRIPT}"
	done
fi

# Signal that provisioning is done. The instance-id in the meta-data file changes on every boot,
# so any copy from a previous boot cycle will have different content.
cp "${MD_CIDATA_MNT}"/meta-data /run/md-boot-done

INFO "Exiting with code $CODE"
exit "$CODE"

