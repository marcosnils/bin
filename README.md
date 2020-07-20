# bin
Manages bin files downloaded from different sources

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

```
bin install github.com/kubernetes-sigs/kind # installs latest Kind release

bin install github.com/kubernetes-sigs/kind/releases/tag/v0.8.0 # installs a specific release

bin install github.com/kubernetes-sigs/kind ~/bin/kind # installs latest on a specific path 
```

You can install Docker images and use them as regular CLIs:

```
bin install docker://hashicorp/terraform:light # install the `light` tag for terraform

bin install docker://quay.io/calico/node # install the latest version of calico/node
```

```
bin install <repo> [path] # Downloads the latest binary and makes it executable 
bin update [bin]... # Scans binaries and prompts for update
bin ls # List current binaries and it's versions
bin remove <bin>... # Deletes one or more binaries
bin purge # Removes from the DB missing binaries
```

## FAQ

### There are some bugs and the code is not tested. 

I know.. and that's not planning to change any time soon unless I start getting some contributions. I did this as a personal tool and I'll probably be fixing stuff and adding features as I personally need them. Contribution are welcome though and I'll be happy to discuss and review them. 


