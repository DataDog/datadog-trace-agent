package model

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"errors"
	"math"

	"github.com/tinylib/msgp/msgp"
)

// cast to int64 values that are int64 but that are sent in uint64
// over the wire. Set to 0 if they overflow the MaxInt64.
func castInt64(v uint64) (int64, bool) {
	if v > math.MaxInt64 {
		return 0, false
	}

	return int64(v), true
}

// try to parse an Int64, falling back to Uint64 if the
// encoding library removes one byte from the encode
func parseInt64(dc *msgp.Reader) (int64, error) {
	// read the int64 representation and return decoded value
	v64, err := dc.ReadInt64()
	if err == nil {
		return v64, err
	}

	// fallback to uint64
	vu64, err := dc.ReadUint64()
	if err != nil {
		return 0, err
	}

	// safe cast to int64 if another library has removed
	// one byte during the encoding
	v64, ok := castInt64(vu64)
	if !ok {
		return v64, errors.New("found uint64, overflows int64")
	}

	return v64, nil
}

// DecodeMsg implements msgp.Decodable
func (z *Span) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zajw uint32
	zajw, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "service":
			z.Service, err = dc.ReadString()
			if err != nil {
				return
			}
		case "name":
			z.Name, err = dc.ReadString()
			if err != nil {
				return
			}
		case "resource":
			z.Resource, err = dc.ReadString()
			if err != nil {
				return
			}
		case "trace_id":
			z.TraceID, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "span_id":
			z.SpanID, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "start":
			z.Start, err = parseInt64(dc)
			if err != nil {
				return
			}
		case "duration":
			z.Duration, err = parseInt64(dc)
			if err != nil {
				return
			}
		case "error":
			z.Error, err = dc.ReadInt32()
			if err != nil {
				return
			}
		case "meta":
			var zwht uint32
			zwht, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Meta == nil && zwht > 0 {
				z.Meta = make(map[string]string, zwht)
			} else if len(z.Meta) > 0 {
				for key, _ := range z.Meta {
					delete(z.Meta, key)
				}
			}
			for zwht > 0 {
				zwht--
				var zxvk string
				var zbzg string
				zxvk, err = dc.ReadString()
				if err != nil {
					return
				}
				zbzg, err = dc.ReadString()
				if err != nil {
					return
				}
				z.Meta[zxvk] = zbzg
			}
		case "metrics":
			var zhct uint32
			zhct, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Metrics == nil && zhct > 0 {
				z.Metrics = make(map[string]float64, zhct)
			} else if len(z.Metrics) > 0 {
				for key, _ := range z.Metrics {
					delete(z.Metrics, key)
				}
			}
			for zhct > 0 {
				zhct--
				var zbai string
				var zcmr float64
				zbai, err = dc.ReadString()
				if err != nil {
					return
				}
				zcmr, err = dc.ReadFloat64()
				if err != nil {
					return
				}
				z.Metrics[zbai] = zcmr
			}
		case "parent_id":
			z.ParentID, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "type":
			z.Type, err = dc.ReadString()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Span) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 12
	// write "service"
	err = en.Append(0x8c, 0xa7, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Service)
	if err != nil {
		return
	}
	// write "name"
	err = en.Append(0xa4, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Name)
	if err != nil {
		return
	}
	// write "resource"
	err = en.Append(0xa8, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Resource)
	if err != nil {
		return
	}
	// write "trace_id"
	err = en.Append(0xa8, 0x74, 0x72, 0x61, 0x63, 0x65, 0x5f, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteUint64(z.TraceID)
	if err != nil {
		return
	}
	// write "span_id"
	err = en.Append(0xa7, 0x73, 0x70, 0x61, 0x6e, 0x5f, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteUint64(z.SpanID)
	if err != nil {
		return
	}
	// write "start"
	err = en.Append(0xa5, 0x73, 0x74, 0x61, 0x72, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.Start)
	if err != nil {
		return
	}
	// write "duration"
	err = en.Append(0xa8, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.Duration)
	if err != nil {
		return
	}
	// write "error"
	err = en.Append(0xa5, 0x65, 0x72, 0x72, 0x6f, 0x72)
	if err != nil {
		return err
	}
	err = en.WriteInt32(z.Error)
	if err != nil {
		return
	}
	// write "meta"
	err = en.Append(0xa4, 0x6d, 0x65, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Meta)))
	if err != nil {
		return
	}
	for zxvk, zbzg := range z.Meta {
		err = en.WriteString(zxvk)
		if err != nil {
			return
		}
		err = en.WriteString(zbzg)
		if err != nil {
			return
		}
	}
	// write "metrics"
	err = en.Append(0xa7, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Metrics)))
	if err != nil {
		return
	}
	for zbai, zcmr := range z.Metrics {
		err = en.WriteString(zbai)
		if err != nil {
			return
		}
		err = en.WriteFloat64(zcmr)
		if err != nil {
			return
		}
	}
	// write "parent_id"
	err = en.Append(0xa9, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteUint64(z.ParentID)
	if err != nil {
		return
	}
	// write "type"
	err = en.Append(0xa4, 0x74, 0x79, 0x70, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Type)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Span) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 12
	// string "service"
	o = append(o, 0x8c, 0xa7, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	o = msgp.AppendString(o, z.Service)
	// string "name"
	o = append(o, 0xa4, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "resource"
	o = append(o, 0xa8, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65)
	o = msgp.AppendString(o, z.Resource)
	// string "trace_id"
	o = append(o, 0xa8, 0x74, 0x72, 0x61, 0x63, 0x65, 0x5f, 0x69, 0x64)
	o = msgp.AppendUint64(o, z.TraceID)
	// string "span_id"
	o = append(o, 0xa7, 0x73, 0x70, 0x61, 0x6e, 0x5f, 0x69, 0x64)
	o = msgp.AppendUint64(o, z.SpanID)
	// string "start"
	o = append(o, 0xa5, 0x73, 0x74, 0x61, 0x72, 0x74)
	o = msgp.AppendInt64(o, z.Start)
	// string "duration"
	o = append(o, 0xa8, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	o = msgp.AppendInt64(o, z.Duration)
	// string "error"
	o = append(o, 0xa5, 0x65, 0x72, 0x72, 0x6f, 0x72)
	o = msgp.AppendInt32(o, z.Error)
	// string "meta"
	o = append(o, 0xa4, 0x6d, 0x65, 0x74, 0x61)
	o = msgp.AppendMapHeader(o, uint32(len(z.Meta)))
	for zxvk, zbzg := range z.Meta {
		o = msgp.AppendString(o, zxvk)
		o = msgp.AppendString(o, zbzg)
	}
	// string "metrics"
	o = append(o, 0xa7, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Metrics)))
	for zbai, zcmr := range z.Metrics {
		o = msgp.AppendString(o, zbai)
		o = msgp.AppendFloat64(o, zcmr)
	}
	// string "parent_id"
	o = append(o, 0xa9, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64)
	o = msgp.AppendUint64(o, z.ParentID)
	// string "type"
	o = append(o, 0xa4, 0x74, 0x79, 0x70, 0x65)
	o = msgp.AppendString(o, z.Type)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Span) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zcua uint32
	zcua, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zcua > 0 {
		zcua--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "service":
			z.Service, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "resource":
			z.Resource, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "trace_id":
			z.TraceID, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "span_id":
			z.SpanID, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "start":
			var v uint64
			v, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
			z.Start, _ = castInt64(v)
		case "duration":
			var v uint64
			v, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
			z.Duration, _ = castInt64(v)
		case "error":
			z.Error, bts, err = msgp.ReadInt32Bytes(bts)
			if err != nil {
				return
			}
		case "meta":
			var zxhx uint32
			zxhx, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Meta == nil && zxhx > 0 {
				z.Meta = make(map[string]string, zxhx)
			} else if len(z.Meta) > 0 {
				for key, _ := range z.Meta {
					delete(z.Meta, key)
				}
			}
			for zxhx > 0 {
				var zxvk string
				var zbzg string
				zxhx--
				zxvk, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				zbzg, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				z.Meta[zxvk] = zbzg
			}
		case "metrics":
			var zlqf uint32
			zlqf, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Metrics == nil && zlqf > 0 {
				z.Metrics = make(map[string]float64, zlqf)
			} else if len(z.Metrics) > 0 {
				for key, _ := range z.Metrics {
					delete(z.Metrics, key)
				}
			}
			for zlqf > 0 {
				var zbai string
				var zcmr float64
				zlqf--
				zbai, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				zcmr, bts, err = msgp.ReadFloat64Bytes(bts)
				if err != nil {
					return
				}
				z.Metrics[zbai] = zcmr
			}
		case "parent_id":
			z.ParentID, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "type":
			z.Type, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Span) Msgsize() (s int) {
	s = 1 + 8 + msgp.StringPrefixSize + len(z.Service) + 5 + msgp.StringPrefixSize + len(z.Name) + 9 + msgp.StringPrefixSize + len(z.Resource) + 9 + msgp.Uint64Size + 8 + msgp.Uint64Size + 6 + msgp.Int64Size + 9 + msgp.Int64Size + 6 + msgp.Int32Size + 5 + msgp.MapHeaderSize
	if z.Meta != nil {
		for zxvk, zbzg := range z.Meta {
			_ = zbzg
			s += msgp.StringPrefixSize + len(zxvk) + msgp.StringPrefixSize + len(zbzg)
		}
	}
	s += 8 + msgp.MapHeaderSize
	if z.Metrics != nil {
		for zbai, zcmr := range z.Metrics {
			_ = zcmr
			s += msgp.StringPrefixSize + len(zbai) + msgp.Float64Size
		}
	}
	s += 10 + msgp.Uint64Size + 5 + msgp.StringPrefixSize + len(z.Type)
	return
}
