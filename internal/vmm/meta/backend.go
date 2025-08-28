package meta

import (
	"context"
	"encoding/json"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path"
)

type Backend interface {
	Task() AbstractTask

	Config() AbstractConfig

	Machine() AbstractMachine

	Image() AbstractImage

	Docker() AbstractDocker
}

type Dir interface {
	Dir() string
}

type AbstractConfig interface {
	Dir
	Get(key string) (*Config, error)
	Set(cfg *Config) error
}

type AbstractMachine interface {
	Dir
	Get(key string) (*Machine, error)
	List() ([]*Machine, error)
	Create(machine *Machine) error
	Update(machine *Machine) error
	Destroy(machine *Machine) error
}

type AbstractImage interface {
	Dir
	Get(key string) (*Image, error)
	List() ([]*Image, error)
	Create(image *Image) error
	Update(image *Image) error
	Pull(image *Image) error
	Remove(image *Image) error
}

type AbstractTask interface {
	Dir
	Get(key string) (*Task, error)
	List() ([]*Task, error)
	Create(image *Task) error
	Stop(image *Task) error
	Remove(image *Task) error
}

type AbstractDocker interface {
	Dir
	Get(key string) (*Docker, error)
	List() ([]*Docker, error)
	Create(image *Docker) error
	Update(machine *Docker) error
	Remove(machine *Docker) error
}

type Docker struct {
	Name    string
	Version string
	VmName  string
	State   string
}

type Task struct {
	Id string `json:"id"`
}

type Config struct {
	AbsDir string `json:"absDir"`
}

func (cfg *Config) Dir() string {
	return cfg.AbsDir
}

type Image struct {
	Name     string            `json:"name"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Version  string            `json:"version"`
	Location string            `json:"location"`
	Labels   map[string]string `json:"labels"`
}

var Local = newLocalOrPanic()

var (
	_ AbstractConfig  = &config{}
	_ AbstractImage   = &image{}
	_ AbstractMachine = &machine{}
)

func DftRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	return path.Join(home, defaultRoot), nil
}

func NewLocalOrDie() Backend {
	var genRoot string
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	genRoot = path.Join(home, defaultRoot)
	return &local{genRoot}
}

func newLocalOrPanic() Backend {
	var genRoot string
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	genRoot = path.Join(home, defaultRoot)
	return &local{genRoot}
}

func NewLocal(root ...string) (Backend, error) {
	var genRoot string
	if len(root) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		genRoot = path.Join(home, defaultRoot)
	} else {
		genRoot = root[0]
	}
	return &local{genRoot}, nil
}

type local struct {
	root string
}

func (l *local) Config() AbstractConfig {
	cfg := &config{root: l.root}
	_, err := os.Stat(cfg.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(cfg.Dir(), 0755)
		}
	}
	return cfg
}

func (l *local) Docker() AbstractDocker {
	cfg := &docker{root: l.root}
	_, err := os.Stat(cfg.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(cfg.Dir(), 0755)
		}
	}
	return cfg
}

func (l *local) Machine() AbstractMachine {
	mch := &machine{root: l.root}
	_, err := os.Stat(mch.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(mch.Dir(), 0755)
		}
	}
	return mch
}

func (l *local) Image() AbstractImage {
	img := &image{root: l.root}
	_, err := os.Stat(img.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(img.Dir(), 0755)
		}
	}
	return img
}

func (l *local) Task() AbstractTask {
	return &task{root: l.root}
}

const (
	defaultRoot = ".meridian"
	machineJson = "machine.json"
	imageJson   = "image.json"
	dockerJson  = "docker.json"
)

type machine struct {
	root string
}

func (m *machine) Dir() string {
	return m.rootLocation()
}

func (m *machine) rootLocation(name ...string) string {
	return path.Join(m.root, "vms", path.Join(name...))
}

func (m *machine) Get(key string) (*Machine, error) {
	mch, err := m.get(key)
	if err != nil {
		return mch, err
	}
	pid, _ := mch.LoadPID()
	mch.SandboxPID = pid.PID
	return mch, nil
}

func (m *machine) get(key string) (*Machine, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(path.Join(pathName, machineJson))
}

func (m *machine) List() ([]*Machine, error) {
	var machines []*Machine
	pathName := m.Dir()
	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		mch, err := m.Get(dirName)
		if err != nil {
			continue
		}
		machines = append(machines, mch)
	}
	return machines, nil
}

func (m *machine) Create(machine *Machine) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}
	machine.AbsDir = pathName

	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *machine) Update(machine *Machine) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *machine) Destroy(machine *Machine) error {
	if machine.Name == "" {
		return fmt.Errorf("machine name is empty")
	}
	return os.RemoveAll(m.rootLocation(machine.Name))
}

func (m *machine) load(machineUri string) (*Machine, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Machine
	err = json.Unmarshal(data, &mch)
	if err != nil {
		return nil, err
	}
	mch.AbsDir = m.rootLocation(mch.Name)
	return &mch, nil
}

type image struct {
	root string
}

func (m *image) Dir() string {
	return m.rootLocation()
}

func (m *image) rootLocation(name ...string) string {
	return path.Join(m.root, "images", path.Join(name...))
}

func (m *image) Get(key string) (*Image, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(path.Join(pathName, imageJson))
}

func (m *image) List() ([]*Image, error) {
	var machines []*Image
	pathName := m.Dir()

	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		img, err := m.Get(dirName)
		if err != nil {
			klog.Errorf("unexpected image dir name: %s", dirName)
			continue
		}
		machines = append(machines, img)
	}
	return machines, nil
}

func (m *image) Create(image *Image) error {
	pathName := m.rootLocation(image.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, imageJson), data, 0644)
}

func (m *image) Update(image *Image) error {
	pathName := m.rootLocation(image.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, imageJson), data, 0644)
}

func (m *image) Remove(image *Image) error {
	if image.Name == "" {
		return fmt.Errorf("image name is empty")
	}
	return os.RemoveAll(m.rootLocation(image.Name))
}

func (m *image) load(machineUri string) (*Image, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Image
	err = json.Unmarshal(data, &mch)
	return &mch, err
}

func (m *image) Pull(image *Image) error {
	f := api.FindImage(image.Name)
	if f == nil {
		return fmt.Errorf("unexpected image name: [%s]", image.Name)
	}

	ctx := context.Background()

	res, err := downloader.Download(ctx, "", f.Location,
		downloader.WithCache(),
		downloader.WithDecompress(true),
		downloader.WithDescription(fmt.Sprintf("%s (%s)", "guest vm image", path.Base(f.Location))),
		downloader.WithExpectedDigest(f.Digest),
	)
	klog.Infof("pull image %s from %s with status: [%v]", image.Name, f.Location, res)
	return err
}

type config struct {
	root string
}

func (c *config) Dir() string {
	return c.rootLocation()
}

func (c *config) rootLocation() string {
	return path.Join(c.root, "config")
}
func (c *config) Get(key string) (*Config, error) {
	return &Config{AbsDir: c.rootLocation()}, nil
}

func (c *config) Set(cfg *Config) error {
	//TODO implement me
	panic("implement me")
}

type task struct {
	root string
}

func (m *task) Dir() string {
	return m.rootLocation()
}

func (m *task) rootLocation(name ...string) string {
	return path.Join(m.root, "tasks", path.Join(name...))
}

func (m *task) Get(key string) (*Task, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(pathName)
}

func (m *task) List() ([]*Task, error) {
	var machines []*Task
	pathName := m.Dir()
	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		machines = append(machines, &Task{Id: dirName})
	}
	return machines, nil
}

func (m *task) Create(image *Task) error {
	pathName := m.rootLocation(image.Id)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathName, data, 0644)
}

func (m *task) Update(image *Task) error {
	pathName := m.rootLocation(image.Id)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathName, data, 0644)
}

func (m *task) Remove(image *Task) error {
	if image.Id == "" {
		return fmt.Errorf("image name is empty")
	}
	return os.RemoveAll(m.rootLocation(image.Id))
}

func (m *task) load(machineUri string) (*Task, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Task
	err = json.Unmarshal(data, &mch)
	return &mch, err
}

func (m *task) Stop(image *Task) error {
	return fmt.Errorf("unimplemented image stop")
}

type docker struct {
	root string
}

func (m *docker) Dir() string {
	return m.rootLocation()
}

func (m *docker) rootLocation(name ...string) string {
	return path.Join(m.root, "docker", path.Join(name...))
}

func (m *docker) Get(key string) (*Docker, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(path.Join(pathName, dockerJson))
}

func (m *docker) List() ([]*Docker, error) {
	var machines []*Docker
	pathName := m.Dir()
	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		mch, err := m.Get(dirName)
		if err != nil {
			continue
		}
		machines = append(machines, mch)
	}
	return machines, nil
}

func (m *docker) Create(machine *Docker) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, dockerJson), data, 0644)
}

func (m *docker) Update(machine *Docker) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *docker) Remove(machine *Docker) error {
	if machine.Name == "" {
		return fmt.Errorf("machine name is empty")
	}
	return os.RemoveAll(m.rootLocation(machine.Name))
}

func (m *docker) load(machineUri string) (*Docker, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Docker
	err = json.Unmarshal(data, &mch)
	if err != nil {
		return nil, err
	}
	return &mch, nil
}
