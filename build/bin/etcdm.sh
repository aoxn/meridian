#!/bin/sh

CERT=/var/lib/etcd/cert

echo "示例命令"
echo      ETCDCTL_API=3 etcdctl \
    --endpoints=https://etcd0.event-subunit7.svc:2379 \
    --cacert=$CERT/server-ca.crt \
    --cert=$CERT/client.crt \
    --key=$CERT/client.key \
    get --prefix=true --keys-only=true  /c8f6160d140c8412985cbf7dd056de1b7

ENDPOINT=$1 ; shift
echo
echo
echo "运行命令:"
ETCDCTL_API=3 etcdctl \
    --endpoints="$ENDPOINT" \
    --cacert=$CERT/server-ca.crt \
    --cert=$CERT/client.crt \
    --key=$CERT/client.key \
    "$@"

