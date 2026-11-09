package main

import (
	"io"
	"log/slog"
)

// nuevoRegistro escribe una línea JSON por evento con los nombres que
// Cloud Logging reconoce (severity y message). Los demás campos (job_id,
// message_id, delivery_attempt, estado_final...) quedan en jsonPayload, así
// una consulta como
//
//	resource.labels.service_name="mooc-worker" jsonPayload.job_id="<id>"
//
// sigue un trabajo de punta a punta.
func nuevoRegistro(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return a
			}
			switch a.Key {
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "severity"
				if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == slog.LevelWarn {
					a.Value = slog.StringValue("WARNING")
				}
			case slog.TimeKey:
				a.Key = "time"
			}
			return a
		},
	}))
}
