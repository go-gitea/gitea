package models

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/pkg/errors"
	"gocloud.dev/blob"
)

// GetAvatarLink provides user avatar link whether it's stored in local or cloud storage
func (u *User) GetAvatarLink() (string, error) {
	if setting.FileStorage.SaveToBucket {
		return u.getAvatarLinkFromBucket()
	}

	// Bucket storage not set
	if !com.IsFile(u.CustomAvatarPath()) {
		return "", errors.Errorf("file doesn't exist")
	}

	return setting.AppSubURL + "/avatars/" + u.Avatar, nil
}

// getAvatarLinkFromBucket provides avatar link from cloud storage
func (u *User) getAvatarLinkFromBucket() (string, error) {
	ctx := context.Background()

	bucket, err := blob.OpenBucket(ctx, setting.FileStorage.Bucket)
	if err != nil {
		return "", fmt.Errorf("Failed to setup bucket: %v", err)
	}
	exist, err := bucket.Exists(ctx, u.CustomAvatarPath())
	if exist {
		return filepath.Join(setting.FileStorage.BucketURL, u.CustomAvatarPath()), nil
	}
	return "", errors.Errorf("file doesn't exist, error %v", err)

}

// SaveAvatar saves avatar
func (u *User) SaveAvatar(img *image.Image) error {
	if setting.FileStorage.SaveToBucket {
		return u.uploadAvatarToBucket(img)
	}

	if err := os.MkdirAll(filepath.Dir(u.CustomAvatarPath()), os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %v", err)
	}
	fw, err := os.Create(u.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()
	if err := png.Encode(fw, *img); err != nil {
		return fmt.Errorf("encode: %v", err)
	}
	return nil
}

// uploadAvatarToBucket stores user avatar to cloud storage
func (u *User) uploadAvatarToBucket(img *image.Image) error {
	ctx := context.Background()
	bucket, err := blob.OpenBucket(ctx, setting.FileStorage.Bucket)
	if err != nil {
		return fmt.Errorf("failed to setup bucket: %v", err)
	}

	buf := new(bytes.Buffer)
	if err = png.Encode(buf, *img); err != nil {
		return fmt.Errorf("failed to encode: %v", err)
	}
	imgData := buf.Bytes()

	bucketWriter, err := bucket.NewWriter(ctx, u.CustomAvatarPath(), nil)
	if err != nil {
		return fmt.Errorf("failed to obtain writer: %v", err)
	}

	if _, err = bucketWriter.Write(imgData); err != nil {
		return fmt.Errorf("error occurred: %v", err)
	}
	if err = bucketWriter.Close(); err != nil {
		return fmt.Errorf("Failed to close: %v", err)
	}

	return nil
}

// DeleteUserAvatar deletes user avatar
func (u *User) DeleteUserAvatar() error {
	if setting.FileStorage.SaveToBucket {
		return u.deleteAvatarFromBucket()
	}
	if err := os.Remove(u.CustomAvatarPath()); err != nil {
		return fmt.Errorf("Failed to remove %s: %v", u.CustomAvatarPath(), err)
	}
	return nil
}

// deleteAvatarFromBucket deletes user avatar from cloud storage
func (u *User) deleteAvatarFromBucket() error {
	ctx := context.Background()
	bucket, err := blob.OpenBucket(ctx, setting.FileStorage.Bucket)
	if err != nil {
		return fmt.Errorf("failed to setup bucket: %v", err)
	}
	exist, err := bucket.Exists(ctx, u.CustomAvatarPath())
	if err != nil {
		return err
	} else if !exist {
		return errors.New("avatar not found")
	}

	return bucket.Delete(ctx, u.CustomAvatarPath())
}
