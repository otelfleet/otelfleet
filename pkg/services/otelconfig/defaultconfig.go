package otelconfig

const DefaultOtelConfig = `receivers:
  otlp:
    nop:

processors:
  batch:

exporters:
  debug:

extensions:
  health_check:

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [nop]
      processors: [batch]
      exporters: [debug]
    metrics:
      receivers: [nop]
      processors: [batch]
      exporters: [debug]
    logs:
      receivers: [nop]
      processors: [batch]
      exporters: [debug]
`
