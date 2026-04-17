package interceptor

import (
	"context"
	"log/slog"
	"path"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor создает серверный унарный интерцептор, который перехватывает
// паники в обработчиках и конвертирует их в gRPC ошибку Internal.
// Без этого интерцептора паника в обработчике уронит весь сервер.
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				method := path.Base(info.FullMethod)
				slog.Error("паника в gRPC методе",
					"method", method,
					"panic", r,
					"stack", string(debug.Stack()),
				)

				err = status.Errorf(codes.Internal, "внутренняя ошибка сервера")
			}
		}()

		return handler(ctx, req)
	}
}
