package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gocloud.dev/blob"

	// Google, Azure and S3 packages for bucket storage
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
)

// FileStorage contains necessary info for reading or writing file to bucket
type FileStorage struct {
	Ctx      context.Context
	Path     string
	FileName string
}

// NewReader provides file reader from bucket and error if occurs
func (fs *FileStorage) NewReader() (*blob.Reader, error) {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket: %v", err)
	}
	defer bucket.Close()

	exist, err := bucket.Exists(fs.Ctx, fs.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the file exists: %v", err)
	} else if !exist {
		return nil, os.ErrNotExist
	}

	return bucket.NewReader(fs.Ctx, fs.FileName, nil)
}

// NewRangeReader returns a file range reader from bucket and error if occurs
func (fs *FileStorage) NewRangeReader(offset, length int64) (*blob.Reader, error) {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket: %v", err)
	}
	defer bucket.Close()

	reader, err := bucket.NewRangeReader(fs.Ctx, fs.FileName, offset, length, nil)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// NewWriter returns a file writer from bucket and error if occurs
func (fs *FileStorage) NewWriter() (*blob.Writer, error) {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket: %v", err)
	}
	defer bucket.Close()

	bw, err := bucket.NewWriter(fs.Ctx, fs.FileName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain writer: %v", err)
	}
	return bw, nil
}

// Delete deletes the specified file and returns error if occurs
func (fs *FileStorage) Delete() error {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		return fmt.Errorf("could not open bucket: %v", err)
	}
	defer bucket.Close()

	exist, err := bucket.Exists(fs.Ctx, fs.FileName)
	if err != nil {
		return fmt.Errorf("failed to check if the file exists: %v", err)
	} else if exist {
		return bucket.Delete(fs.Ctx, fs.FileName)
	}
	return nil
}

// Exists checks if the specified file exists in the bucket
func (fs *FileStorage) Exists() bool {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		log.Error("could not open bucket: %v", err)
		return false
	}
	defer bucket.Close()

	exist, err := bucket.Exists(fs.Ctx, fs.FileName)
	if err != nil {
		log.Error("failed to check if the file exists: %v", err)
		return false
	}
	return exist
}

// Attributes returns attributes for the file stored in the bucket
func (fs *FileStorage) Attributes() (*blob.Attributes, error) {
	bucket, err := OpenBucket(fs.Ctx, fs.Path)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket: %v", err)
	}
	defer bucket.Close()
	attrs, err := bucket.Attributes(fs.Ctx, fs.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read attributes: %v", err)
	}
	return attrs, nil
}

/*
- Path represents AttachmentPath, AvatarUploadPath, RepositoryAvatarUploadPath or LFS.ContentPath
- corresponding default PathValues are : "attachments", "avatars", "repo-avatars" & "lfs"

- appDataUserPath defaults to "data"

There may be two scenarios:

s1:
Path is set in app.ini (rel or abs)

s2:
Path is unset in app.ini
Path <= appDataUserPath + Path

If appDataUserPath is set to abs in app.ini (via APP_DATA_PATH)
	Path is abs
Otherwise
	Path is rel


If Path is abs, the files will be read or stored to that abs Path even if the BUCKET_URL is set.
Otherwise the following occurs,

if BUCKET_URL NOT SET {
	Path = file://{AppWorkPath}/{Path}
} else {
	Path is used as Bucket prefix
}

*/

// OpenBucket returns the bucket associated to path parameter
// and also returns error if occurs
func OpenBucket(ctx context.Context, path string) (*blob.Bucket, error) {
	if filepath.IsAbs(path) {
		if err := os.MkdirAll(path, 0700); err != nil {
			log.Fatal("Failed to create '%s': %v", path, err)
		}
		return blob.OpenBucket(ctx, "file://"+path)
	}

	bURL := setting.BucketURL
	if bURL == "" {
		bURL = "file://" + setting.AppWorkPath
	}

	bucket, err := blob.OpenBucket(ctx, bURL)
	if err != nil {
		return nil, err
	}
	return blob.PrefixedBucket(bucket, path), nil
}
