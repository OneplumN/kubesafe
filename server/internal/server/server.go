package server

import (
	"fmt"

	"github.com/OneplumN/kubesafe/server/internal/buildinfo"
	"github.com/OneplumN/kubesafe/server/internal/config"
	"github.com/OneplumN/kubesafe/server/internal/kube"
	"github.com/OneplumN/kubesafe/server/internal/service"
	"github.com/OneplumN/kubesafe/server/internal/web"
)

func Run(info buildinfo.Info) error {
	cfg := config.Load()

	clusterFactory, err := kube.NewFactory(cfg.KubeconfigPath, cfg.Mode)
	if err != nil {
		return fmt.Errorf("initialize kubernetes client factory: %w", err)
	}

	if info.IsRelease() && !web.HasEmbeddedFrontend() {
		return fmt.Errorf("release build requires embedded frontend assets under server/internal/web/dist/app")
	}

	updateService := service.NewUpdateService(info, cfg.Update, web.HasEmbeddedFrontend())
	systemLockService := service.NewSystemOperationLockService()
	router := newRouter(clusterFactory, updateService, systemLockService, info)
	return router.Run(cfg.HTTPAddr)
}
