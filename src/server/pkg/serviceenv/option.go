package serviceenv

import (
	"os"
	"path/filepath"

	"github.com/pachyderm/pachyderm/src/server/pkg/obj"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/chunk"
	"github.com/pachyderm/pachyderm/src/server/pkg/storage/fileset"
	"github.com/pachyderm/pachyderm/src/server/pkg/uuid"
)

// ChunkStorageOptions returns the chunk storage options for the service environment.
func (env *ServiceEnv) ChunkStorageOptions() ([]chunk.StorageOption, error) {
	var opts []chunk.StorageOption
	if env.StorageUploadConcurrencyLimit > 0 {
		opts = append(opts, chunk.WithMaxConcurrentObjects(0, env.StorageUploadConcurrencyLimit))
	}
	if env.StorageDiskCacheSize > 0 {
		diskCache, err := obj.NewLocalClient(filepath.Join(os.TempDir(), "pfs-cache", uuid.NewWithoutDashes()))
		if err != nil {
			return nil, err
		}
		opts = append(opts, chunk.WithObjectCache(diskCache, env.StorageDiskCacheSize))
	}
	return opts, nil
}

// FileSetStorageOptions returns the fileset storage options for the service environment.
func (env *ServiceEnv) FileSetStorageOptions() []fileset.StorageOption {
	var opts []fileset.StorageOption
	if env.StorageMemoryThreshold > 0 {
		opts = append(opts, fileset.WithMemoryThreshold(env.StorageMemoryThreshold))
	}
	if env.StorageShardThreshold > 0 {
		opts = append(opts, fileset.WithShardThreshold(env.StorageShardThreshold))
	}
	if env.StorageLevelFactor > 0 {
		opts = append(opts, fileset.WithLevelFactor(env.StorageLevelFactor))
	}
	return opts
}
