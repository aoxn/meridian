package nvidia

import (
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/samber/lo"
	"strings"
)

const LabelNvidiaDevice = "nvidia.com/device"

func HasNvidiaDevice() (bool, error) {

	info, err := ghw.GPU()
	if err != nil {
		return false, err
	}
	devs := lo.FilterMap(info.GraphicsCards, func(item *gpu.GraphicsCard, index int) (string, bool) {
		devInfo := item.DeviceInfo.String()
		if strings.Contains(strings.ToLower(devInfo), "nvidia") {
			return devInfo, true
		}
		return devInfo, false
	})
	return len(devs) > 0, nil
}
