package sign

import (
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path"

	"bytes"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"time"
)

var DEFAULT_CA_CONFIG = &Config{
	CN: "kubernetes",
	O:  []string{"xdpin.cn", "apiserver.xdpin.cn"},
	AltN: AltNames{
		DNSNames: []string{
			"localhost",
		},
		IPs: []net.IP{},
	},

	Usage: []x509.ExtKeyUsage{
		x509.ExtKeyUsageAny,
	},
}

type Config struct {
	CN    string
	O     []string
	AltN  AltNames
	Usage []x509.ExtKeyUsage
}
type AltNames struct {
	DNSNames []string
	IPs      []net.IP
}

const duration365d = time.Hour * 24 * 365 * 10

func SignEtcdMember(
	ca, ckey []byte,
	ips []string,
	idx string,
) ([]byte, []byte, error) {

	return SignEtcd(ca, ckey, ips, "member", idx)
}

func SignEtcdServer(
	ca, ckey []byte,
	ips []string,
	idx string,
) ([]byte, []byte, error) {

	return SignEtcd(ca, ckey, ips, "server", idx)
}

func SignEtcdClient(
	ca, ckey []byte,
	ips []string,
	idx string,
) ([]byte, []byte, error) {

	return SignEtcd(ca, ckey, ips, "client", idx)
}

func SignKubernetesClient(
	ca, ckey []byte,
	ips []string,
) ([]byte, []byte, error) {

	return SignKubernetes(ca, ckey, ips)
}

func SignRaven(
	ca, ckey []byte,
	ips []string,
) ([]byte, []byte, error) {

	return SignBy(ca, ckey, "raven-agent", []string{}, ips, []string{"xdpin.cn"})
}

func SignKonnectivity(
	ca, ckey []byte,
	ips []string,
) ([]byte, []byte, error) {

	return SignBy(ca, ckey, "system:konnectivity-server", []string{}, ips, []string{"xdpin.cn"})
}

func GenerateServerCert(dir string) ([]byte, error) {
	cakey, ca, err := SelfSignedPair()
	if err != nil {
		return nil, err
	}
	key, crt, err := SignCert(ca, cakey, []string{})
	if err != nil {
		return nil, err
	}
	getname := func(name string) string {
		return path.Join(dir, name)
	}
	if err := os.WriteFile(getname("tls.crt"), crt, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(getname("tls.key"), key, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(getname("tls.ca"), ca, 0755); err != nil {
		return nil, err
	}
	return ca, nil
}

func GenerateServerTripple() (ca, crt, key []byte, err error) {
	cakey, ca, err := SelfSignedPair()
	if err != nil {
		return
	}
	key, crt, err = SignCert(ca, cakey, []string{})
	return
}

func SelfSignedPair() ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(cryptorand.Reader, 1024)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   DEFAULT_CA_CONFIG.CN,
			Organization: DEFAULT_CA_CONFIG.O,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(duration365d * 10).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(
		cryptorand.Reader,
		&tmpl,
		&tmpl,
		key.Public(),
		key,
	)
	if err != nil {
		return nil, nil, err
	}
	// return key, cert, err
	return EncodePem(derBytes, key)
}

func SelfSignedPairSA() ([]byte, []byte, error) {

	key, err := rsa.GenerateKey(cryptorand.Reader, 1024)
	if err != nil {
		return nil, nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, nil, err
	}
	pemk := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	pempubk := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		},
	)
	return pemk, pempubk, nil
}

func SignEtcd(ca, ckey []byte, ips []string, name, idx string) ([]byte, []byte, error) {
	klog.Infof("sign etcd: %s, %s, %s", name, ips, idx)
	cap, err := ParseCertsPEM(ca)
	if err != nil || len(cap) <= 0 {
		return nil, nil, fmt.Errorf("etcd ca cert parse error: len(cap)=%d, %v", len(cap), err)
	}
	cakey, err := ParsePrivateKeyPEM(ckey)
	if err != nil {
		return nil, nil, fmt.Errorf("key parse error: %s", err.Error())
	}
	netips := []net.IP{
		net.ParseIP("127.0.0.1"),
	}
	for _, ip := range ips {
		netips = append(netips, net.ParseIP(ip))
	}
	cfg := Config{
		CN: fmt.Sprintf("etcd-%s.%s", idx, name),
		O:  []string{"xdpin.cn", "hangzhou"},
		AltN: AltNames{
			DNSNames: append(
				ips,
				[]string{
					"localhost",
					fmt.Sprintf("etcd-%s.local", idx),
					fmt.Sprintf("etcd-%s.member", idx),
				}...,
			),
			IPs: netips,
		},

		Usage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}
	return NewSignedCert(cfg, cap[0], cakey.(*rsa.PrivateKey))
}

func NewSignedCert(cfg Config, caCert *x509.Certificate, caKey *rsa.PrivateKey) ([]byte, []byte, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, nil, err
	}
	if len(cfg.CN) == 0 {
		return nil, nil, fmt.Errorf("must specify a CommonName")
	}
	if len(cfg.Usage) == 0 {
		return nil, nil, fmt.Errorf("must specify at least one ExtKeyUsage")
	}
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CN,
			Organization: cfg.O,
		},
		DNSNames:     cfg.AltN.DNSNames,
		IPAddresses:  cfg.AltN.IPs,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(duration365d * 10).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usage,
	}
	derbyte, err := x509.CreateCertificate(
		cryptorand.Reader,
		&certTmpl,
		caCert,
		key.Public(),
		caKey,
	)
	if err != nil {
		return nil, nil, err
	}
	return EncodePem(derbyte, key)
}

func SignCert(ca, ckey []byte, ip []string) ([]byte, []byte, error) {
	dns := []string{
		"localhost",
		"meridian-operator",
		"meridian-operator.svc.cluster.local",
		"meridian-operator.kube-system.svc",
	}
	return signCert(ca, ckey, ip, dns)
}

func SignCertWithDNS(ca, ckey []byte, dns []string) ([]byte, []byte, error) {
	return signCert(ca, ckey, []string{}, dns)
}

func signCert(ca, ckey []byte, ips, dns []string) ([]byte, []byte, error) {
	cap, err := ParseCertsPEM(ca)
	if err != nil || len(cap) <= 0 {
		return nil, nil, fmt.Errorf("sign ca cert parse error: len(cap)=%d, %v", len(cap), err)
	}
	cakey, err := ParsePrivateKeyPEM(ckey)
	if err != nil {
		return nil, nil, fmt.Errorf("key parse error: %s", err.Error())
	}
	netips := []net.IP{
		net.ParseIP("127.0.0.1"),
	}
	for _, ip := range ips {
		netips = append(netips, net.ParseIP(ip))
	}
	cfg := Config{
		CN: "meridian-operator",
		O:  []string{"xdpin.cn", "hangzhou"},
		AltN: AltNames{
			DNSNames: append(
				ips,
				dns...,
			),
			IPs: netips,
		},

		Usage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}
	return NewSignedCert(cfg, cap[0], cakey.(*rsa.PrivateKey))
}

// SignKubernetes return key,cert,error
func SignKubernetes(ca, ckey []byte, ips []string) ([]byte, []byte, error) {
	klog.V(5).Infof("sign kubernetes: address=%s", ips)
	cap, err := ParseCertsPEM(ca)
	if err != nil || len(cap) <= 0 {
		return nil, nil, fmt.Errorf("kubernetes ca cert parse error: len(cap)=%d, %v", len(cap), err)
	}
	cakey, err := ParsePrivateKeyPEM(ckey)
	if err != nil {
		return nil, nil, fmt.Errorf("key parse error: %s", err.Error())
	}
	netips := []net.IP{
		net.ParseIP("127.0.0.1"),
	}
	for _, ip := range ips {
		netips = append(netips, net.ParseIP(ip))
	}
	cfg := Config{
		CN: "kubernetes-admin",
		O:  []string{"system:masters"},
		AltN: AltNames{
			DNSNames: append(
				ips,
				[]string{
					"localhost",
				}...,
			),
			IPs: netips,
		},

		Usage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
	}
	return NewSignedCert(cfg, cap[0], cakey.(*rsa.PrivateKey))
}

// SignBy return key,cert,error
func SignBy(ca, ckey []byte, cn string, o, ips, dns []string) ([]byte, []byte, error) {
	klog.V(5).Infof("sign by: address=%s", ips)
	cap, err := ParseCertsPEM(ca)
	if err != nil || len(cap) <= 0 {
		return nil, nil, fmt.Errorf("ca cert parse error: len(cap)=%d, %v", len(cap), err)
	}
	cakey, err := ParsePrivateKeyPEM(ckey)
	if err != nil {
		return nil, nil, fmt.Errorf("key parse error: %s", err.Error())
	}
	netips := []net.IP{
		net.ParseIP("127.0.0.1"),
	}
	for _, ip := range ips {
		netips = append(netips, net.ParseIP(ip))
	}
	dns = append(dns, ips...)
	dns = append(dns, "localhost")
	cfg := Config{
		CN: cn,
		O:  o,
		AltN: AltNames{
			DNSNames: dns,
			IPs:      netips,
		},

		Usage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
	}
	return NewSignedCert(cfg, cap[0], cakey.(*rsa.PrivateKey))
}

// return key,cert,error
func Sign(ca, ckey []byte, ips []string, name string) ([]byte, []byte, error) {
	klog.Infof("sign for [%s]: %s", name, ips)
	cap, err := ParseCertsPEM(ca)
	if err != nil || len(cap) <= 0 {
		return nil, nil, fmt.Errorf("kubernetes ca cert parse error: len(cap)=%d, %v", len(cap), err)
	}
	cakey, err := ParsePrivateKeyPEM(ckey)
	if err != nil {
		return nil, nil, fmt.Errorf("key parse error: %s", err.Error())
	}
	netips := []net.IP{
		net.ParseIP("127.0.0.1"),
	}
	for _, ip := range ips {
		netips = append(netips, net.ParseIP(ip))
	}
	cfg := Config{
		CN: name,
		O:  []string{""},
		AltN: AltNames{
			DNSNames: append(
				ips,
				[]string{
					"localhost",
				}...,
			),
			IPs: netips,
		},

		Usage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
	}
	return NewSignedCert(cfg, cap[0], cakey.(*rsa.PrivateKey))
}

// ====================================== Help Functions ==========================================
func ParseCertsPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	var certs []*x509.Certificate
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		// Only use PEM "CERTIFICATE" blocks without extra headers
		if block.Type != "CERTIFICATE" ||
			len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, fmt.Errorf("data does not contain any valid RSA or ECDSA certificates")
	}
	return certs, nil
}

func ParsePrivateKeyPEM(keyData []byte) (interface{}, error) {
	var privateKeyPemBlock *pem.Block
	for {
		privateKeyPemBlock, keyData = pem.Decode(keyData)
		if privateKeyPemBlock == nil {
			break
		}

		switch privateKeyPemBlock.Type {
		case "EC PRIVATE KEY":
			// ECDSA Private Identity in ASN.1 format
			if key, err := x509.ParseECPrivateKey(privateKeyPemBlock.Bytes); err == nil {
				return key, nil
			}
		case "RSA PRIVATE KEY":
			// RSA Private Identity in PKCS#1 format
			if key, err := x509.ParsePKCS1PrivateKey(privateKeyPemBlock.Bytes); err == nil {
				return key, nil
			}
		case "PRIVATE KEY":
			// RSA or ECDSA Private Identity in unencrypted PKCS#8 format
			if key, err := x509.ParsePKCS8PrivateKey(privateKeyPemBlock.Bytes); err == nil {
				return key, nil
			}
		}

		// tolerate non-key PEM blocks for compatibility with things like "EC PARAMETERS" blocks
		// originally, only the first PEM block was parsed and expected to be a key block
	}

	// we read all the PEM blocks and didn't recognize one
	return nil, fmt.Errorf("data does not contain a valid RSA or ECDSA private key")
}

func EncodePem(cert []byte, key *rsa.PrivateKey) ([]byte, []byte, error) {
	// Generate cert, followed by ca
	cbuff := bytes.Buffer{}
	err := pem.Encode(
		&cbuff,
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	// Generate key
	kbuff := bytes.Buffer{}
	err = pem.Encode(
		&kbuff,
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return kbuff.Bytes(), cbuff.Bytes(), nil
}
