package handler

import (
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/encoding"
)

type jsonpbCodec struct {
	runtime.JSONPb
}

func (j *jsonpbCodec) Name() string {
	return "json"
}

func (j *jsonpbCodec) String() string {
	return "json"
}

// based on https://github.com/mwitkow/grpc-proxy
// Apache 2 License by Michal Witkowski (mwitkow)

func codec() *rawCodec {
	return codecWithParent(&jsonpbCodec{})
}

func codecWithParent(fallback encoding.Codec) *rawCodec {
	return &rawCodec{parentCodec: fallback}
}

type rawCodec struct {
	parentCodec encoding.Codec
}

type frame struct {
	payload []byte
}

func (c *rawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.payload, nil

}

func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.payload = data
	return nil
}

func (c *rawCodec) Name() string {
	return fmt.Sprintf("proxy>%s", c.parentCodec.Name())
}

func (c *rawCodec) String() string {
	return c.Name()
}
