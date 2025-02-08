#!/bin/bash

set -e 

function help_message(){
	echo "  eg.  DOS=[darwin | linux | windows]"
	echo "  eg.  DARCH=[x86_64 | aarch64]"
	echo "  eg.  VERSION=[0.1.0]"
	echo 
	echo "  eg.  DOS=darwin DARCH=x86_64 VERSION=0.1.0 sh $0 "
}

function release_target() {
	local target=$1
	case "$target" in
	"script")
		echo "release [install.sh] script to oss..."
		ossutil cp --region ap-northeast-1 --endpoint oss-ap-northeast-1.aliyuncs.com -f iou.sh \
			oss://meridian-tokyo/meridian/iou.sh
		echo use [wget -O iou.sh http://meridian-tokyo.oss-ap-northeast-1.aliyuncs.com/meridian/install.sh] to download
		;;
	"guestbin")
		if [[ -z $DOS ]];
		then
			DOS=linux
			echo "use default DOS=${DOS} "
		fi
	
		if [[ -z $DARCH ]];
		then
			DARCH=x86_64
			echo "use default DARCH=${DARCH}"
		fi

		if [[ -z $VERSION ]];
		then
			VERSION=0.1.0
			echo "use default VERSION=${VERSION}"
		fi
		
		echo "build guest binary with: DOS=${DOS} DARCH=${DARCH} VERSION=${VERSION}"
	        case $DOS in
	        "darwin")
	                make universal
			;;
	        "linux")
	                case $DARCH in
	                "x86_64")
	                        make mlx86
	                        ;;
	                "aarch64")
				make mlarm64
	                        ;;
	                *)
	                        echo "unknown arch: ${DARCH} for darwin"; exit 1
	                        ;;
	                esac
	                ;;
	        *)
	                echo "unknown os"; exit 1
	        esac
	
		rm -rf /tmp/meridian."${DOS}"."${DARCH}".tar.gz /tmp/meridian."${DOS}"."${DARCH}".tar.gz.sum
		tar -cvzf /tmp/meridian."${DOS}"."${DARCH}".tar.gz bin/meridian."${DOS}"."${DARCH}"
		md5sum bin/meridian."${DOS}"."${DARCH}" > /tmp/meridian."${DOS}"."${DARCH}".tar.gz.sum
	
	
		echo "release [meridian] binary.tar.gz to oss"
		ossutil cp -f /tmp/meridian."${DOS}"."${DARCH}".tar.gz \
			oss://host-wdrip-cn-hangzhou/bin/"${DOS}"/"${DARCH}"/"${VERSION}"/meridian."${DOS}"."${DARCH}".tar.gz
	
		echo "release [meridiand binary.sum to oss"
		ossutil cp -f /tmp/meridian."${DOS}"."${DARCH}".tar.gz.sum \
			oss://host-wdrip-cn-hangzhou/bin/"${DOS}"/"${DARCH}"/"${VERSION}"/meridian."${DOS}"."${DARCH}".tar.gz.sum
	
		rm -rf /tmp/meridian."${DOS}"."${DARCH}".tar.gz /tmp/meridian."${DOS}"."${DARCH}".tar.gz.sum
		;;
	"zip")
		make universal
		pushd app; npm run make; popd

		ossutil cp -f \
			app/out/make/zip/darwin/universal/Meridian-darwin-universal-0.1.0.zip \
			oss://host-wdrip-cn-hangzhou/meridian/release/darwin/
		echo "use [ wget -O Meridian-darwin-universal-0.1.0.zip http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/meridian/release/darwin/Meridian-darwin-universal-0.1.0.zip ] to download zip image"
	;;
	"bin")
		ossutil cp -f bin/meridian oss://host-wdrip-cn-hangzhou/meridian/bin/universal/meridian
		;;
	"*")
		echo "unknown target: [$target]"
		;;
	esac
}

case "$1" in
"guestbin")
	DOS=darwin DARCH=x86_64  && release_target "$1"
	DOS=darwin DARCH=aarch64 && release_target "$1"
	DOS=linux  DARCH=x86_64  && release_target "$1"
	DOS=linux  DARCH=aarch64 && release_target "$1"
	;;
"zip")
	release_target "$1"
	;;
"script")
	release_target "$1"
	;;
esac


