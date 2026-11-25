module github.com/equipo-mooc/plataforma-mooc/functions/validar-carga

// Módulo aparte del backend: Terraform sube esta carpeta como zip y Cloud
// Build la compila con el runtime go125. Por eso no importa el módulo del
// backend (pide Go 1.26) ni los clientes oficiales de Storage y Pub/Sub
// (también piden 1.26): ver servicios.go.
go 1.25.0

require (
	github.com/GoogleCloudPlatform/functions-framework-go v1.9.2
	github.com/cloudevents/sdk-go/v2 v2.16.2
	github.com/jackc/pgx/v5 v5.10.0
)

require (
	cloud.google.com/go/functions v1.19.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
