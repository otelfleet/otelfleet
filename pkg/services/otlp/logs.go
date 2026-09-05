package otlp

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/logutil"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	// otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

type LogsServer struct {
	collogspb.UnsafeLogsServiceServer
}

func (s *LogsServer) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	logutil.FromContext(ctx).With("signal", "logs").Info("received")
	return &collogspb.ExportLogsServiceResponse{}, nil
}

var _ collogspb.LogsServiceServer = (*LogsServer)(nil)

func (s *LogsServer) handleProto(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &collogspb.ExportLogsServiceRequest{}
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

func (s *LogsServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &collogspb.ExportLogsServiceRequest{}
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

func (s *LogsServer) handleLogsPost(w http.ResponseWriter, r *http.Request) {
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
