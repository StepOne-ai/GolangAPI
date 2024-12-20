package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	uploadDir = "uploads"
)

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func main() {
	router := gin.Default()

	router.POST("/upload", uploadImage)
	router.GET("/files", getFiles)
	router.GET("/download/:filename", serveDownload)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Println("Starting server.")
	log.Fatal(router.Run("0.0.0.0" + ":" + port))
}

func getFiles(c *gin.Context) {
	files, err := listFiles(uploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to list files"})
		return
	}

	c.JSON(http.StatusOK, files)
}

func uploadImage(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing multipart form"})
		return
	}

	response, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving file"})
		return
	}

	file, err := response.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving file"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(response.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".bmp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only JPG, JPEG, PNG, GIF, BMP formats are allowed"})
		return
	}

	filename := fmt.Sprintf("image-%s%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))), ext)

	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving the file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error copying file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image uploaded successfully", "filename": filename})
}

func serveDownload(c *gin.Context) {
	filename := c.Param("filename")

	filePath := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filename)))
	c.Header("Content-Type", getMimeType(filePath))
	c.File(filePath)
}

func getMimeType(filePath string) string {
	// Get the file extension
	ext := filepath.Ext(filePath)

	// Get the MIME type from the extension
	mimeType := mime.TypeByExtension(ext)

	// If the MIME type is not found, return a default value
	if mimeType == "" {
		return "application/octet-stream"
	}

	return mimeType
}

func listFiles(directory string) ([]FileInfo, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	var fileInfoList []FileInfo

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			return nil, err
		}

		fileInfoList = append(fileInfoList, FileInfo{
			Name: file.Name(),
			Size: info.Size(),
		})
	}

	return fileInfoList, nil
}
