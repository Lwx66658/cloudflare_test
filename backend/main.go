package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/syumai/workers-go"
	"github.com/syumai/workers-go/cloudflare/d1"
	"github.com/syumai/workers-go/cloudflare/r2"
)

const maxImageSize = 10 << 20 // 10 MiB

type Message struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	CreateTime string `json:"create_time"`
}

type Image struct {
	ID           int64  `json:"id"`
	OriginalName string `json:"original_name"`
	ObjectKey    string `json:"object_key"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	ETag         string `json:"etag"`
	CreatedAt    string `json:"created_at"`
	DownloadURL  string `json:"download_url"`
}

func openDB() (*sql.DB, error) {
	connector, err := d1.OpenConnector("DB")
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func openImageBucket() (*r2.Bucket, error) {
	return r2.NewBucket("IMAGES")
}

func newObjectKey(originalName string) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if len(ext) > 10 {
		ext = ""
	}
	now := time.Now().UTC()
	return fmt.Sprintf("images/%04d/%02d/%s%s", now.Year(), now.Month(), hex.EncodeToString(randomBytes), ext), nil
}

func safeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return "image"
	}
	return name
}

func readImagePart(c *gin.Context) ([]byte, string, string, error) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, "", "", errors.New("request must be multipart/form-data")
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", "", errors.New("failed to read multipart body")
		}
		if part.FormName() != "image" || part.FileName() == "" {
			part.Close()
			continue
		}

		data, err := io.ReadAll(io.LimitReader(part, maxImageSize+1))
		part.Close()
		if err != nil {
			return nil, "", "", errors.New("failed to read image")
		}
		if len(data) == 0 {
			return nil, "", "", errors.New("image must not be empty")
		}
		if len(data) > maxImageSize {
			return nil, "", "", fmt.Errorf("image must not exceed %d MiB", maxImageSize>>20)
		}

		contentType := http.DetectContentType(data)
		if !strings.HasPrefix(contentType, "image/") {
			return nil, "", "", errors.New("uploaded file is not a supported image")
		}
		return data, safeFileName(part.FileName()), contentType, nil
	}

	return nil, "", "", errors.New("multipart field \"image\" is required")
}

func scanImage(scanner interface{ Scan(...any) error }) (Image, error) {
	var image Image
	err := scanner.Scan(
		&image.ID,
		&image.OriginalName,
		&image.ObjectKey,
		&image.ContentType,
		&image.Size,
		&image.ETag,
		&image.CreatedAt,
	)
	if err == nil {
		image.DownloadURL = fmt.Sprintf("/images/%d/download", image.ID)
	}
	return image, err
}

func uploadImage(c *gin.Context) {
	data, originalName, contentType, err := readImagePart(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	objectKey, err := newObjectKey(originalName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate object key"})
		return
	}
	bucket, err := openImageBucket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 binding is unavailable"})
		return
	}
	object, err := bucket.Put(objectKey, io.NopCloser(bytes.NewReader(data)), &r2.PutOptions{
		HTTPMetadata: r2.HTTPMetadata{
			ContentType: contentType,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store image"})
		return
	}
	if object == nil {
		_ = bucket.Delete(objectKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 did not return stored object metadata"})
		return
	}

	db, err := openDB()
	if err != nil {
		_ = bucket.Delete(objectKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is unavailable"})
		return
	}
	defer db.Close()

	image, err := scanImage(db.QueryRowContext(c.Request.Context(), `
		INSERT INTO images (original_name, object_key, content_type, size, etag)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, original_name, object_key, content_type, size, etag, created_at`,
		originalName, objectKey, contentType, len(data), object.ETag,
	))
	if err != nil {
		_ = bucket.Delete(objectKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image metadata"})
		return
	}

	c.JSON(http.StatusCreated, image)
}

func listImages(c *gin.Context) {
	db, err := openDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is unavailable"})
		return
	}
	defer db.Close()

	rows, err := db.QueryContext(c.Request.Context(), `
		SELECT id, original_name, object_key, content_type, size, etag, created_at
		FROM images ORDER BY id DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query images"})
		return
	}
	defer rows.Close()

	images := make([]Image, 0)
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image metadata"})
			return
		}
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image metadata"})
		return
	}

	c.JSON(http.StatusOK, images)
}

func downloadImage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image id"})
		return
	}

	db, err := openDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is unavailable"})
		return
	}
	defer db.Close()

	image, err := scanImage(db.QueryRowContext(c.Request.Context(), `
		SELECT id, original_name, object_key, content_type, size, etag, created_at
		FROM images WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query image"})
		return
	}

	bucket, err := openImageBucket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 binding is unavailable"})
		return
	}
	object, err := bucket.Get(image.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
		return
	}
	if object == nil || object.Body == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image object not found"})
		return
	}
	if closer, ok := object.Body.(io.Closer); ok {
		defer closer.Close()
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": image.OriginalName})
	c.Header("Content-Type", image.ContentType)
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Length", strconv.FormatInt(image.Size, 10))
	c.Header("X-Content-Type-Options", "nosniff")
	if image.ETag != "" {
		c.Header("ETag", `"`+strings.Trim(image.ETag, `"`)+`"`)
	}
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, object.Body); err != nil {
		return
	}
}

func createMessage(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must not be empty"})
		return
	}

	db, err := openDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer db.Close()

	var msg Message
	err = db.QueryRowContext(c.Request.Context(),
		"INSERT INTO messages (content) VALUES (?) RETURNING id, content, create_time",
		content,
	).Scan(&msg.ID, &msg.Content, &msg.CreateTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func listMessages(c *gin.Context) {
	db, err := openDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer db.Close()

	rows, err := db.QueryContext(c.Request.Context(),
		"SELECT id, content, create_time FROM messages ORDER BY id ASC",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.Content, &msg.CreateTime); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func latestMessage(c *gin.Context) {
	db, err := openDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer db.Close()

	var msg Message
	err = db.QueryRowContext(c.Request.Context(),
		"SELECT id, content, create_time FROM messages ORDER BY id DESC LIMIT 1",
	).Scan(&msg.ID, &msg.Content, &msg.CreateTime)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no messages yet"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, msg)
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/messages", createMessage)
	router.GET("/messages", listMessages)
	router.GET("/messages/latest", latestMessage)
	router.POST("/images", uploadImage)
	router.GET("/images", listImages)
	router.GET("/images/:id/download", downloadImage)

	workers.Serve(router)
}
