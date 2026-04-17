package interceptor

import (
	"context"
	"log/slog"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggerInterceptor создает серверный унарный интерцептор, который логирует
// информацию о времени выполнения методов gRPC сервера.
func LoggerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Извлекаем имя метода из полного пути
		method := path.Base(info.FullMethod)

		// Логируем начало вызова метода
		slog.Info("начало gRPC метода", "method", method)

		// Засекаем время начала выполнения
		startTime := time.Now()

		// Вызываем обработчик
		resp, err := handler(ctx, req)

		// Вычисляем длительность выполнения
		duration := time.Since(startTime)

		// Форматируем сообщение в зависимости от результата
		if err != nil {
			st, _ := status.FromError(err)
			slog.Error("gRPC метод завершён с ошибкой", "method", method, "code", st.Code(), "error", err, "duration", duration)
		} else {
			slog.Info("gRPC метод завершён успешно", "method", method, "duration", duration)
		}

		return resp, err
	}
}
