
# bin

Manages binary files downloaded from different sources

![bin](https://user-images.githubusercontent.com/1578458/87901619-ee629a80-ca2d-11ea-8609-8a8eb39801d2.gif)

The original utility is introduced by [Marcos Nils](https://github.com/marcosnils/bin)
You are invited to visit his [GitHub page](https://github.com/marcosnils/bin) to read the full documentation.

The repository enables the bin utility to be script-able and avoid requiring TTY user feedback.

There are 2 minor updates:
1. Introduction of **BIN_EXE_DIR**

Previously, the installation/download directory is derived from the `PATH`, when it's unable to do so, the `bin` utility 
demands user feedback. In some cases it's useful to be able to shortcut this process and directly provide it with the installation directory using
the `BIN_EXE_DIR`

```shell
# install `navi` in a specific designated directory, update the bin configuration in the same directory
$ BIN_EXE_DIR=~/.local/bin bin install github.com/denisidoro/navi
```

**Note:** while the original `bin` utility allows for installation at a specific directory

```shell
# Explicit install @ ~/.local/bin
$ bin install github.com/denisidoro/navi  ~/.local/bin
```

The behavior differs. Without `BIN_EXE_DIR` when the initial config is missing, the user will be requested to provide the installation directory.

2. Introduction of a command line file option selection (**--select** | **-s** ) 

In some cases the release may contain multiple files and user intervention is required to choose which one to install. The **select** option was introduced to allow 
the user to specify the exact file to install even when it's inside an archive (tar, xz, etc).

```shell
SYNTAX:
   bin install <URL> [ --select | -s  FileName[:ContainedFile] ]
```

```shell
# Install the file yq_linux_amd64
$ bin install github.com/mikefarah/yq --select yq_linux_amd64

# Install broot selecting for the file x86_64-linux that is in the only tar ball existing in the release
$ bin install github.com/Canop/broot --select :x86_64-linux/broot

# Install the file age/age (age inside a directory with the same name) from the only tarball in the release
$ bin install github.com/filosottile/age -s :age/age

# Install btm file from the tarball bottom_x86_64-unknown-linux-gnu.tar.gz (select both the tarball and a file)
$ bin install github.com/ClementTsang/bottom -s bottom_x86_64-unknown-linux-gnu.tar.gz:btm

```

While it is possible to select partially, IMHO it defaults my original purpose of avoiding any TTY interaction, but why not?!

```shell
# No Selection output 
$ bin install github.com/ClementTsang/bottom  
   • Getting latest release for ClementTsang/bottom

Multiple matches found, please select one:

 [1] bottom_x86_64-unknown-linux-gnu.tar.gz
 [2] bottom_x86_64-unknown-linux-gnu2-17.tar.gz
 [3] bottom_x86_64-unknown-linux-musl.tar.gz
 Select an option: 1
   • Starting download of https://api.github.com/repos/ClementTsang/bottom/releases/assets/123278270
1.85 MiB / 1.85 MiB [------------------------------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% 19.58 MiB p/s 0s

Multiple matches found, please select one:

 [1] btm
 [2] completion/_btm
 [3] completion/_btm.ps1
 [4] completion/btm.bash
 [5] completion/btm.elv
 [6] completion/btm.fish
 Select an option: 
```


```shell
# Partial selection output
$ bin install github.com/ClementTsang/bottom  -s bottom_x86_64-unknown-linux-gnu.tar.gz
   • Getting latest release for ClementTsang/bottom
   • Starting download of https://api.github.com/repos/ClementTsang/bottom/releases/assets/123278270
1.85 MiB / 1.85 MiB [------------------------------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% 22.72 MiB p/s 0s

Multiple matches found, please select one:

 [1] btm
 [2] completion/_btm
 [3] completion/_btm.ps1
 [4] completion/btm.bash
 [5] completion/btm.elv
 [6] completion/btm.fish
 Select an option: 
 ```

```shell
# Full selection output
$ bin install github.com/ClementTsang/bottom  -s bottom_x86_64-unknown-linux-gnu.tar.gz:btm
   • Getting latest release for ClementTsang/bottom
   • Starting download of https://api.github.com/repos/ClementTsang/bottom/releases/assets/123278270
1.85 MiB / 1.85 MiB [------------------------------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% 20.29 MiB p/s 0s
   • Copying for btm@0.9.6 into /home/vscode/.local/bin/btm
   • Done installing btm 0.9.6
```