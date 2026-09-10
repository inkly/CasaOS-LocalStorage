package service

import (
	"encoding/json"
	"testing"

	"github.com/ReCasaOS/CasaOS-LocalStorage/model"
	"gotest.tools/v3/assert"
)

func TestMountedFilesystemStatsIncludesNestedLogicalVolumes(t *testing.T) {
	disk := model.LSBLKModel{
		Name: "sdd",
		Children: []model.LSBLKModel{
			{
				Name:       "sdd2",
				MountPoint: "/boot",
				FSSize:     json.Number("2048"),
				FSAvail:    json.Number("1024"),
				FSUsed:     json.Number("1024"),
			},
			{
				Name: "sdd3",
				Children: []model.LSBLKModel{
					{
						Name:       "ubuntu--vg-ubuntu--lv",
						MountPoint: "/",
						FSSize:     json.Number("4096"),
						FSAvail:    json.Number("3072"),
						FSUsed:     json.Number("1024"),
					},
				},
			},
		},
	}

	stats := MountedFilesystemStats(disk)

	assert.Equal(t, stats.MountCount, 2)
	assert.Equal(t, stats.Size, uint64(6144))
	assert.Equal(t, stats.Avail, uint64(4096))
	assert.Equal(t, stats.Used, uint64(2048))
}

func TestMountedFilesystemsReturnsNestedMounts(t *testing.T) {
	disk := model.LSBLKModel{
		Name: "sdd",
		Children: []model.LSBLKModel{
			{
				Name:       "sdd2",
				MountPoint: "/boot",
			},
			{
				Name: "sdd3",
				Children: []model.LSBLKModel{
					{
						Name:       "ubuntu--vg-ubuntu--lv",
						MountPoint: "/",
					},
				},
			},
		},
	}

	filesystems := MountedFilesystems(disk)

	assert.Equal(t, len(filesystems), 2)
	assert.Equal(t, filesystems[0].MountPoint, "/boot")
	assert.Equal(t, filesystems[1].MountPoint, "/")
}

func TestMountedFilesystemStatsAtFindsNestedRootFilesystem(t *testing.T) {
	disk := model.LSBLKModel{
		Name: "sdd",
		Children: []model.LSBLKModel{
			{
				Name: "sdd3",
				Children: []model.LSBLKModel{
					{
						Name:       "ubuntu--vg-ubuntu--lv",
						MountPoint: "/",
						FSSize:     json.Number("4096"),
						FSAvail:    json.Number("3072"),
						FSUsed:     json.Number("1024"),
					},
				},
			},
		},
	}

	stats, ok := MountedFilesystemStatsAt(disk, "/")

	assert.Assert(t, ok)
	assert.Equal(t, stats.MountCount, 1)
	assert.Equal(t, stats.Size, uint64(4096))
	assert.Equal(t, stats.Avail, uint64(3072))
	assert.Equal(t, stats.Used, uint64(1024))
}
