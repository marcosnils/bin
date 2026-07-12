package providers

import (
	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/assets"
)

// httpSource is a release source that publishes binaries on a plain HTTP
// endpoint (e.g. releases.hashicorp.com, get.helm.sh). It only resolves
// versions and enumerates downloadable assets; the shared asset-selection
// and download flow lives in httpReleaseProvider.
type httpSource interface {
	fetchRelease(version string) (string, []*assets.Asset, error)
	latestVersion() (string, string, error)
}

// httpReleaseProvider implements Provider on top of an httpSource, handling
// the flow shared by all HTTP release sources: version selection, asset
// scoring and archive processing.
type httpReleaseProvider struct {
	id  string
	tag string
	src httpSource
}

func (p *httpReleaseProvider) GetID() string {
	return p.id
}

func (p *httpReleaseProvider) Fetch(opts *FetchOpts) (*File, error) {
	version := p.tag
	if len(opts.Version) > 0 {
		// this is used by the `ensure` command
		version = opts.Version
	}

	if version == "" {
		log.Infof("Getting latest release for %s", p.id)
	} else {
		log.Infof("Getting %s release for %s", version, p.id)
	}

	version, candidates, err := p.src.fetchRelease(version)
	if err != nil {
		return nil, err
	}

	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck, PackageName: opts.PackageName, NamePattern: opts.NamePattern, PreferredAsset: opts.PreviousAsset, PreferredVersion: opts.PreviousVersion, CurrentVersion: version})

	gf, err := f.FilterAssets(p.id, candidates)
	if err != nil {
		return nil, err
	}

	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	// TODO calculate file hash. Not sure if we can / should do it here
	// since we don't want to read the file unnecessarily. Additionally, sometimes
	// releases have .sha256 files, so it'd be nice to check for those also
	return &File{Data: outFile.Source, Name: outFile.Name, Version: version, PackagePath: outFile.PackagePath, SelectedAsset: gf.Name}, nil
}

func (p *httpReleaseProvider) GetLatestVersion() (string, string, error) {
	return p.src.latestVersion()
}
