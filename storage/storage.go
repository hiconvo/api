/*
Package storage provides a high-level, app specific interface for working with storage.
It exposes functions that use keys, which are just strings, that correspond to files in
multiple storage buckets.

There are two "types" of keys: avatar keys and photo keys. The images corresponding
to avatar keys are stored in a public bucket. The images corresponding to photo
keys are also currently stored in a dfferent public bucket, but at some point it is hoped that
this bucket will be made private and that only signed URLs will be used to access
the images.

The biggest mistake I made is writing this package is not creating special types for
the keys. This would be a good refactor later on.
*/
package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	uuid "github.com/gofrs/uuid"
	"gocloud.dev/blob"
	// This sets up the plumbing to use blob with the local file system in development mode.
	_ "gocloud.dev/blob/fileblob"
	// This sets up the plumbing to use blob with GCS in production.
	_ "gocloud.dev/blob/gcsblob"

	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/secrets"
)

// Key corresponds to the path of an object in storage.
// Example: EhEKBlRocmVhZBCAgICYu6KVCg/c2b4a5e6-e302-4ebc-b71a-78cd96c7abb1.jpg
// TODO: Finish this sometime.
// type Key string

var (
	_avatarBucketName string = secrets.Get("AVATAR_BUCKET_NAME", getFallbackBucketName())
	_photoBucketName  string = secrets.Get("PHOTO_BUCKET_NAME", getFallbackBucketName())
)

const _nullKey string = "null-key"

// GetAvatarURLFromKey returns the public URL of the given avatar key.
func GetAvatarURLFromKey(key string) string {
	return getURLPrefix(_avatarBucketName) + key
}

// GetPhotoURLFromKey returns the public URL of the given photo key.
func GetPhotoURLFromKey(key string) string {
	return getURLPrefix(_photoBucketName) + key
}

// GetSignedPhotoURL returns a signed URL of the given photo key.
func GetSignedPhotoURL(ctx context.Context, key string) (string, error) {
	b, err := getPhotoBucket(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.GetSignedPhotoURL: %v", err)
	}

	return b.SignedURL(ctx, key, &blob.SignedURLOptions{
		Expiry: blob.DefaultSignedURLExpiry,
	})
}

// GetKeyFromAvatarURL accepts a url and returns the last segment of the
// URL, which corresponds with the key of the avatar image, used in cloud
// storage.
func GetKeyFromAvatarURL(url string) string {
	if url == "" {
		return _nullKey
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-1]
}

// GetKeyFromPhotoURL accepts a url and returns the last segment of the
// URL, which corresponds with the key of the image, used in cloud
// storage.
func GetKeyFromPhotoURL(url string) string {
	if url == "" {
		return _nullKey
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-2] + "/" + ss[len(ss)-1]
}

// PutAvatarFromURL requests the image at the given URL, resizes it to 256x256 and
// saves it to the avatar bucket. It returns the full avatar URL.
func PutAvatarFromURL(ctx context.Context, uri string) (string, error) {
	res, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("storage.PutAvatarFromURL: Could not download avatar image")
	}

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	bucket, err := getAvatarBucket(ctx)
	if err != nil {
		return "", err
	}
	defer bucket.Close()

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", err
	}
	defer outputBlob.Close()

	var stderr bytes.Buffer

	cmd := exec.Command("convert", "-", "-adaptive-resize", "256x256", "jpeg:-")
	cmd.Stdin = res.Body
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Print(stderr.String())
		return "", fmt.Errorf("storage.PutAvatarFromURL: %v", err)
	}

	return GetAvatarURLFromKey(key), nil
}

// PutAvatarFromBlob crops and resizes the given image blob, saves it, and
// returns the full URL of the image.
func PutAvatarFromBlob(ctx context.Context, dat string, size, x, y int, oldKey string) (string, error) {
	bucket, err := getAvatarBucket(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.PutAvatarFromBlob: %v", err)
	}
	defer bucket.Close()

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", fmt.Errorf("storage.PutAvatarFromBlob: %v", err)
	}
	defer outputBlob.Close()

	inputBlob := base64.NewDecoder(base64.StdEncoding, strings.NewReader(dat))
	var stderr bytes.Buffer

	cropGeo := fmt.Sprintf("%vx%v+%v+%v", size, size, x, y)
	cmd := exec.Command("convert", "-", "-crop", cropGeo, "-adaptive-resize", "256x256", "jpeg:-")
	cmd.Stdin = inputBlob
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Print(stderr.String())
		return "", fmt.Errorf("storage.PutAvatarFromBlob: %v", err)
	}

	if oldKey != "" && oldKey != _nullKey {
		exists, err := bucket.Exists(ctx, oldKey)
		if err != nil {
			log.Alarm(fmt.Errorf("storage.PutAvatarFromBlob: %v)", err))
		}
		if exists {
			bucket.Delete(ctx, oldKey)
		}
	}

	return GetAvatarURLFromKey(key), nil
}

// PutPhotoFromBlob resizes the given image blob, saves it, and returns the key
// of the image.
func PutPhotoFromBlob(ctx context.Context, parentID, dat string) (string, error) {
	if parentID == "" {
		return "", errors.New("storage.PutPhotoFromBlob: No parentID given")
	}

	bucket, err := getPhotoBucket(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.PutPhotoFromBlob: %v", err)
	}
	defer bucket.Close()

	key := parentID + "/" + uuid.Must(uuid.NewV4()).String() + ".jpg"

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", fmt.Errorf("storage.PutPhotoFromBlob: %v", err)
	}
	defer outputBlob.Close()

	inputBlob := base64.NewDecoder(base64.StdEncoding, strings.NewReader(dat))
	var stderr bytes.Buffer

	cmd := exec.Command("convert", "-", "-resize", "2048x2048>", "-quality", "70", "jpeg:-")
	cmd.Stdin = inputBlob
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Print(stderr.String())
		return "", fmt.Errorf("storage.PutPhotoFromBlob: %v", err)
	}

	return key, nil
}

// DeletePhoto deletes the given photo from the photo bucket.
// This does not work for avatars.
func DeletePhoto(ctx context.Context, key string) error {
	bucket, err := getPhotoBucket(ctx)
	if err != nil {
		return fmt.Errorf("storage.DeletePhoto: %v", err)
	}
	defer bucket.Close()

	if err := bucket.Delete(ctx, key); err != nil {
		return fmt.Errorf("storage.DeletePhoto: %v", err)
	}

	return nil
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

// getPhotoBucket gets the bucket for user photos.
func getPhotoBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, _photoBucketName)
}

// getAvatarBucket gets the bucket for user avatars.
func getAvatarBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, _avatarBucketName)
}
