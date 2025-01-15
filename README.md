# docker autohealh

Command line utility to automatically restart docker containers that are unhealthy or stopped/exited.

## Install

### Homebrew

You can install docker-autoheal using homebrew

```
brew tap lehigh-university-libraries/homebrew https://github.com/lehigh-university-libraries/homebrew
brew install lehigh-university-libraries/homebrew/docker-autoheal
```

### Download Binary

Instead of homebrew, you can download a binary for your system from [the latest release](https://github.com/lehigh-university-libraries/docker-autoheal/releases/latest)

Then put the binary in a directory that is in your `$PATH`

## Usage

```
$ docker-autoheal --help 
Usage of ./docker-autoheal:
  -interval duration
        frequency interval (default 10s)
  -lock-file string
        lock file that when exists halts docker autohealh
  -webhook-key string
        JSON key name to add webhook messages (default "text")
  -webhook-url string
        Send messages to a webhook
```

## Updating

### Homebrew

If homebrew was used, you can simply upgrade the homebrew formulae for docker-autoheal

```
brew update && brew upgrade docker-autoheal
```

### Download Binary

If the binary was downloaded and added to the `$PATH` updating docker-autoheal could look as follows. Requires [gh](https://cli.github.com/manual/installation) and `tar`

```
# update for your architecture
ARCH="docker-autoheal_Linux_x86_64.tar.gz"
TAG=$(gh release list --exclude-pre-releases --exclude-drafts --limit 1 --repo lehigh-university-libraries/docker-autoheal | awk '{print $3}')
gh release download $TAG --repo lehigh-university-libraries/docker-autoheal --pattern $ARCH
tar -zxvf $ARCH
mv docker-autoheal /directory/in/path/binary/was/placed
rm $ARCH
```
