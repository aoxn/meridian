package alibaba

import (
	"errors"
	"fmt"
	"github.com/aoxn/meridian/internal/node/host/meta"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"encoding/json"
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	ENDPOINT = "http://100.100.100.200"

	META_VERSION_LATEST = "latest"

	RS_TYPE_META_DATA = "meta-data"
	RS_TYPE_USER_DATA = "user-data"

	DNS_NAMESERVERS    = "dns-conf/nameservers"
	EIPV4              = "eipv4"
	HOSTNAME           = "hostname"
	IMAGE_ID           = "image-id"
	INSTANCE_ID        = "instance-id"
	MAC                = "mac"
	NETWORK_TYPE       = "network-type"
	NTP_CONF_SERVERS   = "ntp-conf/ntp-servers"
	OWNER_ACCOUNT_ID   = "owner-account-id"
	PRIVATE_IPV4       = "private-ipv4"
	REGION             = "region-id"
	SERIAL_NUMBER      = "serial-number"
	SOURCE_ADDRESS     = "source-address"
	VPC_CIDR_BLOCK     = "vpc-cidr-block"
	VPC_ID             = "vpc-id"
	VSWITCH_CIDR_BLOCK = "vswitch-cidr-block"
	VSWITCH_ID         = "vswitch-id"
	ZONE               = "zone-id"
	RAM_SECURITY       = "ram/security-credentials"
)

type Request interface {
	Version(version string) Request
	ResourceType(rtype string) Request
	Resource(resource string) Request
	SubResource(sub string) Request
	Url() (string, error)
	Do(api interface{}) error
}

type AlibabaMeta struct {
	// mock for unit test.
	mock requestMock

	client *http.Client
}

func NewMetaDataAlibaba(client *http.Client) *AlibabaMeta {
	if client == nil {
		client = &http.Client{}
	}
	return &AlibabaMeta{
		client: client,
	}
}

func NewMockMetaData(
	client *http.Client,
	sendRequest requestMock,
) *AlibabaMeta {
	if client == nil {
		client = &http.Client{}
	}
	return &AlibabaMeta{
		client: client,
		mock:   sendRequest,
	}
}

func (m *AlibabaMeta) New() *MetaDataRequest {
	return &MetaDataRequest{
		client:      m.client,
		sendRequest: m.mock,
	}
}

func (m *AlibabaMeta) HostName() (string, error) {
	var hostname ResultList
	err := m.New().Resource(HOSTNAME).Do(&hostname)
	if err != nil {
		return "", err
	}
	return hostname.result[0], nil
}

func (m *AlibabaMeta) ImageID() (string, error) {
	var image ResultList
	err := m.New().Resource(IMAGE_ID).Do(&image)
	if err != nil {
		return "", err
	}
	return image.result[0], err
}

func (m *AlibabaMeta) InstanceID() (string, error) {
	var instanceid ResultList
	err := m.New().Resource(INSTANCE_ID).Do(&instanceid)
	if err != nil {
		return "", err
	}
	return instanceid.result[0], err
}

func (m *AlibabaMeta) Mac() (string, error) {
	var mac ResultList
	err := m.New().Resource(MAC).Do(&mac)
	if err != nil {
		return "", err
	}
	return mac.result[0], nil
}

func (m *AlibabaMeta) NetworkType() (string, error) {
	var network ResultList
	err := m.New().Resource(NETWORK_TYPE).Do(&network)
	if err != nil {
		return "", err
	}
	return network.result[0], nil
}

func (m *AlibabaMeta) OwnerAccountID() (string, error) {
	var owner ResultList
	err := m.New().Resource(OWNER_ACCOUNT_ID).Do(&owner)
	if err != nil {
		return "", err
	}
	return owner.result[0], nil
}

func (m *AlibabaMeta) PrivateIPv4() (string, error) {
	var private ResultList
	err := m.New().Resource(PRIVATE_IPV4).Do(&private)
	if err != nil {
		return "", err
	}
	return private.result[0], nil
}

func (m *AlibabaMeta) Region() (string, error) {
	var region ResultList
	err := m.New().Resource(REGION).Do(&region)
	if err != nil {
		return "", err
	}
	return region.result[0], nil
}

func (m *AlibabaMeta) SerialNumber() (string, error) {
	var serial ResultList
	err := m.New().Resource(SERIAL_NUMBER).Do(&serial)
	if err != nil {
		return "", err
	}
	return serial.result[0], nil
}

func (m *AlibabaMeta) SourceAddress() (string, error) {
	var source ResultList
	err := m.New().Resource(SOURCE_ADDRESS).Do(&source)
	if err != nil {
		return "", err
	}
	return source.result[0], nil

}

func (m *AlibabaMeta) VpcCIDRBlock() (string, error) {
	var vpcCIDR ResultList
	err := m.New().Resource(VPC_CIDR_BLOCK).Do(&vpcCIDR)
	if err != nil {
		return "", err
	}
	return vpcCIDR.result[0], err
}

func (m *AlibabaMeta) VpcID() (string, error) {
	var vpcId ResultList
	err := m.New().Resource(VPC_ID).Do(&vpcId)
	if err != nil {
		return "", err
	}
	return vpcId.result[0], err
}

func (m *AlibabaMeta) VswitchCIDRBlock() (string, error) {
	var cidr ResultList
	err := m.New().Resource(VSWITCH_CIDR_BLOCK).Do(&cidr)
	if err != nil {
		return "", err
	}
	return cidr.result[0], err
}

func (m *AlibabaMeta) VswitchID() (string, error) {
	var vswithcid ResultList
	err := m.New().Resource(VSWITCH_ID).Do(&vswithcid)
	if err != nil {
		return "", err
	}
	return vswithcid.result[0], err
}

func (m *AlibabaMeta) EIPv4() (string, error) {
	var eip ResultList
	err := m.New().Resource(EIPV4).Do(&eip)
	if err != nil {
		return "", err
	}
	return eip.result[0], nil
}

func (m *AlibabaMeta) DNSNameServers() ([]string, error) {
	var data ResultList
	err := m.New().Resource(DNS_NAMESERVERS).Do(&data)
	if err != nil {
		return []string{}, err
	}
	return data.result, nil
}

func (m *AlibabaMeta) NTPConfigServers() ([]string, error) {
	var data ResultList
	err := m.New().Resource(NTP_CONF_SERVERS).Do(&data)
	if err != nil {
		return []string{}, err
	}
	return data.result, nil
}

func (m *AlibabaMeta) Zone() (string, error) {
	var zone ResultList
	err := m.New().Resource(ZONE).Do(&zone)
	if err != nil {
		return "", err
	}
	return zone.result[0], nil
}

func (m *AlibabaMeta) RoleName() (string, error) {
	var roleName ResultList
	err := m.New().Resource("ram/security-credentials/").Do(&roleName)
	if err != nil {
		return "", err
	}
	return roleName.result[0], nil
}

func (m *AlibabaMeta) RamRoleToken(role string) (meta.RoleAuth, error) {
	var roleauth meta.RoleAuth
	err := m.New().Resource(RAM_SECURITY).SubResource(role).Do(&roleauth)
	if err != nil {
		return meta.RoleAuth{}, err
	}
	return roleauth, nil
}

type requestMock func(resource string) (string, error)

type MetaDataRequest struct {
	version      string
	resourceType string
	resource     string
	subResource  string
	client       *http.Client

	sendRequest requestMock
}

func (vpc *MetaDataRequest) Version(version string) Request {
	vpc.version = version
	return vpc
}

func (vpc *MetaDataRequest) ResourceType(rtype string) Request {
	vpc.resourceType = rtype
	return vpc
}

func (vpc *MetaDataRequest) Resource(resource string) Request {
	vpc.resource = resource
	return vpc
}

func (vpc *MetaDataRequest) SubResource(sub string) Request {
	vpc.subResource = sub
	return vpc
}

func (vpc *MetaDataRequest) Url() (string, error) {
	if vpc.version == "" {
		vpc.version = "latest"
	}
	if vpc.resourceType == "" {
		vpc.resourceType = "meta-data"
	}
	if vpc.resource == "" {
		return "", errors.New("the resource you want to visit must not be nil")
	}
	endpoint := os.Getenv("METADATA_ENDPOINT")
	if endpoint == "" {
		endpoint = ENDPOINT
	}
	r := fmt.Sprintf("%s/%s/%s/%s", endpoint, vpc.version, vpc.resourceType, vpc.resource)
	if vpc.subResource == "" {
		return r, nil
	}
	return fmt.Sprintf("%s/%s", r, vpc.subResource), nil
}

func (vpc *MetaDataRequest) Do(api interface{}) error {
	res := ""
	err := wait.ExponentialBackoff(
		wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   2,
			Steps:    4,
		},
		func() (done bool, err error) {
			if vpc.sendRequest != nil {
				res, err = vpc.sendRequest(vpc.resource)
			} else {
				res, err = vpc.send()
			}
			if shouldRetry(err) {
				klog.Errorf("retry meta alibaba: %v\n", err)
				return false, nil
			}
			return true, nil
		},
	)
	if err != nil {
		return err
	}
	return vpc.Decode(res, api)
}

func (vpc *MetaDataRequest) Decode(data string, api interface{}) error {
	if data == "" {
		url, _ := vpc.Url()
		return errors.New(fmt.Sprintf("metadata: alivpc decode data must not be nil. url=[%s]\n", url))
	}
	switch api.(type) {
	case *ResultList:
		api.(*ResultList).result = strings.Split(data, "\n")
		return nil
	case *meta.RoleAuth:
		return json.Unmarshal([]byte(data), api)
	default:
		return errors.New(fmt.Sprintf("metadata: unknow type to decode, type=%s\n", reflect.TypeOf(api)))
	}
}

func (vpc *MetaDataRequest) send() (string, error) {
	url, err := vpc.Url()
	if err != nil {
		return "", err
	}
	requ, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return "", err
	}
	resp, err := vpc.client.Do(requ)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("aliyun metadata API Error: Status Code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type TimeoutError interface {
	error
	Timeout() bool // Is the error a timeout?
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(TimeoutError)
	if ok {
		return true
	}

	switch err {
	case io.ErrUnexpectedEOF, io.EOF:
		return true
	}
	switch e := err.(type) {
	case *net.DNSError:
		return true
	case *net.OpError:
		switch e.Op {
		case "read", "write":
			return true
		}
	case *url.Error:
		// url.Error can be returned either by net/url if a URL cannot be
		// parsed, or by net/http if the response is closed before the headers
		// are received or parsed correctly. In that later case, e.Op is set to
		// the HTTP method name with the first letter uppercased. We don't want
		// to retry on POST operations, since those are not idempotent, all the
		// other ones should be safe to retry.
		switch e.Op {
		case "Get", "Put", "Delete", "Head":
			return shouldRetry(e.Err)
		default:
			return false
		}
	}
	return false
}

type ResultList struct {
	result []string
}
