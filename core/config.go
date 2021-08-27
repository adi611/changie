package core

import (
	"io"
	"os"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v2"

	"github.com/miniscruff/changie/shared"
)

const (
	ConfigPath string = ".changie.yaml"
)

// GetVersions will return, in semver sorted order, all released versions
type GetVersions func(shared.ReadDirer, Config) ([]*semver.Version, error)

type KindConfig struct {
	Label             string   `yaml:"label"`
	Header            string   `yaml:"format,omitempty"`
	ChangeFormat      string   `yaml:"changeFormat,omitempty"`
	SkipGlobalChoices bool     `yaml:"skipGlobalChoices,omitempty"`
	SkipBody          bool     `yaml:"skipBody,omitempty"`
	AdditionalChoices []Custom `yaml:"additionalChoices,omitempty"`
}

func (kc KindConfig) String() string {
	return kc.Label
}

type BodyConfig struct {
	MinLength *int64 `yaml:"minLength,omitempty"`
	MaxLength *int64 `yaml:"minLength,omitempty"`
}

func (b BodyConfig) CreatePrompt(stdinReader io.ReadCloser) Prompt {
	p, _ := Custom{
		Label:     "Body",
		Type:      CustomString,
		MinLength: b.MinLength,
		MaxLength: b.MaxLength,
	}.CreatePrompt(stdinReader)

	return p
}

// Config handles configuration for a changie project
type Config struct {
	ChangesDir        string `yaml:"changesDir"`
	UnreleasedDir     string `yaml:"unreleasedDir"`
	HeaderPath        string `yaml:"headerPath"`
	VersionHeaderPath string `yaml:"versionHeaderPath"`
	ChangelogPath     string `yaml:"changelogPath"`
	VersionExt        string `yaml:"versionExt"`
	// formats
	VersionFormat   string `yaml:"versionFormat"`
	ComponentFormat string `yaml:"componentFormat,omitempty"`
	KindFormat      string `yaml:"kindFormat,omitempty"`
	ChangeFormat    string `yaml:"changeFormat"`
	// custom
	Body          BodyConfig    `yaml:"body,omitempty"`
	Components    []string      `yaml:"components,omitempty"`
	Kinds         []KindConfig  `yaml:"kinds,omitempty"`
	CustomChoices []Custom      `yaml:"custom,omitempty"`
	Replacements  []Replacement `yaml:"replacements,omitempty"`
}

// Save will save the config as a yaml file to the default path
func (config Config) Save(wf shared.WriteFiler) error {
	bs, _ := yaml.Marshal(&config)
	return wf(ConfigPath, bs, os.ModePerm)
}

// LoadConfig will load the config from the default path
func LoadConfig(rf shared.ReadFiler) (Config, error) {
	var c Config

	bs, err := rf(ConfigPath)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal(bs, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}