# Changelog

## v0.0.2
_发布日期：2026-10-24_

- build: require published go-events v0.0.5

## v0.0.1
_发布日期：2026-10-12_

- build: 依赖改为已发布版本，移除本地 replace

## Unreleased

- build: 升级依赖到 go-storage/go-storage-lock v0.1.0
- refactor: hello 命令改在业务库执行，不再依赖 admin 库权限
- fix: GetTime 优先使用 MongoDB 服务端时间，NTP 仅作兜底
