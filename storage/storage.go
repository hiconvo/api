/*
Package storage provides a high-level, app specific interface for working with storage.
It exposes functions that use keys, which are just strings, that correspond to files in
multiple storage buckets.

There are two "types" of keys: avatar keys and photo keys. The images corresponding
to avatar keys are stored in a public bucket. The images corresponding to photo
keys are also currently stored in a dfferent public bucket, but at some point it is hoped that
this bucket will be made private and that only signed URLs will be used to access
the images.
*/
package storage

import (
	"bytes"
	"context"
	"encoding/base64"
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

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/secrets"
)

const _nullKey string = "null-key"

var DefaultClient *Client

func init() {
	DefaultClient = NewClient(secrets.Get("AVATAR_BUCKET_NAME", ""), secrets.Get("PHOTO_BUCKET_NAME", ""))
}

type Client struct {
	avatarBucketName string
	photoBucketName  string
}

func NewClient(avatarBucketName, photoBucketName string) *Client {
	if avatarBucketName == "" || photoBucketName == "" {
		localBucketName := initLocalStorageDir()
		return &Client{
			avatarBucketName: localBucketName,
			photoBucketName:  localBucketName,
		}
	}

	return &Client{
		avatarBucketName: avatarBucketName,
		photoBucketName:  photoBucketName,
	}
}

func initLocalStorageDir() string {
	op := errors.Op("storage.initLocalStorageDir")

	localpath, err := filepath.Abs("./.local-object-store/")
	if err != nil {
		panic(errors.E(op, err))
	}

	fp := fmt.Sprintf("file://%s", localpath)

	log.Printf("storage.initLocalStorageDir: Creating temporary bucket storage at %s", fp)
	if err := os.MkdirAll(localpath, 0777); err != nil {
		panic(errors.E(op, err))
	}

	return fp
}

// GetAvatarURLFromKey returns the public URL of the given avatar key.
func (c *Client) GetAvatarURLFromKey(key string) string {
	return getURLPrefix(c.avatarBucketName) + key
}

// GetPhotoURLFromKey returns the public URL of the given photo key.
func (c *Client) GetPhotoURLFromKey(key string) string {
	return getURLPrefix(c.photoBucketName) + key
}

// GetSignedPhotoURL returns a signed URL of the given photo key.
func (c *Client) GetSignedPhotoURL(ctx context.Context, key string) (string, error) {
	b, err := blob.OpenBucket(ctx, c.photoBucketName)
	if err != nil {
		return "", errors.E(errors.Op("storage.GetSignedPhotoURL"), err)
	}
	defer b.Close()

	return b.SignedURL(ctx, key, &blob.SignedURLOptions{
		Expiry: blob.DefaultSignedURLExpiry,
	})
}

// GetKeyFromAvatarURL accepts a url and returns the last segment of the
// URL, which corresponds with the key of the avatar image, used in cloud
// storage.
func (c *Client) GetKeyFromAvatarURL(url string) string {
	if url == "" {
		return _nullKey
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-1]
}

// GetKeyFromPhotoURL accepts a url and returns the last segment of the
// URL, which corresponds with the key of the image, used in cloud
// storage.
func (c *Client) GetKeyFromPhotoURL(url string) string {
	if url == "" {
		return _nullKey
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-2] + "/" + ss[len(ss)-1]
}

// PutAvatarFromURL requests the image at the given URL, resizes it to 256x256 and
// saves it to the avatar bucket. It returns the full avatar URL.
func (c *Client) PutAvatarFromURL(ctx context.Context, uri string) (string, error) {
	op := errors.Op("storage.PutAvatarFromURL")

	res, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.E(op, errors.Str("Could not download avatar image"))
	}

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	bucket, err := blob.OpenBucket(ctx, c.avatarBucketName)
	if err != nil {
		return "", errors.E(op, err)
	}
	defer bucket.Close()

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", errors.E(op, err)
	}
	defer outputBlob.Close()

	var stderr bytes.Buffer

	cmd := exec.Command("convert", "-", "-adaptive-resize", "256x256", "jpeg:-")
	cmd.Stdin = res.Body
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Print(stderr.String())
		return "", errors.E(op, err)
	}

	return c.GetAvatarURLFromKey(key), nil
}

// PutAvatarFromBlob crops and resizes the given image blob, saves it, and
// returns the full URL of the image.
func (c *Client) PutAvatarFromBlob(ctx context.Context, dat string, size, x, y int, oldKey string) (string, error) {
	op := errors.Op("storage.PutAvatarFromBlob")

	bucket, err := blob.OpenBucket(ctx, c.avatarBucketName)
	if err != nil {
		return "", errors.E(op, err)
	}
	defer bucket.Close()

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", errors.E(op, err)
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
		return "", errors.E(op, err)
	}

	if oldKey != "" && oldKey != _nullKey {
		exists, err := bucket.Exists(ctx, oldKey)
		if err != nil {
			log.Alarm(errors.E(op, err))
		}
		if exists {
			bucket.Delete(ctx, oldKey)
		}
	}

	return c.GetAvatarURLFromKey(key), nil
}

// PutPhotoFromBlob resizes the given image blob, saves it, and returns full url of the image.
func (c *Client) PutPhotoFromBlob(ctx context.Context, parentID, dat string) (string, error) {
	op := errors.Op("storage.PutPhotoFromBlob")

	if parentID == "" {
		return "", errors.E(op, errors.Str("No parentID given"))
	}

	bucket, err := blob.OpenBucket(ctx, c.photoBucketName)
	if err != nil {
		return "", errors.E(op, err)
	}
	defer bucket.Close()

	key := parentID + "/" + uuid.Must(uuid.NewV4()).String() + ".jpg"

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		return "", errors.E(op, err)
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
		return "", errors.E(op, err)
	}

	return c.GetPhotoURLFromKey(key), nil
}

// DeletePhoto deletes the given photo from the photo bucket.
// This does not work for avatars.
func (c *Client) DeletePhoto(ctx context.Context, key string) error {
	op := errors.Op("storage.DeletePhoto")

	bucket, err := blob.OpenBucket(ctx, c.photoBucketName)
	if err != nil {
		return errors.E(op, err)
	}
	defer bucket.Close()

	if err := bucket.Delete(ctx, key); err != nil {
		return errors.E(op, err)
	}

	return nil
}

// getURLPrefix returns the public URL prefix of the given bucket.
// For example, it will convert "gs://convo-avatars" to
// "https://storage.googleapis.com/convo-avatarts/".
func getURLPrefix(bucketName string) string {
	if bucketName[:8] == "file:///" {
		return bucketName + "/"
	}

	if bucketName[:5] == "gs://" {
		return fmt.Sprintf("https://storage.googleapis.com/%s/", bucketName[5:])
	}

	panic(errors.E(
		errors.Op("storage.getURLPrefix"),
		errors.Errorf("'%v' is not a valid bucketName", bucketName)))
}
