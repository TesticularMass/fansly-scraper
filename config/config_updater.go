package config

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
)

// VerifyConfigOnStartup runs all checks to ensure a valid config file exists and is updated.
// This should be called when the application starts.
func VerifyConfigOnStartup() {
	configPath := GetConfigPath()
	if err := EnsureConfigExists(configPath); err != nil {
		log.Printf("Error ensuring config exists: %v", err)
	}
	if _, err := LoadConfig(configPath); err == nil {
		if err := EnsureConfigUpdated(configPath); err != nil {
			log.Printf("Error updating config: %v", err)
		}
	}
}

// EnsureConfigExists checks if a config file is present. If not, it attempts to create one
// by copying an example, creating a default, or downloading it from the repo.
func EnsureConfigExists(configPath string) error {
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config doesn't exist, check for local example-config.toml
		if _, err := os.Stat("example-config.toml"); err == nil {
			err = copyFile("example-config.toml", configPath)
			if err == nil {
				return nil // Successfully copied example config
			}
			log.Printf("Failed to copy example config: %v. Trying next method.", err)
		}

		// Try to create a default config
		defaultConfig := CreateDefaultConfig()
		return SaveConfig(defaultConfig)
	}
	return nil
}

// EnsureConfigUpdated adds settings missing from the config file with their
// default values. Settings present in the file keep their values, including
// explicit false and zero.
func EnsureConfigUpdated(configPath string) error {
	cfg := CreateDefaultConfig()
	metadata, err := toml.DecodeFile(configPath, cfg)
	if err != nil {
		return err
	}
	if err := ValidateConfig(cfg, configPath); err != nil {
		return err
	}

	defaults := CreateDefaultConfig()
	updated := false
	// Blank live-recording names are repaired: an empty extension produces
	// recordings ffmpeg cannot write.
	for _, field := range []struct {
		value    *string
		fallback string
	}{
		{&cfg.LiveSettings.VODsFileExtension, defaults.LiveSettings.VODsFileExtension},
		{&cfg.LiveSettings.FilenameTemplate, defaults.LiveSettings.FilenameTemplate},
		{&cfg.LiveSettings.DateFormat, defaults.LiveSettings.DateFormat},
	} {
		if *field.value == "" {
			*field.value = field.fallback
			updated = true
		}
	}

	var encodedDefaults bytes.Buffer
	if err := toml.NewEncoder(&encodedDefaults).Encode(defaults); err != nil {
		return err
	}
	defaultMetadata, err := toml.Decode(encodedDefaults.String(), &Config{})
	if err != nil {
		return err
	}
	for _, key := range defaultMetadata.Keys() {
		if !metadata.IsDefined(key...) {
			updated = true
			break
		}
	}

	if !updated {
		return nil
	}
	return writeConfig(configPath, cfg)
}

// MergeConfigs merges a new configuration into an existing one.
// It prioritizes values from the new config but fills in missing/default values from the existing and default configs.
func MergeConfigs(existing, new *Config) *Config {
	def := CreateDefaultConfig()
	res := *existing

	// Overlay struct non-zero values and preserve defaults where zero
	mergeStruct(&res.Account, new.Account, def.Account)
	mergeStruct(&res.Options, new.Options, def.Options)
	mergeStruct(&res.LiveSettings, new.LiveSettings, def.LiveSettings)
	mergeStruct(&res.Notifications, new.Notifications, def.Notifications)
	res.SecurityHeaders = new.SecurityHeaders

	return &res
}

func mergeStruct[T any](dst *T, src T, fallback T) {
	dVal := reflect.ValueOf(dst).Elem()
	sVal := reflect.ValueOf(src)
	fVal := reflect.ValueOf(fallback)

	for i := 0; i < sVal.NumField(); i++ {
		sField := sVal.Field(i)
		dField := dVal.Field(i)
		fField := fVal.Field(i)

		switch sField.Kind() {
		case reflect.String:
			if sField.String() != "" {
				dField.SetString(sField.String())
			} else if dField.String() == "" {
				dField.SetString(fField.String())
			}
		case reflect.Int, reflect.Int64:
			if sField.Int() != 0 {
				dField.SetInt(sField.Int())
			} else if dField.Int() == 0 {
				dField.SetInt(fField.Int())
			}
		case reflect.Bool:
			dField.SetBool(sField.Bool())
		}
	}
}

// copyFile is a simple utility to copy a file from a source to a destination.
func copyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ResetConfig removes the current config file (if present) and writes a fresh
// default config. This does not preserve any previous values.
func ResetConfig() error {
	configPath := GetConfigPath()
	// Best-effort remove existing file
	_ = os.Remove(configPath)
	// Ensure directory exists without creating a config file
	if err := os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
		return err
	}
	// Overwrite with a pristine default config
	defaultConfig := CreateDefaultConfig()
	// SaveConfig will not merge since we removed the file
	return SaveConfig(defaultConfig)
}
