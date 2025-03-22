#!/bin/bash
set -e

home=$HOME
if [[ "$home" == "" ]];
then
	echo "unknown home dir";exit 1;
fi

function downloadFor(){
	local name=$1
	local version=$2
	local type=$3
	local arch=$4
	if [[ "$name" == "" || "$version" == "" || "$type" == "" || "$arch" == "" ]];then
		echo "unexpected empty [name,version,type,arch]";exit 1;
	fi
	file=${name}_${version}_${type}_${arch}.tar
	path=$home/.cache/meridian/download/$name
	mkdir -p $path
	wget --tries 10 --no-check-certificate -O $path/$file \
		http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/meridian/default/public/$name/$file
	mkdir -p $path/extract/$version/$arch
	tar xf $path/$file -C $path/extract/$version/$arch
}

function loopDownloads() {
	downloadFor "etcd" "v3.4.3" "elf" "amd64"
	downloadFor "kubernetes" "1.31.1-aliyun.1" "elf" "amd64"
	downloadFor "containerd" "1.6.21" "deb" "amd64"
	downloadFor "containerd" "1.6.28" "deb" "amd64"
	downloadFor "docker" "20.10.24" "deb" "amd64"
	downloadFor "docker" "26.1.4" "deb" "amd64"
}

function buildDir() {
        local name=$1
        local version=$2
        local type=$3
        local arch=$4
        if [[ "$name" == "" || "$version" == "" || "$type" == "" || "$arch" == "" ]];then
                echo "unexpected empty [name,version,type,arch]";exit 1;
        fi
        file=${name}_${version}_${type}_${arch}.tar
        path=$home/.cache/meridian/build/$name
        mkdir -p $path
        mkdir -p $path/extract/$version/$arch
}

function tarPkg() {
        local name=$1
        local version=$2
        local type=$3
        local arch=$4
        if [[ "$name" == "" || "$version" == "" || "$type" == "" || "$arch" == "" ]];then
                echo "unexpected empty [name,version,type,arch]";exit 1;
        fi
        file=${name}_${version}_${type}_${arch}.tar
        path=$home/.cache/meridian/build/$name
        mkdir -p $path
        mkdir -p $path/extract/$version/$arch
        tar cf $path/$file -C $path/extract/$version/$arch .
        echo "======file list: $file======="
        tar tf $path/$file
}

function releasePkg() {
        local name=$1
        local version=$2
        local type=$3
        local arch=$4
        if [[ "$name" == "" || "$version" == "" || "$type" == "" || "$arch" == "" ]];then
                echo "unexpected empty [name,version,type,arch]";exit 1;
        fi
        file=${name}_${version}_${type}_${arch}.tar
        path=$home/.cache/meridian/build/$name
        echo "======file list: $file======="
        ossutil cp -f "$path/$file" oss://host-wdrip-cn-hangzhou/meridian/default/public/$name/$file
}

function loopBuildDir() {
	local arch=$1
	if [[ "$arch" == "" ]];then
                echo "unexpected empty [arch]";exit 1;
        fi
        buildDir "etcd" "v3.4.3" "elf" "$arch"
        buildDir "kubernetes" "1.31.1-aliyun.1" "elf" "$arch"
        buildDir "containerd" "1.6.21" "deb" "$arch"
        buildDir "containerd" "1.6.28" "deb" "$arch"
        buildDir "docker" "20.10.24" "deb" "$arch"
        buildDir "docker" "26.1.4" "deb" "$arch"
}

function loopTarPkg() {
        local arch=$1
        if [[ "$arch" == "" ]];then
                echo "unexpected empty [arch]";exit 1;
        fi
        tarPkg "etcd" "v3.4.3" "elf" "$arch"
        tarPkg "kubernetes" "1.31.1-aliyun.1" "elf" "$arch"
        tarPkg "containerd" "1.6.21" "deb" "$arch"
        tarPkg "containerd" "1.6.28" "deb" "$arch"
        tarPkg "docker" "20.10.24" "deb" "$arch"
        tarPkg "docker" "26.1.4" "deb" "$arch"
}

function loopReleasePkg() {
        local arch=$1
        if [[ "$arch" == "" ]];then
                echo "unexpected empty [arch]";exit 1;
        fi
        releasePkg "etcd" "v3.4.3" "elf" "$arch"
        releasePkg "kubernetes" "1.31.1-aliyun.1" "elf" "$arch"
        releasePkg "containerd" "1.6.21" "deb" "$arch"
        releasePkg "containerd" "1.6.28" "deb" "$arch"
        releasePkg "docker" "20.10.24" "deb" "$arch"
        releasePkg "docker" "26.1.4" "deb" "$arch"
}

function release_one_pkg() {
        local pkg=$1
        local version=$2
        local category=$3
        local arch=$4

        tarPkg "$pkg" "$version" "$category" "$arch"
        buildDir "$pkg" "$version" "$category" "$arch"
        releasePkg "$pkg" "$version" "$category" "$arch"
}

#loopDownloads
#loopBuildDir "arm64"
#loopTarPkg "arm64"
#loopReleasePkg "arm64"

release_one_pkg "nvidia-toolkit" "1.17.5" "deb" "amd64"
