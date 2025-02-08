#!/bin/bash

set -e -x -o pipefail 


function download() {
	NAME=$1
	ARCH=$3
	VERSION=$2
	# Etcd 包比较特殊,下载后需要先解压,然后用包里面的tar包作为源文件.
	#
	Target=$HOME/.cache_meridian/${NAME}
	mkdir -p "$Target"
	if [[ -f $Target/${NAME}-${VERSION}-linux-${ARCH}.tar.gz ]];
	then
		echo "already exist"; return
	fi
	wget --tries 10 --no-check-certificate -q \
		-O $Target/${NAME}-${VERSION}-linux-${ARCH}.tar.gz \
		http://aliacs-k8s-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/public/pkg/${NAME}/${NAME}-${VERSION}-linux-${ARCH}.tar.gz
}

function for_pkg() {
	direction=$1
	if [[ -z $direction ]];then
		direction="download"
	fi
  	for pkg in "${pkgs[@]}"; do
		item=(${pkg//:/ })
		NAME=${item[0]}
		VERSION=${item[1]}
		ARCH=${item[2]}
		echo process item: NAME=$NAME, VERSION=${VERSION}, ARCH=${ARCH}
		case $direction in
		download)
			download "$NAME" "$VERSION" "$ARCH"
			;;
		upload)
			upload "$NAME" "$VERSION" "$ARCH"
			;;
		esac
  	done

}

function upload() {
	NAME=$1
	VERSION=$2
	ARCH=$3
	Target=${NAME}-${VERSION}-linux-${ARCH}.tar.gz
	FROM=$HOME/.cache_meridian/${NAME}/$Target
	TO=oss://host-wdrip-cn-hangzhou/meridian/default/public/$NAME/$Target

	echo upload from $FROM, to $TO
	ossutil --endpoint oss-cn-hangzhou.aliyuncs.com cp $FROM $TO
}


function meridian() {

	ossutil --endpoint oss-cn-hangzhou.aliyuncs.com cp \
		/home/aoxn/vaoxn/code/meridian/bin/meridian \
		oss://host-wdrip-cn-hangzhou/meridian/default/public/meridian/meridian-0.0.1
}

#pkgs=(
#	etcd:v3.4.3:amd64
#	containerd:1.6.21:amd64
#	docker:19.03.15:amd64
#	kubernetes:1.31.1-aliyun.1:amd64
#)

pkgs=(
	etcd:v3.4.3:amd64
)

case $1 in
	meridian|m)
		meridian
		;;
	*)
		for_pkg $@
esac


