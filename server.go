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

	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	uploadDir = "uploads"
)

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type jwtCustomClaims struct {
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
	jwt.RegisteredClaims
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/login", login)

	config := echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(jwtCustomClaims)
		},
		SigningKey: []byte("secret"),
	}

	r := e.Group("/restricted")

	r.Use(echojwt.WithConfig(config))

	// Routes
	r.POST("/upload", uploadImage)
	r.GET("/files", getFiles)
	r.GET("/download/:filename", serveDownload)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Starting server on port %s", port)
	e.Start(":8080")
}

func login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Throws unauthorized error
	if username != "admin" || password != "admin" {
		return echo.ErrUnauthorized
	}

	// Set custom claims
	claims := &jwtCustomClaims{
		"admin",
		true,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString([]byte("secret"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"token": t,
	})
}

func uploadImage(c echo.Context) error {
	// Parse multipart form
	err := c.Request().ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		return c.String(http.StatusBadRequest, "Error parsing multipart form\n")
	}

	file, header, err := c.Request().FormFile("image")
	if err != nil {
		return c.String(http.StatusBadRequest, "Error retrieving file\n")
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".bmp" {
		return c.String(http.StatusBadRequest, "Only JPG, JPEG, PNG, GIF, BMP formats are allowed\n")
	}

	filename := fmt.Sprintf("image-%s%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))), ext)

	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error saving the file\n")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return c.String(http.StatusInternalServerError, "Error copying file\n")
	}

	return c.String(http.StatusOK, fmt.Sprintf("Image uploaded successfully. Filename: %s\n", filename))
}

func getFiles(c echo.Context) error {
	files, err := listFiles(uploadDir)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Unable to list files\n")
	}

	return c.JSON(http.StatusOK, files)
}

func serveDownload(c echo.Context) error {
	filename := c.Param("filename")

	filePath := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "File not found\n")
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filename)))
	c.Response().Header().Set("Content-Type", getMimeType(filePath))
	return c.File(filePath)
}

func getMimeType(filePath string) string {
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
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
