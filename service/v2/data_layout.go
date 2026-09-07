package v2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inkly/CasaOS-Common/utils/constants"
	"github.com/inkly/CasaOS-Common/utils/file"
	"github.com/inkly/CasaOS-LocalStorage/codegen"
	"github.com/inkly/CasaOS-LocalStorage/common"
	mountpkg "github.com/inkly/CasaOS-LocalStorage/pkg/mount"
	model2 "github.com/inkly/CasaOS-LocalStorage/service/model"
	"github.com/moby/sys/mountinfo"
)

const appDataDirectory = "AppData"

var (
	ErrDataMountPointConflict = errors.New("/DATA/AppData is already mounted from an unexpected source")
	ErrDataMountPointNotEmpty = errors.New("/DATA contains data that cannot be safely restored")
)

func usesExternalDataBranches(merge *model2.Merge) bool {
	if merge == nil || merge.MountPoint != common.DefaultMountPoint || len(merge.SourceVolumes) == 0 {
		return false
	}

	return merge.SourceBasePath == nil || strings.TrimSpace(*merge.SourceBasePath) == ""
}

func systemAppDataPath() string {
	return filepath.Join(constants.DefaultFilePath, appDataDirectory)
}

func dataAppDataMountPoint() string {
	return filepath.Join(common.DefaultMountPoint, appDataDirectory)
}

func (s *LocalStorageService) rawMountsAt(mountPoint string) ([]*mountinfo.Info, error) {
	if s._mountinfo == nil {
		return nil, errors.New("mount information is unavailable")
	}
	return s._mountinfo.GetMounts(func(info *mountinfo.Info) (skip bool, stop bool) {
		return info.Mountpoint != mountPoint, false
	})
}

func isBindSource(info *mountinfo.Info, source string) bool {
	return filepath.Clean(info.Source) == filepath.Clean(source) || filepath.Clean(info.Root) == filepath.Clean(source)
}

func isExpectedAppDataMount(info *mountinfo.Info, source string, target string) bool {
	if isBindSource(info, source) {
		return true
	}

	sourceInfo, sourceErr := os.Stat(source)
	targetInfo, targetErr := os.Stat(target)
	return sourceErr == nil && targetErr == nil && os.SameFile(sourceInfo, targetInfo)
}

// ensureAppDataCompatibilityMount keeps the directory expected by CasaOS
// applications backed by the original system-data tree. A bind mount is used
// instead of a symlink because Docker bind mounts must expose the real
// directory tree to containers.
func (s *LocalStorageService) ensureAppDataCompatibilityMount(merge *model2.Merge) error {
	if !usesExternalDataBranches(merge) {
		return nil
	}

	source := systemAppDataPath()
	target := dataAppDataMountPoint()
	if err := ensureDirectory(source); err != nil {
		return fmt.Errorf("create system AppData directory: %w", err)
	}
	if err := ensureDirectory(target); err != nil {
		return fmt.Errorf("create /DATA/AppData directory: %w", err)
	}

	mounts, err := s.rawMountsAt(target)
	if err != nil {
		return err
	}
	if len(mounts) > 0 {
		for _, mounted := range mounts {
			if isExpectedAppDataMount(mounted, source, target) {
				return nil
			}
		}
		return ErrDataMountPointConflict
	}

	if err := mountpkg.Bind(source, target); err != nil {
		return fmt.Errorf("bind %s to %s: %w", source, target, err)
	}
	return nil
}

func (s *LocalStorageService) unmountIfPresent(mountPoint string) error {
	mounts, err := s.GetMounts(codegen.GetMountsParams{MountPoint: &mountPoint})
	if err != nil {
		return err
	}
	if len(mounts) == 0 {
		return nil
	}
	if err := mountpkg.UmountByMountPoint(mountPoint); err != nil {
		return fmt.Errorf("unmount %s: %w", mountPoint, err)
	}
	return nil
}

func (s *LocalStorageService) unmountAppDataCompatibilityMount() error {
	source := systemAppDataPath()
	target := dataAppDataMountPoint()
	mounts, err := s.rawMountsAt(target)
	if err != nil {
		return err
	}
	if len(mounts) == 0 {
		return nil
	}

	for _, mounted := range mounts {
		if !isExpectedAppDataMount(mounted, source, target) {
			return ErrDataMountPointConflict
		}
	}
	return s.unmountIfPresent(target)
}

func (s *LocalStorageService) unmountMergeIfPresent(merge *model2.Merge) error {
	mounts, err := s.GetMounts(codegen.GetMountsParams{MountPoint: &merge.MountPoint})
	if err != nil {
		return err
	}
	if len(mounts) == 0 {
		return nil
	}

	for _, mounted := range mounts {
		if mounted.Fstype == nil ||
			(*mounted.Fstype != merge.FSType && *mounted.Fstype != "mergerfs" && *mounted.Fstype != "fuse.mergerfs") {
			return fmt.Errorf("refusing to unmount %s: it is not the configured mergerfs mount", merge.MountPoint)
		}
	}

	if err := mountpkg.UmountByMountPoint(merge.MountPoint); err != nil {
		return fmt.Errorf("unmount mergerfs at %s: %w", merge.MountPoint, err)
	}
	return nil
}

func ensureDirectory(path string) error {
	if err := file.IsNotExistMkDir(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

// PrepareExternalDataRoot separates the system-data tree from the /DATA
// mountpoint before an external-only merge is created. Existing content is
// moved into the system-data tree without overwriting collisions. A symlink
// from /DATA to the system-data tree is replaced with an empty directory,
// leaving the target tree (including AppData) in place.
func (s *LocalStorageService) PrepareExternalDataRoot(mountPoint string) error {
	if mountPoint != common.DefaultMountPoint {
		return fmt.Errorf("external data root preparation only supports %s", common.DefaultMountPoint)
	}
	return prepareDataRoots(constants.DefaultFilePath, mountPoint)
}

func prepareDataRoots(systemRoot string, dataRoot string) error {
	if filepath.Clean(systemRoot) == filepath.Clean(dataRoot) {
		return fmt.Errorf("system data root and data mountpoint must be different")
	}

	dataInfo, dataErr := os.Lstat(dataRoot)
	if dataErr != nil && !os.IsNotExist(dataErr) {
		return dataErr
	}

	systemInfo, systemErr := os.Lstat(systemRoot)
	if systemErr != nil && !os.IsNotExist(systemErr) {
		return systemErr
	}

	if dataErr == nil && dataInfo.Mode()&os.ModeSymlink != 0 {
		resolvedDataRoot, err := filepath.EvalSymlinks(dataRoot)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", dataRoot, err)
		}
		resolvedSystemRoot, err := filepath.EvalSymlinks(systemRoot)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", systemRoot, err)
		}
		if filepath.Clean(resolvedDataRoot) != filepath.Clean(resolvedSystemRoot) {
			return fmt.Errorf("refusing to replace %s: it points to %s", dataRoot, resolvedDataRoot)
		}
		if err := os.Remove(dataRoot); err != nil {
			return fmt.Errorf("remove %s symlink: %w", dataRoot, err)
		}
		return ensureDirectory(dataRoot)
	}

	if dataErr != nil {
		if systemErr == nil {
			return ensureDirectory(dataRoot)
		}
		if os.IsNotExist(systemErr) {
			if err := ensureDirectory(systemRoot); err != nil {
				return err
			}
			return ensureDirectory(dataRoot)
		}
		if err := os.Rename(dataRoot, systemRoot); err != nil {
			return fmt.Errorf("move %s to %s: %w", dataRoot, systemRoot, err)
		}
		return ensureDirectory(dataRoot)
	}

	if !dataInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", dataRoot)
	}
	if systemErr == nil && !systemInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", systemRoot)
	}
	if systemErr != nil {
		if err := os.Rename(dataRoot, systemRoot); err != nil {
			return fmt.Errorf("move %s to %s: %w", dataRoot, systemRoot, err)
		}
		return ensureDirectory(dataRoot)
	}

	dataStat, err := os.Stat(dataRoot)
	if err != nil {
		return err
	}
	systemStat, err := os.Stat(systemRoot)
	if err != nil {
		return err
	}
	if os.SameFile(dataStat, systemStat) {
		return fmt.Errorf("%s and %s refer to the same directory", dataRoot, systemRoot)
	}

	if err := moveDirectoryContents(dataRoot, systemRoot); err != nil {
		return err
	}
	if err := os.Remove(dataRoot); err != nil {
		return fmt.Errorf("remove empty %s: %w", dataRoot, err)
	}
	return ensureDirectory(dataRoot)
}

func moveDirectoryContents(sourceRoot string, targetRoot string) error {
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(sourceRoot, entry.Name())
		targetPath := filepath.Join(targetRoot, entry.Name())
		targetInfo, targetErr := os.Lstat(targetPath)
		if os.IsNotExist(targetErr) {
			if err := os.Rename(sourcePath, targetPath); err != nil {
				return fmt.Errorf("move %s to %s: %w", sourcePath, targetPath, err)
			}
			continue
		}
		if targetErr != nil {
			return targetErr
		}

		sourceInfo, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		if sourceInfo.IsDir() && targetInfo.IsDir() {
			if err := moveDirectoryContents(sourcePath, targetPath); err != nil {
				return err
			}
			if err := os.Remove(sourcePath); err != nil {
				return fmt.Errorf("remove merged source directory %s: %w", sourcePath, err)
			}
			continue
		}
		return fmt.Errorf("refusing to overwrite existing data path %s", targetPath)
	}
	return nil
}

// restoreSystemDataRoot reverses the InitMerge directory move. It only
// removes an empty mountpoint directory, so files written while the mergerfs
// mount was active are never deleted by an unmerge operation.
func restoreSystemDataRoot() error {
	systemRoot := constants.DefaultFilePath
	dataRoot := common.DefaultMountPoint

	systemInfo, err := os.Stat(systemRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return ensureDirectory(dataRoot)
	}
	if !systemInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", systemRoot)
	}

	dataInfo, err := os.Stat(dataRoot)
	if err == nil {
		if !dataInfo.IsDir() {
			return fmt.Errorf("%s is not a directory", dataRoot)
		}
		entries, readErr := os.ReadDir(dataRoot)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 {
			return fmt.Errorf("%w: %s is not empty", ErrDataMountPointNotEmpty, dataRoot)
		}
		if err := os.Remove(dataRoot); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(systemRoot, dataRoot); err != nil {
		return fmt.Errorf("restore %s to %s: %w", systemRoot, dataRoot, err)
	}
	return nil
}

// RemoveMerge performs a full unmerge for the default CasaOS data root. It
// detaches the compatibility mount first, leaves every external disk mounted,
// restores the original system-data directory, and only then removes the
// merge record from the database.
func (s *LocalStorageService) RemoveMerge(mountPoint string) error {
	merge, err := s.GetFirstMergeFromDB(mountPoint)
	if err != nil {
		return err
	}
	if merge == nil {
		return ErrMergeMountPointDoesNotExist
	}

	if mountPoint == common.DefaultMountPoint {
		if err := s.unmountAppDataCompatibilityMount(); err != nil {
			return err
		}
	}
	if err := s.unmountMergeIfPresent(merge); err != nil {
		return err
	}
	if mountPoint == common.DefaultMountPoint {
		if err := restoreSystemDataRoot(); err != nil {
			return err
		}
	}
	return s.DeleteMergeFromDB(merge)
}
