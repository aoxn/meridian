package client

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ess"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/denverdino/aliyungo/oss"
)

const (
	CloudAlibaba    = "cloud.alibaba"
	TokenSyncPeriod = 10 * time.Minute

	AccessKeyID     = "ACCESS_KEY_ID"
	AccessKeySecret = "ACCESS_KEY_SECRET"
)

// ClientMgr client manager for aliyun sdk
type ClientMgr struct {
	auth api.AuthInfo
	stop <-chan struct{}

	ECS *ecs.Client
	VPC *vpc.Client
	ESS *ess.Client
	RAM *ram.Client
	OSS *oss.Client
	SLB *slb.Client
}

// NewClientMgr return a new client manager
func NewClientMgr(auth api.AuthInfo) (*ClientMgr, error) {

	credential := &credentials.StsTokenCredential{
		AccessKeyId:       auth.AccessKey,
		AccessKeySecret:   auth.AccessSecret,
		AccessKeyStsToken: "",
	}

	ecli, err := ecs.NewClientWithOptions(auth.Region, clientCfg(), credential)
	if err != nil {
		return nil, fmt.Errorf("initialize alibaba ecs client: %s", err.Error())
	}
	ecli.AppendUserAgent(CloudAlibaba, meridian.Version)

	vpcli, err := vpc.NewClientWithOptions(auth.Region, clientCfg(), credential)
	if err != nil {
		return nil, fmt.Errorf("initialize alibaba vpc client: %s", err.Error())
	}
	vpcli.AppendUserAgent(CloudAlibaba, meridian.Version)

	esscli, err := ess.NewClientWithOptions(auth.Region, clientCfg(), credential)
	if err != nil {
		return nil, fmt.Errorf("initialize alibaba pvtz client: %s", err.Error())
	}
	esscli.AppendUserAgent(CloudAlibaba, meridian.Version)

	ramcli, err := ram.NewClientWithOptions(auth.Region, clientCfg(), credential)
	if err != nil {
		return nil, fmt.Errorf("initialize alibaba ram client: %s", err.Error())
	}
	esscli.AppendUserAgent(CloudAlibaba, meridian.Version)

	mgr := &ClientMgr{
		ECS:  ecli,
		VPC:  vpcli,
		ESS:  esscli,
		RAM:  ramcli,
		auth: auth,
		stop: make(<-chan struct{}, 1),
	}
	return mgr, nil
}

func (mgr *ClientMgr) Start(
	settoken func(mgr *ClientMgr, token *DefaultToken) error,
) error {
	initialized := false
	tokenAuth := mgr.GetTokenAuth()

	tokenfunc := func() {
		token, err := tokenAuth.NextToken()
		if err != nil {
			klog.Errorf("fail to get next token: %s", err.Error())
			return
		}
		err = settoken(mgr, token)
		if err != nil {
			klog.Errorf("fail to set token: %s", err.Error())
			return
		}
		initialized = true
	}

	go wait.Until(
		func() { tokenfunc() },
		TokenSyncPeriod,
		mgr.stop,
	)

	return wait.ExponentialBackoff(
		wait.Backoff{
			Steps:    7,
			Duration: 1 * time.Second,
			Jitter:   1,
			Factor:   2,
		}, func() (done bool, err error) {
			tokenfunc()
			klog.Info("wait for Token ready")
			return initialized, nil
		},
	)
}

func (mgr *ClientMgr) GetTokenAuth() TokenAuth {

	if mgr.auth.AccessKey != "" && mgr.auth.AccessSecret != "" {
		klog.Info("use ak mode to get token")
		return &AkAuthToken{
			DefaultToken{
				Region:          mgr.auth.Region,
				AccessKeyId:     mgr.auth.AccessKey,
				AccessKeySecret: mgr.auth.AccessSecret,
			},
		}
	}

	if os.Getenv(AccessKeyID) != "" && os.Getenv(AccessKeySecret) != "" {
		klog.Infof("use ak mode to get token")
		return &AkAuthToken{DefaultToken{
			Region:          mgr.auth.Region,
			AccessKeyId:     mgr.auth.AccessKey,
			AccessKeySecret: mgr.auth.AccessSecret,
		}}
	}

	klog.Info("use ram role mode to get token")
	return &RamRoleToken{}
}

func RefreshToken(mgr *ClientMgr, token *DefaultToken) error {
	klog.V(5).Infof("refresh token: %s", token.Region)
	credential := &credentials.StsTokenCredential{
		AccessKeyId:       token.AccessKeyId,
		AccessKeySecret:   token.AccessKeySecret,
		AccessKeyStsToken: token.SecurityToken,
	}

	err := mgr.ECS.InitWithOptions(token.Region, clientCfg(), credential)
	if err != nil {
		return fmt.Errorf("init ecs sts token config: %s", err.Error())
	}

	err = mgr.VPC.InitWithOptions(token.Region, clientCfg(), credential)
	if err != nil {
		return fmt.Errorf("init vpc sts token config: %s", err.Error())
	}

	setCustomizedEndpoint(mgr)

	return nil
}

func setVPCEndpoint(mgr *ClientMgr) {
	mgr.ECS.Network = "vpc"
	mgr.VPC.Network = "vpc"
}

func setCustomizedEndpoint(mgr *ClientMgr) {
	if ecsEndpoint, err := parseURL(os.Getenv("ECS_ENDPOINT")); err == nil && ecsEndpoint != "" {
		mgr.ECS.Domain = ecsEndpoint
	}
	if vpcEndpoint, err := parseURL(os.Getenv("VPC_ENDPOINT")); err == nil && vpcEndpoint != "" {
		mgr.VPC.Domain = vpcEndpoint
	}
}

func parseURL(str string) (string, error) {
	if str == "" {
		return "", nil
	}

	if !strings.HasPrefix(str, "http") {
		str = "http://" + str
	}
	u, err := url.Parse(str)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func clientCfg() *sdk.Config {
	scheme := "HTTPS"
	if os.Getenv("ALICLOUD_CLIENT_SCHEME") == "HTTP" {
		scheme = "HTTP"
	}
	return &sdk.Config{
		Timeout:   20 * time.Second,
		Transport: http.DefaultTransport,
		Scheme:    scheme,
	}
}

func openapiCfg(region string, credential *credentials.StsTokenCredential, network string) *openapi.Config {
	scheme := "HTTPS"
	if os.Getenv("ALICLOUD_CLIENT_SCHEME") == "HTTP" {
		scheme = "HTTP"
	}
	return &openapi.Config{
		UserAgent:       tea.String(getUserAgent()),
		Protocol:        tea.String(scheme),
		RegionId:        tea.String(region),
		Network:         &network,
		ConnectTimeout:  tea.Int(20000),
		ReadTimeout:     tea.Int(20000),
		AccessKeyId:     tea.String(credential.AccessKeyId),
		AccessKeySecret: tea.String(credential.AccessKeySecret),
		SecurityToken:   tea.String(credential.AccessKeyStsToken),
	}
}

func getUserAgent() string {
	agents := map[string]string{
		CloudAlibaba: meridian.Version,
	}
	ret := ""
	for k, v := range agents {
		ret += fmt.Sprintf(" %s/%s", k, v)
	}
	return ret
}

const (
	AddonTokenFilePath = "/var/addon/token-config"
)

type DefaultToken struct {
	Region          string
	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
}

// TokenAuth is an interface of Token auth method
type TokenAuth interface {
	NextToken() (*DefaultToken, error)
}

// AkAuthToken implement ak auth
type AkAuthToken struct {
	DefaultToken
}

func (f *AkAuthToken) NextToken() (*DefaultToken, error) {
	return &f.DefaultToken, nil
}

type RamRoleToken struct {
	meta cloud.IMetaData
}

func (f *RamRoleToken) NextToken() (*DefaultToken, error) {
	roleName, err := f.meta.RoleName()
	if err != nil {
		return nil, fmt.Errorf("role name: %s", err.Error())
	}
	// use instance ram file way.
	role, err := f.meta.RamRoleToken(roleName)
	if err != nil {
		return nil, fmt.Errorf("ramrole Token retrieve: %s", err.Error())
	}
	region, err := f.meta.Region()
	if err != nil {
		return nil, fmt.Errorf("read region error: %s", err.Error())
	}

	return &DefaultToken{
		Region:          region,
		AccessKeyId:     role.AccessKeyId,
		AccessKeySecret: role.AccessKeySecret,
		SecurityToken:   role.SecurityToken,
	}, nil
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func Decrypt(s string, keyring []byte) ([]byte, error) {
	cdata, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 string, err: %s", err.Error())
	}
	block, err := aes.NewCipher(keyring)
	if err != nil {
		return nil, fmt.Errorf("failed to new cipher, err: %s", err.Error())
	}
	blockSize := block.BlockSize()

	iv := cdata[:blockSize]
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(cdata)-blockSize)

	blockMode.CryptBlocks(origData, cdata[blockSize:])

	origData = PKCS5UnPadding(origData)
	return origData, nil
}
