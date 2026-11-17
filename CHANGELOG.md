# Changelog

## v0.1.0
_发布日期：2026-11-18_

- feat: 语义套件直接覆盖 FairLock 互斥
- chore: 补 MIT LICENSE
- build: 升级依赖到 go-storage/go-storage-lock v0.1.0

## v0.0.3
_发布日期：2026-10-24_

- build: track public module checksums
- ci: run public core suite without nested module discovery
- build: split private storage integration module

## v0.0.2
_发布日期：2026-10-23_

- ci: avoid private sibling repository clones
- ci: authenticate private sibling clones
- ci: fix sibling repository paths
- ci: use published storage modules
- build: drop private storage local replaces
- ci: authenticate sibling repository clones
- ci: fix sibling checkout repository paths
- build: update storage lock test dependencies
- test: 15家存储语义套件适配 + watchdog 阻塞修复 + CI

## v0.0.1
_发布日期：2026-10-12_

- build: 依赖改为已发布版本，移除本地 replace

## Unreleased

- build: go mod tidy 同步 go-storage-lock v0.1.0
