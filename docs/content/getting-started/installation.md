---
title: "Installation"
description: "Install ali1688 from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/ali1688-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `ali1688` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/ali1688-cli/cmd/ali1688@latest
```

That puts `ali1688` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/ali1688-cli
cd ali1688-cli
make build        # produces ./bin/ali1688
./bin/ali1688 version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/ali1688:latest --help
```

## Checking the install

```bash
ali1688 version
```

prints the version and exits.
