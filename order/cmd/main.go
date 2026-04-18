package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	orderHandler "github.com/mercuryqa/order/pkg/handler"
	inventoryv1 "github.com/mercuryqa/shared/pkg/proto/inventory/v1"
	paymentv1 "github.com/mercuryqa/shared/pkg/proto/payment/v1"
)

const (
	httpPort = "8080"

	inventoryServiceAddress = "localhost:50051"
	paymentServiceAddress   = "localhost:50052"

	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
	middlewareTimeout = 10 * time.Second
)

func setupRouter(api http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(middlewareTimeout))

	// OpenAPI
	r.Mount("/api", api)

	return r
}

func grpcClientOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                1 * time.Minute,
			Timeout:             10 * time.Second,
			PermitWithoutStream: false,
		}),
	}
}

func main() {
	// Создать gRPC соединение с InventoryService
	inventoryConn, err := grpc.NewClient(
		inventoryServiceAddress,
		grpcClientOptions()...,
	)
	if err != nil {
		slog.Error("не удалось подключиться к InventoryService", "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()

	paymentConn, err := grpc.NewClient(
		paymentServiceAddress,
		grpcClientOptions()...,
	)
	if err != nil {
		slog.Error("не удалось подключиться к PaymentService", "error", err)
		os.Exit(1)
	}
	defer paymentConn.Close()

	// Создаём хранилище и обработчик
	store := orderHandler.NewOrderStore()
	h := orderHandler.NewOrderHandler(
		inventoryv1.NewInventoryServiceClient(inventoryConn),
		paymentv1.NewPaymentServiceClient(paymentConn),
		store,
	)

	// Создать OpenAPI сервер
	apiServer, err := orderHandler.SetupServer(h)
	if err != nil {
		slog.Error("ошибка создания сервера OpenAPI", "error", err)
		os.Exit(1)
	}

	handler := setupRouter(apiServer)

	server := &http.Server{
		Addr:              net.JoinHostPort("localhost", httpPort),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Контекст, который отменяется по SIGINT/SIGTERM или при падении сервера.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("HTTP-сервер запущен", "port", httpPort)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ошибка сервера", "error", err)
			cancel() // будим main, чтобы не висеть бесконечно
		}
	}()

	// Ждём сигнал от ОС или падение сервера.
	<-ctx.Done()

	slog.Info("остановка HTTP-сервера")

	shutdownCtx, stop := context.WithTimeout(context.Background(), shutdownTimeout)
	defer stop()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("ошибка при остановке сервера", "error", err)
	}

	slog.Info("сервер остановлен")
}
