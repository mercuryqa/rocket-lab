package main

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	svc "github.com/mercuryqa/payment/pkg/service"
	paymentv1 "github.com/mercuryqa/shared/pkg/proto/payment/v1"
)

const grpcAddress = ":50052"

func main() {
	lis, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		slog.Error("не удалось создать listener", "error", err)
		os.Exit(1)
	}

	// TODO: Настроить gRPC сервер с параметрами keepalive
	// Подумайте, какие параметры стоит задать для production-ready сервера
	// См. examples/week_1/GRPC_CONNECTIONS.md
	kaParams := keepalive.ServerParameters{
		MaxConnectionIdle:     5 * time.Minute,  // закрыть idle соединение
		MaxConnectionAge:      30 * time.Minute, // максимальное время жизни соединения
		MaxConnectionAgeGrace: 5 * time.Minute,  // grace-период перед закрытием
		Time:                  1 * time.Minute,  // как часто сервер пингует клиента
		Timeout:               10 * time.Second, // сколько ждать ответа на ping
	}

	kaPolicy := keepalive.EnforcementPolicy{
		MinTime:             1 * time.Minute, // минимальный интервал между ping от клиента
		PermitWithoutStream: false,           // запрещаем ping без активных RPC
	}

	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(kaParams),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
	)
	paymentv1.RegisterPaymentServiceServer(grpcServer, &svc.PaymentServer{})

	// Включаем reflection для postman/grpcurl
	reflection.Register(grpcServer)

	slog.Info("запуск PaymentService", "адрес", grpcAddress)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				slog.Error("ошибка запуска сервера", "error", err)
				os.Exit(1)
			}
		}
	}()

	shutdownTimeout := 10 * time.Second

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("завершение работы gRPC сервера...")

	done := make(chan struct{})

	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("gRPC сервер остановлен корректно")
	case <-time.After(shutdownTimeout):
		slog.Warn("graceful shutdown timeout, принудительная остановка")
		grpcServer.Stop()
	}
}
