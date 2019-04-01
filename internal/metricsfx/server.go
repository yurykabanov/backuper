package metricsfx

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/yurykabanov/backuper/pkg/http/middleware"
)

const (
	ConfigServerAddress      = "server.address"
	ConfigServerTimeoutRead  = "server.timeout.read"
	ConfigServerTimeoutWrite = "server.timeout.write"
	ConfigServerLogRequests  = "server.log.requests"
)

type HttpServerConfig struct {
	Address           string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	EnableRequestsLog bool
}

func HttpServerConfigProvider(v *viper.Viper) (*HttpServerConfig, error) {
	return &HttpServerConfig{
		Address:           v.GetString(ConfigServerAddress),
		ReadTimeout:       v.GetDuration(ConfigServerTimeoutRead),
		WriteTimeout:      v.GetDuration(ConfigServerTimeoutWrite),
		EnableRequestsLog: v.GetBool(ConfigServerLogRequests),
	}, nil
}

func HttpServer(
	config *HttpServerConfig,
	logger *logrus.Logger,
	defaultLogger *log.Logger,
	router *mux.Router,
) (*http.Server, error) {
	var h http.Handler = router

	if config.EnableRequestsLog {
		h = middleware.WithRequestLogging(router, logger)
	}

	h = middleware.WithRequestId(h, middleware.DefaultRequestIdProvider)

	return &http.Server{
		Addr:         config.Address,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		ErrorLog:     defaultLogger,
		Handler:      h,
	}, nil
}

func HttpRouter() (*mux.Router, error) {
	return mux.NewRouter(), nil
}

func Listener(config *HttpServerConfig) (net.Listener, error) {
	return net.Listen("tcp", config.Address)
}

func RunServer(lc fx.Lifecycle, listener net.Listener, server *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go server.Serve(listener)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
