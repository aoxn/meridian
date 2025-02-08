package xdpin

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/aoxn/meridian/internal/tool/address"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func NewXdpDomain() Periodical {
	return &xdpDomain{}
}

type xdpDomain struct {
}

func (s *xdpDomain) Name() string {
	return "xdpin.domain.controller"
}

func (s *xdpDomain) Schedule() string {
	return "*/3 * * * *"
}

func (s *xdpDomain) Run(options Options) error {
	cfg, err := LoadCfg()
	if err != nil {
		return err
	}
	ddns, err := NewDDNS(&cfg)
	if err != nil {
		return errors.Wrapf(err, "build ddns failed")
	}
	klog.Info("start ddns watching and updating...")
	return ddns.Sync(cfg.XdpDomain.DomainName)
}

func doDDNSUpdate() {
	err := WatchAndUpdateDDNS()
	if err != nil {
		klog.Errorf("failed to watch ddns update: %v", err)
	}
}

type MatchSet struct {
	DomainName string
	RR         string
	Type       string
	Value      string
}

type UpdateSet struct {
	RecordId string
	RR       string
	Type     string
	Value    string
}

func WatchAndUpdateDDNS() error {

	cfg, err := LoadCfg()
	if err != nil {
		return err
	}
	ddns, err := NewDDNS(&cfg)
	if err != nil {
		return errors.Wrapf(err, "build ddns failed")
	}
	klog.Info("start ddns watching and updating...")
	wait.UntilWithContext(context.TODO(), func(cxt context.Context) {
		err = ddns.Sync(cfg.XdpDomain.DomainName)
		if err != nil {
			klog.Errorf("sync ddns failed, %v", err)
		}
		klog.Infof("sync ddns record finished.[%s]", cfg.XdpDomain.DomainName)
	}, 10*time.Minute)
	return nil
}

func NewDDNS(f *Config) (*DDNS, error) {
	if f.XdpDomain.DomainName == "" ||
		f.XdpDomain.DomainRR == "" ||
		f.XdpDomain.AccessKeyID == "" ||
		f.XdpDomain.AccessKeySecret == "" ||
		f.XdpDomain.Region == "" || f.XdpDomain.Provider == "" {
		return nil, fmt.Errorf("ddns args must be set:[ domain-name, domain-rr, ak, secret, provider]")
	}
	client, err := alidns.NewClientWithAccessKey(f.XdpDomain.Region, f.XdpDomain.AccessKeyID, f.XdpDomain.AccessKeySecret)
	if err != nil {
		klog.Errorf("init client failed %s", err.Error())
		return nil, errors.Wrapf(err, "init client failed")
	}
	prvd := address.FindBy(f.XdpDomain.Provider)
	if prvd == nil {
		return nil, errors.Errorf("provider not found: [%s], supported=%s", f.XdpDomain.Provider, address.Supported())
	}
	return &DDNS{client: client, domains: f.XdpDomain.DomainRR, prvd: prvd}, nil
}

type DDNS struct {
	domains string
	prvd    address.Resolver
	client  *alidns.Client
}

func (d *DDNS) buildRecord() []MatchSet {
	realIPs, err := d.prvd.GetAddr()
	if err != nil {
		klog.Warningf("get addr failed %s", err.Error())
		return nil
	}
	if realIPs.IPv4 == nil && realIPs.IPv6 == nil {
		klog.Warningf("unable to found addr for %v", realIPs)
		return nil
	}

	// build matchset
	var matchSet []MatchSet
	rrs := strings.Split(d.domains, ",")
	for _, rr := range rrs {
		if realIPs.IPv6 != nil {
			matchSet = append(matchSet, MatchSet{
				RR:    rr,
				Type:  "AAAA",
				Value: realIPs.IPv6.String(),
			})
		}
		if realIPs.IPv4 != nil {
			matchSet = append(matchSet, MatchSet{
				RR:    rr,
				Type:  "A",
				Value: realIPs.IPv4.String(),
			})
		}
	}

	return matchSet
}

func (d *DDNS) Sync(domainName string) error {
	klog.Infof("start to sync dns: domain=%s", domainName)
	matchSet := d.buildRecord()
	klog.Infof("build dns record for domain=%s, match set=[%s]", domainName, matchSet)
	request := alidns.CreateDescribeDomainRecordsRequest()
	request.Scheme = "https"
	request.AcceptFormat = "json"
	request.DomainName = domainName

	response, err := d.client.DescribeDomainRecords(request)
	if err != nil {
		return errors.Wrapf(err, "describe domain records")
	}
	klog.Infof("get dns buildRecord from aliyun dns: %v", response.DomainRecords.Record)
	// matchSet is what we expected
	// updateSet is what we will do update
	// createSet is (matchSet - updateSet) that is we will add

	// 1) add all to createSet
	createSet := make(map[MatchSet]struct{}, 2)
	for _, wantedRecord := range matchSet {
		createSet[wantedRecord] = struct{}{}
	}

	var updateSet []UpdateSet
	for _, existed := range response.DomainRecords.Record {
		for _, wanted := range matchSet {
			if existed.Type != wanted.Type {
				continue
			}
			if existed.RR != wanted.RR {
				continue
			}
			// check semantics
			existedIP := net.ParseIP(existed.Value)
			if existedIP != nil && existedIP.Equal(net.ParseIP(wanted.Value)) {
				delete(createSet, wanted)
				continue
			}

			toUpdate := UpdateSet{
				RecordId: existed.RecordId,
				RR:       existed.RR,
				Type:     existed.Type,
				Value:    wanted.Value,
			}
			updateSet = append(updateSet, toUpdate)
			// remove from createSet
			delete(createSet, wanted)
		}
	}
	if len(updateSet) == 0 {
		klog.Infof("remote dns consist, no need to update: domain=%s", domainName)
		return nil
	}
	for _, up := range updateSet {
		updateRequest := alidns.CreateUpdateDomainRecordRequest()
		updateRequest.Scheme = "https"
		updateRequest.AcceptFormat = "json"
		updateRequest.RecordId = up.RecordId
		updateRequest.Type = up.Type
		updateRequest.RR = up.RR
		updateRequest.Value = up.Value

		updateResp, err := d.client.UpdateDomainRecord(updateRequest)
		if err != nil {
			return errors.Wrapf(err, "update domain record failed: %v", updateRequest)
		}
		klog.Infof("update dns resolve record [%s %s %s], %s %s", domainName, up.RR, up.Value, updateResp.RequestId, updateResp.RequestId)
	}

	for add := range createSet {
		addRequest := alidns.CreateAddDomainRecordRequest()
		addRequest.Scheme = "https"
		addRequest.AcceptFormat = "json"
		addRequest.Type = add.Type
		addRequest.RR = add.RR
		addRequest.Value = add.Value
		addRequest.DomainName = domainName

		createResp, err := d.client.AddDomainRecord(addRequest)
		if err != nil {
			return errors.Wrapf(err, "add domain record")
		}
		klog.Infof("create resolve domain (%s %s %s) %s\n", domainName, add.RR, add.Value, createResp.String())
	}
	return nil
}
