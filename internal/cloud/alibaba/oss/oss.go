package oss

import (
	"bufio"
	"fmt"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	"github.com/denverdino/aliyungo/oss"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"os"
	"strings"
)

var _ cloud.IObjectStorage = &ossProvider{}

func NewOSS(mgr *client.ClientMgr) cloud.IObjectStorage {
	return &ossProvider{ClientMgr: mgr}
}

type ossProvider struct {
	bucket string
	*client.ClientMgr
}

func (n *ossProvider) BucketName() string { return n.bucket }

func (n *ossProvider) EnsureBucket(name string) error {
	if name == "" {
		return fmt.Errorf("empyt bucket name")
	}
	_, err := n.OSS.Bucket(name).Info()
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchBucket") {
			return n.OSS.Bucket(name).PutBucket(oss.Private)
		}
		return errors.Wrapf(err, "query bucket info")
	}

	return nil
}

func (n *ossProvider) GetObject(src string) ([]byte, error) {
	bName, mpath := n.bucket, src
	if strings.HasPrefix(src, "oss://") {
		segs := strings.Split(src, "/")
		if len(segs) < 4 {
			return nil, fmt.Errorf("invalid oss bucket: %s", src)
		}
		// override bucket name by user
		bName = segs[2]
		mpath = strings.Replace(src, fmt.Sprintf("oss://%s/", bName), "", -1)
	}
	klog.Infof("[service]oss get object from [oss://%s/%s]", bName, mpath)
	bucket := n.OSS.Bucket(bName)
	data, err := bucket.Get(mpath)
	if err != nil {
		return nil, errors.Wrapf(err, "get oss object: path=[oss://%s/%s]", bName, mpath)
	}
	return data, nil
}

func (n *ossProvider) GetFile(src, dst string) error {
	bName, mpath := n.bucket, src
	if strings.HasPrefix(src, "oss://") {
		segs := strings.Split(src, "/")
		if len(segs) < 4 {
			return fmt.Errorf("invalid oss bucket: %s", src)
		}
		// override bucket name by user
		bName = segs[2]
		mpath = strings.Replace(src, fmt.Sprintf("oss://%s/", bName), "", -1)
	}
	klog.Infof("[service]oss get file from [oss://%s/%s]", bName, mpath)
	bucket := n.OSS.Bucket(bName)
	reader, err := bucket.GetReader(mpath)
	if err != nil {
		return errors.Wrap(err, "get bucket reader")
	}
	defer reader.Close()
	desc, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return errors.Wrapf(err, "open dest file:%s", dst)
	}
	defer desc.Close()
	cnt, err := io.Copy(bufio.NewWriterSize(desc, 10*1024*1024), reader)
	klog.Infof("[service]get file[%s] from oss, read count[%d], to[%s]", src, cnt, dst)
	return err
}

func (n *ossProvider) PutFile(src, dst string) error {
	bName, mpath := n.bucket, dst
	if strings.HasPrefix(dst, "oss://") {
		segs := strings.Split(dst, "/")
		if len(segs) < 4 {
			return fmt.Errorf("invalid oss bucket: %s", dst)
		}
		// override bucket name by user
		bName = segs[2]
		mpath = strings.Replace(dst, fmt.Sprintf("oss://%s/", bName), "", -1)
	}
	klog.Infof("[service]oss put file to [oss://%s/%s]", bName, mpath)

	bucket := n.OSS.Bucket(bName)
	desc, err := os.OpenFile(src, os.O_RDONLY, 0777)
	if err != nil {
		return errors.Wrapf(err, "open file: %s", src)
	}
	defer desc.Close()
	return bucket.PutFile(mpath, desc, oss.Private, oss.Options{})
}

func (n *ossProvider) PutObject(b []byte, dst string) error {
	bName, mpath := n.bucket, dst
	if strings.HasPrefix(dst, "oss://") {
		segs := strings.Split(dst, "/")
		if len(segs) < 4 {
			return fmt.Errorf("invalid oss bucket: %s", dst)
		}
		// override bucket name by user
		bName = segs[2]
		mpath = strings.Replace(dst, fmt.Sprintf("oss://%s/", bName), "", -1)
	}
	klog.Infof("[service]oss put object to [oss://%s/%s]", bName, mpath)

	bucket := n.OSS.Bucket(bName)
	return bucket.Put(mpath, b, oss.DefaultContentType, oss.Private, oss.Options{})
}

func (n *ossProvider) DeleteObject(dst string) error {
	bName, mpath := n.bucket, dst
	if strings.HasPrefix(dst, "oss://") {
		segs := strings.Split(dst, "/")
		if len(segs) < 4 {
			return fmt.Errorf("invalid oss bucket: %s", dst)
		}
		// override bucket name by user
		bName = segs[2]
		mpath = strings.Replace(dst, fmt.Sprintf("oss://%s/", bName), "", -1)
	}
	klog.Infof("[service]oss delete object [oss://%s/%s]", bName, mpath)
	bucket := n.OSS.Bucket(bName)
	return bucket.Del(mpath)
}

func (n *ossProvider) ListObject(prefix string) ([][]byte, error) {
	bName := n.bucket
	if err := n.EnsureBucket(bName); err != nil {
		return nil, errors.Wrapf(err, "ensure bucket")
	}

	mlist, err := n.OSS.Bucket(bName).List(prefix, "", "", 1000)
	if err != nil {
		return nil, errors.Wrapf(err, "list object: %s", bName)
	}
	var result [][]byte
	for _, v := range mlist.Contents {
		data, err := n.GetObject(v.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "get object by key: %s", v.Key)
		}
		result = append(result, data)
	}
	return result, nil
}
