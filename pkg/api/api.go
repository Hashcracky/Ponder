// Package api contains the handlers for the API endpoints
package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"ponder/pkg/models"
	"ponder/pkg/utils"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Mu is a mutex for synchronizing file writes
var Mu sync.Mutex

// PingHandler is a handler for GET /api/ping
//
// Args:
// c (gin.Context): Gin context
//
// Returns:
// None
func PingHandler(c *gin.Context) {
	startTime := time.Now()

	c.JSON(http.StatusOK, gin.H{
		"duration":     time.Since(startTime).String(),
		"last-updated": models.LastUpdated,
	})
}

// UploadHandler is a handler for POST /api/upload
//
// Args:
// c (gin.Context): Gin context
//
// Returns:
// None
func UploadHandler(c *gin.Context) {
	startTime := time.Now()

	Mu.Lock()
	defer Mu.Unlock()

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "Bad Request",
			"duration": time.Since(startTime).String(),
		})
		return
	}
	defer file.Close()

	targetFile, err := os.OpenFile(models.SourceWordlist, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		utils.LogInternalEvent("Error opening file in upload handler", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Internal Server Error",
			"duration": time.Since(startTime).String(),
		})
		return
	}
	defer targetFile.Close()

	fileInfo, err := targetFile.Stat()
	if err != nil {
		utils.LogInternalEvent("Error getting file info in upload handler", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Internal Server Error",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	if fileInfo.Size() > 0 {
		if _, err := targetFile.Write([]byte("\n")); err != nil {
			utils.LogInternalEvent("Error writing to file in upload handler", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "Internal Server Error",
				"duration": time.Since(startTime).String(),
			})
			return
		}
	}

	buffer := make([]byte, 4*1024*1024)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			utils.LogInternalEvent("Error reading file in upload handler", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "Internal Server Error",
				"duration": time.Since(startTime).String(),
			})
			return
		}
		if n == 0 {
			break
		}

		content := string(buffer[:n])
		lines := strings.Split(content, "\n")
		var transformedLines []string
		for _, line := range lines {
			convertedLine, err := models.ConvertHexToPlaintext(line)
			if err == nil {
				if utils.IsAllDigitsOrSpecialChars(convertedLine) || utils.ContainsOnlyASCII(convertedLine) == false || utils.LikelyContainsWords(convertedLine) == false || utils.IsQualityCandidateCheck(convertedLine) == false {
					continue
				}
				transformedLines = append(transformedLines, strings.TrimSpace(strings.ToLower(convertedLine)))
			} else {
				if utils.IsAllDigitsOrSpecialChars(line) || utils.ContainsOnlyASCII(line) == false || utils.LikelyContainsWords(line) == false || utils.IsQualityCandidateCheck(line) == false {
					continue
				}
				transformedLines = append(transformedLines, strings.TrimSpace(strings.ToLower(line)))
			}
		}
		updatedContent := strings.Join(transformedLines, "\n")

		if _, err := targetFile.Write([]byte(updatedContent)); err != nil {
			utils.LogInternalEvent("Error writing to file in upload handler", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "Internal Server Error",
				"duration": time.Since(startTime).String(),
			})
			return
		}
	}

	utils.LogInternalEvent("File uploaded successfully", fmt.Sprintf("Duration: %s", time.Since(startTime).String()))
	models.LastUploaded = time.Now()
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"duration": time.Since(startTime).String(),
	})
}

// DownloadHandler is a handler for GET /api/download/:n
//
// Args:
// c (gin.Context): Gin context
//
// Returns:
// None
func DownloadHandler(c *gin.Context) {
	startTime := time.Now()

	n := c.Param("n")
	if n == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "Bad Request",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	numberofLines, err := strconv.Atoi(n)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "Bad Request",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	substring := c.Query("substring")

	var lines []string
	if substring != "" {
		lines, err = utils.GetFirstNLines(models.WizardWordlist, numberofLines, substring)
	} else {
		lines, err = utils.GetFirstNLines(models.WizardWordlist, numberofLines)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Internal Server Error",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	if lines == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "Not Found",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	joinedLines := strings.Join(lines, "\n")
	reader := strings.NewReader(joinedLines)

	c.Header("Content-Type", "text/plain")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	buffer := make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return
		}
		if n == 0 {
			break
		}

		if _, err := c.Writer.Write(buffer[:n]); err != nil {
			return
		}
		c.Writer.Flush()
	}

	duration := time.Since(startTime).String()
	utils.LogInternalEvent("File downloaded successfully", fmt.Sprintf("Duration: %s", duration))
}

// EventLogHandler is a handler for GET /api/event-log
//
// Args:
// c (gin.Context): Gin context
//
// Returns:
// None
func EventLogHandler(c *gin.Context) {
	entries, err := utils.ReadLogEntries()
	if err != nil {
		utils.LogInternalEvent("Error reading log entries in event log handler", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal Server Error",
		})
		return
	}
	c.JSON(http.StatusOK,
		gin.H{
			"log_entries": entries,
		})
}

// ImportHandler is a handler for POST /api/import
//
// This handler imports all .txt files from the import directory
// and adds their contents to the source wordlist just like the upload handler.
//
// Args:
// c (gin.Context): Gin context
//
// Returns:
// None
func ImportHandler(c *gin.Context) {
	startTime := time.Now()

	Mu.Lock()
	defer Mu.Unlock()

	// Ensure the import directory exists and create it if it does not
	if _, err := os.Stat(models.ImportDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(models.ImportDirectory, os.ModePerm); err != nil {
			utils.LogInternalEvent("Error creating import directory", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "Internal Server Error",
				"duration": time.Since(startTime).String(),
			})
			return
		}
	}

	files, err := os.ReadDir(models.ImportDirectory)
	if err != nil {
		utils.LogInternalEvent("Error reading import directory", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Internal Server Error",
			"duration": time.Since(startTime).String(),
		})
		return
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
			// ensure the file is not the source wordlist or the wizard wordlist
			if file.Name() == models.SourceWordlist || file.Name() == models.WizardWordlist {
				continue
			}

			filePath := fmt.Sprintf("%s/%s", models.ImportDirectory, file.Name())
			err = appendFileToWordlist(filePath, models.SourceWordlist)
			if err != nil {
				utils.LogInternalEvent("Error appending file to wordlist in import handler", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":    "Internal Server Error",
					"duration": time.Since(startTime).String(),
				})
				return
			}
			// Remove the file after processing
			if err := os.Remove(fmt.Sprintf("%s/%s", models.ImportDirectory, file.Name())); err != nil {
				utils.LogInternalEvent("Error removing file after import", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":    "Internal Server Error",
					"duration": time.Since(startTime).String(),
				})
				return
			}
		}
	}

	models.LastUploaded = time.Now()
	utils.LogInternalEvent("Files imported successfully", fmt.Sprintf("Duration: %s", time.Since(startTime).String()))
	c.JSON(http.StatusOK, gin.H{
		"message":  "Files imported successfully",
		"duration": time.Since(startTime).String(),
	})
}

// appendFileToWordlist appends the contents of a file to the source wordlist
// It reads the file, processes its contents, and writes them to the target
// file in a manner similar to the UploadHandler.
//
// Args:
// filePath (string): Path to the source file to be appended
// targetFilePath (string): Path to the target wordlist file
//
// Returns:
// error: An error if any occurs during the process
func appendFileToWordlist(filePath, targetFilePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()

	targetFile, err := os.OpenFile(targetFilePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return fmt.Errorf("error opening target file %s: %w", targetFilePath, err)
	}
	defer targetFile.Close()

	buffer := make([]byte, 4*1024*1024)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading file %s: %w", filePath, err)
		}
		if n == 0 {
			break
		}

		content := string(buffer[:n])
		lines := strings.Split(content, "\n")
		var transformedLines []string
		for _, line := range lines {
			convertedLine, err := models.ConvertHexToPlaintext(line)
			if err == nil {
				if utils.IsAllDigitsOrSpecialChars(convertedLine) || utils.ContainsOnlyASCII(convertedLine) == false || utils.LikelyContainsWords(convertedLine) == false || utils.IsQualityCandidateCheck(convertedLine) == false {
					continue
				}
				transformedLines = append(transformedLines, strings.TrimSpace(strings.ToLower(convertedLine)))
			} else {
				if utils.IsAllDigitsOrSpecialChars(line) || utils.ContainsOnlyASCII(line) == false || utils.LikelyContainsWords(line) == false || utils.IsQualityCandidateCheck(line) == false {
					continue
				}
				transformedLines = append(transformedLines, strings.TrimSpace(strings.ToLower(line)))
			}
		}
		updatedContent := strings.Join(transformedLines, "\n")

		if _, err := targetFile.Write([]byte(updatedContent)); err != nil {
			return fmt.Errorf("error writing to target file %s: %w", targetFilePath, err)
		}
	}

	return nil
}
