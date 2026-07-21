package otlp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	// otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

type MetricsServer struct {
	l *slog.Logger
	colmetricspb.UnsafeMetricsServiceServer
}

func (s *MetricsServer) Export(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	s.l.Info("received")
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

var _ colmetricspb.MetricsServiceServer = (*MetricsServer)(nil)

func (s *MetricsServer) handleProto(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	resp, err := s.Export(r.Context(), req)
	if err != nil {
		// TODO : map codes
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	retData, err := proto.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", contentTypePb)
	w.WriteHeader(http.StatusOK)
	w.Write(retData)
}

func (s *MetricsServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	resp, err := s.Export(r.Context(), req)
	if err != nil {
		// TODO : map codes
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	retData, err := protojson.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	w.Write(retData)
}

func (s *MetricsServer) handleMetricsPost(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	switch contentType {
	case contentTypePb:
		s.handleProto(w, r)
		return
	case contentTypeJSON:
		s.handleJSON(w, r)
		return
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		fmt.Fprintf(w, "unsupported Content-Type %s", contentType)
		return
	}
}
