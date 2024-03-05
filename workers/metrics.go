package workers

import (
	"context"
	"log"

	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/utils"
)

var (
	cancelMetrics context.CancelFunc
)

func StartMetrics(networkDev string) {
	ctx, c := context.WithCancel(context.Background())
	cancelMetrics = c
	go trackCpu(ctx)
	go trackNetwork(networkDev, ctx)
}

func StopMetrics() {
	cancelMetrics()
}

func trackCpu(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[trackCpu] stopped")
			return
		default:
			// sleeps automatically
			cpu, err := utils.CpuUsage(30)
			if err != nil {
				log.Printf("[trackCpu] Error reasing cpu: %v", err)
				return
			}

			if err := database.Db.Model(&utils.CPULoad{}).Create(cpu.LoadCpu).Error; err != nil {
				log.Printf("[trackCpu] Error saving metric: %v", err)
			}
		}
	}
}

func trackNetwork(networkDev string, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[trackNetwork] stopped")
			return
		default:
			netInfo, err := utils.NetMeasure(networkDev, 15)
			if err != nil {
				log.Println("[trackNetwork] stopped")
				return
			}
			if err := database.Db.Model(&utils.NetInfo{}).Create(netInfo).Error; err != nil {
				log.Printf("[trackCpu] Error saving metric: %v", err)
			}
		}
	}
}
