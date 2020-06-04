# bin
Manages bin files downloaded from different sources

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


## Usage

Install a realse from github with the following command:

```
bin install github.com/kubernetes-sigs/kind # installs latest Kind release

bin install github.com/kubernetes-sigs/kind/releases/tag/v0.8.0 # installs a specific release

bin install github.com/kubernetes-sigs/kind ~/bin/kind # installs latest on a specific path 
```

```
bin install <repo> [path] # Downloads the latest binary and makes it executable 
bin update [bin]... # Scans binaries and prompts for update
bin ls # List current binaries and it's versions
bin remove <bin>... # Deletes one or more binaries
bin purge # Removes from the DB missing binaries
```
