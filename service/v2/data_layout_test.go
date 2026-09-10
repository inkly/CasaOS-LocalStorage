package v2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ReCasaOS/CasaOS-LocalStorage/common"
	model2 "github.com/ReCasaOS/CasaOS-LocalStorage/service/model"
	"github.com/moby/sys/mountinfo"
)

func TestUsesExternalDataBranches(t *testing.T) {
	volume := &model2.Volume{UUID: "data-volume", MountPoint: "/mnt/data"}

	if !usesExternalDataBranches(&model2.Merge{
		MountPoint:     common.DefaultMountPoint,
		SourceVolumes:  []*model2.Volume{volume},
		SourceBasePath: nil,
	}) {
		t.Fatal("expected a merge with external volumes and no base path to use external branches")
	}

	basePath := "/var/lib/casaos/files"
	if usesExternalDataBranches(&model2.Merge{
		MountPoint:     common.DefaultMountPoint,
		SourceVolumes:  []*model2.Volume{volume},
		SourceBasePath: &basePath,
	}) {
		t.Fatal("expected a merge with a system base path to retain the system branch")
	}

	if usesExternalDataBranches(&model2.Merge{MountPoint: common.DefaultMountPoint}) {
		t.Fatal("expected a merge with no source volumes not to use external branches")
	}
}

func TestIsBindSourceMatchesMountRoot(t *testing.T) {
	info := &mountinfo.Info{Source: "/dev/sda1", Root: "/var/lib/casaos/files/AppData"}
	if !isBindSource(info, "/var/lib/casaos/files/AppData") {
		t.Fatal("expected the bind source to match the mount root")
	}
}

func TestPrepareDataRootsPreservesExistingSystemData(t *testing.T) {
	root := t.TempDir()
	systemRoot := filepath.Join(root, "files")
	dataRoot := filepath.Join(root, "DATA")
	if err := os.MkdirAll(filepath.Join(systemRoot, "AppData"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataRoot, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(systemRoot, "AppData", "settings.json"), []byte("system"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataRoot, "Documents", "readme.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareDataRoots(systemRoot, dataRoot); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(systemRoot, "AppData", "settings.json")); err != nil {
		t.Fatalf("system AppData was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(systemRoot, "Documents", "readme.txt")); err != nil {
		t.Fatalf("existing /DATA content was not moved: %v", err)
	}
	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected an empty data mountpoint, found %d entries", len(entries))
	}
}

func TestPrepareDataRootsReplacesSystemDataSymlink(t *testing.T) {
	root := t.TempDir()
	systemRoot := filepath.Join(root, "files")
	dataRoot := filepath.Join(root, "DATA")
	if err := os.MkdirAll(systemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(systemRoot, "AppData"), []byte("system"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(systemRoot, dataRoot); err != nil {
		t.Fatal(err)
	}

	if err := prepareDataRoots(systemRoot, dataRoot); err != nil {
		t.Fatal(err)
	}

	dataInfo, err := os.Lstat(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !dataInfo.IsDir() || dataInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected a real empty data directory, got mode %s", dataInfo.Mode())
	}
	if _, err := os.Stat(filepath.Join(systemRoot, "AppData")); err != nil {
		t.Fatalf("system data was not preserved after replacing symlink: %v", err)
	}
}

func TestPrepareDataRootsCreatesBothRootsWhenAbsent(t *testing.T) {
	root := t.TempDir()
	systemRoot := filepath.Join(root, "files")
	dataRoot := filepath.Join(root, "DATA")

	if err := prepareDataRoots(systemRoot, dataRoot); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{systemRoot, dataRoot} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", path)
		}
	}
}
