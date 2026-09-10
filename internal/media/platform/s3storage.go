// Package platform contiene las implementaciones reales de las interfaces
// que domain define para infraestructura (almacenamiento de objetos, cola
// de trabajos). Ni usecase ni worker importan minio-go o asynq
// directamente: solo conocen domain.ObjectStorage y domain.JobQueue.
package platform

import (
	"context"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) PresignUpload(key string, contentType string, expiresIn time.Duration) (string, error) {
	// Nota: PresignedPutObject no fija el Content-Type dentro de la firma;
	// el frontend debe mandar el mismo header al hacer el PUT contra esta
	// URL. contentType queda sin usar aquí a propósito (se deja en la
	// firma para que la función sea fácil de leer y de ampliar si más
	// adelante se firma con condiciones extra, ej. PresignedPostPolicy).
	u, err := s.client.PresignedPutObject(context.Background(), s.bucket, key, expiresIn)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3Storage) PresignDownload(key string, expiresIn time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(context.Background(), s.bucket, key, expiresIn, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// StatObject confirma que el objeto existe y devuelve su tamaño y ETag.
//
// ADVERTENCIA IMPORTANTE: el ETag de S3/MinIO NO es un SHA256 confiable
// del archivo completo cuando la carga fue multipart (es un hash MD5 de
// los hashes de cada parte, con un sufijo "-N"). Para el MVP esto alcanza
// para detectar "el archivo llegó / no llegó", pero si de verdad quieren
// verificar el checksum SHA256 declarado por el cliente (sección 6:
// "verificación de integridad"), hay dos caminos mejores:
//  1. Calcular el SHA256 real en el worker, al descargar el original
//     (io.Copy hacia un hash.Hash mientras se escribe a disco), y comparar
//     ahí en vez de en StatObject.
//  2. Pedirle al cliente que mande el checksum como metadata custom
//     (x-amz-meta-sha256) al subir, y leerlo de vuelta con StatObject.
// Dejo la firma de ObjectStorage tal cual para no bloquear el resto del
// equipo, pero avisa a tu profesor/equipo de esta limitación conocida.
func (s *S3Storage) StatObject(key string) (int64, string, bool, error) {
	info, err := s.client.StatObject(context.Background(), s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NotFound" {
			return 0, "", false, nil
		}
		return 0, "", false, err
	}
	return info.Size, info.ETag, true, nil
}

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
