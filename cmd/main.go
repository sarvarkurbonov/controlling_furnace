package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"controlling_furnace/internal/handlers"
	"controlling_furnace/internal/logger"
	"controlling_furnace/internal/repository"
	"controlling_furnace/internal/server"
	"controlling_furnace/internal/service"

	"github.com/spf13/viper"
)

const defaultSimTick = 1 * time.Second

func main() {
	// init logger
	log := logger.Get(logger.InfoLevel)

	// load config.yml
	if err := loadConfig(); err != nil {
		log.Fatalw("error reading config", "err", err)
	}

	// open DB
	db, err := openDB(log)
	if err != nil {
		log.Fatalw("failed to init sqlite", "err", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Fatalw("failed to close sqlite", "err", cerr)
		}
	}()

	// wire dependencies
	repos := repository.NewRepository(db)
	services := service.NewService(repos)
	apiHandler := handlers.NewHandler(services, log)

	// context for background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start simulator (via composed service)
	go services.Simulator.Run(ctx, defaultSimTick)

	// start HTTP server
	srv := &server.Server{}
	runHTTPServer(srv, viper.GetString("port"), apiHandler, log)

	// graceful shutdown
	waitForShutdown(cancel, srv, log)
}

// ... existing code ...

func loadConfig() error {
	viper.AddConfigPath("configs") // configs/config.yml
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}

// openDB initializes the SQLite database using configuration.
func openDB(log *logger.Logger) (*sql.DB, error) {
	dbPath := viper.GetString("db.path")
	if dbPath == "" {
		log.Infow("db.path not set in config; using default file", "default", "app.db")
		dbPath = "app.db"
	}
	return repository.InitDB(dbPath)
}

// runHTTPServer runs the HTTP server in a separate goroutine.
func runHTTPServer(srv *server.Server, port string, handler *handlers.Handler, log *logger.Logger) {
	go func() {
		if port == "" {
			port = "8080"
		}
		if err := srv.Run(port, handler.InitRoutes()); err != nil {
			log.Fatalw("error starting server", "err", err)
		}
	}()
}

// waitForShutdown listens for termination signals and performs graceful shutdown.
func waitForShutdown(cancel context.CancelFunc, srv *server.Server, log *logger.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Infow("shutting down server...")

	// stop background goroutines
	cancel()

	// allow in-flight requests to complete
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalw("server forced to shutdown", "err", err)
	}
}
