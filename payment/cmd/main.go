package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/mercuryqa/payment/internal/interceptor"
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

		// Интерцепторы: recovery (перехват паник) + логирование запросов
		grpc.ChainUnaryInterceptor(
			interceptor.RecoveryInterceptor(),
			interceptor.LoggerInterceptor(),
		),
	)

	paymentv1.RegisterPaymentServiceServer(grpcServer, &svc.PaymentServer{})

	// Включаем reflection для postman/grpcurl
	reflection.Register(grpcServer)

	// Контекст, который отменяется по SIGINT/SIGTERM или при падении сервера.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("gRPC сервер запущен", "address", grpcAddress)
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			slog.Error("ошибка запуска сервера", "error", serveErr)
			cancel() // будим main, чтобы не висеть бесконечно
		}
	}()

	// Ждём сигнал от ОС или падение сервера.
	<-ctx.Done()
	slog.Info("остановка gRPC сервера")
	grpcServer.GracefulStop()
	slog.Info("сервер остановлен")
}
