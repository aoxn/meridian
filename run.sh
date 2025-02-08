#!/bin/bash
meridian delete cluster k001
meridian get task|grep -v NAME|awk '{print $1}'|xargs -I '{}' meridian delete task {}

# wget -O meridian http://oss-cn-hangzhou.aliyuncs.com/bin/meridian