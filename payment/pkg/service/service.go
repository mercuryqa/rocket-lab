package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	paymentv1 "github.com/mercuryqa/shared/pkg/proto/payment/v1"
)

// PaymentServer реализует gRPC сервис оплаты.
type PaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
}

// PayOrder обрабатывает оплату заказа.
func (s *PaymentServer) PayOrder(
	ctx context.Context,
	req *paymentv1.PayOrderRequest,
) (*paymentv1.PayOrderResponse, error) {
	orderUUID := req.GetOrderUuid()
	// 1. Проверить, что order_uuid не пустой → INVALID_ARGUMENT
	if orderUUID == "" {
		return nil, status.Error(codes.InvalidArgument, "order_uuid пустой")
	}
	// 2. Проверить, что payment_method != UNSPECIFIED → INVALID_ARGUMENT
	if req.PaymentMethod == paymentv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "payment_method UNSPECIFIED")
	}
	// 3. Проверить формат UUID → INVALID_ARGUMENT
	if _, err := uuid.Parse(orderUUID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный формат order_uuid")
	}
	// 4. Сгенерировать transaction_uuid (UUID v4)
	transactionUUID := uuid.New()
	// 5. Вывести в лог: "оплата прошла успешно, order_uuid: X, transaction_uuid: Y"
	slog.Info("оплата прошла успешно",
		"order_uuid", req.GetOrderUuid(),
		"transaction_uuid", transactionUUID,
	)
	// 6. Вернуть transaction_uuid
	return &paymentv1.PayOrderResponse{
		TransactionUuid: transactionUUID.String(),
	}, nil
}
