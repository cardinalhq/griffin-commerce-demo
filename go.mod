module github.com/cardinalhq/griffin-commerce-demo

go 1.25

require (
	github.com/cardinalhq/griffin-commerce-demo/services/cart v0.0.0-00010101000000-000000000000
	github.com/cardinalhq/griffin-commerce-demo/services/catalog v0.0.0-00010101000000-000000000000
	github.com/cardinalhq/griffin-commerce-demo/services/images v0.0.0-00010101000000-000000000000
	github.com/cardinalhq/griffin-commerce-demo/services/payment v0.0.0-00010101000000-000000000000
	github.com/cardinalhq/griffin-commerce-demo/services/recommendations v0.0.0-00010101000000-000000000000
	github.com/cardinalhq/griffin-commerce-demo/services/shipping v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.10.1
)

require (
	github.com/cardinalhq/griffin-commerce-demo/common v0.0.0-local // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/cardinalhq/griffin-commerce-demo/services/catalog => ./services/catalog

replace github.com/cardinalhq/griffin-commerce-demo/services/cart => ./services/cart

replace github.com/cardinalhq/griffin-commerce-demo/services/payment => ./services/payment

replace github.com/cardinalhq/griffin-commerce-demo/services/shipping => ./services/shipping

replace github.com/cardinalhq/griffin-commerce-demo/services/images => ./services/images

replace github.com/cardinalhq/griffin-commerce-demo/services/recommendations => ./services/recommendations

replace github.com/cardinalhq/griffin-commerce-demo/common => ./common
