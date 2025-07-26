package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/joshuatcasey/libdependency/retrieve"
	"github.com/joshuatcasey/libdependency/upstream"
	"github.com/joshuatcasey/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type Asset struct {
	BrowserDownloadUrl string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type PnpmMetadata struct {
	SemverVersion *semver.Version
}

func (pnpmMetadata PnpmMetadata) Version() *semver.Version {
	return pnpmMetadata.SemverVersion
}

func main() {
	retrieve.NewMetadata("pnpm", getAllVersions, generateMetadata)
}

func generateMetadata(versionFetcher versionology.VersionFetcher) ([]versionology.Dependency, error) {
	version := versionFetcher.Version().String()
	releases, err := NewGithubClient(NewWebClient()).GetReleaseTags("pnpm", "pnpm")
	if err != nil {
		return nil, fmt.Errorf("could not get releases: %w", err)
	}

	for _, release := range releases {
		tagName := "v" + version
		if release.TagName == tagName {
			dependency, err := createDependencyVersion(version, tagName)
			if err != nil {
				return nil, fmt.Errorf("could not create pnpm version: %w", err)
			}

			return []versionology.Dependency{{
				ConfigMetadataDependency: dependency,
				SemverVersion:            versionFetcher.Version(),
			}}, nil
		}
	}

	return nil, fmt.Errorf("could not find pnpm version %s", version)
}

func getAllVersions() (versionology.VersionFetcherArray, error) {
	githubClient := NewGithubClient(NewWebClient())
	releases, err := githubClient.GetReleaseTags("pnpm", "pnpm")
	if err != nil {
		return nil, fmt.Errorf("could not get releases: %w", err)
	}

	var versions []versionology.VersionFetcher
	for _, release := range releases {
		versionTagName := strings.TrimPrefix(release.TagName, "v")
		version, err := semver.NewVersion(versionTagName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version: %w", err)
		}
		/** Versions less than 5.18.10 does not provide assets **/
		if version.LessThan(semver.MustParse("5.18.10")) {
			continue
		}
		if version.Prerelease() != "" {
			continue
		}

		versions = append(versions, PnpmMetadata{version})
	}

	return versions, nil

}

func createDependencyVersion(version, tagName string) (cargo.ConfigMetadataDependency, error) {
	webClient := NewWebClient()
	githubClient := NewGithubClient(webClient)

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

	assetName := "pnpm-linux-x64"
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
		CPE:             fmt.Sprintf("cpe:2.3:a:pnpm:pnpm:%s:*:*:*:*:*:*:*", version),
		Checksum:        dependencySHA,
		ID:              "pnpm",
		Licenses:        retrieve.LookupLicenses(asset.BrowserDownloadUrl, upstream.DefaultDecompress),
		Name:            "pnpm",
		PURL:            retrieve.GeneratePURL("pnpm", version, dependencySHA, asset.BrowserDownloadUrl),
		Source:          asset.BrowserDownloadUrl,
		SourceChecksum:  dependencySHA,
		Stacks:          []string{"io.buildpacks.stacks.bionic", "io.buildpacks.stacks.jammy"},
		URI:             asset.BrowserDownloadUrl,
		Version:         version,
		DeprecationDate: nil,
		StripComponents: 1,
	}, nil
}
