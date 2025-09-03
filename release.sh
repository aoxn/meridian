#!/bin/bash

set -e 

version=0.1.0

function help_message(){
	echo "  eg.  os=[darwin | linux | windows]"
	echo "  eg.  arch=[x86_64 | aarch64]"
	echo "  eg.  version=[0.1.0]"
	echo 
	echo "  eg.  os=darwin arch=x86_64 version=0.1.0 sh $0 "
}

function release::installation_script() {
	echo "release [install.sh] script to oss..."
	ossutil cp --region ap-northeast-1 \
	            --endpoint oss-ap-northeast-1.aliyuncs.com -f iou.sh \
		    oss://meridian-tokyo/meridian/iou.sh
	echo use [wget -O iou.sh http://meridian-tokyo.oss-ap-northeast-1.aliyuncs.com/meridian/install.sh] to download
}

function release::meridian() {
	 
	local os=$1
	local arch=$2
	if [[ "$os" == "" || "" == "$arch" ]];then
		echo "fatal: os & arch must not be empty"; exit 1
	fi
	GOOS=${os} GOARCH=${arch} make meridian
	GOOS=${os} GOARCH=${arch} make meridiand
	GOOS=${os} GOARCH=${arch} make meridian-vm
	
	target=/tmp/meridian."${os}"."${arch}".tar.gz
	target_md5=/tmp/meridian."${os}"."${arch}".tar.gz.sum

	echo "clean up $target $target_md5"
	if [[ "$target" == "" || "$target" == "/" ]];then
		echo "fatal: can not remve root dir[$target]";exit 1
	fi
	if [[ "$target_md5" == "" || "$target_md5" == "/" ]];then
		echo "fatal: can not remve root dir[$target_md5]";exit 1
	fi
	rm -rf "$target" "$target_md5"
	
	tar -cvzf "$target" bin/meridian."${os}"."${arch}" bin/meridiand."${os}"."${arch}" bin/meridian-vm."${os}"."${arch}"
	
	md5sum bin/meridian."${os}"."${arch}" > $target_md5

	
	echo "release [meridian] binary.tar.gz to oss"
	ossutil cp -f "$target" \
		oss://host-wdrip-cn-hangzhou/bin/"${os}"/"${arch}"/"${version}"/meridian."${os}"."${arch}".tar.gz

	echo "release [meridiand binary.sum to oss"
	ossutil cp -f "$target_md5" \
		oss://host-wdrip-cn-hangzhou/bin/"${os}"/"${arch}"/"${version}"/meridian."${os}"."${arch}".tar.gz.sum

    echo "[download] from http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/bin/${os}/${arch}/${version}/meridian.${os}.${arch}.tar.gz"
	rm -rf "$target" "$target_md5"
}

function release::meridian_target() {
	 
	local name=$1
	local os=$2
	local arch=$3
	if [[ "$arch" == "" || "$os" == "" || "$name" == "" ]];then
		echo "[$name]fatal: os & arch must not be empty"; exit 1
	fi
	GOOS=${os} GOARCH=${arch} make $name
	
	target=/tmp/${name}."${os}"."${arch}".tar.gz
	target_md5=/tmp/${name}."${os}"."${arch}".tar.gz.sum

	echo "[$name]clean up $target $target_md5"
	if [[ "$target" == "" || "$target" == "/" ]];then
		echo "[$name]fatal: can not remve root dir[$target]";exit 1
	fi
	if [[ "$target_md5" == "" || "$target_md5" == "/" ]];then
		echo "[$name]fatal: can not remve root dir[$target_md5]";exit 1
	fi
	rm -rf "$target" "$target_md5"
	
	tar -cvzf "$target" bin/$name."${os}"."${arch}" 
	
	md5sum bin/$name."${os}"."${arch}" > $target_md5

	
	echo "[$name]release tar package to oss: $target"
	ossutil cp -f "$target" \
		oss://host-wdrip-cn-hangzhou/bin/"${os}"/"${arch}"/"${version}"/$name."${os}"."${arch}".tar.gz

	echo "[$name]release tar package md5sum to oss: $target_md5"
	ossutil cp -f "$target_md5" \
		oss://host-wdrip-cn-hangzhou/bin/"${os}"/"${arch}"/"${version}"/$name."${os}"."${arch}".tar.gz.sum

    echo "[download] from http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/bin/${os}/${arch}/${version}/$name.${os}.${arch}.tar.gz"
	rm -rf "$target" "$target_md5"
}

function release::mac_universal_package() {
        make universal
        pushd app; npm run make; popd

        ossutil cp -f \
                app/out/make/zip/darwin/universal/Meridian-darwin-universal-0.1.0.zip \
                oss://host-wdrip-cn-hangzhou/meridian/release/darwin/
        echo "use [ wget -O Meridian-darwin-universal-0.1.0.zip http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/meridian/release/darwin/Meridian-darwin-universal-0.1.0.zip ] to download zip image"		
}

function release::meridian_binary() {
    local os=$1
    local arch=$2
	make meridian; 	 ossutil cp -f bin/meridian oss://host-wdrip-cn-hangzhou/meridian/bin/meridian
	
	make meridiand ; ossutil cp -f bin/meridiand oss://host-wdrip-cn-hangzhou/meridian/bin/meridiand
	
	make meridian-node ;  ossutil cp -f bin/meridian-node oss://host-wdrip-cn-hangzhou/meridian/bin/meridian-node
	
	make meridian-guest ; ossutil cp -f bin/meridian-guest oss://host-wdrip-cn-hangzhou/meridian/bin/meridian-guest
}

echo "[oss]download with url: http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/meridian/bin/"

case "$1" in
"binary")
	release::meridian_binary "linux" "amd64"
	;;
"script")
	release::installation_script
	;;
"universal")
	release::mac_universal_package
	;;
"meridian")
	release::meridian "darwin" "arm64"
	release::meridian "darwin" "amd64"
	;;
"meridian-node")
	release::meridian_target "meridian-node" "linux" "amd64"
	release::meridian_target "meridian-node" "linux" "arm64"
	;;
"meridian-guest")
	release::meridian_target "meridian-guest" "linux" "amd64"
	release::meridian_target "meridian-guest" "linux" "arm64"
	;;
"help")
	echo "release.sh [ binary | script | universal | meridia | meridian-node | meridian-guest ]"
	;;
"*")
	;;
esac


