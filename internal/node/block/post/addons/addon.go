package addons

import (
	"embed"
	_ "embed"
	"encoding/base64"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/sign"
	"io/fs"
	"k8s.io/klog/v2"
	"path/filepath"
	"sort"
	"strings"

	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/google/uuid"
)

var (

	//go:embed addon.TEMPLATE.d
	tfs embed.FS
)

var (
	dftTemplateVersion = "0.1.0"
	DftAllAddons       = []*v1.Addon{
		MYSQLD,
		FLANNEL,
		FLANNEL_MASTER,
		CORDDNS,
		CCM,
		CSI_PROVISION,
		CSI_PLUGIN,
		OWNCLOUD,
		XDPIN,
		QBITTORRENT,
		JELLYFIN,
		PALWORLD,
		RAVEN_MASTER,
		RAVEN_WORKER,
		METRICS_SERVER,
		KUBEPROXY_MASTER,
		KUBEPROXY_WORKER,
		INGRESS,
		TERWAY,
		KONNECTIVITY_AGENT_MASTER,
		KONNECTIVITY_AGENT_WORKER,
	}

	NodeGroupAddons = []*v1.Addon{
		CSI_PLUGIN,
	}
	ProviderAddons = []*v1.Addon{
		CCM,
		CSI_PROVISION,
	}
	CCM = &v1.Addon{
		Name:     "ccm",
		Version:  "v2.8.1",
		Category: "System",
	}
	CORDDNS = &v1.Addon{
		Name:     "coredns",
		Version:  "v1.9.3.10-7dfca203-aliyun",
		Category: "System",
	}

	CSI_PLUGIN = &v1.Addon{
		Name:     "csi-plugin",
		Version:  "v1.31.3-df937b8-aliyun",
		Category: "System",
	}

	CSI_PROVISION = &v1.Addon{
		Name:     "csi-provision",
		Version:  "v1.31.3-df937b8-aliyun",
		Category: "System",
	}
	FLANNEL = &v1.Addon{
		Name:     "flannel",
		Version:  "v0.15.1.5-11d1c700-aliyun",
		Category: "System",
	}
	FLANNEL_MASTER = &v1.Addon{
		Name:     "flannel-master",
		Category: "System",
		Version:  "v0.15.1.5-11d1c700-aliyun",
	}

	INGRESS = &v1.Addon{
		Name:     "ingress-controller",
		Replicas: 2,
		Category: "System",
		Version:  "v0.22.0.5-552e0db-aliyun",
	}

	JELLYFIN = &v1.Addon{
		Name:     "jellyfin",
		Version:  "2024072905",
		Category: "Customized",
	}

	KUBEPROXY_MASTER = &v1.Addon{
		Name:     "kubeproxy-master",
		Version:  "v1.31.1-aliyun.1",
		Category: "System",
	}

	KUBEPROXY_WORKER = &v1.Addon{
		// UUID to force pod restart when apply
		//UUID:    uuid.New().String(),
		Name:     "kubeproxy-worker",
		Version:  "v1.31.1-aliyun.1",
		Category: "System",
	}
	METRICS_SERVER = &v1.Addon{
		Name:     "metrics-server",
		Version:  "v1.0.0.2-cc3b2d6-aliyun",
		Category: "System",
	}
	MYSQLD = &v1.Addon{
		Name:     "mysqld",
		Version:  "8.0",
		Category: "Customized",
	}

	OWNCLOUD = &v1.Addon{
		Name:     "owncloud",
		Version:  "10.15",
		Category: "Customized",
	}

	PALWORLD = &v1.Addon{
		Name:     "palworld",
		Version:  "latest",
		Category: "Customized",
	}
	QBITTORRENT = &v1.Addon{
		Name:     "qbittorrent-nox",
		Version:  "5.0.2-1",
		Category: "Customized",
	}
	TERWAY = &v1.Addon{
		Name:     "terway",
		Version:  "v1.2.1",
		Category: "System",
	}

	XDPIN = &v1.Addon{
		Name:     "xdpin",
		Version:  "0.1.0",
		Category: "System",
	}
	RAVEN_MASTER = &v1.Addon{
		Name:     "raven-master",
		Version:  "0.4.2",
		Category: "System",
	}
	RAVEN_WORKER = &v1.Addon{
		Name:     "raven-worker",
		Version:  "0.4.2",
		Category: "System",
	}
	KONNECTIVITY_AGENT_MASTER = &v1.Addon{
		Name:     "konnectivity-master",
		Version:  "0.0.37",
		Category: "System",
	}
	KONNECTIVITY_AGENT_WORKER = &v1.Addon{
		Name:     "konnectivity-worker",
		Version:  "0.0.37",
		Category: "System",
	}
)

func SetDftNodeGroupAddons(ng *v1.NodeGroup) {
	if len(ng.Spec.Addons) > 0 {
		return
	}
	ng.Spec.Addons = NodeGroupAddons
}

func SetDftClusterAddons(r *v1.RequestSpec) {
	if len(r.Config.Addons) > 0 {
		return
	}
	clusterAddons := []*v1.Addon{
		CORDDNS,
		XDPIN,
		METRICS_SERVER,
		FLANNEL,
		FLANNEL_MASTER,
		KUBEPROXY_MASTER,
		KUBEPROXY_WORKER,
		KONNECTIVITY_AGENT_MASTER,
		KONNECTIVITY_AGENT_WORKER,
	}
	r.Config.Addons = clusterAddons
}

var (
	addons AddonTpls
)

func init() {
	a, err := LoadTemplates()
	if err != nil {
		klog.Warningf("LoadAddonTpls failed: %v", err)
		return
	}
	addons = a
	klog.V(5).Infof("addons templates initialized: %d", len(addons))
}

type AddonTpls map[string][]*AddonTemplate

type AddonTemplate struct {
	Name    string
	Version string
	Data    string
}

type RenderData struct {
	R         *v1.Request
	NodeGroup string
	AuthInfo  v1.AuthInfo
}

// GetAddonByName exported get addon by name
func GetAddonByName(name string) (v1.Addon, error) {
	for _, addon := range DftAllAddons {
		if addon.Name == name {
			return *addon, nil
		}
	}
	return v1.Addon{}, fmt.Errorf("addon %s not found", name)
}

// GetAddonTplBy  exported get addon template
func GetAddonTplBy(name, tplVersion string) *AddonTemplate {
	return addons.GetAddonTplBy(name, tplVersion)
}

// RenderAddon exported render addon method
func RenderAddon(name string, data *RenderData) (string, error) {
	return addons.RenderAddon(name, data)
}

// RenderedRequestedAddons exported render addons in vm.Request.Addons
func RenderedRequestedAddons(cfg *v1.Request) (map[string]string, error) {
	var (
		addonMap = make(map[string]string)
	)
	ads := cfg.Spec.Config.Addons
	for _, ad := range ads {
		if ad.Name == "" || ad.Version == "" {
			return nil, fmt.Errorf("invalid addon: %+v", ad)
		}
		str, err := addons.RenderAddon(ad.Name, &RenderData{R: cfg})
		if err != nil {
			return nil, err
		}
		addonMap[ad.Name] = str
	}
	return addonMap, nil
}

func (addons AddonTpls) RenderAddon(name string, data *RenderData) (string, error) {
	addon := addonByName(name)
	if addon == nil {
		return "", fmt.Errorf("addon not found: %s", name)
	}
	tpl := addons.GetAddonTplBy(addon.Name, addon.TemplateVersion)
	if tpl == nil {
		return "", fmt.Errorf("addon template %s, name=%s not found", addon.TemplateVersion, addon.Name)
	}
	if addon.TemplateVersion == "" {
		addon.TemplateVersion = tpl.Version
		klog.Infof("empty tmeplate version, set addon %s template version to %s", addon.Name, addon.TemplateVersion)
	}
	return renderAddon(addon, string(tpl.Data), data)
}

func (addons AddonTpls) GetAddonTplBy(name, version string) *AddonTemplate {
	tab, ok := addons[name]
	if !ok {
		return nil
	}
	for _, t := range tab {
		if version == "" {
			return t
		}
		if t.Version == version {
			return t
		}
	}
	return nil
}

func addonByName(name string) *v1.Addon {
	for _, addon := range DftAllAddons {
		if addon.Name == name {
			return addon
		}
	}
	return nil
}

func LoadTemplates() (AddonTpls, error) {
	var (
		addonTpls = AddonTpls{}
	)
	dir, err := fs.ReadDir(tfs, "addon.TEMPLATE.d")
	if err != nil {
		return addonTpls, err
	}
	for _, file := range dir {
		if !file.IsDir() {
			continue
		}
		addonName := file.Name()
		vdir := filepath.Join("addon.TEMPLATE.d", addonName)
		ydir, err := tfs.ReadDir(vdir)
		if err != nil {
			continue
		}
		var addons []*AddonTemplate
		for _, yfile := range ydir {
			if yfile.IsDir() {
				continue
			}
			if !strings.HasSuffix(yfile.Name(), ".yml") {
				continue
			}
			tplVersion := strings.TrimSuffix(yfile.Name(), ".yml")
			klog.V(5).Infof("load default addon:%s/%s", addonName, tplVersion)
			data, err := tfs.ReadFile(filepath.Join(vdir, yfile.Name()))
			if err != nil {
				klog.Warningf("load default addon: %s/%s err: %v", addonName, tplVersion, err)
				continue
			}
			tpl := &AddonTemplate{
				Name:    addonName,
				Version: tplVersion,
				Data:    string(data),
			}
			addons = append(addons, tpl)
		}
		addonTpls[addonName] = addons
	}
	for key, _ := range addonTpls {
		sort.SliceStable(addonTpls[key], func(i, j int) bool {
			if addonTpls[key][i].Version > addonTpls[key][j].Version {
				return true
			}
			return false
		})
	}
	return addonTpls, nil
}

func renderAddon(addon *v1.Addon, tpldata string, data *RenderData) (string, error) {
	var (
		sgid string
		vsw  string
		err  error
	)
	var (
		req       = data.R
		addonYaml = ""
		cfg       = req.Spec.Config
	)
	tmp := strings.Split(cfg.Registry, ".")
	if len(tmp) != 4 {
		return addonYaml, fmt.Errorf("config generic format error: %s, "+
			"must be registry.${region}.aliyuncs.com/acs", cfg.Registry)
	}
	ip, err := tool.GetDNSIP(cfg.Network.SVCCIDR, 10)
	if err != nil {
		return addonYaml, fmt.Errorf("SVCCIDR must be an ip range: %s , "+
			"for 192.168.0.1/16", cfg.Network.SVCCIDR)
	}
	tpl := &ConfigTpl{
		Tpl:          tpldata,
		Name:         addon.Name,
		ImageVersion: addon.Version,
	}
	klog.V(5).Infof("debug cluster config: %s", tool.PrettyJson(cfg))
	port := "6443"
	if req.Spec.AccessPoint.APIPort != "" {
		port = req.Spec.AccessPoint.APIPort
	}
	tpl.IntranetApiServerEndpoint = fmt.Sprintf("https://%s:%s",
		req.Spec.AccessPoint.APIDomain, port)
	if data.NodeGroup != "" {
		tpl.ToNodeGroup = data.NodeGroup
	}
	tpl.AuthInfo = data.AuthInfo
	tpl.ProxyMode = "iptables"
	tpl.Namespace = "kube-system"
	tpl.IngressSlbNetworkType = "internet"
	tpl.ComponentRevision = "1111"
	tpl.Region = tmp[1]
	tpl.CIDR = cfg.Network.PodCIDR
	tpl.Action = "Ensure"
	tpl.IPStack = "ipv4"
	tpl.KubeDnsClusterIp = ip.String()
	tpl.ServiceCIDR = cfg.Network.SVCCIDR
	tpl.PodVswitchId = vsw
	tpl.APIPort = req.Spec.AccessPoint.APIPort
	tpl.TunnelPort = req.Spec.AccessPoint.TunnelPort
	tpl.APIDomain = req.Spec.AccessPoint.APIDomain
	tpl.APIAccessPoint = req.Spec.AccessPoint.Internet
	if tpl.APIAccessPoint == "" {
		tpl.APIAccessPoint = "127.0.0.1"
	}

	tpl.SecurityGroupID = sgid
	tpl.UUID = uuid.New().String()
	if tpl.Name == "xdpin" {
		ca, crt, key, err := sign.GenerateServerTripple()
		if err != nil {
			return addonYaml, err
		}
		tpl.WebHookCA = base64.StdEncoding.EncodeToString(ca)
		tpl.WebHookTLSCert = base64.StdEncoding.EncodeToString(crt)
		tpl.WebHookTLSKey = base64.StdEncoding.EncodeToString(key)
	}
	if tpl.Name == "raven-master" {
		tpl.RavenPSK = base64.StdEncoding.EncodeToString([]byte(tool.RandomID(10)))
	}
	addonYaml, err = tool.RenderConfig(fmt.Sprintf("addon.%s.tpl", tpl.Name), tpl.Tpl, tpl)
	if err != nil {
		return addonYaml, fmt.Errorf("render config: %s", err.Error())
	}
	klog.V(5).Infof("render addon: %s, %s", tpl.Name, addonYaml)

	return addonYaml, nil
}

type ConfigTpl struct {
	UUID                      string
	Name                      string
	Replicas                  string
	Namespace                 string
	Action                    string
	Region                    string
	ImageVersion              string
	KubeDnsClusterIp          string
	CIDR                      string
	Tpl                       string
	ComponentRevision         string
	IngressSlbNetworkType     string
	ProxyMode                 string
	IntranetApiServerEndpoint string

	APIPort         string
	TunnelPort      string
	APIDomain       string
	APIAccessPoint  string
	IPStack         string
	ServiceCIDR     string
	SecurityGroupID string
	PodVswitchId    string

	WebHookCA      string
	WebHookTLSCert string
	WebHookTLSKey  string

	RavenPSK    string
	ToNodeGroup string
	AuthInfo    v1.AuthInfo
}
