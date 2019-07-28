package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"

	"github.com/hiconvo/api/utils/secrets"
)

var avatarBucketName string

func init() {
	// For local development
	localpath, err := filepath.Abs("./.local-object-store/")
	if err != nil {
		panic(err)
	}
	avatarBucketName = secrets.Get("AVATAR_BUCKET_NAME", fmt.Sprintf("file://%s", localpath))

	// Make sure the storage dir exists when doing local dev
	if avatarBucketName[:8] == "file:///" {
		if err := os.MkdirAll(localpath, 0777); err != nil {
			panic(err)
		}
	}
}

func GetAvatarBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, avatarBucketName)
}
