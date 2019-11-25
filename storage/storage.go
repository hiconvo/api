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
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"

	"github.com/hiconvo/api/utils/reporter"
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

func GetFullAvatarURL(key string) string {
	return avatarURLPrefix + key
}

func GetFullPhotoURL(key string) string {
	return photoURLPrefix + key
}

func GetSignedPhotoURL(ctx context.Context, key string) (string, error) {
	b, err := GetPhotoBucket(ctx)
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
		return "null-key"
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-1]
}

func PutAvatarFromURL(ctx context.Context, uri string) (string, error) {
	res, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("storage.PutAvatarFromURL: Could not download avatar image")
	}

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	bucket, err := GetAvatarBucket(ctx)
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
		reporter.Log(stderr.String())
		return "", fmt.Errorf("storage.PutAvatarFromURL: %v", err)
	}

	return GetFullAvatarURL(key), nil
}

// PutAvatarFromBlob crops and resizes the given image blob, saves it, and
// returns the full URL of the image.
func PutAvatarFromBlob(ctx context.Context, dat string, size, x, y int, oldKey string) (string, error) {
	bucket, err := GetAvatarBucket(ctx)
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
		reporter.Log(stderr.String())
		return "", fmt.Errorf("storage.PutAvatarFromBlob: %v", err)
	}

	if oldKey != "" && oldKey != "null-key" {
		exists, err := bucket.Exists(ctx, oldKey)
		if err != nil {
			reporter.Report(fmt.Errorf("storage.PutAvatarFromBlob: %v)", err))
		}
		if exists {
			bucket.Delete(ctx, oldKey)
		}
	}

	return GetFullAvatarURL(key), nil
}

// PutPhotoFromBlob resizes the given image blob, saves it, and returns the *key*
// of the image.
func PutPhotoFromBlob(ctx context.Context, parentID, dat string) (string, error) {
	if parentID == "" {
		return "", errors.New("storage.PutPhotoFromBlob: No parentID given")
	}

	bucket, err := GetPhotoBucket(ctx)
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

	cmd := exec.Command("convert", "-", "-resize", "2048x2048>", "jpeg:-")
	cmd.Stdin = inputBlob
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		reporter.Log(stderr.String())
		return "", fmt.Errorf("storage.PutPhotoFromBlob: %v", err)
	}

	return key, nil
}
