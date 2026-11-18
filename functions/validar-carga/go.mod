module github.com/equipo-mooc/plataforma-mooc/functions/validar-carga

// Módulo aparte del backend: Cloud Build compila solo esta carpeta (la sube
// Terraform como zip). Mateo agrega las dependencias con `go mod tidy`
// (functions-framework-go, cloudevents, pgx, cloud.google.com/go/storage y
// pubsub) y sube go.sum junto con el código.
go 1.25
