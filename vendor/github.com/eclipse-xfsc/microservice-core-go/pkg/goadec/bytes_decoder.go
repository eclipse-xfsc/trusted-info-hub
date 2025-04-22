// Package goadec provides Goa Request Decoders.
package goadec

import (
	"fmt"
	"io"
	"net/http"

	goahttp "goa.design/goa/v3/http"
)

// BytesDecoder returns an HTTP request body decoder that
// just reads the request body bytes.
func BytesDecoder(r *http.Request) goahttp.Decoder {
	return newBytesDecoder(r.Body)
}

type bytesDecoder struct {
	r io.Reader
}

func newBytesDecoder(r io.Reader) *bytesDecoder {
	return &bytesDecoder{r: r}
}

func (d *bytesDecoder) Decode(v interface{}) error {
	b, err := io.ReadAll(d.r)
	if err != nil {
		return err
	}
	switch c := v.(type) {
	case *string:
		*c = string(b)
	case *[]byte:
		*c = b
	default:
		return fmt.Errorf("cannot decode request: unsupported value type: %T", c)
	}
	return nil
}
