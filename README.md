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
be more than happy to brainstrom about possibilities.

## Installing

1. Download `bin` from the [releases](https://github.com/marcosnils/bin/releases)
2. Run `./bin install github.com/marcosnils/bin` so `bin` is managed by `bin` itself
3. Run `bin ls` to make sure bin has been installed correctly. You can now remove the first file you downloaded.
4. Enjoy!

## Usage

Install a release from github with the following command:

```shell
bin install github.com/kubernetes-sigs/kind # installs latest Kind release

bin install github.com/kubernetes-sigs/kind/releases/tag/v0.8.0 # installs a specific release

bin install github.com/kubernetes-sigs/kind ~/bin/kind # installs latest on a specific path
```

You can install Docker images and use them as regular CLIs:

```shell
bin install docker://hashicorp/terraform:light # install the `light` tag for terraform

bin install docker://quay.io/calico/node # install the latest version of calico/node
```

```shell
bin ensure # Ensures that all binaries listed in the configuration are present
bin help # Help about any command
bin install <repo> [path] # Downloads the latest binary and makes it executable
bin list # List current binaries and it's versions
bin prune # Removes from the DB missing binaries
bin remove <bin>... # Deletes one or more binaries
bin update [bin]... # Scans binaries and prompts for update
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
