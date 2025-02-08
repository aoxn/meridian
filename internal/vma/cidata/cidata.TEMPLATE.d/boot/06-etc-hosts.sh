#!/bin/bash
set -ex -o pipefail

# Define host.md.internal in case the hostResolver is disabled. When using
# the hostResolver, the name is provided by the md resolver itself because
# it doesn't have access to /etc/hosts inside the VM.
if [[ -n $MD_CIDATA_SLIRP_GATEWAY ]];
then
        sed -i '/host.md.internal/d' /etc/hosts
        echo -e "${MD_CIDATA_SLIRP_GATEWAY}\thost.md.internal" >>/etc/hosts
fi

