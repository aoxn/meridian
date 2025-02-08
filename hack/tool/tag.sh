#!/usr/bin/env bash

#git describe --tags --long|awk -F '-' '{print $1"-"$3}'|awk -F "-g" '{print $1"-"$2}'
git describe --tags --long|awk -F '-' '{print $1}'
