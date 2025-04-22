// nolint:revive
package design

import . "goa.design/goa/v3/dsl"

var _ = API("infohub", func() {
	Title("Information Hub Service")
	Description("Information Hub Service exposes HTTP API for exporting and importing information.")
	Server("infohub", func() {
		Description("Information Hub Server")
		Host("development", func() {
			Description("Local development server")
			URI("http://localhost:8084")
		})
	})
})

var _ = Service("infohub", func() {
	Description("Information Hub Service enables exporting and importing information.")

	Method("Export", func() {
		Description("Export returns data signed as Verifiable Presentation.")
		Payload(ExportRequest)
		Result(Any)
		HTTP(func() {
			GET("/v1/export/{exportName}")
			Response(StatusOK)
		})
	})

	Method("Import", func() {
		Description("Import the given data wrapped as Verifiable Presentation into the Cache.")
		Payload(ImportRequest)
		Result(ImportResult)
		HTTP(func() {
			POST("/v1/import")
			Body("data")
			Response(StatusOK)
		})
	})
})

var _ = Service("health", func() {
	Description("Health service provides health check endpoints.")

	Method("Liveness", func() {
		Payload(Empty)
		Result(HealthResponse)
		HTTP(func() {
			GET("/liveness")
			Response(StatusOK)
		})
	})

	Method("Readiness", func() {
		Payload(Empty)
		Result(HealthResponse)
		HTTP(func() {
			GET("/readiness")
			Response(StatusOK)
		})
	})
})

var _ = Service("openapi", func() {
	Description("The openapi service serves the OpenAPI(v3) definition.")
	Meta("swagger:generate", "false")
	HTTP(func() {
		Path("/swagger-ui")
	})
	Files("/openapi.json", "./gen/http/openapi3.json", func() {
		Description("JSON document containing the OpenAPI(v3) service definition")
	})
	Files("/{*filepath}", "./swagger/")
})
