package model

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// DecodeMsg implements msgp.Decodable
func (z *ServicesMetadata) DecodeMsg(dc *msgp.Reader) (err error) {
	var zxhx uint32
	zxhx, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	if (*z) == nil && zxhx > 0 {
		(*z) = make(ServicesMetadata, zxhx)
	} else if len((*z)) > 0 {
		for key, _ := range *z {
			delete((*z), key)
		}
	}
	for zxhx > 0 {
		zxhx--
		var zajw string
		var zwht map[string]string
		zajw, err = dc.ReadString()
		if err != nil {
			return
		}
		var zlqf uint32
		zlqf, err = dc.ReadMapHeader()
		if err != nil {
			return
		}
		if zwht == nil && zlqf > 0 {
			zwht = make(map[string]string, zlqf)
		} else if len(zwht) > 0 {
			for key, _ := range zwht {
				delete(zwht, key)
			}
		}
		for zlqf > 0 {
			zlqf--
			var zhct string
			var zcua string
			zhct, err = dc.ReadString()
			if err != nil {
				return
			}
			zcua, err = dc.ReadString()
			if err != nil {
				return
			}
			zwht[zhct] = zcua
		}
		(*z)[zajw] = zwht
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z ServicesMetadata) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(uint32(len(z)))
	if err != nil {
		return
	}
	for zdaf, zpks := range z {
		err = en.WriteString(zdaf)
		if err != nil {
			return
		}
		err = en.WriteMapHeader(uint32(len(zpks)))
		if err != nil {
			return
		}
		for zjfb, zcxo := range zpks {
			err = en.WriteString(zjfb)
			if err != nil {
				return
			}
			err = en.WriteString(zcxo)
			if err != nil {
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ServicesMetadata) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, uint32(len(z)))
	for zdaf, zpks := range z {
		o = msgp.AppendString(o, zdaf)
		o = msgp.AppendMapHeader(o, uint32(len(zpks)))
		for zjfb, zcxo := range zpks {
			o = msgp.AppendString(o, zjfb)
			o = msgp.AppendString(o, zcxo)
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ServicesMetadata) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zobc uint32
	zobc, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	if (*z) == nil && zobc > 0 {
		(*z) = make(ServicesMetadata, zobc)
	} else if len((*z)) > 0 {
		for key, _ := range *z {
			delete((*z), key)
		}
	}
	for zobc > 0 {
		var zeff string
		var zrsw map[string]string
		zobc--
		zeff, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			return
		}
		var zsnv uint32
		zsnv, bts, err = msgp.ReadMapHeaderBytes(bts)
		if err != nil {
			return
		}
		if zrsw == nil && zsnv > 0 {
			zrsw = make(map[string]string, zsnv)
		} else if len(zrsw) > 0 {
			for key, _ := range zrsw {
				delete(zrsw, key)
			}
		}
		for zsnv > 0 {
			var zxpk string
			var zdnj string
			zsnv--
			zxpk, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
			zdnj, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
			zrsw[zxpk] = zdnj
		}
		(*z)[zeff] = zrsw
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z ServicesMetadata) Msgsize() (s int) {
	s = msgp.MapHeaderSize
	if z != nil {
		for zkgt, zema := range z {
			_ = zema
			s += msgp.StringPrefixSize + len(zkgt) + msgp.MapHeaderSize
			if zema != nil {
				for zpez, zqke := range zema {
					_ = zqke
					s += msgp.StringPrefixSize + len(zpez) + msgp.StringPrefixSize + len(zqke)
				}
			}
		}
	}
	return
}
