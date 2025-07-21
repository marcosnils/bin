# bin

Manages binary files downloaded from different sources

![bin](https://user-images.githubusercontent.com/1578458/87901619-ee629a80-ca2d-11ea-8609-8a8eb39801d2.gif)

## Rationale

`bin` started as an idea given the popularity of single binary releases due to the surge of  languages like
Go, Rust, Deno, etc which can easily produce dynamically and statically compiled binarines.

I found myself downloading binaries (or tarballs) directly from VCS (Github mostly) and then it was hard
to keep control and update such dependencies whenever a new version was released. So, with the objective
to solve that problem and also looking for something that will allow me to get the latest releases, I created `bin`.
In addition to that, I was also looking for something that doesn't require `sudo` or `root` privileges to install
these binaries as downloading, making them executable and storing it somewhere in your PATH would be sufficient.

After I finished the first MVP, a friend pointed out that [brew](https://brew.sh) was now supported in linux which almost
made me abandon the project. After checking out brew (never been an osx user), I found it a bit bloated and seems
to be doing way more than what I'm actually needing. So, I've decided to continue `bin` and hopefully add more features
that could result useful to someone else.

If you find `bin` helpful and you have any ideas or suggestions, please create an issue here or send a PR and I'll
be more than happy to brainstorm about possibilities.

Supported providers
* [Github](#Github)
* [Gitlab](#Gitlab)
* [Hashicorp](#Hashicorp)
* [Docker](#Docker)

## Installing

1. Download `bin` from the [releases](https://github.com/marcosnils/bin/releases)
2. Run `./bin install github.com/marcosnils/bin` so `bin` is managed by `bin` itself
3. Run `bin ls` to make sure bin has been installed correctly. You can now remove the first file you downloaded.
4. Enjoy!

## Usage

```shell
bin ensure # Ensures that all binaries listed in the configuration are present
bin help # Help about any command
bin install <repo> [path] # Downloads the latest binary and makes it executable
bin list # List current binaries and it's versions
bin prune # Removes from the DB missing binaries
bin remove <bin>... # Deletes one or more binaries
bin update [bin]... # Scans binaries and prompts for update
bin pin <bin>... # Pins current version of one or more binaries
bin unpin <bin>... # Unpins one or more binaries
```

## Supported providers

By default `bin` understand when you use github, gitlab or docker. But in some conditions you could specify the provider to use.

### Github

Github provider will use Github API to found releases matching your workstation specs.

**Supported optional environment variables**:

* `GITHUB_AUTH_TOKEN` set a token to avoid rate limit or if you need to download from private repo, see [FAQ](#FAQ)
* `GITHUB_TOKEN` deprecated variable, fallback if `GITHUB_AUTH_TOKEN` empty.
* `GHES_BASE_URL` [github enterprise](https://github.com/github/gh-es) base URL (often is your GitHub Enterprise hostname).
* `GHES_UPLOAD_URL` [github enterprise](https://github.com/github/gh-es) upload URL (often is your GitHub Enterprise hostname).
* `GHES_AUTH_TOKEN` [github enterprise](https://github.com/github/gh-es) auth token similar to `GITHUB_AUTH_TOKEN`

```bash
bin install github.com/kubernetes-sigs/kind # installs latest Kind release

bin install github.com/kubernetes-sigs/kind/releases/tag/v0.8.0 # installs a specific release

bin install github.com/kubernetes-sigs/kind ~/bin/kind # installs latest on a specific path
```

or explicit

```bash
bin install --provider github github.companyname.com/custom/repo
```

### Gitlab

Gitlab provider will use Gitlab API to found releases matching your workstation specs

**Mandatory environment variables**
* `GITLAB_TOKEN` now gitlab enforce token usage and don't have public api, you could setup a [personal access token](https://docs.gitlab.com/user/profile/personal_access_tokens/), a [GAT](https://docs.gitlab.com/user/group/settings/group_access_tokens/) or a [PAT](https://docs.gitlab.com/user/project/settings/project_access_tokens/)

```bash
bin install gitlab.com/gitlab-org/cli
```

or explicit

```bash
bin install --provider gitlab gitlab.companyname.com/custom/repo
```

### Hashicorp

Hashicorp have a [dedicated releases](https://releases.hashicorp.com) page and don't use github/lab releases, `bin` support it.

```bash
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.12.1
```

If you need multiple versions, specify a destination

```bash
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.5.7 ~/bin/terraform-1.5.7
bin install --provider hashicorp https://releases.hashicorp.com/terraform/1.12.1 ~/bin/terraform-1.12.1
```

### Docker

Docker is also supported or any Docker client compatible runtime.

You can install Docker images and use them as regular CLIs:

```shell
bin install docker://hashicorp/terraform:light # install the `light` tag for terraform

bin install docker://quay.io/calico/node # install the latest version of calico/node
```

**Supported optional environment variables**: any variable supported by Docker, see [https://docs.docker.com/reference/cli/docker/](https://docs.docker.com/reference/cli/docker/)

For other runtime (like Podman) or for remote docker engine, simply export `DOCKER_HOST` envvar:

```bash
export DOCKER_HOST="unix:///path/to/unix/socket" 
bin install docker://quay.io/calico/node
```

## FAQ

### Can you give some example tools

Yes. Following [list](https://github.com/marcosnils/bin/wiki/Tools-list)

### There are some bugs and the code is not tested

I know... and that's not planning to change any time soon unless I start getting some contributions. I did this as a personal tool and I'll probably be fixing stuff and adding features as I personally need them. Contributions are welcome though and I'll be happy to discuss and review them.

### I see releases on Github, but `bin` does not pick them up

At the moment, `bin` does only consider the [latest release from Github](https://docs.github.com/en/rest/reference/repos#get-the-latest-release) according to the following definition:

> The latest release is the most recent non-prerelease, non-draft release, sorted by the `created_at` attribute. The `created_at` attribute is the date of the commit used for the release, and not the date when the release was drafted or published.

You _can_ however install a specific pre-release by specifying the URL for the pre-release, e.g. `bin install https://github.com/bufbuild/buf/releases/tag/v0.40.0`.

### I used `bin` and I got rate limited by Github or want to access private repos, what can I do?

Create a Github personal access token by following the steps in this guide: [Creating a personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token). The access token used with `bin` does not need any scopes.

Create an environment variable named `GITHUB_AUTH_TOKEN` with the value of your newly created access token. For example in bash: `export GITHUB_AUTH_TOKEN=<your_token_value>`.
