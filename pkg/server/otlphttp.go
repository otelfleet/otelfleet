package server

import (
	"io"
	"log/slog"
	"net/http"

	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

type OTLPForwarder[T proto.Message] interface {
	Handler(w http.ResponseWriter, r *http.Request)
}

type metricsForwarder struct {
	logger *slog.Logger
}

func NewMetricsForwarder(logger *slog.Logger) OTLPForwarder[*collectormetricspb.ExportMetricsServiceRequest] {
	return &metricsForwarder{logger: logger}
}

func (f *metricsForwarder) Handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.logger.With("err", err).Error("failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req collectormetricspb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		f.logger.With("err", err).Error("failed to unmarshal metrics payload")
		http.Error(w, "failed to unmarshal metrics payload", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

var _ OTLPForwarder[*collectormetricspb.ExportMetricsServiceRequest] = (*metricsForwarder)(nil)
