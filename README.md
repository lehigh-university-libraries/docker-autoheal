# docker autoheal

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

By default, every 10 seconds this service will check if any docker containers have exited or are unhealthy. It will then attempt to `docker restart` the unhealthy container(s).

### Webhook

If the `--webhook-url` is passed, a message will be sent to the webhook when the failure is detected. Once docker is healthy again an "All is well" message will be sent.

```
:white_check_mark: All is well
```

### Lock file

If you have a lock file sent when code changes are rolled out to your docker service via CI/CD, you can pass the path to the lockfile to this service. If the file exists, autoheal will not be attempted to avoid colliding with the rollout process.

## Install

Ideally this service runs on your host system (and not in a docker container). Mainly so if this service dies for some reason, systemd can restart it (since this service can not restart itself).

```
$ cat << EOF > /etc/systemd/system/docker-autoheal.service
[Unit]
Description=Monitor docker health
After=docker.service
StartLimitIntervalSec=120
StartLimitBurst=3

[Service]
EnvironmentFile=/path/to/.env
ExecStart=/usr/bin/docker-autoheal \
  --webhook-url "$SLACK_WEBHOOK" \
  --webhook-key "msg" \
  --lock-file "/path/to/rollout.lock"
Restart=on-failure
RestartSec=15s

[Install]
WantedBy=multi-user.target

$ systemctl enable docker-autoheal.service
$ systemctl start docker-autoheal.service
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
