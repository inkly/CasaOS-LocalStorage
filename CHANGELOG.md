# Changelog

All notable changes to CasaOS LocalStorage are documented here.

## [Unreleased]

### Changed

- The install-time migration script no longer geo-locates the host. `__get_download_domain` curled `ipconfig.io/country`, falling back to `ifconfig.io/country_code`, at the top level of `build/scripts/migration/script.d/04-migrate-local-storage.sh` — and `install.sh` runs every script in that directory on every install and every upgrade, so both third-party services were contacted each time regardless of whether a migration applied. Migration tools are fetched from `https://github.com/`; `CASAOS_DOWNLOAD_DOMAIN`, trailing slash included, selects a mirror explicitly. The migration lists are unchanged.

## [0.4.30] - 2026-09-06

### Fixed

- Disk health no longer reads a missing `smart_status` (virtual disks such as QEMU/Proxmox, standby or unopenable devices) as a failure, which showed a red "Damage" tag on the home Storage widget while Storage Manager reported the same disk healthy. `sys_disk` and each item of `GET /v1/disks` now also carry `smart_status` (`passed`, `failed` or `unavailable`); the existing `health` fields keep their type and mean "not failed".

## [0.4.29] - 2026-09-05

### Changed

- First release of the inkly distribution: the release pipeline publishes under `inkly` with GoReleaser; the npm publish and test-server workflows that could only run at IceWhale are removed.

## [0.4.28] - 2026-08-20

### Changed

- No code changes since v0.4.27. Republished from `main` after the boot fixes were merged ([CasaOS-LocalStorage #10](https://github.com/alvins82/CasaOS-LocalStorage/pull/10)) so the release commit is the one actually merged into `main`.
- Backfilled the CHANGELOG entries for v0.4.26 and v0.4.27, which previously existed only as release notes.

### Verification

- `git diff v0.4.27 v0.4.28` shows only the CHANGELOG.md change; the binary sources are identical.

## [0.4.27] - 2026-08-19

### Fixed

- The before-docker init step (`casaos-local-storage-first`, `casaos-local-storage -init`) now waits until every persisted merged mount is up before reporting done, so `/DATA` is complete before Docker restores containers. This closes the boot window that left user apps exited (127) when branch disks appeared after dockerd.

### Verification

- Linux-targeted build and tests pass.
- Reboot-verified on real hardware (two reboots with 6+ branch disks): all user apps come back automatically; the storage-first unit exits 0 with all merges mounted before Docker starts.

## [0.4.26] - 2026-08-19

### Fixed

- Merged storage restore keeps retrying until the source disks appear instead of giving up on the first pass, so a slow branch-disk enumeration during boot no longer leaves `/DATA` unmounted. The last restore failure is surfaced in the merge status endpoint.

### Verification

- Linux-targeted build and tests pass.
- Reboot-verified on real hardware.

## [0.4.25] - 2026-08-14

### Fixed

- Restore persisted mergerfs mounts before creating default `/DATA` directories, preventing upgrades and service restarts from leaving the configured merged storage unmounted ([CasaOS-LocalStorage #9](https://github.com/alvins82/CasaOS-LocalStorage/pull/9)).

### Verification

- Added a regression test documenting the startup ordering contract.
- Linux-targeted build and focused tests pass.

## [0.4.24] - 2026-08-13

### Added

- Create the standard `Documents`, `Downloads`, `Gallery`, and `Media` directories in `/DATA` when an external merged storage is created and they are missing ([CasaOS-LocalStorage #7](https://github.com/alvins82/CasaOS-LocalStorage/pull/7)).

### Changed

- Reuse the default-directory helper during startup and merged-storage recovery while preserving the special system `AppData` compatibility mount.

### Verification

- Focused default-directory tests pass.
- Linux cross-compilation succeeds for the service and root package.

## [0.4.23] - 2026-08-13

### Added

- Add a protected `PUT /v1/storage/rename` endpoint for ext2/ext3/ext4 volumes and keep system storage protected from renaming ([CasaOS-LocalStorage #6](https://github.com/alvins82/CasaOS-LocalStorage/pull/6)).

### Fixed

- Read the filesystem label directly with `blkid` when `lsblk` has not refreshed udev data yet, so a successful rename is reflected immediately in Storage Manager ([CasaOS-LocalStorage #6](https://github.com/alvins82/CasaOS-LocalStorage/pull/6)).

### Verification

- `go generate ./...`
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...`
- Invalid-device API validation probe completed successfully.

## [0.4.22] - 2026-08-13

### Changed

- Traverse nested `lsblk` trees so filesystems under partitions and LVM logical volumes are represented accurately.
- Preserve the physical parent disk model and path for each storage entry ([CasaOS-LocalStorage #5](https://github.com/alvins82/CasaOS-LocalStorage/pull/5)).

### Fixed

- Report used and available space from the mounted filesystem rather than the allocated physical disk, with coverage for nested mounts and logical volumes ([CasaOS-LocalStorage #5](https://github.com/alvins82/CasaOS-LocalStorage/pull/5)).

### Verification

- `GOOS=linux GOARCH=amd64 go build ./...`
- `GOOS=linux GOARCH=amd64 go test -c ./service`
