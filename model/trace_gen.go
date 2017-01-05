package model

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// DecodeMsg implements msgp.Decodable
func (z *Trace) DecodeMsg(dc *msgp.Reader) (err error) {
	var zbai uint32
	zbai, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap((*z)) >= int(zbai) {
		(*z) = (*z)[:zbai]
	} else {
		(*z) = make(Trace, zbai)
	}
	for zbzg := range *z {
		err = (*z)[zbzg].DecodeMsg(dc)
		if err != nil {
			return
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Trace) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(uint32(len(z)))
	if err != nil {
		return
	}
	for zcmr := range z {
		err = z[zcmr].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Trace) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, uint32(len(z)))
	for zcmr := range z {
		o, err = z[zcmr].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Trace) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zwht uint32
	zwht, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap((*z)) >= int(zwht) {
		(*z) = (*z)[:zwht]
	} else {
		(*z) = make(Trace, zwht)
	}
	for zajw := range *z {
		bts, err = (*z)[zajw].UnmarshalMsg(bts)
		if err != nil {
			return
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Trace) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for zhct := range z {
		s += z[zhct].Msgsize()
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Traces) DecodeMsg(dc *msgp.Reader) (err error) {
	var zpks uint32
	zpks, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap((*z)) >= int(zpks) {
		(*z) = (*z)[:zpks]
	} else {
		(*z) = make(Traces, zpks)
	}
	for zlqf := range *z {
		var zjfb uint32
		zjfb, err = dc.ReadArrayHeader()
		if err != nil {
			return
		}
		if cap((*z)[zlqf]) >= int(zjfb) {
			(*z)[zlqf] = ((*z)[zlqf])[:zjfb]
		} else {
			(*z)[zlqf] = make(Trace, zjfb)
		}
		for zdaf := range (*z)[zlqf] {
			err = (*z)[zlqf][zdaf].DecodeMsg(dc)
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Traces) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(uint32(len(z)))
	if err != nil {
		return
	}
	for zcxo := range z {
		err = en.WriteArrayHeader(uint32(len(z[zcxo])))
		if err != nil {
			return
		}
		for zeff := range z[zcxo] {
			err = z[zcxo][zeff].EncodeMsg(en)
			if err != nil {
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Traces) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, uint32(len(z)))
	for zcxo := range z {
		o = msgp.AppendArrayHeader(o, uint32(len(z[zcxo])))
		for zeff := range z[zcxo] {
			o, err = z[zcxo][zeff].MarshalMsg(o)
			if err != nil {
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Traces) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zdnj uint32
	zdnj, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap((*z)) >= int(zdnj) {
		(*z) = (*z)[:zdnj]
	} else {
		(*z) = make(Traces, zdnj)
	}
	for zrsw := range *z {
		var zobc uint32
		zobc, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if cap((*z)[zrsw]) >= int(zobc) {
			(*z)[zrsw] = ((*z)[zrsw])[:zobc]
		} else {
			(*z)[zrsw] = make(Trace, zobc)
		}
		for zxpk := range (*z)[zrsw] {
			bts, err = (*z)[zrsw][zxpk].UnmarshalMsg(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Traces) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for zsnv := range z {
		s += msgp.ArrayHeaderSize
		for zkgt := range z[zsnv] {
			s += z[zsnv][zkgt].Msgsize()
		}
	}
	return
}
