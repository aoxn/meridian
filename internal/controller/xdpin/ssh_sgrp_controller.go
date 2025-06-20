package xdpin

import (
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aoxn/meridian/internal/tool/address"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
)

func NewSSHSGRP(mgr manager.Manager) Periodical {
	return &sshSgrp{mgr: mgr}
}

type sshSgrp struct {
	mgr manager.Manager
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
	err = EnsureSecurityGroup(s.mgr.GetClient(), &cfg)
	if err != nil {
		klog.Errorf("ensure security group failed: %s", err.Error())
	}

	return EnsureACL(s.mgr.GetClient())
}

func EnsureSecurityGroup(cli client.Client, f *Config) error {
	if f.SSHSecrurityGroup.Provider == "" {
		return nil
	}
	ip, err := address.GetAddress()
	if err != nil {
		return fmt.Errorf("ip fetch failed: %s", err.Error())
	}
	if ip.IPv4 == nil {
		return fmt.Errorf("ip not found: %s", err.Error())
	}

	auth, err := GetAuth(cli, f.SSHSecrurityGroup.Provider)
	if err != nil {
		return errors.Wrapf(err, "sg: get auth provider[%s] failed", f.SSHSecrurityGroup.Provider)
	}
	if auth.Spec.AccessKey == "" || auth.Spec.AccessSecret == "" {
		return fmt.Errorf("ddns: auth provider %s is invalid, empty acceessKey & secret", f.SSHSecrurityGroup.Provider)
	}
	client, err := ecs.NewClientWithAccessKey(
		f.SSHSecrurityGroup.Region,
		auth.Spec.AccessKey,
		auth.Spec.AccessSecret,
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
