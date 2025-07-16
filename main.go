// Package main is the entry point for the Ponder application.
package main

import (
	"fmt"
	"ponder/pkg/api"
	"ponder/pkg/clientside"
	"ponder/pkg/generate"
	"ponder/pkg/models"
	"ponder/pkg/utils"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	_, err := models.LoadConfig()
	if err != nil {
		fmt.Println(fmt.Errorf("Error loading config: %v", err))
	}

	utils.LogInternalEvent("Server started", "Performing initial setup.")
	utils.MakeFileIfNotExist(models.SourceWordlist)
	utils.MakeFileIfNotExist(models.WizardWordlist)

	waitTime := 15 * time.Minute
	ticker := time.NewTicker(waitTime)
	go func() {
		time.Sleep(15 * time.Second)
		utils.LogInternalEvent("Starting updater", fmt.Sprintf("Starting the updater with a %v interval.", waitTime))
		for {
			select {
			case <-ticker.C:
				// If there has been an update since the last time the wordlist
				// was updated, update the wordlist
				if models.LastUploaded.After(models.LastUpdated) {
					overallStartTime := time.Now()
					currentProcessStartTime := time.Now()
					overallEndTime := time.Time{}
					currentProcessEndTime := time.Time{}

					utils.LogInternalEvent("Starting a wordlist update", fmt.Sprintf("Last uploaded %v.", models.LastUploaded))
					api.Mu.Lock()
					currentProcessStartTime = time.Now()
					utils.LogInternalEvent("Creating wizard wordlist", fmt.Sprintf("Generating %v.", models.WizardWordlist))
					generate.CreateWizardWordlist(models.SourceWordlist, models.WizardWordlist)
					currentProcessEndTime = time.Now()
					utils.LogInternalEvent("Wizard wordlist created", fmt.Sprintf("Duration: %v.", currentProcessEndTime.Sub(currentProcessStartTime)))
					api.Mu.Unlock()
					models.LastUpdated = time.Now()
					overallEndTime = time.Now()
					utils.LogInternalEvent("Wordlist update complete", fmt.Sprintf("Duration: %v.", overallEndTime.Sub(overallStartTime)))
				}
			}
		}
	}()
}

func main() {
	ginRouter := gin.Default()
	ginRouter.SetTrustedProxies([]string{""})
	ginRouter.Static("/static/css", "/etc/ponder/static/css")
	ginRouter.Static("/static/js", "/etc/ponder/static/js")
	ginRouter.Static("/static/img", "/etc/ponder/static/img")
	ginRouter.LoadHTMLGlob("/etc/ponder/static/*.html")

	public := ginRouter.Group("/")
	publicAPI := ginRouter.Group("/api")

	public.GET("/", clientside.ClientIndexHandler)
	publicAPI.GET("/ping", api.PingHandler)
	publicAPI.GET("/event-log", api.EventLogHandler)
	publicAPI.POST("/upload", api.UploadHandler)
	publicAPI.GET("/download/:n", api.DownloadHandler)
	publicAPI.POST("/import", api.ImportHandler)

	err := ginRouter.Run(":8080")
	if err != nil {
		fmt.Println(err)
	}
}
