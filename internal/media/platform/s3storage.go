// Package platform contiene las implementaciones reales de las interfaces
// que domain define para infraestructura (almacenamiento de objetos, cola
// de trabajos). Ni usecase ni worker importan minio-go o asynq
// directamente: solo conocen domain.ObjectStorage y domain.JobQueue.
package platform

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool

	// PublicEndpoint es el host:puerto que debe quedar firmado dentro de
	// las URLs prefirmadas que se entregan a un cliente EXTERNO (el
	// navegador del profesor, Postman, un reproductor HLS). Endpoint, en
	// cambio, es el host que usa este mismo proceso (API/worker) para
	// hablar con MinIO/S3 directamente (StatObject, multipart, descargas
	// del worker) — dentro de docker-compose eso es "minio:9000", que solo
	// resuelve DENTRO de la red de contenedores. Si un cliente externo
	// usara ese mismo host firmado, la petición nunca llegaría (DNS no
	// resuelve "minio" fuera de docker). Si PublicEndpoint queda vacío, se
	// usa Endpoint para todo (comportamiento anterior, válido cuando la
	// API también corre fuera de docker o hay un proxy que sí resuelve el
	// nombre interno).
	PublicEndpoint string
	PublicUseSSL   bool

	// Region fuerza la region que minio-go usa para firmar (SigV4 la
	// necesita). Si queda vacio, minio-go la resuelve solo la primera vez
	// con una llamada real GetBucketLocation contra el endpoint del
	// cliente -- y eso es un problema para publicClient: su endpoint
	// (ej. "localhost:9000") es el que ve un cliente EXTERNO, no
	// necesariamente algo alcanzable desde dentro de este proceso. Sin
	// Region fijo, firmar una URL terminaria intentando una conexion real
	// hacia ese host publico y fallando con 500 si no es alcanzable desde
	// aqui (por ejemplo, "localhost" adentro de un contenedor no es MinIO).
	// MinIO usa "us-east-1" por defecto si no se configura otra cosa.
	Region string
}

type S3Storage struct {
	client *minio.Client
	// core expone las operaciones multipart de bajo nivel (NewMultipartUpload,
	// ListObjectParts, CompleteMultipartUpload, AbortMultipartUpload), que
	// el cliente "alto nivel" de minio-go no expone porque normalmente él
	// mismo decide cuándo trocear un archivo. Aquí lo troceamos nosotros
	// a propósito, porque quien sube el archivo es el navegador del
	// profesor, no nuestro backend.
	core   *minio.Core
	bucket string

	// publicClient firma exactamente igual que client (mismas credenciales)
	// pero apuntando al host público, y SOLO se usa para generar las URLs
	// que salen hacia afuera (PresignDownload, PresignUploadPart). Las
	// operaciones que este mismo proceso ejecuta contra el bucket siguen
	// yendo por client/core, sobre la red interna.
	publicClient *minio.Client
}

// s3Credentials elige el proveedor de credenciales de minio-go. Con
// AccessKey/SecretKey vacíos (el caso de la VM en AWS: no hay llaves en el
// .env ni en la imagen) se asume el rol IAM de la instancia EC2 y se piden
// credenciales temporales al servicio de metadatos; NewIAM("") las
// refresca sola antes de que expiren. Si se definieron llaves estáticas
// (el caso de MinIO en docker-compose), se usan tal cual.
func s3Credentials(accessKey, secretKey string) *credentials.Credentials {
	if accessKey == "" && secretKey == "" {
		return credentials.NewIAM("")
	}
	return credentials.NewStaticV4(accessKey, secretKey, "")
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	creds := s3Credentials(cfg.AccessKey, cfg.SecretKey)

	opts := &minio.Options{
		Creds:  creds,
		Secure: cfg.UseSSL,
		Region: region,
	}
	client, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, err
	}
	core, err := minio.NewCore(cfg.Endpoint, opts)
	if err != nil {
		return nil, err
	}

	publicEndpoint := cfg.PublicEndpoint
	publicUseSSL := cfg.PublicUseSSL
	if publicEndpoint == "" {
		publicEndpoint = cfg.Endpoint
		publicUseSSL = cfg.UseSSL
	}
	publicClient, err := minio.New(publicEndpoint, &minio.Options{
		Creds:  creds,
		Secure: publicUseSSL,
		Region: region,
	})
	if err != nil {
		return nil, err
	}

	return &S3Storage{client: client, core: core, bucket: cfg.Bucket, publicClient: publicClient}, nil
}

func (s *S3Storage) PresignDownload(key string, expiresIn time.Duration) (string, error) {
	u, err := s.publicClient.PresignedGetObject(context.Background(), s.bucket, key, expiresIn, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3Storage) StatObject(key string) (int64, bool, error) {
	info, err := s.client.StatObject(context.Background(), s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NotFound" {
			return 0, false, nil
		}
		return 0, false, err
	}
	return info.Size, true, nil
}

// ---------- multipart ----------

func (s *S3Storage) CreateMultipartUpload(key string, contentType string) (string, error) {
	return s.core.NewMultipartUpload(context.Background(), s.bucket, key, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

// PresignUploadPart usa Presign, el método GENÉRICO de minio-go que firma
// cualquier combinación de método HTTP + query params — no hay un atajo
// específico para "firmar una parte" en la API de alto nivel, pero una
// carga de parte es, a fin de cuentas, un PUT a la misma key con
// ?partNumber=N&uploadId=X en la URL, así que esto alcanza.
func (s *S3Storage) PresignUploadPart(key string, uploadID string, partNumber int, expiresIn time.Duration) (string, error) {
	params := url.Values{}
	params.Set("partNumber", strconv.Itoa(partNumber))
	params.Set("uploadId", uploadID)

	u, err := s.publicClient.Presign(context.Background(), http.MethodPut, s.bucket, key, expiresIn, params)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// ListUploadedParts es la base de la reanudación. NOTA: verifica el
// nombre exacto de los campos del resultado contra la versión de
// minio-go que quede fijada en go.mod — la forma general (una lista de
// partes con número, ETag y tamaño) es estable entre versiones, pero el
// nombre del campo en el struct de respuesta ha cambiado antes entre
// versiones mayores de la librería.
func (s *S3Storage) ListUploadedParts(key string, uploadID string) ([]domain.UploadedPart, error) {
	result, err := s.core.ListObjectParts(context.Background(), s.bucket, key, uploadID, 0, 10000)
	if err != nil {
		return nil, err
	}
	parts := make([]domain.UploadedPart, 0, len(result.ObjectParts))
	for _, p := range result.ObjectParts {
		parts = append(parts, domain.UploadedPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
			SizeBytes:  p.Size,
		})
	}
	return parts, nil
}

func (s *S3Storage) CompleteMultipartUpload(key string, uploadID string, parts []domain.UploadedPart) error {
	completeParts := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		completeParts[i] = minio.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag}
	}
	_, err := s.core.CompleteMultipartUpload(context.Background(), s.bucket, key, uploadID, completeParts, minio.PutObjectOptions{})
	return err
}

func (s *S3Storage) AbortMultipartUpload(key string, uploadID string) error {
	return s.core.AbortMultipartUpload(context.Background(), s.bucket, key, uploadID)
}

// ---------- usadas por el worker ----------

func (s *S3Storage) DownloadObject(key string, destPath string) error {
	return s.client.FGetObject(context.Background(), s.bucket, key, destPath, minio.GetObjectOptions{})
}

func (s *S3Storage) UploadObject(sourcePath string, key string, contentType string) error {
	_, err := s.client.FPutObject(context.Background(), s.bucket, key, sourcePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *S3Storage) DeleteObject(key string) error {
	return s.client.RemoveObject(context.Background(), s.bucket, key, minio.RemoveObjectOptions{})
}