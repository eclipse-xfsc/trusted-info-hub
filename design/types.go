// nolint:revive
package design

import . "goa.design/goa/v3/dsl"

var ExportRequest = Type("ExportRequest", func() {
	Field(1, "exportName", String, "Name of export to be performed.", func() {
		Example("testexport")
	})
	Required("exportName")
})

var ImportRequest = Type("ImportRequest", func() {
	Field(1, "data", Bytes, "Data wrapped in Verifiable Presentation that will be imported into Cache.", func() {
		Example("data")
	})
	Required("data")
})

var ImportResult = Type("ImportResult", func() {
	Field(1, "importIds", ArrayOf(String), "importIds is an array of unique identifiers used as Cache keys to retrieve the imported data entries later.", func() {
		Example([]string{"585a999a-f36d-419d-bed3-8ebfa5bb79c9"})
	})
	Required("importIds")
})

var HealthResponse = Type("HealthResponse", func() {
	Field(1, "service", String, "Service name.")
	Field(2, "status", String, "Status message.")
	Field(3, "version", String, "Service runtime version.")
	Required("service", "status", "version")
})
