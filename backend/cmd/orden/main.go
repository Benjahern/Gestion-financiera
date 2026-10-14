package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orden/backend/internal/orden"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command != "serve" && command != "migrate" && command != "activation-code" && command != "recovery-code" {
		return fmt.Errorf("comandos: serve | migrate | activation-code | recovery-code")
	}
	config, err := orden.LoadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := orden.Open(ctx, config.DatabaseURL)
	if err != nil {
		cancel()
		return err
	}
	defer db.Close()
	if err = orden.Migrate(ctx, db); err != nil {
		cancel()
		return fmt.Errorf("no se pudieron aplicar las migraciones")
	}
	if err = orden.ValidateOwner(ctx, db, config.OwnerEmail); err != nil {
		cancel()
		return err
	}
	cancel()
	if command == "migrate" {
		fmt.Println("Migraciones aplicadas")
		return nil
	}
	if command == "activation-code" || command == "recovery-code" {
		purpose := "activate"
		if command == "recovery-code" {
			purpose = "recover"
		}
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		code, e := orden.IssueCode(ctx, db, purpose)
		if e != nil {
			return e
		}
		fmt.Printf("Código de un solo uso (caduca en 15 minutos):\n%s\n", code)
		return nil
	}
	app := orden.NewServer(db, config)
	workerContext, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go app.RunGmail(workerContext)
	go app.RunMaintenance(workerContext)
	server := &http.Server{Addr: config.Address, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stop)
	go func() {
		<-stop
		stopWorker()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	slog.Info("Orden backend listo")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("no se pudo iniciar HTTP")
	}
	return nil
}
