package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/libdependency/github"
	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type Asset struct {
	BrowserDownloadUrl string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

func main() {
	retrieve.NewMetadataWithPlatforms("pnpm", getAllVersions, generateMetadataWithPlatform)
}

func generateMetadataWithPlatform(versionFetcher versionology.VersionFetcher, platform retrieve.Platform) ([]versionology.Dependency, error) {

	dependency, err := createDependencyVersionWithPlatform(versionFetcher, platform)
	if err != nil {
		return nil, fmt.Errorf("could not create pnpm version: %w", err)
	}

	return []versionology.Dependency{{
		ConfigMetadataDependency: dependency,
		SemverVersion:            versionFetcher.Version(),
		Target:                   "bionic",
	}}, nil
}

func getAllVersions() (versionology.VersionFetcherArray, error) {
	return github.GetAllVersions(os.Getenv("GITHUB_TOKEN"), "pnpm", "pnpm")()
}

func createDependencyVersionWithPlatform(versionFetcher versionology.VersionFetcher, platform retrieve.Platform) (cargo.ConfigMetadataDependency, error) {
	webClient := NewWebClient()
	githubClient := NewGithubClient(webClient)

	version := versionFetcher.Version().String()
	tagName := versionFetcher.Version().Original()

	releaseAssetDir, err := os.MkdirTemp("", "pnpm")
	if err != nil {
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {

		}
	}(releaseAssetDir)

	releaseAssetPath := filepath.Join(releaseAssetDir, fmt.Sprintf("pnpm-%s.tar.gz", tagName))

	arch, err := archName(platform)
	if err != nil {
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("failed to find architecture name: %w", err)
	}

	assetName := fmt.Sprintf("pnpm-%s-%s", platform.OS, arch)
	assetUrl, err := githubClient.DownloadReleaseAsset("pnpm", "pnpm", tagName, assetName, releaseAssetPath)
	if err != nil {
		if errors.Is(err, AssetNotFound{AssetName: assetName}) {
			return cargo.ConfigMetadataDependency{}, NoSourceCodeError{Version: version}
		}
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("could not download asset url: %w", err)
	}

	assetContent, err := webClient.Get(assetUrl)
	if err != nil {
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("could not get asset content from asset url: %w", err)
	}

	asset := Asset{}
	err = json.Unmarshal(assetContent, &asset)
	if err != nil {
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("could not unmarshal asset url content: %w", err)
	}

	var dependencySHA string
	if asset.Digest != "" {
		dependencySHA = asset.Digest
	}

	return cargo.ConfigMetadataDependency{
		Arch:            platform.Arch,
		CPE:             fmt.Sprintf("cpe:2.3:a:pnpm:pnpm:%s:*:*:*:*:*:*:*", version),
		Checksum:        dependencySHA,
		ID:              "pnpm",
		Licenses:        []interface{}{"MIT"},
		Name:            "pnpm",
		OS:              platform.OS,
		PURL:            retrieve.GeneratePURL("pnpm", version, dependencySHA, asset.BrowserDownloadUrl),
		Source:          asset.BrowserDownloadUrl,
		SourceChecksum:  dependencySHA,
		Stacks:          []string{"*"},
		URI:             asset.BrowserDownloadUrl,
		Version:         version,
		DeprecationDate: nil,
		StripComponents: 1,
	}, nil
}

func archName(platform retrieve.Platform) (string, error) {
	switch platform.Arch {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", errors.New("not supported architecture")
	}
}
