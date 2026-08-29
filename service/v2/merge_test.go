package v2

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/sqlite"
	model2 "github.com/IceWhaleTech/CasaOS-LocalStorage/service/model"
	"github.com/moby/sys/mountinfo"
	"gorm.io/gorm"
	"gotest.tools/v3/assert"
)

var (
	_db      *gorm.DB
	_service *LocalStorageService
)

func init() {
	logger.LogInitConsoleOnly()
	_db = sqlite.GetDBByFile("file::memory:?cache=shared")

	sqlite.Hooks[sqlite.HookAfterDelete] = append(sqlite.Hooks[sqlite.HookAfterDelete], hookAfterDeleteVolume)

	_service = NewLocalStorageService(_db, nil)
}

func TestHookAfterDeleteSerialDisk(t *testing.T) {
	// create two serial disks in db
	expectedVolume1 := model2.Volume{
		UUID:       "85022acb-b5a2-424e-bfa9-6acb67d17cb8",
		MountPoint: "/mnt/sda",
	}

	expectedVolume2 := model2.Volume{
		UUID:       "36c94c85-debf-49b6-9f19-866c14b3a0c6",
		MountPoint: "/mnt/sdb",
	}

	_db.Create(&expectedVolume1)
	_db.Create(&expectedVolume2)

	// create a merge in db, associated with two serial disks

	expectedMerge := model2.Merge{
		MountPoint: "/mnt/merge",
		SourceVolumes: []*model2.Volume{
			&expectedVolume1,
			&expectedVolume2,
		},
	}

	_db.Create(&expectedMerge)

	// verify the merge is associated with two serial disks
	var actualMerges []model2.Merge
	if err := _db.Preload(model2.MergeSourceVolumes).Find(&actualMerges).Error; err != nil {
		t.Error(err)
	}

	assert.Equal(t, len(actualMerges), 1)

	actualMerge := actualMerges[0]
	assert.Equal(t, len(actualMerge.SourceVolumes), 2)

	assert.DeepEqual(t, actualMerge, expectedMerge)

	// delete one serial disk
	if err := _db.InstanceSet("gdb", _db).Delete(&expectedVolume1).Error; err != nil {
		t.Error(err)
	}

	// check if the merge is updated
	if err := _db.Preload(model2.MergeSourceVolumes).Find(&actualMerges).Error; err != nil {
		t.Error(err)
	}

	assert.Equal(t, len(actualMerges), 1)

	actualMerge = actualMerges[0]
	assert.Equal(t, len(actualMerge.SourceVolumes), 1)

	assert.DeepEqual(t, *actualMerge.SourceVolumes[0], expectedVolume2)

	// delete the other serial disk
	if err := _db.Delete(&expectedVolume2).Error; err != nil {
		t.Error(err)
	}

	// check if the merge is updated
	if err := _db.Preload(model2.MergeSourceVolumes).Find(&actualMerges).Error; err != nil {
		t.Error(err)
	}

	assert.Equal(t, len(actualMerges), 1)

	actualMerge = actualMerges[0]
	assert.Equal(t, len(actualMerge.SourceVolumes), 0)
}

func TestCreateMergeReportsNonEmptyMountPointEntries(t *testing.T) {
	mountPoint := t.TempDir()
	basePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(mountPoint, "leftover.txt"), []byte("leftover"), 0o644); err != nil {
		t.Fatalf("failed to create leftover file: %v", err)
	}

	merge := &model2.Merge{
		MountPoint:     mountPoint,
		SourceBasePath: &basePath,
	}

	err := _service.CreateMerge(merge)
	if !errors.Is(err, ErrMountPointIsNotEmpty) {
		t.Fatalf("expected ErrMountPointIsNotEmpty, got %v", err)
	}
	if !strings.Contains(err.Error(), "leftover.txt") {
		t.Fatalf("expected the offending entry in the error message, got %q", err)
	}
}

func TestMergeRestoreErrorRegistry(t *testing.T) {
	svc := NewLocalStorageService(nil, nil)

	svc.recordMergeErrorLocked("/DATA", errors.New("mountpoint is not empty: contains: Documents"))
	if got := svc.LastMergeRestoreError("/DATA"); got != "mountpoint is not empty: contains: Documents" {
		t.Fatalf("expected recorded restore error, got %q", got)
	}
	svc.clearMergeErrorLocked("/DATA")
	if got := svc.LastMergeRestoreError("/DATA"); got != "" {
		t.Fatalf("expected cleared restore error, got %q", got)
	}
}

func TestAreAllMergesMounted(t *testing.T) {
	db := sqlite.GetDBByFile("file:mergeall?mode=memory&cache=shared")

	if err := db.Create(&model2.Merge{MountPoint: "/mnt/mergeA", FSType: "fuse.mergerfs"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model2.Merge{MountPoint: "/mnt/mergeB", FSType: "fuse.mergerfs"}).Error; err != nil {
		t.Fatal(err)
	}

	newSvc := func(mounts ...*mountinfo.Info) *LocalStorageService {
		return NewLocalStorageService(db, &fakeMountInfo{mounts: mounts})
	}

	if newSvc().AreAllMergesMounted() {
		t.Fatal("expected no merges to be mounted")
	}
	if newSvc(&mountinfo.Info{Mountpoint: "/mnt/mergeA", FSType: "fuse.mergerfs"}).AreAllMergesMounted() {
		t.Fatal("expected partially mounted merges to be reported as not all mounted")
	}
	if !newSvc(
		&mountinfo.Info{Mountpoint: "/mnt/mergeA", FSType: "fuse.mergerfs"},
		&mountinfo.Info{Mountpoint: "/mnt/mergeB", FSType: "fuse.mergerfs"},
		&mountinfo.Info{Mountpoint: "/media/disk1", FSType: "ext4"},
	).AreAllMergesMounted() {
		t.Fatal("expected all merges to be mounted")
	}
}

func TestWaitForMerges(t *testing.T) {
	t.Run("already mounted", func(t *testing.T) {
		restoreCalls := 0
		ok := WaitForMerges(func() { restoreCalls++ }, func() bool { return true }, 3, time.Millisecond)
		if !ok {
			t.Fatal("expected all merges mounted")
		}
		if restoreCalls != 0 {
			t.Fatalf("expected restore not to be called, got %d calls", restoreCalls)
		}
	})

	t.Run("mounted by restore", func(t *testing.T) {
		restoreCalls := 0
		mounted := false
		ok := WaitForMerges(func() { restoreCalls++; mounted = true }, func() bool { return mounted }, 3, time.Millisecond)
		if !ok {
			t.Fatal("expected all merges mounted")
		}
		if restoreCalls != 1 {
			t.Fatalf("expected 1 restore call, got %d", restoreCalls)
		}
	})

	t.Run("never mounted", func(t *testing.T) {
		restoreCalls := 0
		ok := WaitForMerges(func() { restoreCalls++ }, func() bool { return false }, 3, time.Millisecond)
		if ok {
			t.Fatal("expected merges to remain unmounted")
		}
		if restoreCalls != 3 {
			t.Fatalf("expected 3 restore calls, got %d", restoreCalls)
		}
	})
}

type fakeMountInfo struct {
	mounts []*mountinfo.Info
}

func (f *fakeMountInfo) GetMounts(filter mountinfo.FilterFunc) ([]*mountinfo.Info, error) {
	results := make([]*mountinfo.Info, 0)
	for _, m := range f.mounts {
		skip, stop := filter(m)
		if stop {
			break
		}
		if !skip {
			results = append(results, m)
		}
	}
	return results, nil
}

func TestIsMergeMounted(t *testing.T) {
	svc := NewLocalStorageService(nil, &fakeMountInfo{
		mounts: []*mountinfo.Info{
			{Mountpoint: "/DATA", FSType: "fuse.mergerfs"},
			{Mountpoint: "/media/disk1", FSType: "ext4"},
		},
	})

	if !svc.IsMergeMounted("/DATA", "fuse.mergerfs") {
		t.Fatal("expected /DATA to be reported as a mergerfs mount")
	}
	if svc.IsMergeMounted("/DATA", "ext4") {
		t.Fatal("expected /DATA with the wrong fstype to be reported as not mounted")
	}
	if svc.IsMergeMounted("/OTHER", "fuse.mergerfs") {
		t.Fatal("expected an unknown mount point to be reported as not mounted")
	}
}
