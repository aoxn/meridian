package xdpin

import (
	"fmt"
	"k8s.io/klog/v2"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aoxn/meridian/internal/tool/address"
)

func NewSSHSGRP() Periodical {
	return &sshSgrp{}
}

type sshSgrp struct {
}

func (s *sshSgrp) Name() string {
	return "ssh.securityGroup.controller"
}

func (s *sshSgrp) Schedule() string {
	return "*/10 * * * *"
}

func (s *sshSgrp) Run(options Options) error {
	cfg, err := LoadCfg()
	if err != nil {
		return err
	}
	klog.Infof("sync security group: %s", cfg.SSHSecrurityGroup.SecurityGroupID)
	err = EnsureSecurityGroup(&cfg)
	if err != nil {
		klog.Errorf("ensure security group failed: %s", err.Error())
	}

	return EnsureACL()
}

func WaitAndUpdate() {
	// aliyun has minimum TTL 600 (s)
	t := time.NewTicker(11 * time.Minute)
	defer t.Stop()

	for {
		cfg, err := LoadCfg()
		if err != nil {
			klog.Errorf("xdpin waitAndUpdate err: %v", err)
			continue
		}
		klog.Infof("sync security group: %s", cfg.SSHSecrurityGroup.SecurityGroupID)
		select {
		case <-t.C:
			err := EnsureSecurityGroup(&cfg)
			if err != nil {
				klog.Errorf("ensure security group failed: %s", err.Error())
			}

			err = EnsureACL()
			if err != nil {
				klog.Errorf("ensure acl group failed: %s", err.Error())
			}
			klog.Infof("tick for next security group update in 11 minutes: %s", cfg.SSHSecrurityGroup.SecurityGroupID)

		}
	}

}

func EnsureSecurityGroup(f *Config) error {

	prvd := address.FindBy(f.SSHSecrurityGroup.Provider)
	if prvd == nil {
		return fmt.Errorf("address provider not found: %s", f.SSHSecrurityGroup.Provider)
	}
	ip, err := prvd.GetAddr()
	if err != nil {
		return fmt.Errorf("ip fetch failed: %s", err.Error())
	}
	if ip.IPv4 == nil {
		return fmt.Errorf("ip not found: %s", err.Error())
	}

	auth := f.SSHSecrurityGroup.Auth

	client, err := ecs.NewClientWithAccessKey(
		auth.Region,
		auth.AccessKeyID,
		auth.AccessKeySecret,
	)
	if err != nil {
		return err
	}
	id := f.SSHSecrurityGroup.SecurityGroupID

	req := ecs.CreateDescribeSecurityGroupAttributeRequest()
	req.SecurityGroupId = id
	req.RegionId = f.SSHSecrurityGroup.Region
	res, err := client.DescribeSecurityGroupAttribute(req)
	if err != nil {
		return err
	}
	s2 := strings.Split(ip.IPv4.String(), ".")
	nrule := newrule(fmt.Sprintf("%s.0.0.0/8", s2[0]))
	found := false
	for _, p := range res.Permissions.Permission {
		klog.Infof("secure group: %s, %s, %s", f.SSHSecrurityGroup.SecurityGroupID, p.IpProtocol, p.SourceCidrIp)
		if strings.HasPrefix(p.Description, ruleIdentity(f.SSHSecrurityGroup.RuleIdentity, "")) {
			found = true
			klog.Infof("found tagged rule: %+v", p)
			s1 := strings.Split(p.SourceCidrIp, ".")
			if len(s1) != len(s2) && len(s1) != 4 {
				continue
			}
			if s1[0] != s2[0] {
				klog.Infof("ip not matched: pre=%s, next=%s", p.SourceCidrIp, ip)
				klog.Infof("revoke rule: %s", p.SourceCidrIp)
				err := revoke(client, p, id)
				if err != nil {
					fmt.Println("xxx")
				}
				desc := ruleIdentity(f.SSHSecrurityGroup.RuleIdentity, ip.IPv4.String())
				klog.Infof("insert new rule: %s", ip.IPv4.String())
				return insert(client, nrule, id, desc)
			}

		}
	}
	if !found {
		klog.Infof("first insert new rule: %s", ip.IPv4.String())
		return insert(client, nrule, id, ruleIdentity(f.SSHSecrurityGroup.RuleIdentity, ip.IPv4.String()))
	}
	return nil
}

func ruleIdentity(id, ip string) string {
	if id == "" {
		id = "default"
	}
	return fmt.Sprintf("%s:%s,%s", AuthTag, id, ip)
}

const AuthTag = "auth:aoxn"

func newrule(srcidr string) ecs.Permission {
	return ecs.Permission{
		Policy:          "Accept",
		SourceCidrIp:    srcidr,
		PortRange:       "-1/-1",
		SourcePortRange: "-1/-1",
		IpProtocol:      "ALL",
		NicType:         "intranet",
	}
}

func insert(client *ecs.Client, p ecs.Permission, id, desc string) error {
	autho := ecs.CreateAuthorizeSecurityGroupRequest()
	autho.Policy = p.Policy
	autho.SourceCidrIp = p.SourceCidrIp
	autho.PortRange = p.PortRange
	autho.SourcePortRange = p.SourcePortRange
	autho.IpProtocol = p.IpProtocol
	autho.NicType = p.NicType
	autho.Priority = "1"
	autho.SecurityGroupId = id
	autho.Description = desc
	_, err := client.AuthorizeSecurityGroup(autho)
	// autho.
	return err
}

func revoke(client *ecs.Client, p ecs.Permission, id string) error {
	rev := ecs.CreateRevokeSecurityGroupRequest()
	rev.SourceCidrIp = p.SourceCidrIp
	rev.PortRange = p.PortRange
	rev.NicType = p.NicType
	rev.Policy = p.Policy
	rev.SecurityGroupId = id

	// delete & recreate
	_, err := client.RevokeSecurityGroup(rev)
	return err
}
