module github.com/griffincommerce/demo/services/shipping

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/griffincommerce/demo/common v0.0.0-local
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
)

replace github.com/griffincommerce/demo/common => ../../common
