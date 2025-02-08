#!/bin/bash
set -e

target_repo=registry.cn-hangzhou.aliyuncs.com

function build() {

	local url=$1
	local ns=$2
	local name=$3
	local tag_amd=$4
	local tag_arm=$5
	r_prefix=$url/$ns/$name
	if [[ "$url" == "" ]];then
		r_prefix=$ns/$name
	fi
	docker pull $r_prefix:$tag_amd
	docker pull $r_prefix:$tag_arm
	docker tag $r_prefix:$tag_amd $target_repo/aoxn/$name:$tag_amd
	docker tag $r_prefix:$tag_arm $target_repo/aoxn/$name:$tag_arm
	
	docker push $target_repo/aoxn/$name:$tag_amd
	docker push $target_repo/aoxn/$name:$tag_arm
		
	docker manifest rm registry.cn-hangzhou.aliyuncs.com/aoxn/jellyfin:latest || true
	docker manifest create $target_repo/aoxn/$name:latest $target_repo/aoxn/$name:$tag_amd $target_repo/aoxn/$name:$tag_arm
	
	#docker manifest annotate $target_repo/aoxn/$name:latest $target_repo/aoxn/$name:$tag_amd --os linux --arch amd64
	#docker manifest annotate $target_repo/aoxn/$name:latest $target_repo/aoxn/$name:$tag_arm --os linux --arch arm64

	docker manifest push $target_repo/aoxn/$name:latest
	docker manifest inspect $target_repo/aoxn/$name:latest
}

build "" "jellyfin" "jellyfin" "2024120905-amd64" "2024120905-arm64"
