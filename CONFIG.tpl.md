## 配置文件模版
查看 ./test/ 目录

1. XDPIN.CFG 配置文件
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    xdpin.cn/mark: ""
  name: xdpin.cfg
  namespace: kube-system
data:
  config: |
    dns:
      state: enable
      authProvider: kubernetes.aoxn
      auth:
        description: spacex_nice@163.com
      region: cn-hangzhou
      domainRR: "@,www"
      domainName: "xdpin.cn"
    mapping:
    - state: enable
      externalPort: 22
      internalPort: 22
      protocol: TCP
      description: "ssh"
    - state: enable
      externalPort: 6443
      internalPort: 6443
      protocol: TCP
      description: "kubernetes"
    - state: enable
      externalPort: 8096
      internalPort: 8096
      protocol: TCP
      description: "jellyfin"
```

2. AuthProvider配置文件样板

```yaml
apiVersion: xdpin.cn/v1
kind: Provider
metadata:
  name: kubernetes.aoxn
spec:
  authInfo:
    type: alibaba
    region: cn-hangzhou
    access-key: aaaaaaaa
    access-secret:  bbbbbbbb
```

3. NodeGroup
```yaml

apiVersion: xdpin.cn/v1
kind: NodeGroup
metadata:
  name: game
spec:
  replicas: 1
  provider: kubernetes.aoxn

```