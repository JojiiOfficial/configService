package configService

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type ConfigService struct {
	*Config
	configModTimes map[string]time.Time
}

type Config struct {
	Environment        string
	ENVPrefix          string
	Debug              bool
	Verbose            bool
	Silent             bool
	AutoReload         bool
	AutoReloadInterval time.Duration
	AutoReloadCallback func(config interface{})

	// In case of json files, this field will be used only when compiled with
	// go 1.10 or later.
	// This field will be ignored when compiled with go versions lower than 1.10.
	ErrorOnUnmatchedKeys bool
}

// New initialize a ConfigService
func New(config *Config) *ConfigService {
	if config == nil {
		config = &Config{}
	}

	if os.Getenv("CONFIGOR_DEBUG_MODE") != "" {
		config.Debug = true
	}

	if os.Getenv("CONFIGOR_VERBOSE_MODE") != "" {
		config.Verbose = true
	}

	if os.Getenv("CONFIGOR_SILENT_MODE") != "" {
		config.Silent = true
	}

	if config.AutoReload && config.AutoReloadInterval == 0 {
		config.AutoReloadInterval = time.Second
	}

	return &ConfigService{Config: config}
}

var testRegexp = regexp.MustCompile("_test|(\\.test$)")

// GetEnvironment get environment
func (configService *ConfigService) GetEnvironment() string {
	if configService.Environment == "" {
		if env := os.Getenv("CONFIGOR_ENV"); env != "" {
			return env
		}

		if testRegexp.MatchString(os.Args[0]) {
			return "test"
		}

		return "development"
	}
	return configService.Environment
}

// GetErrorOnUnmatchedKeys returns a boolean indicating if an error should be
// thrown if there are keys in the config file that do not correspond to the
// config struct
func (configService *ConfigService) GetErrorOnUnmatchedKeys() bool {
	return configService.ErrorOnUnmatchedKeys
}

//Save saves the config file
func (configService *ConfigService) Save(config interface{}, filename string) error {
	var js []byte
	var err error

	switch {
	case strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml"):
		js, err = yaml.Marshal(&config)
	case strings.HasSuffix(filename, ".json"):
		js, err = json.Marshal(&config)
	default:
		return errors.New("Unknown file type")
	}

	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, js, 0600)
}

// Load will unmarshal configurations to struct from files that you provide
func (configService *ConfigService) Load(config interface{}, files ...string) (err error) {
	defaultValue := reflect.Indirect(reflect.ValueOf(config))
	if !defaultValue.CanAddr() {
		return fmt.Errorf("Config %v should be addressable", config)
	}
	err, _ = configService.load(config, false, files...)

	if configService.Config.AutoReload {
		go func() {
			timer := time.NewTimer(configService.Config.AutoReloadInterval)
			for range timer.C {
				reflectPtr := reflect.New(reflect.ValueOf(config).Elem().Type())
				reflectPtr.Elem().Set(defaultValue)

				var changed bool
				if err, changed = configService.load(reflectPtr.Interface(), true, files...); err == nil && changed {
					reflect.ValueOf(config).Elem().Set(reflectPtr.Elem())
					if configService.Config.AutoReloadCallback != nil {
						configService.Config.AutoReloadCallback(config)
					}
				} else if err != nil {
					fmt.Printf("Failed to reload configuration from %v, got error %v\n", files, err)
				}
				timer.Reset(configService.Config.AutoReloadInterval)
			}
		}()
	}
	return
}

// Init inits the config default values
func (configService *ConfigService) Init(config interface{}, files ...string) (err error) {
	defaultValue := reflect.Indirect(reflect.ValueOf(config))
	if !defaultValue.CanAddr() {
		return fmt.Errorf("Config %v should be addressable", config)
	}
	err, _ = configService.init(config, false, files...)
	return
}

//SetupConfig create config file if not exists and fill it with default values
//Returns true if config was created
func (configService *ConfigService) SetupConfig(config interface{}, file string, initValues func(interface{}) interface{}) (bool, error) {
	s, err := os.Stat(file)
	if err != nil || s.Size() == 0 {
		configService.Init(config, file)
		config = initValues(config)
		err = configService.Save(config, file)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// ENV return environment
func ENV() string {
	return New(nil).GetEnvironment()
}

// Load will unmarshal configurations to struct from files that you provide
func Load(config interface{}, files ...string) error {
	return New(nil).Load(config, files...)
}

//Save saves a config
func Save(config interface{}, file string) error {
	return New(nil).Save(config, file)
}

//SetupConfig create config file if not exists and fill it with default values
//Returns true if config was created
func SetupConfig(config interface{}, file string, initValues func(interface{}) interface{}) (bool, error) {
	return New(nil).SetupConfig(config, file, initValues)
}

//NoChange don't change the config file
var NoChange = func(a interface{}) interface{} {
	return a
}
