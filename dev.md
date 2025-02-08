

1. 公网访问慢的问题：判断节点类型，是阿里云机器并且是杭州的Region的话，可以考虑使用内网域名下载
2. [done]添加meridian --cache 参数，使用缓存的安装包。
3. 判断节点上的组件version, 必要时候可以不用重新下载并安装。只做配置apply.
4. [done]meridian new request 命令的token需要随机生成。
5. xdpin => 支持管理账号Secret，以及账号自动备份恢复。
6. xdpin => state 改用Json文件。LocalState作为一级对象。
7. [done]owncloud 需要 SSL及 mysql db.
8. meridian destroy 清理iptables规则. 删除node object

```
#!/bin/bash

ossutil cp bin/meridian oss://host-wdrip-cn-hangzhou/bin/meridian

ossutil cp request.yaml oss://host-wdrip-cn-hangzhou/bin/request.yaml


wget -O /usr/local/bin/meridian http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/bin/meridian
```


```
apiVersion: v1
kind: Secret
metadata:
  name: application-passwd
type: kubernetes.io/basic-auth
stringData:
  password: test1234

```

## Title Run Local Kubernetes the Easy Way.
1. Out of Box
2. Resilient Kubernetes
3. Cloud Access

## todo list
1. 创建VM的命令支持 --cpus 4 --memorys 4GB --with-kubernetes --config /var/lib/vmconfig.yml 选项。(meridian create; meridian guest Request; guest info)
    1) guest 支持Request创建kubernetes集群
    2) guestInfo 支持conditions
2. vm daemon 化，与meridiand解耦掉。
3. vm autostart, 开机自启动。
4. vm port foward. 包括docker sock 和kubenetes svc port.
5. windows support
6. get vm ; NAME STATE ADDR
7. mount host directory


docker download path:
```
https://download.docker.com/linux/ubuntu/dists/noble/pool/stable/amd64/docker-ce-cli_26.1.4-1~ubuntu.24.04~noble_amd64.deb

MAC for arm
https://download.docker.com/mac/static/stable/aarch64/docker-26.1.4.tgz

MAC for amd
https://download.docker.com/mac/static/stable/x86_64/docker-26.1.4.tgz

sudo xattr -rc docker
sudo cp docker/docker /usr/local/bin/


Win for amd
https://download.docker.com/win/static/stable/x86_64/docker-26.1.4.zip
PS C:\> Expand-Archive /path/to/<FILE>.zip -DestinationPath $Env:ProgramFiles

PS C:\> &$Env:ProgramFiles\Docker\dockerd --register-service
PS C:\> Start-Service docker


```
