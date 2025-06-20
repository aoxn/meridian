package address

import (
	"context"
	"fmt"
	"net"
	"time"

	"io"
	"net/http"
	"regexp"

	"github.com/go-resty/resty/v2"
	u "github.com/syncthing/syncthing/lib/upnp"
	"k8s.io/klog/v2"
)

type Resolver interface {
	Name() string
	GetAddr() (*Addr, error)
}

func GetAddress(name ...string) (*Addr, error) {
	if len(name) > 0 {
		for _, v := range name {
			switch v {
			case UPNP:
				return NewUPNP().GetAddr()
			case MyIP:
				return NewMyIP().GetAddr()
			case IPIFY:
				return NewMyIP().GetAddr()
			case POLL:
				return NewPOLL().GetAddr()
			default:
				return NewRoundRobin().GetAddr()
			}
		}
	}
	return NewRoundRobin().GetAddr()
}

type Addr struct {
	IPv4 net.IP
	IPv6 net.IP
}

func NewAddrFromSlice(addrs []string) *Addr {
	res := &Addr{}
	for _, s := range addrs {
		a := net.ParseIP(s)
		if a == nil {
			continue
		}
		if a.To4() == nil {
			res.IPv6 = a
		} else {
			res.IPv4 = a
		}
	}

	return res
}

const (
	UPNP  = "upnp"
	MyIP  = "myip"
	IPIFY = "ipify"
	STUN  = "stun"
	POLL  = "poll"

	Round = "round"
)

func NewRoundRobin() Resolver {
	return &roundRobin{
		under: []Resolver{
			NewUPNP(),
			NewPOLL(),
			NewIPify(),
			NewMyIP(),
		},
	}
}

type roundRobin struct {
	under []Resolver
}

func (i *roundRobin) Name() string { return Round }

func (i *roundRobin) GetAddr() (*Addr, error) {
	for _, r := range i.under {
		addr, err := r.GetAddr()
		if err == nil {
			return addr, nil
		}
		klog.Infof("poll gateway address from[%s] error: %s", r.Name(), err.Error())
	}
	return nil, fmt.Errorf("no address found for %s", i.Name())
}

func NewUPNP() Resolver {
	return &upnp{}
}

type upnp struct {
}

func (i *upnp) Name() string { return UPNP }

func (i *upnp) GetAddr() (*Addr, error) {
	ctx := context.TODO()
	devices := u.Discover(ctx, 0, time.Second)
	if len(devices) <= 0 {
		return nil, fmt.Errorf("dns: no router device discoverd")
	}
	device := devices[0]
	klog.Infof("dns: total %d devices discovered, use the first one", len(devices))
	eip, err := device.GetExternalIPv4Address(ctx)
	if err != nil {
		return nil, fmt.Errorf("dns: get router ip failed: %s", err.Error())
	}
	klog.Infof("external router ip: %v", eip)
	return NewAddrFromSlice([]string{eip.String()}), nil
}

func NewMyIP() Resolver { return &myip{} }

type myip struct {
}

func (i *myip) Name() string { return MyIP }

func (i *myip) GetAddr() (*Addr, error) {
	var result Result
	client := resty.New()

	resp, err := client.R().SetResult(&result).Get("https://api.myip.com")
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("resp error %s\n", resp.String())
	}
	return NewAddrFromSlice([]string{result.IP}), nil
}

type Result struct {
	IP string `json:"ip"`
}

func NewIPify() Resolver {
	return &ipify{}
}

type ipify struct {
}

func (i *ipify) Name() string { return IPIFY }

func (i *ipify) GetAddr() (*Addr, error) {
	var result Result
	client := resty.New()

	resp, err := client.R().SetResult(&result).Get("https://api.ipify.org?format=json")
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("resp error %s\n", resp.String())
	}
	return NewAddrFromSlice([]string{result.IP}), nil
}

func NewPOLL() Resolver {
	return &poll{}
}

type poll struct {
}

func (i *poll) Name() string { return POLL }

func (i *poll) GetAddr() (*Addr, error) {
	ip, err := GetPublicIP()
	if err != nil {
		return nil, fmt.Errorf("get public address: %s", err.Error())
	}
	return NewAddrFromSlice([]string{ip}), nil
}

var (
	APIs = [...]string{
		"https://ifconfig.me",
		"https://icanhazip.com",
		"https://ipinfo.io/json",
		"https://api.ipify.org",
		"https://api.my-ip.io/ip",
		"https://ip4.seeip.org",
	}
)

var IPv4RE = regexp.MustCompile(`(?:\d{1,3}\.){3}\d{1,3}`)

func GetPublicIP() (string, error) {
	for _, api := range APIs {
		ip, err := getFromAPI(api)
		if err == nil {
			return ip, nil
		}
	}
	return "", fmt.Errorf("error get public ip by any of the apis: %v", APIs)
}

func getFromAPI(api string) (string, error) {
	resp, err := http.Get(api)
	if err != nil {
		return "", fmt.Errorf("retrieving public ip from %s: %v", api, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading api response from %s: %v", api, err)
	}
	return parseIPv4(string(body))
}

func parseIPv4(body string) (string, error) {
	matches := IPv4RE.FindAllString(body, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no ipv4 found in: %q", body)
	}
	return matches[0], nil
}
