module github.com/mercuryqa/payment

go 1.26.0

require (
	github.com/google/uuid v1.6.0
	github.com/mercuryqa/shared v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.80.0
)

require (
	go.opentelemetry.io/otel/sdk/metric v1.42.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/mercuryqa/shared => ./../shared
