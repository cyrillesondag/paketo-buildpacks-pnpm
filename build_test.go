package pnpm_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/pnpm"
	"github.com/paketo-buildpacks/pnpm/fakes"
	"github.com/sclevine/spec"

	//nolint Ignore SA1019, informed usage of deprecated package
	"github.com/paketo-buildpacks/packit/v2/paketosbom"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir         string
		workingDir        string
		cnbDir            string
		dependencyManager *fakes.DependencyManager
		sbomGenerator     *fakes.SBOMGenerator

		buffer *bytes.Buffer

		buildContext packit.BuildContext
		build        packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		dependencyManager = &fakes.DependencyManager{}
		dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
			ID:       "pnpm",
			Name:     "pnpm-dependency-name",
			Checksum: "sha256:pnpm-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "pnpm-dependency-uri",
			Version:  "pnpm-dependency-version",
		}
		dependencyManager.GenerateBillOfMaterialsCall.Returns.BOMEntrySlice = []packit.BOMEntry{
			{
				Name: "pnpm",
				Metadata: paketosbom.BOMMetadata{
					URI:     "pnpm-dependency-uri",
					Version: "pnpm-dependency-version",
					Checksum: paketosbom.BOMChecksum{
						Algorithm: paketosbom.SHA256,
						Hash:      "pnpm-dependency-sha",
					},
				},
			},
		}

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateFromDependencyCall.Returns.SBOM = sbom.SBOM{}

		buffer = bytes.NewBuffer(nil)

		buildContext = packit.BuildContext{
			WorkingDir: workingDir,
			CNBPath:    cnbDir,
			Stack:      "some-stack",
			BuildpackInfo: packit.Info{
				Name:        "Some Buildpack",
				Version:     "some-version",
				SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
			},
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name: "pnpm",
					},
				},
			},
			Platform: packit.Platform{Path: "platform"},
			Layers:   packit.Layers{Path: layersDir},
		}

		build = pnpm.Build(dependencyManager,
			sbomGenerator,
			chronos.DefaultClock,
			scribe.NewEmitter(buffer))
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	it("returns a result that installs pnpm", func() {
		result, err := build(buildContext)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(1))
		layer := result.Layers[0]

		Expect(layer.Name).To(Equal("pnpm"))
		Expect(layer.Path).To(Equal(filepath.Join(layersDir, "pnpm")))
		Expect(layer.Metadata).To(Equal(map[string]interface{}{
			pnpm.DependencyCacheKey: "sha256:pnpm-dependency-sha",
		}))

		Expect(layer.SBOM.Formats()).To(HaveLen(2))

		cdx := layer.SBOM.Formats()[0]
		spdx := layer.SBOM.Formats()[1]

		Expect(cdx.Extension).To(Equal("cdx.json"))

		content, err := io.ReadAll(cdx.Content)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(MatchJSON(`{
			"$schema": "http://cyclonedx.org/schema/bom-1.3.schema.json",
			"bomFormat": "CycloneDX",
			"metadata": {
				"tools": [
					{
						"name": "",
						"vendor": "anchore"
					}
				]
			},
			"specVersion": "1.3",
			"version": 1
		}`))

		Expect(spdx.Extension).To(Equal("spdx.json"))
		content, err = io.ReadAll(spdx.Content)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(MatchJSON(`{
			"SPDXID": "SPDXRef-DOCUMENT",
			"creationInfo": {
				"created": "0001-01-01T00:00:00Z",
				"creators": [
					"Organization: Anchore, Inc",
					"Tool: -"
				],
				"licenseListVersion": "3.25"
			},
			"dataLicense": "CC0-1.0",
			"documentNamespace": "https://paketo.io/unknown-source-type/unknown-9ecf240a-d971-5a3c-8e7b-6d3f3ea4d9c2",
			"name": "unknown",
			"packages": [
				{
					"SPDXID": "SPDXRef-DocumentRoot-Unknown-",
					"copyrightText": "NOASSERTION",
					"downloadLocation": "NOASSERTION",
					"filesAnalyzed": false,
					"licenseConcluded": "NOASSERTION",
					"licenseDeclared": "NOASSERTION",
					"name": "",
					"supplier": "NOASSERTION"
				}
			],
			"relationships": [
				{
					"relatedSpdxElement": "SPDXRef-DocumentRoot-Unknown-",
					"relationshipType": "DESCRIBES",
					"spdxElementId": "SPDXRef-DOCUMENT"
				}
			],
			"spdxVersion": "SPDX-2.2"
		}`))

		Expect(dependencyManager.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
		Expect(dependencyManager.ResolveCall.Receives.Id).To(Equal("pnpm"))
		Expect(dependencyManager.ResolveCall.Receives.Stack).To(Equal("some-stack"))

		Expect(dependencyManager.DeliverCall.Receives.Dependency).To(Equal(postal.Dependency{
			ID:       "pnpm",
			Name:     "pnpm-dependency-name",
			Checksum: "sha256:pnpm-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "pnpm-dependency-uri",
			Version:  "pnpm-dependency-version",
		}))
		Expect(dependencyManager.DeliverCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyManager.DeliverCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "pnpm")))
		Expect(dependencyManager.DeliverCall.Receives.PlatformPath).To(Equal("platform"))

		// Legacy SBOM
		Expect(dependencyManager.GenerateBillOfMaterialsCall.Receives.Dependencies).To(Equal([]postal.Dependency{{
			ID:       "pnpm",
			Name:     "pnpm-dependency-name",
			Checksum: "sha256:pnpm-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "pnpm-dependency-uri",
			Version:  "pnpm-dependency-version",
		},
		}))

		Expect(sbomGenerator.GenerateFromDependencyCall.Receives.Dependency).To(Equal(postal.Dependency{
			ID:       "pnpm",
			Name:     "pnpm-dependency-name",
			Checksum: "sha256:pnpm-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "pnpm-dependency-uri",
			Version:  "pnpm-dependency-version",
		}))
		Expect(sbomGenerator.GenerateFromDependencyCall.Receives.Dir).To(Equal(layer.Path))

		Expect(buffer.String()).To(ContainSubstring("Some Buildpack some-version"))
		Expect(buffer.String()).To(ContainSubstring("Executing build process"))
		Expect(buffer.String()).To(ContainSubstring("Installing PNPM"))
	})

	context("when the plan entry requires the dependency during the build and launch phases", func() {
		it.Before(func() {
			buildContext.Plan.Entries[0].Metadata = map[string]interface{}{
				"build":  true,
				"launch": true,
			}
		})

		it("makes the layer available in those phases", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			layer := result.Layers[0]

			Expect(layer.Name).To(Equal("pnpm"))
			Expect(layer.Path).To(Equal(filepath.Join(layersDir, "pnpm")))
			Expect(layer.Build).To(BeTrue())
			Expect(layer.Launch).To(BeTrue())
			Expect(layer.Cache).To(BeTrue())
			Expect(layer.Metadata).To(Equal(map[string]interface{}{
				pnpm.DependencyCacheKey: "sha256:pnpm-dependency-sha",
			}))
		})
	})

	context("failure cases", func() {
		context("when the pnpm layer cannot be retrieved", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(layersDir, "pnpm.toml"), nil, 0000)
				Expect(err).NotTo(HaveOccurred())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata")))
			})
		})

		context("when the dependency cannot be resolved", func() {
			it.Before(func() {
				dependencyManager.ResolveCall.Returns.Error = errors.New("failed to resolve dependency")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError("failed to resolve dependency"))
			})
		})

		context("when the layers directory cannot be written to", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 4444)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the dependency cannot be installed", func() {
			it.Before(func() {
				dependencyManager.DeliverCall.Returns.Error = errors.New("failed to install dependency")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError("failed to install dependency"))
			})
		})

		context("when generating the SBOM returns an error", func() {
			it.Before(func() {
				buildContext.BuildpackInfo = packit.BuildpackInfo{SBOMFormats: []string{"random-format"}}
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError("unsupported SBOM format: 'random-format'"))
			})
		})

		context("when formatting the SBOM returns an error", func() {
			it.Before(func() {
				sbomGenerator.GenerateFromDependencyCall.Returns.Error = errors.New("failed to generate SBOM")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to generate SBOM")))
			})
		})

		context("when BP_DISABLE_SBOM is set incorrectly", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_DISABLE_SBOM", "not-a-bool")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_DISABLE_SBOM")).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to parse BP_DISABLE_SBOM")))
			})
		})
	})
}
