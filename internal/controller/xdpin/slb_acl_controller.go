package xdpin

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"github.com/aoxn/meridian/internal/tool/address"
)

func NewSLBACL(mgr manager.Manager) Periodical {
	return &slbAcl{mgr: mgr}
}

type slbAcl struct {
	mgr manager.Manager
}

func (s *slbAcl) Name() string {
	return "slb.acl.controller"
}

func (s *slbAcl) Schedule() string {
	return "*/10 * * * *"
}

func (s *slbAcl) Run(options Options) error {
	return EnsureACL(s.mgr.GetClient())
}

func EnsureACL(cli client.Client) error {
	cfgKey := client.ObjectKey{
		Name:      "xdpin.cfg",
		Namespace: "kube-system",
	}
	cfg, err := Load(cli, cfgKey)
	if err != nil {
		return err
	}
	if cfg.LbACL.Provider == "" {
		return nil
	}
	ip, err := address.GetAddress()
	if err != nil {
		return fmt.Errorf("ip fetch failed: %s", err.Error())
	}
	if ip.IPv4 == nil {
		return fmt.Errorf("ip not found: %v", err)
	}
	auth, err := GetAuth(cli, cfg.LbACL.Provider)
	if err != nil {
		return errors.Wrapf(err, "lbacl: get auth provider[%s] failed", cfg.LbACL.Provider)
	}
	if auth.Spec.AccessKey == "" || auth.Spec.AccessSecret == "" {
		return fmt.Errorf("ddns: auth provider %s is invalid, empty acceessKey & secret", cfg.LbACL.Provider)
	}
	client, err := slb.NewClientWithAccessKey(
		cfg.LbACL.Region,
		auth.Spec.AccessKey,
		auth.Spec.AccessSecret,
	)
	if err != nil {
		return err
	}
	req := slb.CreateDescribeAccessControlListAttributeRequest()

	req.AclId = cfg.LbACL.AclID
	req.RegionId = cfg.LbACL.Region
	res, err := client.DescribeAccessControlListAttribute(req)
	if err != nil {
		return err
	}
	found := false
	for _, p := range res.AclEntrys.AclEntry {
		klog.Infof("acl group: %s, %s, %s", cfg.LbACL.AclID, p.AclEntryIP, p.AclEntryComment)
		if p.AclEntryComment != comment {
			continue
		}
		found = true
		// found
		klog.Infof("found dashboard rule: %s, %s, %s", cfg.LbACL.AclID, p.AclEntryIP, p.AclEntryComment)
		if p.AclEntryIP != ip.IPv4.String() {
			// delete
			if err := remove(client, cfg.LbACL.AclID, p.AclEntryIP); err != nil {
				return errors.Wrap(err, "remove failed")
			}
			// add
			return add(client, cfg.LbACL.AclID, ip.IPv4.String())
		}
	}
	if !found {
		klog.Infof("first insert new rule: %s", ip.IPv4.String())
		return add(client, cfg.LbACL.AclID, ip.IPv4.String())
	}
	return nil
}

const (
	comment = "dashboard"
)

func add(client *slb.Client, id, ip string) error {
	klog.Infof("trying to add acl: %s, %s", id, ip)
	req := slb.CreateAddAccessControlListEntryRequest()
	req.AclId = id
	if strings.Index(ip, "/") == -1 {
		ip = fmt.Sprintf("%s/32", ip)
	}
	data, err := json.Marshal([]AclEntry{
		{AclEntryIP: ip, AclEntryComment: comment},
	})
	if err != nil {
		return errors.Wrap(err, "acl entry:")
	}
	klog.Infof("add data: %s", string(data))
	req.AclEntrys = string(data)
	_, err = client.AddAccessControlListEntry(req)
	return err
}

func remove(client *slb.Client, id, ip string) error {
	klog.Infof("trying to remove acl: %s, %s", id, ip)
	req := slb.CreateRemoveAccessControlListEntryRequest()
	req.AclId = id
	if strings.Index(ip, "/") == -1 {
		ip = fmt.Sprintf("%s/32", ip)
	}
	data, err := json.Marshal([]AclEntry{
		{AclEntryIP: ip, AclEntryComment: comment},
	})
	if err != nil {
		return errors.Wrap(err, "acl entry:")
	}
	klog.Infof("remove data: %s", string(data))
	req.AclEntrys = string(data)
	_, err = client.RemoveAccessControlListEntry(req)
	return err
}

type AclEntry struct {
	AclEntryIP      string `json:"entry" xml:"AclEntryIP"`
	AclEntryComment string `json:"comment" xml:"AclEntryComment"`
}
