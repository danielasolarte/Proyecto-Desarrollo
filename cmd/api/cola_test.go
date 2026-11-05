package main

import (
	"testing"

	mediaPlatform "github.com/equipo-mooc/plataforma-mooc/internal/media/platform"
)

func TestNewMediaQueueUsesPubSubWhenTopicIsSet(t *testing.T) {
	t.Setenv("PUBSUB_TOPIC", "mooc-media-jobs")
	t.Setenv("PUBSUB_PROJECT", "mooc-local")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")
	t.Setenv("REDIS_ADDR", "localhost:6379")

	q, err := newMediaQueue()
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if _, ok := q.(*mediaPlatform.PubSubQueue); !ok {
		t.Fatalf("con PUBSUB_TOPIC se esperaba Pub/Sub, llegó %T", q)
	}
}

func TestNewMediaQueueFallsBackToAsynqOnlyLocally(t *testing.T) {
	t.Setenv("PUBSUB_TOPIC", "")
	t.Setenv("REDIS_ADDR", "localhost:6379")

	q, err := newMediaQueue()
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if _, ok := q.(*mediaPlatform.AsynqQueue); !ok {
		t.Fatalf("sin PUBSUB_TOPIC y con REDIS_ADDR se esperaba asynq, llegó %T", q)
	}
}

func TestNewMediaQueueRequiresConfiguration(t *testing.T) {
	t.Setenv("PUBSUB_TOPIC", "")
	t.Setenv("REDIS_ADDR", "")
	if _, err := newMediaQueue(); err == nil {
		t.Fatal("sin cola configurada debía fallar")
	}
}

func TestNewMediaQueueRejectsTopicWithoutProject(t *testing.T) {
	t.Setenv("PUBSUB_TOPIC", "mooc-media-jobs")
	t.Setenv("PUBSUB_PROJECT", "")
	if _, err := newMediaQueue(); err == nil {
		t.Fatal("topic sin proyecto debía fallar")
	}
}
