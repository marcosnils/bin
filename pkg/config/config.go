package config

import (
  "encoding/json"
  "fmt"
  "github.com/tidwall/gjson"
  "io"
  "os"
  "path"
  "path/filepath"
  "reflect"
  "runtime"

  "github.com/apex/log"
)

const (
	configFile = "config.json"
	binaryFile = "bins.json"
)

var (
	cfg  config
	bins map[string]*Binary
)

type config struct {
	DefaultPath string `json:"default_path"`
}

type Binary struct {
	Path       string `json:"path"`
	RemoteName string `json:"remote_name"`
	Version    string `json:"version"`
	Hash       string `json:"hash"`
	URL        string `json:"url"`
	Provider   string `json:"provider"`
}

func CheckAndLoad(ensureBasePath bool) error {
	configDir, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.Mkdir(configDir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Error creating config directory [%v]", err)
	}
	log.Debugf("Config directory is: %s", configDir)

	_, err = os.Stat(filepath.Join(configDir, configFile))
	_, binErr := os.Stat(filepath.Join(configDir, binaryFile))

	// we have a `config.json` but no `bins.json`, let's try to load a legacy configuration
	if !os.IsNotExist(err) && os.IsNotExist(binErr) {
		err = loadLegacyConfig(filepath.Join(configDir, configFile))
		if bins != nil {
      // successfully loaded binary from legacy config, write them to disk
		  err = writeConfig()
    }
	} else {
		err = loadConfig(filepath.Join(configDir, configFile), filepath.Join(configDir, binaryFile))
	}
	if err != nil {
		return err
	}

	if bins == nil {
		bins = make(map[string]*Binary)
		if err := writeBins(); err != nil {
			return err
		}
	}

	if ensureBasePath && len(cfg.DefaultPath) == 0 {
		cfg.DefaultPath, err = getDefaultPath()
		if err != nil {
			return err
		}
		if err := writeConfig(); err != nil {
			return err
		}
		log.Debugf("Download path set to %s", cfg.DefaultPath)
	}
	return nil
}

func loadConfig(cfgPath, binPath string) error {
	cf, err := os.Open(cfgPath)
	if os.IsNotExist(err) {
		cfg = config{}
	} else if os.IsNotExist(err) {
		return err
	} else {
		defer cf.Close()
		err = json.NewDecoder(cf).Decode(&cfg)
		if err != nil {
			return err
		}
	}

	bf, err := os.Open(binPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	defer bf.Close()
	err = json.NewDecoder(bf).Decode(&bins)
	return err
}

func loadLegacyConfig(cfgPath string) error {
	err := backupConfig(cfgPath, cfgPath+".sav")
	if err != nil {
		return err
	}

	jStr, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jStr, &cfg)
	if err != nil {
		return err
	}

	b := gjson.Get(string(jStr), "bins")
	if b.Type == gjson.JSON && len(b.Raw) > 0 {
		err = json.Unmarshal([]byte(b.Raw), &bins)
	}
	return err
}

func backupConfig(src, dst string) error {
	log.Debugf("Found a legacy configuration, doing a backup: [%s]", dst)
	cfgFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	bakFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer bakFile.Close()

	_, err = io.Copy(bakFile, cfgFile)
	return err
}

func Get() (*config, map[string]*Binary) {
	return &cfg, bins
}

func GetValue(key string) (interface{}, error) {
	val := reflect.ValueOf(&cfg).Elem()
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		tag := field.Tag.Get("json")
		if field.Name == key || tag == key {
			return val.Field(i).Interface(), nil
		}
	}

	return nil, fmt.Errorf("configuration field not found: %s", key)
}

func SetValue(key string, value interface{}) error {
  val := reflect.ValueOf(&cfg).Elem()
  for i := 0; i < val.NumField(); i++ {
    field := val.Type().Field(i)
    tag := field.Tag.Get("json")
    if field.Name == key || tag == key {
      val.Field(i).SetString(value.(string))
    }
  }
  return writeConfig()
}

// UpsertBinary adds or updats an existing
// binary resource in the config
func UpsertBinary(c *Binary) error {
	if c != nil {
		bins[c.Path] = c
		err := writeBins()
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveBinaries removes the specified paths
// from bin configuration. It doesn't care about the order
func RemoveBinaries(paths []string) error {
	for _, p := range paths {
		delete(bins, p)
	}

	return writeBins()
}

func writeConfig() error {
	return write(configFile, cfg)
}

func writeBins() error {
	return write(binaryFile, bins)
}

func write(file string, data interface{}) error {
	configDir, err := getConfigPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(configDir, file), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	err = encoder.Encode(data)

	if err != nil {
		return err
	}

	return nil
}

// GetArch is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.
func GetArch() []string {
	res := []string{runtime.GOARCH}
	if runtime.GOARCH == "amd64" {
		// Adding x86_64 manually since the uname syscall (man 2 uname)
		// is not implemented in all systems
		res = append(res, "x86_64")
		res = append(res, "x64")
		res = append(res, "64")
	}
	return res
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() []string {
	res := []string{runtime.GOOS}
	if runtime.GOOS == "windows" {
		// Adding win since some repositories release with that as the indicator of a windows binary
		res = append(res, "win")
	}
	return res
}

// getConfigPath returns the path to the configuration directory respecting
// the `XDG Base Directory specification` using the following strategy:
//   - to prevent breaking of existing configurations, check if "$HOME/.bin/config.json"
//     exists and return "$HOME/.bin"
//   - if "XDG_CONFIG_HOME" is set, return "$XDG_CONFIG_HOME/bin"
//   - if "$HOME/.config" exists, return "$home/.config/bin"
//   - default to "$HOME/.bin/"
// ToDo: move the function to config_unix.go and add a similar function for windows,
//       %APPDATA% might be the right place on windows
func getConfigPath() (string, error) {
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		if _, err := os.Stat(filepath.Join(home, ".bin", "config.json")); !os.IsNotExist(err) {
			return filepath.Join(path.Join(home, ".bin")), nil
		}
	}

	c := os.Getenv("XDG_CONFIG_HOME")
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		return filepath.Join(c, "bin"), nil
	}

	if homeErr != nil {
		return "", homeErr
	}
	c = filepath.Join(home, ".config")
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		return filepath.Join(c, "bin"), nil
	}

	return filepath.Join(home, ".bin"), nil
}

func GetOSSpecificExtensions() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{"AppImage"}
	case "windows":
		return []string{"exe"}
	default:
		return nil
	}
}
