# bin - Effortless Binary Manager

[![GitHub release](https://img.shields.io/github/release/marcosnils/bin.svg)](https://github.com/marcosnils/bin/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/marcosnils/bin)](https://goreportcard.com/report/github.com/marcosnils/bin)
[![License](https://img.shields.io/github/license/marcosnils/bin.svg)](https://github.com/marcosnils/bin/blob/main/LICENSE)

A lightweight, cross-platform binary manager that simplifies downloading, installing, and managing binaries without requiring root privileges.

![bin](https://user-images.githubusercontent.com/1578458/87901619-ee629a80-ca2d-11ea-8609-8a8eb39801d2.gif)

## 🚀 Why bin?

Modern development tools are increasingly distributed as single binary releases thanks to languages like Go, Rust, and Deno.
While this makes distribution easier, it creates challenges for updates and tracking.

`bin` solves these problems by:

- **Zero Configuration**: Works out of the box without complex setup, scoring & filtering found the right package
- **No Root Required**: Install binaries to user directories without `sudo`
- **Version Management**: Track, update, and rollback binary versions
- **Lightweight**: Minimal overhead compared to manually download and install releases
- **Multiple Sources**:
  - [GitHub Releases](#github-releases)
  - [Gitlab Releases](#gitlab-releases)
  - [Codeberg Releases](#codeberg-releases)
  - [Docker Images](#docker-images)
  - [Hashicorp Releases](#hashicorp-releases)
  - [Go Install](#go-install)

For a comprehensive list, see the [Tools Wiki](https://github.com/marcosnils/bin/wiki/Tools-list).

## 📦 Installation

### Quick Install

1. Download `bin` from the [releases](https://github.com/marcosnils/bin/releases)
2. Run `./bin install github.com/marcosnils/bin` so `bin` is managed by `bin` itself
3. Run `bin ls` to make sure bin has been installed correctly. You can now remove the first file you downloaded.
4. Enjoy!

### Quick install from Scoop (Windows)

Run these commands to install `bin` from `scoop`.

```bash
scoop bucket add extras
scoop install extras/bin
```

## 📚 Commands Reference

| Command                     | Description                                | Example                          |
| --------------------------- | ------------------------------------------ | -------------------------------- |
| `bin install <repo> [path]` | Install binary from GitHub or Docker       | `bin install github.com/cli/cli` |
| `bin list`                  | List installed binaries and versions       | `bin list`                       |
| `bin update [binary...]`    | Update binaries (all or specified)         | `bin update`                     |
| `bin remove <binary...>`    | Remove one or more binaries                | `bin remove gh kubectl`          |
| `bin ensure`                | Ensure all configured binaries are present | `bin ensure`                     |
| `bin pin <binary...>`       | Pin current version (prevent updates)      | `bin pin terraform`              |
| `bin unpin <binary...>`     | Unpin binaries (allow updates)             | `bin unpin terraform`            |
| `bin prune`                 | Remove missing binaries from database      | `bin prune`                      |
| `bin help`                  | Show help for any command                  | `bin help install`               |

**Tips**: if `bin` is unable to found the right package, try `bin install -a` to show all possible download options (skip scoring & filtering).

## 🎯 Supported providers

### GitHub Releases

Github provider will use Github API to found releases matching your workstation specs.

At the moment, `bin` does only consider the [latest release from Github](https://docs.github.com/en/rest/reference/repos#get-the-latest-release) according to the following definition:

> The latest release is the most recent non-prerelease, non-draft release, sorted by the `created_at` attribute. The `created_at` attribute is the date of the commit used for the release, and not the date when the release was drafted or published.

You _can_ however install a specific pre-release by specifying the URL for the pre-release, e.g. `bin install https://github.com/bufbuild/buf/releases/tag/v0.40.0`.

#### Configuration

| Environment Variable | Mandatory | Description                                                                                                                                                                                                                                |
| -------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GITHUB_AUTH_TOKEN`  | no        | set a [token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token). The access token used with `bin` does not need any scopes to avoid rate limit or if you need to download from private repo\*\* |
| `GHES_BASE_URL`      | no        | [github enterprise](https://github.com/github/gh-es) base URL (often is your GitHub Enterprise hostname).                                                                                                                                  |
| `GHES_UPLOAD_URL`    | no        | [github enterprise](https://github.com/github/gh-es) upload URL (often is your GitHub Enterprise hostname).                                                                                                                                |
| `GHES_AUTH_TOKEN`    | no        | [github enterprise](https://github.com/github/gh-es) auth token similar to `GITHUB_AUTH_TOKEN`.                                                                                                                                            |

#### Usage

```shell
# installs latest Kind release
bin install github.com/kubernetes-sigs/kind

# installs a specific release
bin install github.com/kubernetes-sigs/kind/releases/tag/v0.8.0

# installs latest on a specific path
bin install github.com/kubernetes-sigs/kind ~/bin/kind

# installs latest on a specific path and show all possible download options (skip scoring & filtering)
bin install -a github.com/yt-dlp/yt-dlp ~/bin/kind
```

or explicit

```shell
bin install --provider github github.companyname.com/custom/repo
```

### Gitlab Releases

Gitlab provider will use Gitlab API to found releases matching your workstation specs

#### Configuration

| Environment Variable | Mandatory | Description                                                                                                                                                                                                                                                                                                                   |
| -------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GITLAB_TOKEN`       | yes       | now gitlab enforce token usage and don't have public api, you could setup a [personal access token](https://docs.gitlab.com/user/profile/personal_access_tokens/), a [GAT](https://docs.gitlab.com/user/group/settings/group_access_tokens/) or a [PAT](https://docs.gitlab.com/user/project/settings/project_access_tokens/) |

#### Usage

```shell
bin install gitlab.com/gitlab-org/cli
```

or explicit

```shell
bin install --provider gitlab gitlab.companyname.com/custom/repo
```

### Codeberg Releases

Codeberg provider uses the Gitea/Forgejo API (GitHub-compatible) to find releases matching your workstation specs. Codeberg is a free and open-source alternative to GitHub, hosted at [codeberg.org](https://codeberg.org).

#### Configuration

| Environment Variable | Mandatory | Description                                                                                                               |
| -------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------- |
| `CODEBERG_TOKEN`     | no        | set a [token](https://docs.codeberg.org/advanced/access-token/) for authentication. Useful for rate limiting or private repos |

#### Usage

```shell
# installs latest mergiraf release
bin install codeberg.org/mergiraf/mergiraf

# installs a specific release
bin install codeberg.org/mergiraf/mergiraf/releases/tag/v1.0.0

# installs latest on a specific path
bin install codeberg.org/mergiraf/mergiraf ~/bin/mergiraf
```

or explicit

```shell
bin install --provider codeberg codeberg.org/custom/repo
```

### Docker Images

Docker is also supported or any Docker client compatible runtime.

#### Configuration

Any variable supported by Docker, see [https://docs.docker.com/reference/cli/docker/](https://docs.docker.com/reference/cli/docker/)

#### Usage

```shell
# install the `light` tag for terraform
bin install docker://hashicorp/terraform:light

# install the latest version of calico/node
bin install docker://quay.io/calico/node
```

For other runtime (like Podman) or for remote docker engine, simply export `DOCKER_HOST` envvar:

```shell
export DOCKER_HOST="unix:///path/to/unix/socket"
bin install docker://quay.io/calico/node
```

### Hashicorp Releases

#### Configuration

None.

#### Usage

Hashicorp have a [dedicated releases](https://releases.hashicorp.com) page and don't use github/lab releases, `bin` support it.

```shell
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.12.1
```

If you need multiple versions, specify a destination

```shell
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.5.7 ~/bin/terraform-1.5.7
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.12.1 ~/bin/terraform-1.12.1
```

### Go Install

#### Configuration

Ensure `go` is present in your `PATH`.

#### Usage

`bin` will run go install, and copy the file from `GOPATH` to your dest.

```shell
bin install goinstall://github.com/jrhouston/tfk8s@v0.1.8
```

## 🔧 Configuration

### Configuration file

`bin` maintains a configuration file to track installed binaries.

#### Linux/MacOS

Path to the configuration directory respects the `XDG Base Directory specification` using the following strategy:

- Honor `BIN_CONFIG` is set
- To prevent breaking of existing configurations, check if `$HOME/.bin/config.json` exists and return `$HOME/.bin`
- If `XDG_CONFIG_HOME` is set, return `$XDG_CONFIG_HOME/bin`
- If `$HOME/.config` exists, return `$home/.config/bin`
- Default to `$HOME/.bin/`

#### Windows

Same than linux but uses `%USERPROFILE%` without `XDG_CONFIG_HOME`.

### Binary Storage

By default, `bin` stores binaries in:

- **Linux/macOS**: `~/.local/bin/`
- **Windows**: `%LOCALAPPDATA%\bin\`

Ensure this directory is in your `$PATH`.

## 🤝 Contributing

There are some bugs and the code is not tested by lake of time but contributions are welcome though and I'll be happy to discuss and review them.

- Report bugs or request features via [GitHub Issues](https://github.com/marcosnils/bin/issues)
- Submit pull requests for improvements
- Update documentation

### Development Setup

```shell
# Clone the repository
git clone https://github.com/marcosnils/bin.git
cd bin

# Clean and init
make clean download

# Run syntax check
make lint verify

# Run tests
make test

# Build from source
make build
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📋 Rationale

`bin` started as an idea given the popularity of single binary releases due to the surge of languages like Go, Rust, Deno, etc which can easily produce dynamically and statically compiled binaries.

I found myself downloading binaries (or tarballs) directly from VCS (Github mostly) and then it was hard to keep control and update such dependencies whenever a new version was released. So, with the objective to solve that problem and also looking for something that will allow me to get the latest releases, I created `bin`.

In addition to that, I was also looking for something that doesn't require `sudo` or `root` privileges to install these binaries as downloading, making them executable and storing it somewhere in your PATH would be sufficient.

After I finished the first MVP, a friend pointed out that [brew](https://brew.sh) was now supported in linux which almost made me abandon the project. After checking out brew (never been an osx user), I found it a bit bloated and seems to be doing way more than what I'm actually needing. So, I've decided to continue `bin` and hopefully add more features that could result useful to someone else.
