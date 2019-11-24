package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"

	"github.com/hiconvo/api/utils/secrets"
)

var avatarBucketName string
var photoBucketName string
var avatarURLPrefix string
var photoURLPrefix string

func init() {
	avatarBucketName = secrets.Get("AVATAR_BUCKET_NAME", getFallbackBucketName())
	photoBucketName = secrets.Get("PHOTO_BUCKET_NAME", getFallbackBucketName())

	avatarURLPrefix = getURLPrefix(avatarBucketName)
	photoURLPrefix = getURLPrefix(photoBucketName)
}

// getFallbackBucketName provides a fallback bucketName for local development
// which is just a local directory name.
func getFallbackBucketName() string {
	localpath, err := filepath.Abs("./.local-object-store/")
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("file://%s", localpath)
}

// getURLPrefix returns the public URL prefix of the given bucket.
// For example, it will convert "gs://convo-avatars" to
// "https://storage.googleapis.com/convo-avatarts/". It also ensures
// that, when doing local development, the local bucket directory exists.
func getURLPrefix(bucketName string) string {
	// Make sure the storage dir exists when doing local dev
	if bucketName[:8] == "file:///" {
		if err := os.MkdirAll(bucketName[7:], 0777); err != nil {
			panic(fmt.Errorf("storage.getURLPrefix: %v", err))
		}

		return bucketName + "/"
	}

	if bucketName[:5] == "gs://" {
		return fmt.Sprintf("https://storage.googleapis.com/%s/", bucketName[5:])
	}

	panic(fmt.Errorf("storage.getURLPrefix: '%v' is not a valid bucketName", bucketName))
}

func GetPhotoBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, photoBucketName)
}

func GetAvatarBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, avatarBucketName)
}

func GetFullAvatarURL(object string) string {
	return avatarURLPrefix + object
}

func GetFullPhotoURL(parentID, object string) string {
	return photoURLPrefix + parentID + "/" + object
}

func GetSignedPhotoURL(ctx context.Context, parentID, object string) (string, error) {
	b, err := GetPhotoBucket(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.GetFullPhotoURL: %v", err)
	}

	key := parentID + "/" + object
	return b.SignedURL(ctx, key, &blob.SignedURLOptions{
		Expiry: blob.DefaultSignedURLExpiry,
	})
}

// GetKeyFromAvatarURL accepts a url and returns the last segment of the
// URL, which corresponds with the key of the avatar image, used in cloud
// storage.
func GetKeyFromAvatarURL(url string) string {
	if url == "" {
		return "null-key"
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-1]
}
