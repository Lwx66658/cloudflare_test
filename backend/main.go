package main

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/syumai/workers-go"
	"github.com/syumai/workers-go/cloudflare/d1"
)

type Message struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	CreateTime string `json:"create_time"`
}

func openDB() (*sql.DB, error) {
	connector, err := d1.OpenConnector("DB")
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
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

	workers.Serve(router)
}
