package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"

	"github.com/miniscruff/changie/core"
	. "github.com/miniscruff/changie/testutils"
)

var _ = Describe("Batch", func() {

	var (
		fs              *MockFS
		afs             afero.Afero
		mockError       error
		testConfig      core.Config
		fileCreateIndex int
		futurePath      string
		newVerPath      string

		orderedTimes = []time.Time{
			time.Date(2019, 5, 25, 20, 45, 0, 0, time.UTC),
			time.Date(2017, 4, 25, 15, 20, 0, 0, time.UTC),
			time.Date(2015, 3, 25, 10, 5, 0, 0, time.UTC),
		}
	)

	BeforeEach(func() {
		fs = NewMockFS()
		afs = afero.Afero{Fs: fs}
		mockError = errors.New("dummy mock error")
		testConfig = core.Config{
			ChangesDir:    "news",
			UnreleasedDir: "future",
			HeaderPath:    "header.rst",
			ChangelogPath: "news.md",
			VersionExt:    "md",
			VersionFormat: "## {{.Version}}",
			KindFormat:    "\n### {{.Kind}}",
			ChangeFormat:  "* {{.Body}}",
			Kinds: []core.KindConfig{
				{Label: "added"},
				{Label: "removed"},
				{Label: "other"},
			},
		}

		futurePath = filepath.Join("news", "future")
		newVerPath = filepath.Join("news", "v0.2.0.md")
		fileCreateIndex = 0

		// reset our shared values
		versionHeaderPath = ""
		keepFragments = false
	})

	// this mimics the change.SaveUnreleased but prevents clobbering in same
	// second saves
	writeChangeFile := func(change core.Change) {
		bs, _ := yaml.Marshal(&change)
		nameParts := make([]string, 0)

		if change.Component != "" {
			nameParts = append(nameParts, change.Component)
		}

		if change.Kind != "" {
			nameParts = append(nameParts, change.Kind)
		}

		// add an index to prevent clobbering
		nameParts = append(nameParts, fmt.Sprintf("%v", fileCreateIndex))
		fileCreateIndex++

		filePath := fmt.Sprintf(
			"%s/%s/%s.yaml",
			testConfig.ChangesDir,
			testConfig.UnreleasedDir,
			strings.Join(nameParts, "-"),
		)

		Expect(afs.WriteFile(filePath, bs, os.ModePerm)).To(Succeed())
	}

	writeHeader := func(header string) {
		headerData := []byte(header)
		headerPath := filepath.Join(futurePath, versionHeaderPath)
		Expect(afs.WriteFile(headerPath, headerData, os.ModePerm)).To(Succeed())
	}

	It("can batch version", func() {
		// declared path but missing is accepted
		testConfig.VersionHeaderPath = "header.md"
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "A"})
		writeChangeFile(core.Change{Kind: "added", Body: "B"})
		writeChangeFile(core.Change{Kind: "removed", Body: "C"})

		err := batchPipeline(afs, "v0.2.0")
		Expect(err).To(BeNil())

		verContents := `## v0.2.0

### added
* A
* B

### removed
* C`

		Expect(newVerPath).To(HaveContents(afs, verContents))

		infos, err := afs.ReadDir(futurePath)
		Expect(err).To(BeNil())
		Expect(len(infos)).To(Equal(0))
	})

	It("can batch version keeping change files", func() {
		keepFragments = true
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "A"})
		writeChangeFile(core.Change{Kind: "added", Body: "B"})
		writeChangeFile(core.Change{Kind: "removed", Body: "C"})

		err := batchPipeline(afs, "v0.2.0")
		Expect(err).To(BeNil())

		infos, err := afs.ReadDir(futurePath)
		Expect(err).To(BeNil())
		Expect(len(infos)).To(Equal(3))
	})

	It("can batch version with header", func() {
		versionHeaderPath = "h.md"
		testConfig.VersionHeaderPath = "h.md"
		testConfig.VersionFormat = testConfig.VersionFormat + "\n"
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "A"})
		writeChangeFile(core.Change{Kind: "added", Body: "B"})
		writeChangeFile(core.Change{Kind: "removed", Body: "C"})
		writeHeader("this is a new version that adds cool features")

		Expect(batchPipeline(afs, "v0.2.0")).To(Succeed())

		verContents := `## v0.2.0

this is a new version that adds cool features

### added
* A
* B

### removed
* C`

		Expect(newVerPath).To(HaveContents(afs, verContents))

		infos, err := afs.ReadDir(futurePath)
		Expect(err).To(BeNil())
		Expect(len(infos)).To(Equal(0))
	})

	It("can batch version with header parameter", func() {
		versionHeaderPath = "head.md"
		testConfig.VersionFormat = testConfig.VersionFormat + "\n"
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "A"})
		writeChangeFile(core.Change{Kind: "added", Body: "B"})
		writeChangeFile(core.Change{Kind: "removed", Body: "C"})
		writeHeader("this is a new version that adds cool features")

		Expect(batchPipeline(afs, "v0.2.0")).To(Succeed())

		verContents := `## v0.2.0

this is a new version that adds cool features

### added
* A
* B

### removed
* C`

		Expect(newVerPath).To(HaveContents(afs, verContents))

		infos, err := afs.ReadDir(futurePath)
		Expect(err).To(BeNil())
		Expect(len(infos)).To(Equal(0))
	})

	It("returns error on bad semver", func() {
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "A"})

		err := batchPipeline(afs, "not-semanticly-correct")
		Expect(err).To(Equal(core.ErrBadVersionOrPart))
	})

	It("returns error on bad config", func() {
		configData := []byte("not a proper config")
		err := afs.WriteFile(core.ConfigPaths[0], configData, os.ModePerm)
		Expect(err).To(BeNil())

		Expect(batchPipeline(afs, "v1.0.0")).NotTo(Succeed())
	})

	It("returns error on bad changes", func() {
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		aVer := []byte("not a valid change")
		err := afs.WriteFile(filepath.Join(futurePath, "a.yaml"), aVer, os.ModePerm)
		Expect(err).To(BeNil())

		err = batchPipeline(afs, "v1.0.0")
		Expect(err).NotTo(BeNil())
	})

	It("returns error on bad batch", func() {
		testConfig.VersionFormat = "{{bad.format}"
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "removed", Body: "C"})

		Expect(batchPipeline(afs, "v1.0.0")).NotTo(Succeed())
	})

	It("returns error on bad delete", func() {
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		writeChangeFile(core.Change{Kind: "added", Body: "C"})

		fs.MockRemove = func(name string) error {
			return mockError
		}

		Expect(batchPipeline(afs, "v1.0.0")).NotTo(Succeed())
	})

	It("returns error on bad header file", func() {
		Expect(testConfig.Save(afs.WriteFile)).To(Succeed())

		versionHeaderPath = "header.md"
		badOpen := errors.New("bad open file")
		fs.MockOpen = func(name string) (afero.File, error) {
			if strings.HasSuffix(name, versionHeaderPath) {
				return nil, badOpen
			}
			return fs.MemFS.Open(name)
		}

		writeChangeFile(core.Change{Kind: "added", Body: "A"})

		Expect(batchPipeline(afs, "v0.2.0")).To(Equal(badOpen))
	})

	It("can get all changes", func() {
		writeChangeFile(core.Change{Kind: "removed", Body: "third", Time: orderedTimes[2]})
		writeChangeFile(core.Change{Kind: "added", Body: "first", Time: orderedTimes[0]})
		writeChangeFile(core.Change{Kind: "added", Body: "second", Time: orderedTimes[1]})

		ignoredPath := filepath.Join(futurePath, "ignored.txt")
		Expect(afs.WriteFile(ignoredPath, []byte("ignored"), os.ModePerm)).To(Succeed())

		changes, err := getChanges(afs, testConfig)
		Expect(err).To(BeNil())
		Expect(changes[0].Body).To(Equal("first"))
		Expect(changes[1].Body).To(Equal("second"))
		Expect(changes[2].Body).To(Equal("third"))
	})

	It("can get all changes with components", func() {
		writeChangeFile(core.Change{
			Component: "compiler",
			Kind:      "removed",
			Body:      "A",
			Time:      orderedTimes[2],
		})
		writeChangeFile(core.Change{
			Component: "linker",
			Kind:      "added",
			Body:      "B",
			Time:      orderedTimes[0],
		})
		writeChangeFile(core.Change{
			Component: "linker",
			Kind:      "added",
			Body:      "C",
			Time:      orderedTimes[1],
		})

		ignoredPath := filepath.Join(futurePath, "ignored.txt")
		Expect(afs.WriteFile(ignoredPath, []byte("ignored"), os.ModePerm)).To(Succeed())

		testConfig.Components = []string{"linker", "compiler"}
		changes, err := getChanges(afs, testConfig)
		Expect(err).To(BeNil())
		Expect(changes[0].Body).To(Equal("B"))
		Expect(changes[1].Body).To(Equal("C"))
		Expect(changes[2].Body).To(Equal("A"))
	})

	It("returns err if unable to read directory", func() {
		fs.MockOpen = func(path string) (afero.File, error) {
			var f afero.File
			return f, mockError
		}

		_, err := getChanges(afs, testConfig)
		Expect(err).To(Equal(mockError))
	})

	It("returns err if bad changes file", func() {
		badYaml := []byte("not a valid yaml:::::file---___")
		err := afs.WriteFile(filepath.Join(futurePath, "a.yaml"), badYaml, os.ModePerm)
		Expect(err).To(BeNil())

		_, err = getChanges(afs, testConfig)
		Expect(err).NotTo(BeNil())
	})

	It("can create new version file", func() {
		vFile := NewMockFile(fs, "v0.1.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Kind: "added", Body: "w"},
			{Kind: "added", Body: "x"},
			{Kind: "removed", Body: "y"},
			{Kind: "removed", Body: "z"},
		}

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: changes,
		})
		Expect(err).To(BeNil())

		expected := `## v0.1.0

### added
* w
* x

### removed
* y
* z`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("can create new version file without kind headers", func() {
		vFile := NewMockFile(fs, "v0.2.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Body: "w", Kind: "added"},
			{Body: "x", Kind: "added"},
			{Body: "y", Kind: "removed"},
			{Body: "z", Kind: "removed"},
		}

		testConfig.KindFormat = ""
		testConfig.ChangeFormat = "* {{.Body}} ({{.Kind}})"
		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.2.0"),
			Changes: changes,
		})
		Expect(err).To(BeNil())

		expected := `## v0.2.0
* w (added)
* x (added)
* y (removed)
* z (removed)`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("can create new version file with component headers", func() {
		vFile := NewMockFile(fs, "v0.1.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Body: "w", Kind: "added", Component: "linker"},
			{Body: "x", Kind: "added", Component: "linker"},
			{Body: "y", Kind: "removed", Component: "linker"},
			{Body: "z", Kind: "removed", Component: "compiler"},
		}

		testConfig.Components = []string{"linker", "compiler"}
		testConfig.ComponentFormat = "\n## {{.Component}}"
		testConfig.KindFormat = "### {{.Kind}}"
		testConfig.ChangeFormat = "* {{.Body}}"
		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: changes,
		})
		Expect(err).To(BeNil())

		expected := `## v0.1.0

## linker
### added
* w
* x
### removed
* y

## compiler
### removed
* z`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("can create new version file with header", func() {
		vFile := NewMockFile(fs, "v0.1.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Body: "w", Kind: "added"},
			{Body: "x", Kind: "added"},
			{Body: "y", Kind: "removed"},
			{Body: "z", Kind: "removed"},
		}

		testConfig.VersionFormat = testConfig.VersionFormat + "\n"
		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: changes,
			Header:  "Some header we want included in our new version.\nCan also have newlines.",
		})
		Expect(err).To(BeNil())

		expected := `## v0.1.0

Some header we want included in our new version.
Can also have newlines.

### added
* w
* x

### removed
* y
* z`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("can create new version file with custom kind header", func() {
		vFile := NewMockFile(fs, "v0.2.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Body: "z", Kind: "added"},
		}

		testConfig.Kinds = []core.KindConfig{
			{Label: "added", Header: "\n:rocket: Added"},
		}
		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.2.0"),
			Changes: changes,
		})
		Expect(err).To(BeNil())

		expected := `## v0.2.0

:rocket: Added
* z`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("can create new version file with custom kind change format", func() {
		vFile := NewMockFile(fs, "v0.1.0.md")

		fs.MockCreate = func(path string) (afero.File, error) {
			return vFile, nil
		}

		changes := []core.Change{
			{Body: "x", Kind: "added"},
		}

		testConfig.Kinds = []core.KindConfig{
			{Label: "added", ChangeFormat: "* added -> {{.Body}}"},
		}
		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: changes,
		})
		Expect(err).To(BeNil())

		expected := `## v0.1.0

### added
* added -> x`
		Expect(vFile.String()).To(Equal(expected))
	})

	It("returns error where when failing to create version file", func() {
		fs.MockCreate = func(path string) (afero.File, error) {
			var f afero.File
			return f, mockError
		}

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: []core.Change{},
		})
		Expect(err).To(Equal(mockError))
	})

	It("returns error when using bad version template", func() {
		testConfig.VersionFormat = "{{juuunk...}}"

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: []core.Change{},
		})
		Expect(err).NotTo(BeNil())
	})

	It("returns error when using bad kind template", func() {
		testConfig.KindFormat = "{{randoooom../././}}"

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: []core.Change{
				{Body: "x", Kind: "added"},
			},
		})
		Expect(err).NotTo(BeNil())
	})

	It("returns error when using bad component template", func() {
		testConfig.ComponentFormat = "{{deja vu}}"

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: []core.Change{
				{Component: "x", Kind: "added"},
			},
		})
		Expect(err).NotTo(BeNil())
	})

	It("returns error when using bad change template", func() {
		testConfig.ChangeFormat = "{{not.valid.syntax....}}"

		err := batchNewVersion(fs, testConfig, batchData{
			Version: semver.MustParse("v0.1.0"),
			Changes: []core.Change{
				{Body: "x", Kind: "added"},
			},
		})
		Expect(err).NotTo(BeNil())
	})

	templateTests := []struct {
		key    string
		prefix string
	}{
		{key: "version", prefix: "## "},
		{key: "component", prefix: "\n## "},
		{key: "kind", prefix: "\n### "},
		{key: "change", prefix: "* "},
	}

	for _, test := range templateTests {
		prefix := test.prefix
		It(fmt.Sprintf("returns error when failing to execute %s template", test.key), func() {
			vFile := NewMockFile(fs, "v0.1.0.md")

			fs.MockCreate = func(path string) (afero.File, error) {
				return vFile, nil
			}

			vFile.MockWrite = func(data []byte) (int, error) {
				if strings.HasPrefix(string(data), prefix) {
					return len(data), mockError
				}
				return vFile.MemFile.Write(data)
			}

			changes := []core.Change{
				{Component: "A", Kind: "added", Body: "w"},
				{Component: "A", Kind: "added", Body: "x"},
				{Component: "A", Kind: "removed", Body: "y"},
				{Component: "B", Kind: "removed", Body: "z"},
			}

			testConfig.ComponentFormat = "\n## {{.Component}}"
			testConfig.Components = []string{"A", "B"}
			err := batchNewVersion(fs, testConfig, batchData{
				Version: semver.MustParse("v0.1.0"),
				Changes: changes,
			})
			Expect(err).To(Equal(mockError))
		})
	}

	It("delete unreleased removes unreleased files including header", func() {
		err := afs.MkdirAll(futurePath, 0644)
		Expect(err).To(BeNil())

		for _, name := range []string{"a.yaml", "b.yaml", "c.yaml", ".gitkeep", "header.md"} {
			var f afero.File
			f, err = afs.Create(filepath.Join(futurePath, name))
			Expect(err).To(BeNil())

			Expect(f.Close()).To(Succeed())
		}

		err = deleteUnreleased(afs, testConfig, "header.md")
		Expect(err).To(BeNil())

		infos, err := afs.ReadDir(futurePath)
		Expect(err).To(BeNil())
		Expect(len(infos)).To(Equal(1))
	})

	It("delete unreleased fails if remove fails", func() {
		var err error

		for _, name := range []string{"a.yaml"} {
			var f afero.File
			f, err = afs.Create(filepath.Join(futurePath, name))
			Expect(err).To(BeNil())

			err = f.Close()
			Expect(err).To(BeNil())
		}

		fs.MockRemove = func(name string) error {
			return mockError
		}

		err = deleteUnreleased(afs, testConfig, "")
		Expect(err).To(Equal(mockError))
	})
})
