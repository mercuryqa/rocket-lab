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

// CreateOrder реализует операцию createOrder
// POST /api/v1/orders.

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (orderv1.CreateOrderRes, error) {
	hullUUID := req.GetHullUUID()
	engineUUID := req.GetEngineUUID()
	shieldUUID := req.GetShieldUUID()
	weaponUUID := req.GetWeaponUUID()

	// Собираем все UUID для единого вызова ListParts
	uuids := []string{hullUUID.String(), engineUUID.String()}

	if shieldUUID.Set && !shieldUUID.Null {
		uuids = append(uuids, shieldUUID.Value.String())
	}
	if weaponUUID.Set && !weaponUUID.Null {
		uuids = append(uuids, weaponUUID.Value.String())
	}

	// Валидация обязательных полей
	if hullUUID.String() == "" || engineUUID.String() == "" {
		return &orderv1.CreateOrderBadRequest{
			Code:    400,
			Message: "в заказе нет hull_uuid или engine_uuid",
		}, nil
	}

	inventoryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1. Получить все детали через InventoryService.ListParts
	resp, err := h.inventoryClient.ListParts(inventoryCtx, &inventoryv1.ListPartsRequest{
		Uuids: uuids,
	})
	if err != nil {
		return &orderv1.CreateOrderInternalServerError{
			Code:    500,
			Message: "inventory сервис не доступен",
		}, nil
	}

	if len(resp.Parts) != len(uuids) {
		return &orderv1.CreateOrderConflict{
			Code:    409,
			Message: "недостаточно деталей",
		}, nil
	}

	// 2. Создаем map для быстрого поиска по UUID
	partsMap := make(map[string]*inventoryv1.Part)
	for _, part := range resp.Parts {
		partsMap[part.Uuid] = part
	}

	// 3. Проверяем наличие и stock_quantity для всех деталей + вычисляем total_price
	var totalPrice int64
	for _, uuid := range uuids {
		part, exists := partsMap[uuid]
		if !exists || part.StockQuantity <= 0 {
			return &orderv1.CreateOrderConflict{
				Code:    409,
				Message: "недостаточно деталей",
			}, nil
		}
		totalPrice += part.Price
	}

	// 4. Сгенерировать order_uuid (UUID v4)
	orderUUID := uuid.New()

	// 5. Создать заказ со статусом PENDING_PAYMENT
	order := Order{
		OrderUUID:  orderUUID,
		HullUUID:   hullUUID,
		EngineUUID: engineUUID,
		ShieldUUID: optToPtr(shieldUUID),
		WeaponUUID: optToPtr(weaponUUID),
		TotalPrice: totalPrice,
		Status:     string(orderv1.OrderStatusPENDINGPAYMENT),
		CreatedAt:  time.Now(),
	}

	// 6. Сохранить в store
	h.store.mu.Lock()
	h.store.orders[orderUUID] = order
	h.store.mu.Unlock()

	// 7. Вернуть order_uuid и total_price
	return &orderv1.CreateOrderResponse{
		OrderUUID:  orderUUID,
		TotalPrice: totalPrice,
	}, nil
}

//
// PayOrder реализует операцию payOrder
// POST /api/v1/orders/{order_uuid}/pay.

func (h *OrderHandler) PayOrder(ctx context.Context, req *orderv1.PayOrderRequest, params orderv1.PayOrderParams) (orderv1.PayOrderRes, error) {
	// 1. Найти заказ в store
	h.store.mu.RLock()
	order, ok := h.store.orders[params.OrderUUID]
	h.store.mu.RUnlock()

	// Если не найден — вернуть 404
	if !ok {
		return &orderv1.PayOrderNotFound{
			Code:    http.StatusNotFound,
			Message: "заказ не найден",
		}, nil
	}

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

	paymentCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 3. Вызвать h.paymentClient.PayOrder для обработки платежа
	resp, err := h.paymentClient.PayOrder(
		paymentCtx,
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
	payMethod := paymentMethod.String()

	// 4. Обновить статус на PAID и сохранить transaction_uuid
	order.Status = string(orderv1.OrderStatusPAID)
	order.TransactionUUID = &transactionUUID
	order.PaymentMethod = &payMethod

	h.store.mu.Lock()
	h.store.orders[params.OrderUUID] = order
	h.store.mu.Unlock()

	// 5. Вернуть transaction_uuid
	return &orderv1.PayOrderResponse{
		TransactionUUID: transactionUUID,
	}, nil
}

// CancelOrder реализует операцию cancelOrder
// POST /api/v1/orders/{order_uuid}/cancel.

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

	h.store.mu.Lock()
	h.store.orders[params.OrderUUID] = order
	h.store.mu.Unlock()

	return &orderv1.CancelOrderResponse{}, nil
}

func optToPtr(v orderv1.OptNilUUID) *uuid.UUID {
	if v.Set && !v.Null {
		return &v.Value
	}
	return nil
}
