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

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Trace) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for zcmr := range z {
		s += z[zcmr].Msgsize()
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Traces) DecodeMsg(dc *msgp.Reader) (err error) {
	var zxhx uint32
	zxhx, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap((*z)) >= int(zxhx) {
		(*z) = (*z)[:zxhx]
	} else {
		(*z) = make(Traces, zxhx)
	}
	for zhct := range *z {
		var zlqf uint32
		zlqf, err = dc.ReadArrayHeader()
		if err != nil {
			return
		}
		if cap((*z)[zhct]) >= int(zlqf) {
			(*z)[zhct] = ((*z)[zhct])[:zlqf]
		} else {
			(*z)[zhct] = make(Trace, zlqf)
		}
		for zcua := range (*z)[zhct] {
			err = (*z)[zhct][zcua].DecodeMsg(dc)
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
	for zdaf := range z {
		err = en.WriteArrayHeader(uint32(len(z[zdaf])))
		if err != nil {
			return
		}
		for zpks := range z[zdaf] {
			err = z[zdaf][zpks].EncodeMsg(en)
			if err != nil {
				return
			}
		}
	}
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Traces) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for zdaf := range z {
		s += msgp.ArrayHeaderSize
		for zpks := range z[zdaf] {
			s += z[zdaf][zpks].Msgsize()
		}
	}
	return
}
