package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	orderv1 "github.com/mercuryqa/shared/pkg/openapi/order/v1"
	inventoryv1 "github.com/mercuryqa/shared/pkg/proto/inventory/v1"
	paymentv1 "github.com/mercuryqa/shared/pkg/proto/payment/v1"
)

// Order представляет заказ на постройку космического корабля.
type Order struct {
	OrderUUID       uuid.UUID
	HullUUID        uuid.UUID
	EngineUUID      uuid.UUID
	ShieldUUID      *uuid.UUID // опциональный
	WeaponUUID      *uuid.UUID // опциональный
	TotalPrice      int64      // в копейках
	TransactionUUID *uuid.UUID
	PaymentMethod   *string
	Status          string // PENDING_PAYMENT, PAID, CANCELLED
	CreatedAt       time.Time
}

// OrderStore — хранилище заказов (in-memory).
type OrderStore struct {
	mu     sync.RWMutex
	orders map[uuid.UUID]Order
}

// NewOrderStore создаёт новое пустое хранилище заказов.
func NewOrderStore() *OrderStore {
	return &OrderStore{
		orders: make(map[uuid.UUID]Order),
	}
}

// OrderHandler реализует интерфейс orderv1.Handler, сгенерированный ogen.
type OrderHandler struct {
	orderv1.UnimplementedHandler
	inventoryClient inventoryv1.InventoryServiceClient
	paymentClient   paymentv1.PaymentServiceClient
	store           *OrderStore
}

// NewOrderHandler создаёт новый обработчик заказов.
func NewOrderHandler(
	inventoryClient inventoryv1.InventoryServiceClient,
	paymentClient paymentv1.PaymentServiceClient,
	store *OrderStore,
) *OrderHandler {
	return &OrderHandler{
		inventoryClient: inventoryClient,
		paymentClient:   paymentClient,
		store:           store,
	}
}

// SetupServer создаёт OpenAPI сервер на основе обработчика.
func SetupServer(h *OrderHandler) (*orderv1.Server, error) {
	return orderv1.NewServer(h)
}

// GetOrder реализует операцию getOrder (пример реализации).
// GET /api/v1/orders/{order_uuid}.
func (h *OrderHandler) GetOrder(_ context.Context, params orderv1.GetOrderParams) (orderv1.GetOrderRes, error) {
	// 1. Найти заказ в store (с блокировкой для thread-safety)
	h.store.mu.RLock()
	order, ok := h.store.orders[params.OrderUUID]
	h.store.mu.RUnlock()

	// 2. Если не найден — вернуть 404
	if !ok {
		return &orderv1.GetOrderNotFound{
			Code:    http.StatusNotFound,
			Message: "заказ не найден",
		}, nil
	}

	// 3. Преобразовать в DTO и вернуть
	var shieldUUID orderv1.OptNilUUID
	if order.ShieldUUID != nil {
		shieldUUID = orderv1.NewOptNilUUID(*order.ShieldUUID)
	}

	var weaponUUID orderv1.OptNilUUID
	if order.WeaponUUID != nil {
		weaponUUID = orderv1.NewOptNilUUID(*order.WeaponUUID)
	}

	var transactionUUID orderv1.OptNilUUID
	if order.TransactionUUID != nil {
		transactionUUID = orderv1.NewOptNilUUID(*order.TransactionUUID)
	}

	var paymentMethod orderv1.OptNilPaymentMethod
	if order.PaymentMethod != nil {
		paymentMethod = orderv1.NewOptNilPaymentMethod(orderv1.PaymentMethod(*order.PaymentMethod))
	}

	return &orderv1.OrderDto{
		OrderUUID:       order.OrderUUID,
		HullUUID:        order.HullUUID,
		EngineUUID:      order.EngineUUID,
		ShieldUUID:      shieldUUID,
		WeaponUUID:      weaponUUID,
		TotalPrice:      order.TotalPrice,
		TransactionUUID: transactionUUID,
		PaymentMethod:   paymentMethod,
		Status:          orderv1.OrderStatus(order.Status),
		CreatedAt:       order.CreatedAt,
	}, nil
}

// TODO: Реализовать остальные методы интерфейса orderv1.Handler:
//
// CreateOrder реализует операцию createOrder
// POST /api/v1/orders
// func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (orderv1.CreateOrderRes, error) {
//     // 1. Валидация: hull_uuid и engine_uuid обязательны
//     // 2. Получить детали через InventoryService.GetPart
//     // 3. Проверить stock_quantity > 0
//     // 4. Вычислить total_price
//     // 5. Сгенерировать order_uuid (UUID v4)
//     // 6. Создать заказ со статусом PENDING_PAYMENT
//     // 7. Сохранить в store
//     // 8. Вернуть order_uuid и total_price
// }
//
// PayOrder реализует операцию payOrder
// POST /api/v1/orders/{order_uuid}/pay

func (h *OrderHandler) PayOrder(ctx context.Context, req *orderv1.PayOrderRequest, params orderv1.PayOrderParams) (orderv1.PayOrderRes, error) {
	// 1. Найти заказ в store
	h.store.mu.RLock()
	order := h.store.orders[params.OrderUUID]
	h.store.mu.RUnlock()

	// 2. Проверить статус == PENDING_PAYMENT
	if order.Status != string(orderv1.OrderStatusPENDINGPAYMENT) {
		switch order.Status {
		case string(orderv1.OrderStatusPAID):
			return &orderv1.PayOrderConflict{
				Code:    http.StatusConflict,
				Message: "заказ уже оплачен или отменён",
			}, nil
		case string(orderv1.OrderStatusCANCELLED):
			return &orderv1.PayOrderConflict{
				Code:    http.StatusConflict,
				Message: "заказ уже оплачен или отменён",
			}, nil

		default:
			return &orderv1.PayOrderConflict{
				Code:    http.StatusNotFound,
				Message: "заказ не найден",
			}, nil
		}
	}

	var paymentMethod paymentv1.PaymentMethod

	switch req.PaymentMethod {
	case orderv1.PaymentMethodCARD:
		paymentMethod = paymentv1.PaymentMethod_PAYMENT_METHOD_CARD
	case orderv1.PaymentMethodSBP:
		paymentMethod = paymentv1.PaymentMethod_PAYMENT_METHOD_SBP
	case orderv1.PaymentMethodCREDITCARD:
		paymentMethod = paymentv1.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD
	case orderv1.PaymentMethodINVESTORMONEY:
		paymentMethod = paymentv1.PaymentMethod_PAYMENT_METHOD_INVESTOR_MONEY
	default:
		return nil, errors.New("unknown payment method")
	}

	// 3. Вызвать h.paymentClient.PayOrder для обработки платежа
	resp, err := h.paymentClient.PayOrder(
		ctx,
		&paymentv1.PayOrderRequest{
			OrderUuid:     params.OrderUUID.String(),
			PaymentMethod: paymentMethod,
		},
	)
	if err != nil {
		return &orderv1.PayOrderInternalServerError{
			Code:    500,
			Message: "ошибка оплаты",
		}, nil
	}
	transactionUUID := uuid.MustParse(resp.TransactionUuid)

	// 4. Обновить статус на PAID и сохранить transaction_uuid
	order.Status = string(orderv1.OrderStatusPAID)
	order.TransactionUUID = &transactionUUID

	h.store.mu.RLock()
	h.store.orders[params.OrderUUID] = order
	h.store.mu.RUnlock()

	// 5. Вернуть transaction_uuid
	return &orderv1.PayOrderResponse{
		TransactionUUID: transactionUUID,
	}, nil
}

// CancelOrder реализует операцию cancelOrder
// POST /api/v1/orders/{order_uuid}/cancel

func (h *OrderHandler) CancelOrder(ctx context.Context, params orderv1.CancelOrderParams) (orderv1.CancelOrderRes, error) {
	h.store.mu.RLock()
	order, ok := h.store.orders[params.OrderUUID]
	h.store.mu.RUnlock()

	if !ok {
		return &orderv1.CancelOrderNotFound{
			Code:    http.StatusNotFound,
			Message: "заказ не найден",
		}, nil
	}

	if order.Status != string(orderv1.OrderStatusPENDINGPAYMENT) {
		switch order.Status {
		case string(orderv1.OrderStatusPAID):
			return &orderv1.CancelOrderConflict{
				Code:    http.StatusConflict,
				Message: "заказ уже оплачен и не может быть отменён",
			}, nil
		case string(orderv1.OrderStatusCANCELLED):
			return &orderv1.CancelOrderConflict{
				Code:    http.StatusConflict,
				Message: "заказ уже отменен",
			}, nil
		}
	}

	order.Status = string(orderv1.OrderStatusCANCELLED)

	h.store.mu.RLock()
	h.store.orders[params.OrderUUID] = order
	h.store.mu.RUnlock()

	return &orderv1.CancelOrderResponse{}, nil
}
