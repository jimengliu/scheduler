package service

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	rancherClient "github.com/rancher/go-rancher/client"
	"github.com/rancher/scheduler/cache"
	schedulerClient "github.com/rancher/scheduler/client"
)

func TimerWorker() {
	log.Info("TimerWorker started")
	const period = 30 * time.Second

	for {
		time.Sleep(period)
		timeNow := time.Now()

		// check if there is an update to any host in the cache manager,
		// if so, update the host on cattle side using cattle API
		if cache.Manager.Envs == nil || len(cache.Manager.Envs) == 0 {
			continue
		}
		for _, env := range cache.Manager.Envs {
			if env.HostInfos == nil || len(env.HostInfos) == 0 {
				continue
			}
			for hostId, hostInfo := range env.HostInfos {
				duration := timeNow.Sub(hostInfo.TimeLastAllocated)

				// any allocation happend in past period ?
				if duration <= period {
					log.Infof("some resource allcation happened in past period: %d seconds, on host id: %s", period/time.Second, hostId)
					err := updateHostAllocationStats(hostInfo)
					if err != nil {
						log.Errorf("can't update cattle host hostId: %s with allocation change, error: %s", hostId, err.Error())
					} else {
						log.Infof("updated cattle host hostId: %s using newly allocation changes", hostId)
					}
				}
			}
		}
	}
}

func updateHostAllocationStats(hostInfo *cache.HostInfo) error {
	// create a new map for allocatedInfo field inside cattle host.Info field
	allocatedInfo := make(map[string]interface{})
	allocatedInfo["cpuUsed"] = hostInfo.CpuUsed
	allocatedInfo["memUsedInMB"] = hostInfo.MemUsedInMB
	diskAllocatedInfo := make(map[string]interface{})

	diskInfo, ok := hostInfo.Disks[cache.DefaultDiskPath]
	if !ok {
		log.Info("no disk on host for disk path:", cache.DefaultDiskPath)
	} else {
        readWriteIopsAllocated := make(map[string]interface{})
		readWriteIopsAllocated["readIopsAllocated"] = diskInfo.Iops.ReadAllocated
		readWriteIopsAllocated["writeIopsAllocated"] = diskInfo.Iops.WriteAllocated
        diskAllocatedInfo[diskInfo.DevicePath] = readWriteIopsAllocated
	}
	allocatedInfo["iopsUsed"] = diskAllocatedInfo

	// get the cattle host object
	host, err := schedulerClient.GetHost(hostInfo.HostId)
	if err != nil {
		return err
	}

	// get a new value of Info field of the host API
	info, ok := host.Info.(map[string]interface{})
	if !ok {
		return fmt.Errorf("updateHostAllocationStats: can't get host.Info from cattle host object, hostId: %s", hostInfo.HostId)
	}

	// get the old allocatedInfo for logging purpose
	oldInfoInterface, ok := info["allocatedInfo"]
	if ok {
		oldInfo, ok := oldInfoInterface.(map[string]interface{})
		if ok {
			_, ok := oldInfo["cpuUsed"]
			if ok {
				cpuUsed, ok := oldInfo["cpuUsed"].(float64)
				if ok {
					log.Infof("before udpate, cpuUsed: %f. After update, it should be: %f", cpuUsed, allocatedInfo["cpuUsed"])
				}
			}
			_, ok = oldInfo["memUsedInMB"]
			if ok {
				memUsedInMB, ok := oldInfo["memUsedInMB"].(float64)
				if ok {
					log.Infof("before udpate, memUsedInMB used: %f. After update, it should be: %f", memUsedInMB, allocatedInfo["memUsedInMB"])
				}
			}
			_, ok = oldInfo["iopsUsed"]
			if ok {
				iopsUsedMapMap, ok := oldInfo["iopsUsed"].(map[string]interface{})
				if ok {
                    for devicePath, iopsUsedMap := range iopsUsedMapMap {
                        oldIopsUsed := iopsUsedMap.(map[string]interface{})
                        oldRead := oldIopsUsed["readIopsAllocated"].(float64)
                        oldWrite := oldIopsUsed["writeIopsAllocated"].(float64)
                        log.Infof("devicePath: %s, before allocated readIopsAllocated: %f, writeIopsAllocated: %f. After update readIopsAllocated: %d, writeIopsAllocated: %d",
                            devicePath, oldRead, oldWrite, diskInfo.Iops.ReadAllocated, diskInfo.Iops.WriteAllocated)
                    }
				}
			}
		}
	}
	info["allocatedInfo"] = allocatedInfo
	err = schedulerClient.UpdateHost(host, &rancherClient.Host{Info: info})

	return err
}
