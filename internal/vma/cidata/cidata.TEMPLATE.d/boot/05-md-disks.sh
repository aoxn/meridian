#!/bin/bash

set -ex -o pipefail

test "$MD_CIDATA_DISKS" -gt 0 || exit 0

get_disk_var() {
	diskvarname="MD_CIDATA_DISK_${1}_${2}"
	eval echo \$"$diskvarname"
}

for i in $(seq 0 $((MD_CIDATA_DISKS - 1))); do
	DISK_NAME="$(get_disk_var "$i" "NAME")"
	DEVICE_NAME="$(get_disk_var "$i" "DEVICE")"
	FORMAT_DISK="$(get_disk_var "$i" "FORMAT")"
	FORMAT_FSTYPE="$(get_disk_var "$i" "FSTYPE")"
	FORMAT_FSARGS="$(get_disk_var "$i" "FSARGS")"

	test -n "$FORMAT_DISK" || FORMAT_DISK=true
	test -n "$FORMAT_FSTYPE" || FORMAT_FSTYPE=ext4

	# first time setup
	if [[ ! -b "/dev/disk/by-label/md-${DISK_NAME}" ]]; then
		if $FORMAT_DISK; then
			echo 'type=linux' | sfdisk --label gpt "/dev/${DEVICE_NAME}"
			# shellcheck disable=SC2086
			mkfs.$FORMAT_FSTYPE $FORMAT_FSARGS -L "md-${DISK_NAME}" "/dev/${DEVICE_NAME}1"
		fi
	fi

	mkdir -p "/mnt/md-${DISK_NAME}"
	mount -t $FORMAT_FSTYPE "/dev/${DEVICE_NAME}1" "/mnt/md-${DISK_NAME}"
	if command -v growpart >/dev/null 2>&1 && command -v resize2fs >/dev/null 2>&1; then
		growpart "/dev/${DEVICE_NAME}" 1 || true
		# Only resize when filesystem is in a healthy state
		if command -v "fsck.$FORMAT_FSTYPE" -f -p "/dev/disk/by-label/md-${DISK_NAME}"; then
			if [[ $FORMAT_FSTYPE == "ext2" || $FORMAT_FSTYPE == "ext3" || $FORMAT_FSTYPE == "ext4" ]]; then
				resize2fs "/dev/disk/by-label/md-${DISK_NAME}" || true
			elif [ "$FORMAT_FSTYPE" == "xfs" ]; then
				xfs_growfs "/dev/disk/by-label/md-${DISK_NAME}" || true
			else
				echo >&2 "WARNING: unknown fs '$FORMAT_FSTYPE'. FS will not be grew up automatically"
			fi
		fi
	fi
done

