# Welcome to Meridian

Merdian is a sandbox management tool suitable for AI Agent and  Containers Develop include OutOfTheBox Kubernetes environment.

It runs on intel & Apple silicon Mac and provided user with Linux &MacOS VM capabilitys by Apple's  virtualization.framework in near-native performance.

Meridian also create and manage Docker & Kubernetes environment in minutes, and with gVisor support specially.

Both Local CLI and API are provided to control sandbox lifecycle for easy  manipulation and integration

## Virtual Machine
Get up and running multiple virtual machine for easy test. 
Planned for multiple os platform support on mac/windows/linux. (MacOS only for now)

## Container Management
Meridian provide container running environment with docker engine. You can build, push, pull and run container with docker command.

## Kubernetes
Meridian is built in with kubernetes support. It can create multiple kubernetes environment on local desktop out of box.

## Multiple Cloud
You can add cloud workers into the kubernetes cluster, alibaba cloud is supported , aws worker support is coming soon.


## Roadmap
- linux support
- windows support
- aws nodegroup support
- helm support

## Quick Guide

### Install

```shell

root# curl get.xdpin.cn/meridian/iou.sh|bash -x
```

set meridian alia command `alias m=meridian`

### VM manage

Find available vm images

```shell
➜  meridian git:(main) ✗ m get image -d
NAME                          OS        ARCH      VERSION        LABELS         
ubuntu24.04.x86_64            Linux     x86_64    24.04          gui:false      
ubuntu24.04.arm64             Linux     aarch64   24.04          gui:false      
macos.15.6.aarch64            Darwin    aarch64   15.6           gui:true       
macos.latest.aarch64          Darwin    aarch64   latest         gui:true       
➜  meridian git:(main) ✗ m get image
NAME                          OS        ARCH      VERSION        LABELS         
ubuntu24.04.x86_64            Linux     x86_64    24.04          gui:false  

```

Pull image by os & arch, it takes time to download image.

```shell
➜  meridian git:(main) ✗ m pull image ubuntu24.04.arm64
I0904 21:42:05.756013    1488 pull.go:86] pulling image: [ubuntu24.04.arm64]
58.78 KiB / 551.19 MiB [>____________________________________] 0.01% 32.32 KiB/s

```

create vm with proper image

```shell
➜  meridian git:(main) ✗ m create vm aoxn --image ubuntu24.04.arm64
➜  meridian git:(main) ✗ m get vm
NAME           OS        ARCH    CPUs    MEMs    STATE     ADDRESS             
abc            Linux     x86_64  4       4GiB    Stopped   192.168.64.2/24     
aoxn           Linux     x86_64  4       4GiB    Running   192.168.64.3/24  
```

or run vm with specified image directly, it takes time to download image

```shell
➜  meridian git:(main) ✗ m run vm aoxn --image ubuntu24.04.arm64
```

manage vm status with
```shell

➜  meridian git:(main) ✗ m start vm aoxn
➜  meridian git:(main) ✗ m stop vm aoxn 
```

ssh into vm with 
```shell
➜  meridian git:(main) ✗ ssh aoxn@192.168.64.3
```

### Docker manage

create docker in specified vm 

```shell
➜  meridian git:(main) ✗ m create docker --in aoxn 
➜  meridian git:(main) ✗ m get docker
NAME           VERSION        REF_VM         ENDPOINT                      
aoxn           1.6.28         aoxn           [docker context use aoxn]     
➜  meridian git:(main) ✗ docker context use aoxn
aoxn
Current context is now "aoxn"
➜  meridian git:(main) ✗ docker ps              
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES

➜  meridian git:(main) ✗ m delete docker aoxn
I0904 21:54:51.651301    2824 common.go:128] delete [docker/aoxn] Accepted

```

create docker in specified vm with `gvisor` runtime

```shell
➜  meridian git:(main) ✗ m create docker --in aoxn --gvisor
```

### kubernetes manage

```shell

➜  meridian git:(main) ✗ m create k8s --in aoxn 
➜  meridian git:(main) ✗ m get k8s
NAME           VERSION             REF_VM         STATE     ENDPOINT                      
aoxn           1.31.1-aliyun.1     aoxn           Deploying [kubectl context use aoxn]  
➜  meridian git:(main) ✗ kubectl config get-contexts
CURRENT   NAME                                       CLUSTER                 AUTHINFO             NAMESPACE
*         meridian.user.aoxn@meridian.cluster.aoxn   meridian.cluster.aoxn   meridian.user.aoxn   
➜  meridian git:(main) ✗ kubectl get no 
NAME                       STATUS   ROLES           AGE   VERSION
cn-hangzhou.192.168.64.3   Ready    control-plane   31s   v1.31.1-aliyun.1

```

## Develop Guide

### build project

```shell
## Build Server
➜  meridian git:(main) ✗ make meridiand 

## Build CLI
➜  meridian git:(main) ✗ make meridian

## Build vmm 
➜  meridian git:(main) ✗ make meridian-vm
```

## For More Information
visit [Docs](https://aoxn.github.io/meridian-docs/)

Virtual machine initialization is partially based on lima, thanks for their work.

