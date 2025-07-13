package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/airshhlay/va-booking-bot/internal/util"
	"github.com/airshhlay/va-booking-bot/internal/va"
	"github.com/go-co-op/gocron"
)

const (
	// everyday at 855pm
	_bookingTimeDaily  = "0 20 * * *"
	_refreshTokenDaily = "0 18 * * *"
)

func main() {
	// server := gin.Default()
	// server.GET("/", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{
	// 		"message": "Hello, World!",
	// 	})
	// })
	// server.GET("/ping", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{
	// 		"message": "pong",
	// 	})
	// })
	// server.Run(":8080")
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile)

	loc, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	scheduler := gocron.NewScheduler(loc)
	scheduler.Cron(_refreshTokenDaily).Do(func() {
		_, err := va.GetToken(context.Background())
		if err != nil {
			log.Fatalf("get token: err %v", err)
		}
	})
	scheduler.Cron(_bookingTimeDaily).Do(func() {
		err := va.BookClass(context.Background(), va.BookClassParams{
			SiteID:          va.SiteIDPayaLebar,
			ClassTime24Hour: "11:30",
			ClassDay:        int(time.Sunday),
			ClassName:       va.ClassNameCycleSpirit,
		})
		if err != nil {
			log.Fatalf("book class: time: %v, err %v", util.CurrentTimeInSG(), err)
		}
	})

	scheduler.StartBlocking()
}
