package ess

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ess"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	"github.com/pkg/errors"
)

var _ cloud.IElasticScalingGroup = &elasticScalingGroup{}

func NewESS(mgr *client.ClientMgr) cloud.IElasticScalingGroup {
	return &elasticScalingGroup{ClientMgr: mgr}
}

type elasticScalingGroup struct {
	*client.ClientMgr
}

func (n *elasticScalingGroup) FindESSBy(ctx context.Context, id cloud.Id) (cloud.ScalingGroupModel, error) {
	var (
		model = cloud.ScalingGroupModel{}
		req   = ess.CreateDescribeScalingGroupsRequest()
	)
	if id.Id != "" {
		req.ScalingGroupId = &[]string{id.Id}
	}
	if id.Name != "" {
		req.ScalingGroupName = id.Name
	}

	r, err := n.ESS.DescribeScalingGroups(req)
	if err != nil {
		return model, err
	}
	if len(r.ScalingGroups.ScalingGroup) == 0 {
		return model, cloud.NotFound
	}
	if len(r.ScalingGroups.ScalingGroup) > 1 {
		klog.Infof("multiple ess group found: %d", len(r.ScalingGroups.ScalingGroup))
	}
	model.ScalingGroupId = r.ScalingGroups.ScalingGroup[0].ScalingGroupId
	klog.V(5).Infof("[service] find ess: %s, returned %s", id.Name, model.ScalingGroupId)
	return model, nil
}

func (n *elasticScalingGroup) ListESS(ctx context.Context, id cloud.Id) ([]cloud.ScalingGroupModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *elasticScalingGroup) CreateESS(ctx context.Context, vpcId string, model cloud.ScalingGroupModel) (string, error) {
	if vpcId == "" {
		return "", fmt.Errorf("vpcId must be provided")
	}
	var ids []string
	for _, v := range model.VSwitchId {
		ids = append(ids, v.VSwitchId)
	}
	req := ess.CreateCreateScalingGroupRequest()

	req.ScalingGroupName = model.ScalingGroupName
	req.MinSize = requests.NewInteger(model.Min)
	req.MaxSize = requests.NewInteger(model.Max)
	req.VSwitchIds = &ids
	req.RegionId = model.Region

	klog.V(5).Infof("[service] create ess: %s", req.ScalingGroupName)
	r, err := n.ESS.CreateScalingGroup(req)
	if err != nil {
		return "", err
	}
	time.Sleep(5 * time.Second)
	return r.ScalingGroupId, err
}

func (n *elasticScalingGroup) ScaleNodeGroup(
	ctx context.Context, model cloud.ScalingGroupModel, desired uint,
) error {
	var err error
	if desired > 0 {

		ud := base64.StdEncoding.EncodeToString([]byte(model.ScalingConfig.UserData))
		sreq := ess.CreateModifyScalingConfigurationRequest()

		sreq.UserData = ud
		sreq.ScalingConfigurationId = model.ScalingConfig.ScalingCfgId
		_, err := n.ESS.ModifyScalingConfiguration(sreq)
		if err != nil {
			return errors.Wrap(err, "failed to modify scaling group")
		}
	}
	req := ess.CreateModifyScalingRuleRequest()
	req.RegionId = model.Region
	req.ScalingRuleId = model.ScalingRule.ScalingRuleId
	req.ScalingRuleName = model.ScalingRule.ScalingRuleName
	req.AdjustmentType = "TotalCapacity"
	req.AdjustmentValue = requests.NewInteger(int(desired))
	// ScalingRuleType: "SimpleScalingRule",

	_, err = n.ESS.ModifyScalingRule(req)
	if err != nil {
		return fmt.Errorf("set scaling rule to %d fail: %s", desired, err.Error())
	}
	klog.Infof("modify scaling rule[%s] desired capacity: %d.", req.ScalingRuleId, desired)

	err = WaitActivity(n.ESS, model.ScalingGroupId, model.Region)
	if err != nil {
		return fmt.Errorf("wait for activity enable: %s", model.ScalingGroupId)
	}
	ereq := ess.CreateExecuteScalingRuleRequest()

	ereq.ScalingRuleAri = model.ScalingRule.ScalingRuleAri
	_, err = n.ESS.ExecuteScalingRule(ereq)
	if err != nil {
		if strings.Contains(err.Error(), "will not change") {
			klog.Warningf("desired state consist: apiError, the total capacity will not change")
			return nil
		}
		return fmt.Errorf("execute node scaling rule failed: %s", err.Error())
	}
	klog.Infof("[ScaleNodeGroup] execute scale rule: target=%d, wait for finish", desired)

	return WaitActivity(n.ESS, model.ScalingGroupId, model.Region)
}

func WaitActivity(
	client *ess.Client,
	id string,
	region string,
) error {
	return wait.PollImmediate(
		10*time.Second, 4*time.Minute,
		func() (done bool, err error) {
			req := ess.CreateDescribeScalingActivitiesRequest()
			req.ScalingGroupId = id
			req.StatusCode = "InProgress"
			req.RegionId = region

			result, err := client.DescribeScalingActivities(req)
			if err != nil {
				klog.Errorf("[scaling group] wait scaling activity to complete: %s", err.Error())
				return false, nil
			}
			if len(result.ScalingActivities.ScalingActivity) != 0 {
				klog.Infof("[scaling group] %d activities is InProgress", len(result.ScalingActivities.ScalingActivity))
				return false, nil
			}
			klog.Infof("[scaling group] all scaling activity finished.")
			return true, nil
		},
	)
}

func (n *elasticScalingGroup) UpdateESS(ctx context.Context, ess cloud.ScalingGroupModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *elasticScalingGroup) DeleteESS(ctx context.Context, m cloud.ScalingGroupModel) error {
	if m.ScalingGroupId == "" {
		return fmt.Errorf("unexpected empty scaling group id")
	}
	req := ess.CreateDeleteScalingGroupRequest()
	req.ScalingGroupId = m.ScalingGroupId
	_, err := n.ESS.DeleteScalingGroup(req)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil
		}
	}
	waitOnDelete := func(ctx context.Context) (done bool, err error) {
		_, err = n.FindESSBy(ctx, cloud.Id{Id: m.ScalingGroupId, Name: m.ScalingGroupName})
		if err != nil {
			if errors.Is(err, cloud.NotFound) {
				return true, nil
			}
			klog.Errorf("wait on delete scaling group %s, %s", m.ScalingGroupId, err.Error())
			return false, err
		}
		return false, nil
	}
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, false, waitOnDelete)
}

func (n *elasticScalingGroup) CreateScalingConfig(ctx context.Context, scalingGrpId string, cfg cloud.ScalingConfig) (string, error) {
	if scalingGrpId == "" ||
		cfg.SecurityGrpId == "" ||
		cfg.ScalingCfgName == "" ||
		cfg.ImageId == "" ||
		cfg.UserData == "" {
		return "", fmt.Errorf("create scaling group with empty para")
	}

	klog.V(5).Infof("debug create scalingconfig: [%s][%s][%s]", scalingGrpId, cfg.SecurityGrpId, cfg.ImageId)

	// params: {"RegionId":"cn-hangzhou","ScalingGroupId":"asg-bp1d39mt8iu72ap1iaw2","ScalingConfigurationName":"abcc","ImageId":"aliyun_3_x64_20G_alibase_20241103.vhd","IoOptimized":"optimized","InternetChargeType":"PayByTraffic","InternetMaxBandwidthOut":50,"SystemDisk.Category":"cloud_auto","SystemDisk.Size":40,"SystemDisk.ProvisionedIops":0,"SystemDisk.BurstingEnabled":true,"LoadBalancerWeight":50,"Tags":{},"UserData":"","PasswordInherit":false,"SecurityEnhancementStrategy":"Active","SpotStrategy":"NoSpot","ResourceGroupId":"rg-acfmws6i533ciay","InstancePatternInfos.1.InstanceFamilyLevel":"EnterpriseLevel","InstancePatternInfos.1.ExcludedInstanceTypes.1":"ecs.c8y.xlarge","InstancePatternInfos.1.MinimumCpuCoreCount":4,"InstancePatternInfos.1.MaximumCpuCoreCount":4,"InstancePatternInfos.1.MinimumMemorySize":8,"InstancePatternInfos.1.MaximumMemorySize":8,"SecurityGroupIds.1":"sg-bp1b8sjd76rng53ngqbs","ImageOptions.LoginAsNonRoot":false}
	tags, _ := json.Marshal(cfg.Tag)
	// create EIP address automatically
	// "InternetChargeType":"PayByBandwidth","InternetMaxBandwidthOut":1
	req := ess.CreateCreateScalingConfigurationRequest()
	req.ScalingGroupId = scalingGrpId
	req.SecurityGroupId = cfg.SecurityGrpId
	req.InstanceType = cfg.InstanceType
	req.ImageId = cfg.ImageId
	req.ScalingConfigurationName = cfg.ScalingCfgName
	req.InternetChargeType = "PayByTraffic"
	req.RamRoleName = cfg.RamRole
	req.UserData = base64.StdEncoding.EncodeToString([]byte(cfg.UserData))
	req.InternetMaxBandwidthOut = requests.NewInteger(50)
	//req.InstanceName: ""
	req.Tags = string(tags)
	req.KeyPairName = ""
	req.IoOptimized = "optimized"
	req.SystemDiskSize = requests.NewInteger(100)
	req.SystemDiskCategory = "cloud_essd"

	klog.Infof("[service] create scaling configuration with request: %s", cfg.ScalingCfgName)
	r, err := n.ESS.CreateScalingConfiguration(req)
	if err != nil {
		return "", err
	}
	return r.ScalingConfigurationId, nil
}

func (n *elasticScalingGroup) EnableScalingGroup(ctx context.Context, scalingGrpId, scalingCfgId string) error {
	if scalingGrpId == "" ||
		scalingCfgId == "" {
		return fmt.Errorf("enable scaling group with empty para")
	}
	req := ess.CreateEnableScalingGroupRequest()
	req.ScalingGroupId = scalingGrpId
	req.ActiveScalingConfigurationId = scalingCfgId

	klog.Infof("[service] enable scaling configuration %s", scalingGrpId)
	_, err := n.ESS.EnableScalingGroup(req)
	return err
}

//func (n *elasticScalingGroup) DescribeScalingGroups(ctx context.Context, id cloud.Tag) (cloud.ScalingGroupModel, error) {
//	if do.Id == "" &&
//		do.ScalingGrpId == "" &&
//		do.Name == "" {
//		return do, fmt.Errorf("describe scaling group with empty para")
//	}
//
//	req := ess.DescribeScalingGroupsArgs{}
//	if do.Id != "" {
//		req.ScalingGroupId = common.FlattenArray{do.Id}
//	}
//	if do.ScalingGrpId != "" {
//		req.ScalingGroupId = common.FlattenArray{do.ScalingGrpId}
//	}
//	if do.Name != "" {
//		req.ScalingGroupName = common.FlattenArray{do.Name}
//	}
//	r, _, err := n.ESS.DescribeScalingGroups(&req)
//	if err != nil {
//		return do, err
//	}
//	if len(r) <= 0 {
//		return do, NotFound
//	}
//	if len(r) > 1 {
//		klog.Infof("multiple scaling group found: %d", len(r))
//	}
//	do.State = string(r[0].LifecycleState)
//	return do, nil
//}

func (n *elasticScalingGroup) CreateScalingRule(ctx context.Context, scalingGrpId string, model cloud.ScalingRule) (cloud.ScalingRule, error) {

	req := ess.CreateCreateScalingRuleRequest()
	req.RegionId = model.Region
	req.ScalingGroupId = scalingGrpId
	req.ScalingRuleName = model.ScalingRuleName
	req.AdjustmentType = "TotalCapacity"
	req.AdjustmentValue = requests.NewInteger(1)

	klog.Infof("[service] create scaling rule for %s with name %s", scalingGrpId, model.ScalingRuleName)
	r, err := n.ESS.CreateScalingRule(req)
	if err != nil {
		return model, err
	}
	model.ScalingRuleId = r.ScalingRuleId
	model.ScalingRuleAri = r.ScalingRuleAri
	return model, nil
}

func (n *elasticScalingGroup) FindScalingConfig(ctx context.Context, id cloud.Id) (cloud.ScalingConfig, error) {
	model := cloud.ScalingConfig{}
	req := ess.CreateDescribeScalingConfigurationsRequest()
	req.RegionId = id.Region

	if id.Id != "" {
		req.ScalingConfigurationId = &[]string{id.Id}
	}
	if id.Name != "" {
		req.ScalingConfigurationName = &[]string{id.Name}
	}
	r, err := n.ESS.DescribeScalingConfigurations(req)
	if err != nil {
		return model, err
	}
	if len(r.ScalingConfigurations.ScalingConfiguration) == 0 {
		return model, cloud.NotFound
	}
	if len(r.ScalingConfigurations.ScalingConfiguration) > 1 {
		klog.Infof("[service] multiple scaling config found: %d", len(r.ScalingConfigurations.ScalingConfiguration))
	}

	model.ScalingCfgId = r.ScalingConfigurations.ScalingConfiguration[0].ScalingConfigurationId
	model.ScalingCfgName = r.ScalingConfigurations.ScalingConfiguration[0].ScalingConfigurationName
	return model, nil
}

func (n *elasticScalingGroup) FindScalingRule(ctx context.Context, id cloud.Id) (cloud.ScalingRule, error) {
	klog.Infof("find scaling rule for %v", id)
	model := cloud.ScalingRule{}
	req := ess.CreateDescribeScalingRulesRequest()
	req.RegionId = id.Region

	if id.Id != "" {
		req.ScalingRuleId = &[]string{id.Id}
	}
	if id.Name != "" {
		req.ScalingRuleName = &[]string{id.Name}
	}
	r, err := n.ESS.DescribeScalingRules(req)
	if err != nil {
		return model, err
	}
	if len(r.ScalingRules.ScalingRule) == 0 {
		return model, cloud.NotFound
	}
	if len(r.ScalingRules.ScalingRule) > 1 {
		klog.Infof("[service] multiple scaling rules found: %d", len(r.ScalingRules.ScalingRule))
	}
	model.ScalingRuleId = r.ScalingRules.ScalingRule[0].ScalingRuleId
	model.ScalingRuleName = r.ScalingRules.ScalingRule[0].ScalingRuleName
	model.ScalingRuleAri = r.ScalingRules.ScalingRule[0].ScalingRuleAri
	return model, nil
}

func (n *elasticScalingGroup) ExecuteScalingRule(ctx context.Context, scalingRuleAri string) (string, error) {
	if scalingRuleAri == "" {
		return "", fmt.Errorf("execute scalingrule, unexpected empty id")
	}
	req := ess.CreateExecuteScalingRuleRequest()
	req.ScalingRuleAri = scalingRuleAri

	r, err := n.ESS.ExecuteScalingRule(req)
	if err != nil {
		return "", err
	}
	return r.ScalingActivityId, nil
}

func (n *elasticScalingGroup) DeleteScalingConfig(ctx context.Context, cfgId string) error {
	//TODO implement me
	panic("implement me")
}
func (n *elasticScalingGroup) DeleteScalingRule(ctx context.Context, ruleId string) error {
	//TODO implement me
	panic("implement me")
}
