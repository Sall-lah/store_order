module github.com/Sall-lah/store_order

go 1.26.5

require (
	github.com/Sall-lah/store_proto v0.0.0-00010101000000-000000000000
	github.com/go-chi/chi/v5 v5.3.1
	github.com/go-chi/cors v1.2.2
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.22.0
	github.com/segmentio/kafka-go v0.4.51
	github.com/shopspring/decimal v1.4.0
	github.com/steebchen/prisma-client-go v0.47.0
	google.golang.org/grpc v1.83.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	go.mongodb.org/mongo-driver/v2 v2.0.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/Sall-lah/store_proto => ../store_proto
