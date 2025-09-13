package command

import (
	"context"
	"fmt"
	"strings"

	"os"
	"time"

	"bufio"
	"encoding/json"
	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// NewCommandPull returns a new cobra.Command for cluster creation
func NewCommandPull() *cobra.Command {
	var discover bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "meridian pull image",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if discover {
				klog.V(5).Infof("list available images")
				fmt.Printf("%-20s %-10s %-10s %s\n", "NAME", "OS", "ARCH", "DESCRIPTION")
				for _, v := range api.DftImages() {
					fmt.Printf("%-20s %-10s %-10s %s\n", v.Name, v.OS, v.Arch, "no description")
				}
				return nil
			}
			if len(args) < 2 {
				return fmt.Errorf("image name is needed")
			}
			if args[0] != "image" {
				return fmt.Errorf("only support [image]")
			}
			return PullImage(args[1])
		},
	}
	cmd.PersistentFlags().BoolVarP(&discover, "discover", "d", false, "discover available images")
	return cmd
}

func PullImage(name string) error {

	f := api.FindImage(name)
	if f == nil {
		return fmt.Errorf("unexpected image name: [%s], use[ m get image -d ] obtain available images", name)
	}
	backend := meta.Local
	_, err := backend.Image().Get(name)
	if err == nil {
		return fmt.Errorf("already exist: %s", name)
	}
	client, err := user.Client(ListenSock)
	if err != nil {
		return err
	}
	rst := client.Raw()
	r, err := rst.Get(context.TODO()).
		PathPrefix("/api/v1").
		Resource("image/pull").
		ResourceName(name).Stream()
	if err != nil {
		return errors.Wrapf(err, "pull image")
	}

	defer r.Close()
	bar, err := New(0)
	if err != nil {
		return err
	}
	bar.Start()
	defer bar.Finish()
	scanner := bufio.NewScanner(r)
	klog.Infof("pulling image: [%s]", name)
	var (
		lastErr  string
		complete = false
	)
	for scanner.Scan() {
		var data meta.Status
		err := json.Unmarshal([]byte(scanner.Text()), &data)
		if err != nil {
			return err
		}
		lastErr = data.Err
		if lastErr != "" {
			if strings.Contains(data.Err, "PullComplete") {
				bar.SetTotal(data.Data[0].Total)
				bar.SetCurrent(data.Data[0].Current)
				complete = true
				break
			}
			klog.Errorf("server responed: %s", data.Err)
			return errors.Wrapf(err, "server error:")
		}
		//bar.SetTotal(100)
		//bar.SetCurrent(int64(cnt))
		bar.SetTotal(data.Data[0].Total)
		bar.SetCurrent(data.Data[0].Current)
	}
	if !complete {
		return fmt.Errorf("interrupted with last error: %s", lastErr)
	}
	return nil
}

func getImage() ([]*meta.Image, error) {
	backend, err := meta.NewLocal()
	if err != nil {
		return nil, err
	}
	return backend.Image().List()
}

func New(size int64) (*pb.ProgressBar, error) {
	bar := pb.New64(size)

	bar.Set(pb.Bytes, true)
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		bar.SetTemplateString(`{{counters . }} {{bar . | green }} {{percent .}} {{speed . "%s/s"}}`)
		bar.SetRefreshRate(200 * time.Millisecond)
	} else {
		bar.Set(pb.Terminal, false)
		bar.Set(pb.ReturnSymbol, "\n")
		bar.SetTemplateString(`{{counters . }} ({{percent .}}) {{speed . "%s/s"}}`)
		bar.SetRefreshRate(5 * time.Second)
	}
	bar.SetWidth(80)
	if err := bar.Err(); err != nil {
		return nil, err
	}

	return bar, nil
}
