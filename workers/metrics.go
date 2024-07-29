package workers

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/models"
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
			log.Infoln("[trackCpu] stopped")
			return
		default:
			// sleeps automatically
			cpu, err := helpers.CpuUsage(30)
			if err != nil {
				log.Errorf("[trackCpu] Error reasing cpu: %s", err)
				return
			}

			if err := models.Db.Model(&helpers.CPULoad{}).Create(cpu.LoadCpu).Error; err != nil {
				log.Errorf("[trackCpu] Error saving metric: %s", err)
			}
		}
	}
}

func trackNetwork(networkDev string, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[trackNetwork] stopped")
			return
		default:
			netInfo, err := helpers.NetMeasure(networkDev, 15)
			if err != nil {
				log.Errorln("[trackNetwork] stopped")
				return
			}
			if err := models.Db.Model(&helpers.NetInfo{}).Create(netInfo).Error; err != nil {
				log.Errorf("[trackCpu] Error saving metric: %s", err)
			}
		}
	}
}
