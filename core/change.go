package core

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"

	"gopkg.in/yaml.v2"

	"github.com/miniscruff/changie/shared"
)

const timeFormat string = "20060102-150405"

type ChangesConfigSorter struct {
	changes []Change
	config  Config
}

func SortByConfig(config Config) *ChangesConfigSorter {
	return &ChangesConfigSorter{
		config: config,
	}
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (s *ChangesConfigSorter) Sort(changes []Change) {
	s.changes = changes
	sort.Sort(s)
}

// Len is part of sort.Interface.
func (s *ChangesConfigSorter) Len() int {
	return len(s.changes)
}

// Swap is part of sort.Interface.
func (s *ChangesConfigSorter) Swap(i, j int) {
	s.changes[i], s.changes[j] = s.changes[j], s.changes[i]
}

// Less will compare two Change values with the config settings.
// * Components, if enabled, are sorted by index in config
// * Kind, if enabled, are sorted by index in config
// * Time sorted newest first
func (s *ChangesConfigSorter) Less(i, j int) bool {
	a, b := &s.changes[i], &s.changes[j]

	// Start by sorting by component index
	if len(s.config.Components) > 0 && a.Component != b.Component {
		for _, c := range s.config.Components {
			if a.Component == c {
				return true
			} else if b.Component == c {
				return false
			}
		}
	}

	// Then sort by kind index
	if len(s.config.Kinds) > 0 && a.Kind != b.Kind {
		for _, k := range s.config.Kinds {
			if a.Kind == k.Label {
				return true
			} else if b.Kind == k.Label {
				return false
			}
		}
	}

	// Finish sort by newest first
	return a.Time.After(b.Time)
}

// Change represents an atomic change to a project
type Change struct {
	Component string `yaml:",omitempty"`
	Kind      string `yaml:",omitempty"`
	Body      string
	Time      time.Time
	Custom    map[string]string `yaml:",omitempty"`
}

// SaveUnreleased will save an unreleased change to the unreleased directory
func (change Change) SaveUnreleased(wf shared.WriteFiler, config Config) error {
	bs, _ := yaml.Marshal(&change)
	nameParts := make([]string, 0)

	if change.Component != "" {
		nameParts = append(nameParts, change.Component)
	}

	if change.Kind != "" {
		nameParts = append(nameParts, change.Kind)
	}

	nameParts = append(nameParts, change.Time.Format(timeFormat))

	filePath := fmt.Sprintf(
		"%s/%s/%s.yaml",
		config.ChangesDir,
		config.UnreleasedDir,
		strings.Join(nameParts, "-"),
	)

	return wf(filePath, bs, os.ModePerm)
}

func kindFromLabel(config Config, label string) *KindConfig {
	for _, kindConfig := range config.Kinds {
		if kindConfig.Label == label {
			return &kindConfig
		}
	}

	panic("label not part of any kind")
}

// AskPrompts will ask the user prompts based on the configuration
// updating the change as prompts are answered.
func AskPrompts(change *Change, config Config, stdinReader io.ReadCloser) error {
	var (
		err  error
		kind *KindConfig
	)

	if len(config.Components) > 0 {
		compPrompt := promptui.Select{
			Label: "Component",
			Items: config.Components,
			Stdin: stdinReader,
		}

		_, change.Component, err = compPrompt.Run()
		if err != nil {
			return err
		}
	}

	if len(config.Kinds) > 0 {
		kindPrompt := promptui.Select{
			Label: "Kind",
			Items: config.Kinds,
			Stdin: stdinReader,
		}

		_, change.Kind, err = kindPrompt.Run()
		if err != nil {
			return err
		}

		kind = kindFromLabel(config, change.Kind)
	}

	if kind == nil || !kind.SkipBody {
		bodyPrompt := config.Body.CreatePrompt(stdinReader)
		change.Body, err = bodyPrompt.Run()

		if err != nil {
			return err
		}
	}

	change.Custom = make(map[string]string)
	userChoices := make([]Custom, 0)

	if kind == nil || !kind.SkipGlobalChoices {
		userChoices = append(userChoices, config.CustomChoices...)
	}

	if kind != nil {
		userChoices = append(userChoices, kind.AdditionalChoices...)
	}

	for _, custom := range userChoices {
		prompt, err := custom.CreatePrompt(stdinReader)
		if err != nil {
			return err
		}

		change.Custom[custom.Key], err = prompt.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

// LoadChange will load a change from file path
func LoadChange(path string, rf shared.ReadFiler) (Change, error) {
	var c Change

	bs, err := rf(path)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal(bs, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}