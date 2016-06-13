package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/scheduler/service"
	"net/http"
)

func main() {
	log.Infof("Starting Rancher Scheduler service")
	router := service.NewRouter()
	handler := service.MuxWrapper{true, router}
	go service.TimerWorker()
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 8090), &handler))
}
