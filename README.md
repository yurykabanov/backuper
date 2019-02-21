# Backuper

This tool is designed to perform automated generic backups using docker
containers. It is a quite simple scheduler of backup tasks represented by
docker containers running specified commands.

## Requirements

1. [Docker](https://www.docker.com/).

## Scheme of work

Backuper will run commands defined in config file using cron-like scheduler
without task overlapping. As soon as command successfully dumps whatever
it should, these files are moved to target directory (usually it would be
mounted external storage).

## Quickstart

For example, lets configure backups for MySQL database every hour (not very
practical though).

1. Prepare two directories: temporary `/srv/backuper/tmp` (where unfinished
backups will be stored) and target `/srv/backups/mysql` (where successfully
finished backups will be stored)
2. Adjust `./config/backuper.example.yml` for your setup. You may want to
change:
    - docker socket, usually for localhost default unix socket at
    `/var/run/docker.sock` can be used
    - Cron spec &mdash; how often backup task should start. Default cron
    definition can be used or `@every {time.Duration}`
    - Timeout &mdash; how long could backup task execute.
    - Preserve at most &mdash; how many backups to store
    - Command &mdash; usually you want to specify correct user/pass
3. Run the backuper itself (you probably want to adjust it for your needs):
```bash
docker run                                                          \
    --rm                                                            \
    --net=host                                                      \
    --name backuper                                                 \
    -v /srv/backuper/db:/srv/backuper/db                            \
    -v /srv/backuper/config/backuper.yml:/etc/backuper/backuper.yml \
    -v /var/run/docker.sock:/var/run/docker.sock                    \
    -v /srv/backuper/tmp:/srv/backuper/tmp                          \
    -v /srv/backups/mysql:/srv/backups/mysql                        \
    backuper
```

## Configuration

Backuper utilizes [Viper](https://github.com/spf13/viper) which provides wide
variety of ways to configure application:
- it supports JSON, TOML, YAML, HCL or Java properties formats
- it provides ability to use environment variables, command line flags and
configuration files

Viper will use the following [precedence](https://github.com/spf13/viper#why-viper):
- flags
- environment variables
- configuration file
- defaults

### Command line flags

Read help using:
```bash
./backuper -h
```

### Environment variables

Environment variables use the following naming convention: any flag with name
`some.flag.value` corresponds to `BACKUPER_SOME_FLAG_VALUE` variable (note the
prefix `BACKUPER_`).

### Configuration files

Backuper will try to search config file in `./`, `./config/` and
`/etc/backuper/` directories with names `backuper.*` (any format
described above will work).

Example of configuration file in YAML is provided in `./config/` directory.

## Build from scratch

Without docker:

```bash
go mod download
go build -o ./build/backuper ./cmd/backuper/main.go 
```

With docker:
```bash
docker build . -t yurykabanov/backuper:latest
```

## TODO

- docker container links (now only host network is supported)
- other transfer managers (upload from the tool itself)
