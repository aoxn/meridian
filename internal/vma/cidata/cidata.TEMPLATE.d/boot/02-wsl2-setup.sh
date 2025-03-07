#!/bin/sh
# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This script replaces the cloud-init functionality of creating a user and setting its SSH keys
# when using a WSL2 VM.
[ "$MD_CIDATA_VMTYPE" = "wsl2" ] || exit 0

# create user
sudo useradd -u "${MD_CIDATA_UID}" "${MD_CIDATA_USER}" -d "${MD_CIDATA_HOME}"
sudo mkdir "${MD_CIDATA_HOME}"/.ssh/
sudo cp "${MD_CIDATA_MNT}"/ssh_authorized_keys "${MD_CIDATA_HOME}"/.ssh/authorized_keys
sudo chown "${MD_CIDATA_USER}" "${MD_CIDATA_HOME}"/.ssh/authorized_keys

# add $MD_CIDATA_USER to sudoers
echo "${MD_CIDATA_USER} ALL=(ALL) NOPASSWD:ALL" | sudo tee -a /etc/sudoers.d/99_md_sudoers

# copy some CIDATA to the hardcoded path for requirement checks (TODO: make this not hardcoded)
sudo mkdir -p /mnt/md-cidata
sudo cp "${MD_CIDATA_MNT}"/meta-data /mnt/md-cidata/meta-data

