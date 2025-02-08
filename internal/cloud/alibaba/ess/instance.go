package ess

import (
	"context"
	"encoding/base64"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"time"
)

var _ cloud.IInstance = &instance{}

func NewInstance(mgr *client.ClientMgr) cloud.IInstance {
	return &instance{mgr}
}

type instance struct {
	*client.ClientMgr
}

func (n *instance) FindInstance(ctx context.Context, id cloud.Id) (cloud.InstanceModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *instance) ListInstance(ctx context.Context, i cloud.Id) ([]cloud.InstanceModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *instance) CreateInstance(ctx context.Context, i cloud.InstanceModel) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (n *instance) UpdateInstance(ctx context.Context, i cloud.InstanceModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *instance) DeleteInstance(ctx context.Context, id cloud.Id) error {
	//TODO implement me
	panic("implement me")
}

func (n *instance) RunCommand(ctx context.Context, id cloud.Id, cmd string) (string, error) {
	content := base64.StdEncoding.EncodeToString([]byte(cmd))

	commandType := "RunShellScript"
	waitInvocation := func(timeout time.Duration) error {
		mfunc := func() (bool, error) {
			req := ecs.CreateDescribeInvocationsRequest()
			req.InstanceId = id.Id
			req.RegionId = id.Region
			// 阿里云垃圾API。擦
			// 该API的InvokeStatus 过滤有问题，需要手动过滤
			req.InvokeStatus = "Running"
			req.CommandType = commandType
			inv, err := n.ECS.DescribeInvocations(req)
			if err != nil {
				klog.Errorf("descirbe invocation: %s", err.Error())
				return false, nil
			}
			if inv.TotalCount == 0 {
				return true, nil
			}
			cnt := 0
			//兼容
			for _, i := range inv.Invocations.Invocation {
				if i.InvocationStatus == "Running" {
					cnt++
					klog.Infof("command [%s.%s.%s] is still in running", id, i.InvokeId, i.CommandId)
				}
			}
			if cnt == 0 {
				return true, nil
			}
			klog.Infof("invocation still in progress: total %d, %s", cnt, id)
			return false, nil
		}
		return wait.PollImmediate(3*time.Second, timeout, mfunc)
	}
	err := waitInvocation(4 * time.Minute)
	if err != nil {
		return "", errors.Wrapf(err, "wait invocation: ")
	}

	// run command
	req := ecs.CreateRunCommandRequest()
	req.Timed = requests.NewBoolean(false)
	req.CommandContent = content
	req.ContentEncoding = "Base64"
	req.Type = commandType
	req.InstanceId = &[]string{id.Id}
	req.KeepCommand = requests.NewBoolean(false)
	klog.Infof("run command: [%s]", cmd)
	rcmd, err := n.ECS.RunCommand(req)
	if err != nil {
		return "", errors.Wrapf(err, "run command")
	}

	waitResult := func(ivk *ecs.Invocation, timeout time.Duration) error {
		mfunc := func() (bool, error) {
			req := ecs.CreateDescribeInvocationResultsRequest()
			req.InstanceId = id.Id
			req.InvokeId = rcmd.InvokeId
			inv, err := n.ECS.DescribeInvocationResults(req)
			if err != nil {
				klog.Errorf("describe run command result: %s", err.Error())
				return false, nil
			}
			result := inv.Invocation.InvocationResults.InvocationResult
			if len(result) == 0 {
				klog.Infof("invokeid not found: %s, %s", id, rcmd.InvokeId)
				return false, nil
			}
			if result[0].InvokeRecordStatus != "Running" {
				*ivk = inv.Invocation
				return true, nil
			}
			klog.Infof("wait run command, in progress: %s, %s", id, rcmd.InvokeId)
			return false, nil
		}
		return wait.PollImmediate(4*time.Second, timeout, mfunc)
	}
	ivk := ecs.Invocation{}
	if err := waitResult(&ivk, 4*time.Minute); err != nil {
		return "", errors.Wrapf(err, "wait command result")
	}
	r := ivk.InvocationResults.InvocationResult
	if len(r) == 0 {
		klog.Infof("[runcommand] empty result, %s[%s]", id.Id, rcmd.InvokeId)
		return "", nil
	}
	output, err := base64.StdEncoding.DecodeString(r[0].Output)
	if err != nil {
		return "", err
	}
	return string(output), nil
	//return pd.Result{Status: ivk.InvocationStatus, OutPut: string(output)}, nil
}
