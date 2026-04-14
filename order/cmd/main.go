package main

import (
	"log/slog"
	"net/http"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orderHandler "github.com/mercuryqa/order/pkg/handler"
	inventoryv1 "github.com/mercuryqa/shared/pkg/proto/inventory/v1"
	paymentv1 "github.com/mercuryqa/shared/pkg/proto/payment/v1"
)

const (
	inventoryServiceAddress = "localhost:50051"
	paymentServiceAddress   = "localhost:50052"
)

func main() {
	// TODO: Настроить gRPC клиент с параметрами keepalive
	// Подумайте, какие параметры стоит задать для gRPC клиента
	// См. examples/week_1/GRPC_CONNECTIONS.md

	// Создать gRPC соединение с InventoryService
	inventoryConn, err := grpc.NewClient(inventoryServiceAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("не удалось подключиться к InventoryService", "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()

	// TODO: Создать gRPC клиент PaymentService

	// Создаём хранилище и обработчик
	store := orderHandler.NewOrderStore()
	h := orderHandler.NewOrderHandler(
		inventoryv1.NewInventoryServiceClient(inventoryConn),
		paymentv1.NewPaymentServiceClient(paymentConn),
		store,
	)

	// TODO: Сгенерировать код ogen из OpenAPI спецификации
	// Команда: task ogen:gen

	// Создать OpenAPI сервер
	orderServer, err := orderHandler.SetupServer(h)
	if err != nil {
		slog.Error("ошибка создания сервера OpenAPI", "error", err)
		os.Exit(1)
	}

	// TODO: Настроить HTTP сервер с таймаутами
	// Подумайте, какие таймауты стоит задать для production-ready сервера
	// См. examples/week_1/HTTP_SERVER.md

	// TODO: Реализовать graceful shutdown для HTTP сервера
	// При получении сигнала SIGINT/SIGTERM сервер должен:
	// 1. Перестать принимать новые соединения
	// 2. Дождаться завершения текущих запросов (с таймаутом)
	// 3. Закрыть gRPC соединения
	// 4. Корректно завершить работу
	// Подсказка: используйте signal.Notify и httpServer.Shutdown(ctx)

	slog.Info("запуск OrderService", "port", 8080)

	err = http.ListenAndServe(":8080", orderServer)
	if err != nil {
		slog.Error("ошибка запуска сервера", "error", err)
		os.Exit(1)
	}
}
