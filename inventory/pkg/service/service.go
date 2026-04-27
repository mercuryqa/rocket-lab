package service

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	inventoryv1 "github.com/mercuryqa/shared/pkg/proto/inventory/v1"
)

// Part представляет деталь космического корабля.
type Part struct {
	UUID          string
	Name          string
	Description   string
	Price         int64 // в копейках
	PartType      inventoryv1.PartType
	StockQuantity int64
	CreatedAt     *timestamppb.Timestamp
}

// InventoryServer реализует gRPC сервис.
type InventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	parts map[uuid.UUID]Part
}

// NewInventoryServer создаёт сервер с предзагруженными seed-данными.
func NewInventoryServer() *InventoryServer {
	now := timestamppb.Now()

	return &InventoryServer{
		parts: map[uuid.UUID]Part{
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440001",
				Name:          "Алюминиевый корпус",
				Description:   "Лёгкий корпус для небольших кораблей",
				Price:         500000, // 5000₽
				PartType:      inventoryv1.PartType_PART_TYPE_HULL,
				StockQuantity: 10,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440002"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440002",
				Name:          "Титановый корпус",
				Description:   "Прочный корпус для средних кораблей",
				Price:         1500000, // 15000₽
				PartType:      inventoryv1.PartType_PART_TYPE_HULL,
				StockQuantity: 5,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440003"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440003",
				Name:          "Ионный двигатель C",
				Description:   "Базовый ионный двигатель класса C",
				Price:         300000, // 3000₽
				PartType:      inventoryv1.PartType_PART_TYPE_ENGINE,
				StockQuantity: 8,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440004"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440004",
				Name:          "Ионный двигатель B",
				Description:   "Улучшенный ионный двигатель класса B",
				Price:         800000, // 8000₽
				PartType:      inventoryv1.PartType_PART_TYPE_ENGINE,
				StockQuantity: 3,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440005"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440005",
				Name:          "Энергетический щит",
				Description:   "Стандартный энергетический щит",
				Price:         400000, // 4000₽
				PartType:      inventoryv1.PartType_PART_TYPE_SHIELD,
				StockQuantity: 6,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440006"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440006",
				Name:          "Лазерная пушка",
				Description:   "Точная лазерная пушка",
				Price:         250000, // 2500₽
				PartType:      inventoryv1.PartType_PART_TYPE_WEAPON,
				StockQuantity: 7,
				CreatedAt:     now,
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440007"): {
				UUID:          "550e8400-e29b-41d4-a716-446655440007",
				Name:          "Плазменный корпус",
				Description:   "Экспериментальный корпус (нет на складе)",
				Price:         2000000, // 20000₽
				PartType:      inventoryv1.PartType_PART_TYPE_HULL,
				StockQuantity: 0,
				CreatedAt:     now,
			},
		},
	}
}

// GetPart возвращает деталь по UUID.
func (s *InventoryServer) GetPart(
	ctx context.Context,
	req *inventoryv1.GetPartRequest,
) (*inventoryv1.GetPartResponse, error) {
	// 2. Валидировать формат UUID → INVALID_ARGUMENT
	partUUID, err := uuid.Parse(req.GetUuid())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "неверный формат part_uuid: %s", req.GetUuid())
	}
	// 3. Найти деталь в map
	part, ok := s.parts[partUUID]

	// 4. Если не найдена → NOT_FOUND
	if !ok {
		return nil, status.Error(codes.NotFound, "part not found")
	}

	// 5. Преобразовать в inventoryv1.Part
	// 6. Вернуть деталь
	return &inventoryv1.GetPartResponse{
		Part: &inventoryv1.Part{
			Uuid:          part.UUID,
			Name:          part.Name,
			Description:   part.Description,
			Price:         part.Price,
			PartType:      part.PartType,
			StockQuantity: part.StockQuantity,
			CreatedAt:     part.CreatedAt,
		},
	}, nil
}

// ListParts возвращает список деталей с опциональной фильтрацией по типу.
func (s *InventoryServer) ListParts(
	ctx context.Context,
	req *inventoryv1.ListPartsRequest,
) (*inventoryv1.ListPartsResponse, error) {
	// 1. Если передан список uuids → найти детали по UUID (сохраняя порядок запроса)
	if len(req.GetUuids()) > 0 {
		result := make([]*inventoryv1.Part, 0, len(req.GetUuids()))

		for _, id := range req.GetUuids() {
			//    - Проверить формат каждого UUID → INVALID_ARGUMENT
			//    - Если хоть один UUID не найден → NOT_FOUND
			u, err := uuid.Parse(id)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "неверный формат uuid: %s", id)
			}

			part, ok := s.parts[u]
			if !ok {
				return nil, status.Errorf(codes.NotFound, "part не найдена: %s", id)
			}

			result = append(result, &inventoryv1.Part{
				Uuid:          part.UUID,
				Name:          part.Name,
				Description:   part.Description,
				Price:         part.Price,
				PartType:      part.PartType,
				StockQuantity: part.StockQuantity,
				CreatedAt:     part.CreatedAt,
			})
		}

		return &inventoryv1.ListPartsResponse{
			Parts: result,
		}, nil
	}
	// 2. Иначе если part_type == UNSPECIFIED → вернуть все детали
	parts := make([]Part, 0, len(s.parts))
	for _, p := range s.parts {
		parts = append(parts, p)
	}

	// 3. Иначе → фильтровать по типу
	if req.GetPartType() != inventoryv1.PartType_PART_TYPE_UNSPECIFIED {
		filtered := make([]Part, 0)

		for _, p := range parts {
			if p.PartType == req.GetPartType() {
				filtered = append(filtered, p)
			}
		}

		parts = filtered
	}
	// 4. Отсортировать по имени (только для фильтрации по типу, не для uuids)
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Name < parts[j].Name
	})

	result := make([]*inventoryv1.Part, 0, len(parts))
	for _, p := range parts {
		result = append(result, &inventoryv1.Part{
			Uuid:          p.UUID,
			Name:          p.Name,
			Description:   p.Description,
			Price:         p.Price,
			PartType:      p.PartType,
			StockQuantity: p.StockQuantity,
			CreatedAt:     p.CreatedAt,
		})
	}

	return &inventoryv1.ListPartsResponse{
		Parts: result,
	}, nil
}
