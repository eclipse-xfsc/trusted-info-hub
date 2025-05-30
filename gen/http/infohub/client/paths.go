// Code generated by goa v3.20.1, DO NOT EDIT.
//
// HTTP request path constructors for the infohub service.
//
// Command:
// $ goa gen github.com/eclipse-xfsc/trusted-info-hub/design

package client

import (
	"fmt"
)

// ExportInfohubPath returns the URL path to the infohub service Export HTTP endpoint.
func ExportInfohubPath(exportName string) string {
	return fmt.Sprintf("/v1/export/%v", exportName)
}

// ImportInfohubPath returns the URL path to the infohub service Import HTTP endpoint.
func ImportInfohubPath() string {
	return "/v1/import"
}
