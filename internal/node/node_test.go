package node

import (
	"encoding/base64"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNode(t *testing.T) {
	req := api.Request{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Request",
			APIVersion: api.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-001",
			Namespace: "default",
		},
		Spec: api.RequestSpec{
			Config: api.ClusterConfig{
				Registry: "registry.cn-hangzhou.aliyuncs.com",
				TLS: map[string]*api.KeyCert{
					"etcd-peer": {
						Key:  []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDWEFJQkFBS0JnUUN3dUxYTFBBU2pmSGw0STFyeTU1UXVmb0ttV0grS0VUb0p4cmE4c0x6MGtBekpzNnhLCmVvUVEyM0kzeUJYL0g2MVlIT0F6VlZsMnlTbjJCVjRUM0Voa0hpRDRpOWJxcEpXUWJIeml1bWRzcFI4ZHFndTkKVW5LMFpON1lXcXVyYjNIUXREUk5BeFdocUZvRWk3NHF6Z1c0UnRjMjd4c1drMGRMMjBkZGFLaHBNd0lEQVFBQgpBb0dBSTlta3o1RGlaQVFKWkV6UDAzMFZPNFZnbmJ4UVMwTUpZaGhVMDl5S3lKWThjZUtvTGdmQ3FPVEx1L00wCk95aEM4eUFRZjNsdUI4SHhhRDVZZ25EcW9rYUpnVkFTYS9BMTVhMWg1UjJpVnBkSGVKOUlOcDhqYmUyUndvNFkKTzBhVXc2c2RLQ1ZiNmJqcnVpc1MwVG9wZEJkM0gzcys5cnl2dFVUU09JOHRvaGtDUVFEVFExaHpEa2xBakE5bApYd3hEaTlYS0xUM1A2ZXFwejM0cWZGMFhrQk9HQUFBK2FITXAydmJIOHVsT2o1aWNVK1dyOU1La1VXcHE3dmlFClJnUCtUME5OQWtFQTFpVGFsVjc1NEZCN1B1MUVJZlBBRzR1Y2NaNEFweXhUOVZkS3J3VFlmM3RJZXl3QjJldlYKb0cyRVIrTmZnN0NwbDdqNnIveEJzNUpyZWRRck83OGVmd0pBRUFCMTNxRWlZMFU0bFZFUnVMd0t3WG1UeVAvSwp5bm53OEg3aS9qbm5nS3JYV2VMSGRsQWppUm1aR2w0K0RQazkyRHg5MGJ4bzl4aUtzbG9yUzBQdHNRSkFKL3RCCmhGbnpOVnBSYUhKTUlqcXNSM2hOZ1RrS3ppdU1rV1gyMzY1NzdYRkxHeFFnVkZ1Znl4QW5mblNKUk1FYktPUzAKaVY4RHRVOUZHYjN2Ukh4dWFRSkJBS0VMWlNJZ2dML3AydGNudlJLRXpiRTVOd2RoYmRtYlh3Um1aYXdZMXRrRwpKUzVhbDZQQ2IvUmpQQ09OUU1GTHhiYlBuWThicE5CK2R2VkdEWHhScVJJPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo="),
						Cert: []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNOakNDQVorZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREErTVNjd0R3WURWUVFLRXdob1lXNW4KZW1odmRUQVVCZ05WQkFvVERXRnNhV0poWW1FZ1kyeHZkV1F4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNdwpJQmNOTWpRd01qQTBNVE15TVRJd1doZ1BNakV5TkRBeE1URXhNekl4TWpCYU1ENHhKekFQQmdOVkJBb1RDR2hoCmJtZDZhRzkxTUJRR0ExVUVDaE1OWVd4cFltRmlZU0JqYkc5MVpERVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmwKY3pDQm56QU5CZ2txaGtpRzl3MEJBUUVGQUFPQmpRQXdnWWtDZ1lFQXNMaTF5endFbzN4NWVDTmE4dWVVTG42QwpwbGgvaWhFNkNjYTJ2TEM4OUpBTXliT3NTbnFFRU50eU44Z1YveCt0V0J6Z00xVlpkc2twOWdWZUU5eElaQjRnCitJdlc2cVNWa0d4ODRycG5iS1VmSGFvTHZWSnl0R1RlMkZxcnEyOXgwTFEwVFFNVm9haGFCSXUrS3M0RnVFYlgKTnU4YkZwTkhTOXRIWFdpb2FUTUNBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgovd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGS1lDWGN2Z1BOVHljZ0d5UnFiU2UwWnl0eG5kTUEwR0NTcUdTSWIzCkRRRUJDd1VBQTRHQkFBM29Uc3ZFNlhpbFdKOXNLMFF2RFBwdjd5T2xTdUlqY3hsaVlxSHVNblI0SmlhRjh0TmsKaERwcUZIaG42Yzc3S3JjZStIeStPRkl0MWpEcEIrWC9qcGFnc2QwaFJaeGZhTGVpM2FZZkhhdUozaVZVMFg0UQpJSEtqTHJlZG1KVDhsOXhHeGV3MXlRQ0UrRVZkNTZnRFN4WCsyS0MreklucktiMW5jb1ErZFJHcgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="),
					},
					"etcd-server": {
						Key:  []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDWFFJQkFBS0JnUURidkVwVDBORDlTTURucDhQTk5SYXRsZ2N2aVZadm1YYmJhVU5HR3UvOWxsNkRMWWh3CjBVNXRWalRlcHExQzVUK2RRM2VSWjZiMnByelhSRk9GZEtPaWFDVkkybExlNFJpbmtEVDEvN1Bab1NJVm9LSlMKcURuUk5Uak13dHN4UlRWZG5SOS8zRUdKcCszNzlRQW9rd3Z6NHF5bkNzcUFsVEJ2R3htdUlVL1R5UUlEQVFBQgpBb0dBTkpHOGVvYm5xT0VCL2FuK1J4YmZZUklXa0FIY1l3Q2xGZUwrREVTZmk5eXdxZE4zNC8yL05KcytOTkpOCmUwYWJUSVY2a3Bmb3N2TzdGQWt0cml6MGhKR1ErUXltcWNMakVaeWtzRTdtV2hnS3hBQndnOUxkVllXRjFVL0EKZkhJQVZ0ZzBsS2duSXRyZVJrSTRBSUFiQ2o5Y2hMblJBd096QTk3ZEVHS1o4ejBDUVFEOE10TWpGOCtmbnJBdApFRmsvMGpzZEtOSWRsdTlWT0kvVGZ4Z09KSWxQV3dCNXpKVitHLzhVMlNSUXVLc0dHRm9teFZPSWhxVFE0MTBRClBtU1lKN3diQWtFQTN3d3l4MklrSFFrN2owRlZVcmlCcUlSZnVQdDdDc0M0OTV0U3JwUGdDYUVTcUxhVXpKOVUKR3c0cGpKRS9KUlZESUVHMjF4K3Q2RWxhMVN3dnNkYmw2d0pCQUtsc2ozRGszeU5SWFBONUp5djcxS0NiT3NTTQpFRTZGQ0FKQ1FHdkgyY0xJMU1IK1VYTjk1VmdoSkFkaWQrcEpVODcyQTA4VmZRV2pxSEp3Sis0YnkzOENRQVIrCkVtZkJxa2lMYncrcm1UUlpVd001NTFPcWZRZnlhY2RTOFk5aW14aVdqZkduKzhkRFRrWmRPcWtDSSt0elNpN1UKSkFLaE9MZDlBcjlZYkgyQWZwRUNRUUNhQk5WWlhzQVZyUmJ0dUV5NlRVNUtWKzhRczh1Umg3TmpYNmt5T05kcApORFUyano1OEp5bVY1WllGY1grcDlXRmx4dVhUWXBvd2F0S29kQU5SbzVBcwotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo="),
						Cert: []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNOakNDQVorZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREErTVNjd0R3WURWUVFLRXdob1lXNW4KZW1odmRUQVVCZ05WQkFvVERXRnNhV0poWW1FZ1kyeHZkV1F4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNdwpJQmNOTWpRd01qQTBNVE15TVRJd1doZ1BNakV5TkRBeE1URXhNekl4TWpCYU1ENHhKekFQQmdOVkJBb1RDR2hoCmJtZDZhRzkxTUJRR0ExVUVDaE1OWVd4cFltRmlZU0JqYkc5MVpERVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmwKY3pDQm56QU5CZ2txaGtpRzl3MEJBUUVGQUFPQmpRQXdnWWtDZ1lFQTI3eEtVOURRL1VqQTU2ZkR6VFVXclpZSApMNGxXYjVsMjIybERSaHJ2L1paZWd5MkljTkZPYlZZMDNxYXRRdVUvblVOM2tXZW05cWE4MTBSVGhYU2pvbWdsClNOcFMzdUVZcDVBMDlmK3oyYUVpRmFDaVVxZzUwVFU0ek1MYk1VVTFYWjBmZjl4QmlhZnQrL1VBS0pNTDgrS3MKcHdyS2dKVXdieHNacmlGUDA4a0NBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgovd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGR2k3elUrYzViWUNXcVVobHVyakt1NHlRN2VGTUEwR0NTcUdTSWIzCkRRRUJDd1VBQTRHQkFCZWVOTVhIZW40eHVacmthYURlenhZSk5lL0ZkUjRkZWREM3lITWV2OEZrVEw0NjUwWjIKalpHdEdLYzlNWFJEdXZSMndJb2xxNmxPTEl4aW05VEF0alFTeTduMjhOTDczV1VTTngyNTBJYWNtb0Q2VktJRwpyREIzeU9rRm41UWtNNTFUS014VDJQRWh5UkVVNWIvcGFMY285ZjNrSkloQWg1L2FWNWVvVFJBYgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="),
					},
					"front-proxy": {
						Key:  []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDWEFJQkFBS0JnUUNaRmtYUlRaMFRCL2NSQ0NFejdLbUxxRTBteUxZVUkzemZJRUV1WStDd2VSWC9KYzVtClc4SWRFcVhjTUZBenorRDFudzZNMEt3WTU5d0RacWVCb28rajJZWW8ybGZIcG5VUDlGRUpyN0w2eG5YbUtvRVAKUXYwK1ZKODhvUnBZaURKanY5cmoyZGdoa0pQcGhBaVVZNDEwWXIvWlYzalQwVUVMZnRvZFR6dndlUUlEQVFBQgpBb0dBZXZjN0NZNlFWeE1VeitkNTFCMkxhcFM3dENXUXE4aDlxelJiWndudkY1R0xxN0VRZzRPOC9tRldQUEhKCjJuUm1QS1dRcUdlTmRVdjRtL3EvSGhBWDA4VUJobTg5cXdzeU94NWV4SktrREZyRmErTFhTaHRjTGp1RTFWYS8KSEpBWDRUVlNhaWtqbnJkSjBlVFYycUZua2ZPQkk3c3p6YzMxRnBDSkRHWjAvVWtDUVFERWJ6RWVvVE1oUFQ5aQpSU0VWZXIxbUtsL0t0d1kyckZiRDF5bmhlbUd1YkVNb0JKbWptUXF6WHlhYUxvazNna3VDQzg5WG5uR1J6NUZrCk5CUS96ZUM3QWtFQXg0SWM1SkpxejQ3VkZ4Y00wMEFYOUxuSlJOS2phRkJHT3h2dFlneFE2d0dpaEpUT2h3KzcKMTlNeGdXZVcvT3QzTFZsaTAzU3UzSHhnbXFwQWdGVktXd0pBZVVQVFpQOUsyemcrU3VJMlBGWmJXaGpLcmhBeQo2OG1VZnEzemt0akVPTE5vK2VsdEY0dkJDVjZ5Sy9pU2lRd01wU201UkhQeDFIdjVXNHl5KzNpVFJRSkFWR3MrCjlJenIrMFdSNzBKR29BRG40aHJYQ25NaXg5bm56YzBrWmkrVjhjcndUSzkyc0htODN6Y3pKSEdEMXlOL2UwWHUKWmxGaVNGT3N3T1UzZzlZVEx3SkJBSTNDMldtdW13am8xdEtDbTI1Ty9acXIwUDlWc1VXMFJOa3l5eUxKLzV2LwpwaUhQVlo3RWEyVTlneDB2emNqNzJXSU44WTFQaCtPNnhLeWF2Q1hvSHdRPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo="),
						Cert: []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNOakNDQVorZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREErTVNjd0R3WURWUVFLRXdob1lXNW4KZW1odmRUQVVCZ05WQkFvVERXRnNhV0poWW1FZ1kyeHZkV1F4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNdwpJQmNOTWpRd01qQTBNVE15TVRJd1doZ1BNakV5TkRBeE1URXhNekl4TWpCYU1ENHhKekFQQmdOVkJBb1RDR2hoCmJtZDZhRzkxTUJRR0ExVUVDaE1OWVd4cFltRmlZU0JqYkc5MVpERVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmwKY3pDQm56QU5CZ2txaGtpRzl3MEJBUUVGQUFPQmpRQXdnWWtDZ1lFQW1SWkYwVTJkRXdmM0VRZ2hNK3lwaTZoTgpKc2kyRkNOODN5QkJMbVBnc0hrVi95WE9abHZDSFJLbDNEQlFNOC9nOVo4T2pOQ3NHT2ZjQTJhbmdhS1BvOW1HCktOcFh4NloxRC9SUkNhK3krc1oxNWlxQkQwTDlQbFNmUEtFYVdJZ3lZNy9hNDluWUlaQ1Q2WVFJbEdPTmRHSy8KMlZkNDA5RkJDMzdhSFU4NzhIa0NBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgovd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGR2NjN2ptcFdMVzlDOHR3QmZaU0M4Z2s4NUdCTUEwR0NTcUdTSWIzCkRRRUJDd1VBQTRHQkFJWXdRY2p0bTl0WXRWUjRNMlBWcC9WZDRhWnhvdmRIaXRaNDdYNlRDRG9IRm1oTVU3VFoKaXh3bTBESnV1aGp5N282cGIySXNSd080UkRxWkVCV1NmTmJNS1dEUTRNMkxHWklUVXhVWUtHdlExWmhjVVNadQpWRmdSMmRUaUJFZGZvMW5YOFBXVTA4cVNwYys0VTFaRlNBdjJ0eXVPWGNBMHV1TVJMODN1SENiWQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="),
					},
					"root": {
						Key:  []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDWEFJQkFBS0JnUURjL3F5cFkrVEJ3M3MxM0RDWGdqUUk2UUF0U29oUDZsOXpxYU9aREhTeks2SnE1aGhnCndEOTc3U0xneFp0N2ZEVXA0cDZ5ZFdETDcvU2JKSWh1clpkVHc0Lzk4Q05NYzFCeUZDeTJXUmNPMzc5dVMxUWoKUzNiQWdMdERHTEJkamEyMnVxMWswUWV4UTdGMVIxdGwraXZCVi9PNnpkSDkrL0tPbkFzOE40UXNKUUlEQVFBQgpBb0dBQVJsck5uUit3TS8rSHVoM2ZXbHlaTkR6NVVYSG84NGdwMnlpbWJKTUtrLy9RTlRnTGlzR3BkRXFLVUFTClkremNQdGNYdnYrQ2VjRTUwRVBBUWZ5dkVnTTYxQnN4bmR4R1owbkN2MS9sd2ZZZ3g3NUY2ZVpRME9qNzUxZ0cKTHh6WU95Z3VERHZTS0lja0ZsYlQzeStaRHdPempEVmgrTndYMk9CTUdNNFcya0VDUVFEbFFySlpDajZZeFozMwo1Q2QxNnl1dWhoZkFOV0pFTnlXTzNsZ0M0bzRuWmNaMlFpVExXNzdORnk5MktqTEpOOVM4TGpqNjRBK3oyOFcyCjR2L0RiMUFwQWtFQTlzVXR1M0F0V1l5OS9tbzRBQjl2RDZqeXhYLytHWWgxYUpYOGF2d0lzOERlcXlFY0FLYjIKSjVlS00vaW1RZ2x1WGhRYkFldDFURDdib052NzZpUkxuUUpCQUp4eE8rU1lvZ2g3NllUTDhzVjdtYzQ1QUtJUAppNlBEQWVVUkFudk5mM1dROUxHa0J4bWgzSHgxRXRVT2pLTlVidDJPcVNGQW5sWjhaTm1jNHl6SW02a0NRR25FCjFmY3krNTBZWUE3K0JBYTVjbWJwNlRTUnlaMjBDVzdNYXFhSVpFcDNibmsyOWNPcHpIUG4xZ3EwbHI1VFFJVCsKWlIwTGlQa25NQWZnZ2pjM1cxa0NRRXRZZ3g0YzFNWm9jbDA1RGpSbVZ3cEVuaWdXbFd2WjlZT0tES3hDOTQ5eQo5ZXhoOEg5bjdsUTUxdjBTMU9ZQ0owSVhkbDN3VWVQM2pGTm8xQnNLc2ZVPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo="),
						Cert: []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNOakNDQVorZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREErTVNjd0R3WURWUVFLRXdob1lXNW4KZW1odmRUQVVCZ05WQkFvVERXRnNhV0poWW1FZ1kyeHZkV1F4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNdwpJQmNOTWpRd01qQTBNVE15TVRJd1doZ1BNakV5TkRBeE1URXhNekl4TWpCYU1ENHhKekFQQmdOVkJBb1RDR2hoCmJtZDZhRzkxTUJRR0ExVUVDaE1OWVd4cFltRmlZU0JqYkc5MVpERVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmwKY3pDQm56QU5CZ2txaGtpRzl3MEJBUUVGQUFPQmpRQXdnWWtDZ1lFQTNQNnNxV1Brd2NON05kd3dsNEkwQ09rQQpMVXFJVCtwZmM2bWptUXgwc3l1aWF1WVlZTUEvZSswaTRNV2JlM3cxS2VLZXNuVmd5Ky8wbXlTSWJxMlhVOE9QCi9mQWpUSE5RY2hRc3Rsa1hEdCsvYmt0VUkwdDJ3SUM3UXhpd1hZMnR0cnF0Wk5FSHNVT3hkVWRiWmZvcndWZnoKdXMzUi9mdnlqcHdMUERlRUxDVUNBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgovd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGQ2d0SWVVcGM1UFIyTkoydXZYZ0dpYW5wMTdYTUEwR0NTcUdTSWIzCkRRRUJDd1VBQTRHQkFJWTJIbXE5L0FGWHY1TUtlOHl2VzJaSGZLdjRWUWpaenlhSm0xK1AxYlNLQ2VMNVFMcGUKL1RYU08ycHpmVlM5blNLWFdmS3lyUWVGL0w1ZjBKQXJQaUI1L25MWUFrS1hYVzB3MGVGVkNHZmRVT0Z2T1NtMQpTUm9XcmhnOWIrbFBpS1cwVlBMeHFHaDhsV3RkeHNGNnQrUTkwOWxVZHhmZVV0TGF2L3E0dzN0MgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="),
					},
					"svc": {
						Key:  []byte("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlDV3dJQkFBS0JnUUNjMVYvd3UvdExWZDJiTHNZNVFCcUxSNmtaNHRRUjV5UllnL0hoVVRFSWFkbVdZbW5zClJIMTR4RGRId0JiT2E2WWFGb1YzVmwwTTJINnZBbEEyb3JnZ1hHNWN3OGo2T1R6Yzl5enNYL1I5ak1PV2c0d3UKY1VjTFhuL2g4VzVRa3Vobmd0azdFNktsYzM5b0YzQ1FHcHVqWG56SXo0ekpJRXVSMVhWcnc4Rjhkd0lEQVFBQgpBb0dBRll3TFBsdlZUcWhoWmV5ck56cGphemRST0VrOXNhZjhDbDAyWWlweGZpSnN6b2grM1FNYjZmZXJVS1JICmZkeTBXV2sydFFxY2hpTklaR3NBOGtDdzR5bzVrOFp2Rkg0T0lycWJEU0IvekhxWUhYMXBlS2YvT1JUalVXN2EKcDdzODh1Rkp3b2EwTzBqNGI0eTl1Y0kwZUdjVEYvSE9UcFhhazdrTW1xcHJzNkVDUVFETzVnS1BlaGsvd3hwUwpkcVJ1TUZkWWF0QXBzNEg0SVd5dGtlNHBTeWRpbTQvaGNUZENHZTlVVmdFQmZPSUx5Z2Nha256M003UUhLNGlLCjBwL2FtcXdkQWtFQXdnMnlrZkFzcjBBeEtrcDVwKzVRY1diQ3lERDdJWGQ1QzFUUE5xMStVUWpGOEcyMDRPd3EKaTFNREZnZUMxL0JiT2V2cWxTOVZXWFl5VzVKdGkzeWVvd0pBWVZPSHp0Q0VBaCtZU1VSd1V6bEFUV0pwcThRNgpsbXU2d09lTjNqVHhRUXltb1VsdDBoVjdKUFFVSXd3SkZieWluTmhlR3JkaXI2REY2Vy90TEp0bjdRSkFWMDl3CmErZERRNnEvTkVjRUM4SFhJZDdaZnRkQzl1RFpibmEvTU52SXZNOFV1RU8wSVl0QTdTVHhlNFR2b3hiN0JNbVgKNTMyL2lodjdObVpnc1dUbHZ3SkFORWdnNlNPbkRHYlB0RkVSVzRTWkVKZFIwUUJZNW1vd2JDTVE1SUdLb2xxUQpsY0plWXg2K09oRTZlSTJMMUNXSnhLTlZjNVZGZDRqTVNUOVJKVTN0Q1E9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo="),
						Cert: []byte("LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlHZk1BMEdDU3FHU0liM0RRRUJBUVVBQTRHTkFEQ0JpUUtCZ1FDYzFWL3d1L3RMVmQyYkxzWTVRQnFMUjZrWgo0dFFSNXlSWWcvSGhVVEVJYWRtV1ltbnNSSDE0eERkSHdCYk9hNllhRm9WM1ZsME0ySDZ2QWxBMm9yZ2dYRzVjCnc4ajZPVHpjOXl6c1gvUjlqTU9XZzR3dWNVY0xYbi9oOFc1UWt1aG5ndGs3RTZLbGMzOW9GM0NRR3B1alhuekkKejR6SklFdVIxWFZydzhGOGR3SURBUUFCCi0tLS0tRU5EIFBVQkxJQyBLRVktLS0tLQo="),
					},
				},
				Token: "q6rlzl.nroihsvsaee7kzs6",
			},
			AccessPoint: api.AccessPoint{
				//Internet: cluster.Status.InfraState.GetInternet(),
				Intranet: "127.0.0.1",
			},
			//Provider: *G.cfg.GetCurrent(),
		},
	}
	for k, v := range req.Spec.Config.TLS {
		key, err := base64.StdEncoding.DecodeString(string(v.Key))
		if err != nil {
			t.Fatalf("key base64 : %v", err)
		}
		crt, err := base64.StdEncoding.DecodeString(string(v.Cert))
		if err != nil {
			t.Fatalf("crt base64 : %v", err)
		}
		req.Spec.Config.TLS[k] = &api.KeyCert{
			Key: key, Cert: crt,
		}
	}
	md, err := NewMeridianNode(api.ActionInit, api.NodeRoleMaster, "", "", &req)
	if err != nil {
		t.Fatalf("run node initialize failed: %s", err.Error())
	}
	err = md.EnsureNode()
	if err != nil {
		t.Fatalf("ensure node failed: %s", err.Error())
	}
}

func TestCSR(t *testing.T) {
	addr := "192.168.64.57:6443"
	token := "ceadll.cy145q2pdodb4664"
	csr, err := GetCSR(addr, token, "xdpin")
	if err != nil {
		t.Fatalf("get csr failed: %s", err.Error())
	}
	t.Logf("xxxxxxxxxxxx csr: %s", tool.PrettyYaml(csr))
}
